package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	redisManage "sw/ocpp/csms/internal/cache"
	conf "sw/ocpp/csms/internal/config"
	helpers "sw/ocpp/csms/internal/helpers"
	httplistener "sw/ocpp/csms/internal/http"
	"sw/ocpp/csms/internal/logging"
	mqmodels "sw/ocpp/csms/internal/models/mq"
	svc "sw/ocpp/csms/internal/models/service"
	svcmodels "sw/ocpp/csms/internal/models/service"
	mq "sw/ocpp/csms/internal/mq"
	service "sw/ocpp/csms/internal/service"
	"sw/ocpp/csms/internal/telemetry"

	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"
	"github.com/puzpuzpuz/xsync/v3"
)

var (
	// Globals
	exitNotification chan T
	log              = logging.Logger
	serviceState     *ServiceState
)

type T = struct{}

func initialise() *ServiceState {
	config := conf.ReadConfig()
	serviceContext := getServiceContext()

	telemetryHook, err := telemetry.NewTelemetryClient(config.Logging.AppInsightsInstrumentationKey, serviceContext.HostName)
	if err != nil {
		return &ServiceState{LastError: err}
	}

	// Setup auth cache
	var cacheClient *redis.Client
	cacheConfig := config.Services.CsmsServer.Cache
	if len(cacheConfig.HostPort) > 0 {
		cacheClient, err = redisManage.ConnectRedis(cacheConfig.HostPort, cacheConfig.Password, cacheConfig.DbId)
		if err != nil {
			return &ServiceState{LastError: err}
		}
	} else {
		log.Warn("Not connecting to redis auth cache")
	}

	mqConnection := mq.SetupMqConnection(config.Mq, config.Mq.MangosMq.CsmsListenUrl, "", config.Mq.MangosMq.CsmsListenRequestUrl, "")

	err = mqConnection.MqConnect()
	if err != nil {
		return &ServiceState{LastError: err}
	}

	err = mqConnection.MqQueueDeclare(mq.MqChannelName_Notify)
	if err != nil {
		return &ServiceState{LastError: err}
	}

	mq.SetupMqReceiver(mqConnection, config.Mq.Type, serviceContext.HostName, mq.MqChannelName_MessagesOut)

	return &ServiceState{
		Cache:           cacheClient,
		Config:          config,
		MqBus:           mqConnection,
		Connections:     xsync.NewMap(),
		Context:         serviceContext,
		AppInsightsHook: telemetryHook,
		MessagesWaiting: xsync.NewMap(),
	}
}

func getServiceContext() ServiceContext {
	return ServiceContext{HostName: helpers.GetHostName()}
}

func main() {

	sigchnl := make(chan os.Signal, 1)
	signal.Notify(sigchnl, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	exitNotification = make(chan T)

	go func() {
		sig := <-sigchnl
		multiSignalHandler(sig)
		log.Debug("Caught close...")
		<-exitNotification // send notification to unblock and exit
	}()

	log = logging.LoggingSetup(true, "csms-server") // start with debug enabled until overridden in config later

	log.Infof("--- CSMS OCPP Server - v%s ---", service.Version)

	serviceState = initialise()
	if serviceState.LastError != nil {
		log.Errorf("Error in initialisation: %s", serviceState.LastError.Error())
		os.Exit(1)
	}
	config := serviceState.Config.Services.CsmsServer

	log = logging.LoggingSetup(config.Debug, "csms-server")
	if len(serviceState.Config.Logging.AppInsightsInstrumentationKey) > 0 {
		log.AddHook(serviceState.AppInsightsHook)
	}

	log.Debugf("standalone_mode: %t", serviceState.Config.Services.CsmsServer.StandaloneMode)
	log.Debugf("enable_auth: %t", serviceState.Config.Services.CsmsServer.EnableAuth)

	// TODO
	//go expungeOldWaitingMessages(serviceState.MessagesWaiting, 1*time.Minute) // Expunge every 1 minute

	go serviceState.MqBus.RunMqTopicReceiver(ProcessRecvMqMessage, mq.MqChannelName_MessagesOut, serviceState)

	var err error
	listenNetPort := fmt.Sprintf("%s:%d", config.ListenAddress, config.ListenPort)
	log.Info("OCPP listening on: ", listenNetPort)

	hostName := serviceState.Context.HostName
	mq.MqNotifyNodeConnected(serviceState.MqBus, hostName)

	ioCloser, err := httplistener.ListenAndServeWithClose(listenNetPort, HttpHandler(serviceState))
	serviceState.IoCloser = &ioCloser

	if err != nil {
		log.Fatalln(err)
		os.Exit(1)
	}
	log.Debug("block...")
	exitNotification <- struct{}{} // block until exit notification received
	log.Debug("Service closing...")
	mq.MqNotifyNodeDisconnected(serviceState.MqBus, hostName)

	dispose()

	os.Exit(0)
}

func expungeOldWaitingMessages(m *xsync.Map, ageLimit time.Duration) {
	for {
		time.Sleep(20 * time.Second) // Adjust the interval as needed

		now := time.Now()
		m.Range(func(key string, value interface{}) bool {
			val, ok := value.(svc.WaitingMessage)
			if !ok {
				return true // Skip if not the expected type
			}

			// Check if the entry is older than the specified age limit
			if now.Sub(val.CreatedTimestamp) > ageLimit {
				m.Delete(key)
				log.Infof("Deleted key: %s\n", key)
			}
			return true // Continue iteration
		})
	}
}

func dispose() {
	if serviceState.IoCloser != nil {
		log.Debug("Close websocket listener")
		(*serviceState.IoCloser).Close()
	}
	if serviceState.MqBus != nil {
		log.Debug("Close MQ")
		serviceState.MqBus.Close()
	}
	if serviceState.Cache != nil {
		log.Debug("Close cache")
		serviceState.Cache.Close()
	}
}

func multiSignalHandler(signal os.Signal) {

	switch signal {
	case syscall.SIGHUP:
		log.Debug("Signal: ", signal.String())
	case syscall.SIGINT:
		log.Debug("Signal: ", signal.String())
	case syscall.SIGTERM:
		log.Debug("Signal: ", signal.String())
	case syscall.SIGQUIT:
		log.Debug("Signal: ", signal.String())
	default:
		log.Warnf("Unhandled/unknown signal %s", signal.String())
	}
}

func ProcessRecvMqMessage(messageBy []byte, state any) { //message amqp.Delivery
	msgEnvelope := new(mqmodels.MqMessageEnvelope)
	err := json.Unmarshal(messageBy, &msgEnvelope)
	if err != nil {
		log.Errorf("MQ Received Message, unmarshall error: %s\n", err.Error())
		// TODO reply with OCPP error?
		return
	}

	// Forward MQ received message to the client
	serviceState := state.(*ServiceState)
	val, ok := serviceState.Connections.Load(msgEnvelope.Client)

	if ok {
		connection := val.(*svcmodels.ConnectionState)
		//ocppEnvelopeFields := new(ocppmodels.OcppMessage)
		ocppEnvelopeFields := msgEnvelope.Body.(map[string]interface{})
		/*err := json.Unmarshal(msgEnvelope.Body, &ocppEnvelopeFields)
		if err != nil {
			log.Errorf("[ %s ] Unable to unmarshall message envelope: %s - %s", msgEnvelope.Client, string(messageBy), err.Error())
			// TODO reply with OCPP error ?
			return
		}*/

		body, err := json.Marshal(ocppEnvelopeFields["messageBody"])
		if err != nil {
			log.Errorf("[ %s ] Unable to marshall messageBody for envelope: %s - %s", msgEnvelope.Client, string(messageBy), err.Error())
			// TODO reply with OCPP error ?
			return
		}
		msgId := ocppEnvelopeFields["msgId"].(string)

		var msgReply string
		direction := int(ocppEnvelopeFields["direction"].(float64))
		if direction == 2 {
			waitMessage := &svc.WaitingMessage{}
			waitMessage.Notify = make(chan int)
			waitMessage.CreatedTimestamp = time.Now()
			serviceState.MessagesWaiting.Store(msgId, waitMessage)

			// TODO this is a bit of a hack
			msgReply = fmt.Sprintf("[%d,\"%s\",\"%s\",%s]",
				direction,
				ocppEnvelopeFields["msgId"].(string),
				ocppEnvelopeFields["messageType"].(string),
				body)
		} else {
			msgReply = fmt.Sprintf("[%d,\"%s\",%s]",
				direction,
				ocppEnvelopeFields["msgId"].(string),
				body)
		}

		log.Debugf("[ %s ] Reply: %s", msgEnvelope.Client, msgReply)
		connection.WebSocketMutex.Lock()
		err = connection.WebSocket.WriteMessage(websocket.TextMessage, []byte(msgReply))
		connection.WebSocketMutex.Unlock()
		if err != nil {
			log.Errorf("[ %s ] Error writing msg to client, msg: %s - %s", msgEnvelope.Client, string(msgReply), err.Error())
		}
	} else {
		log.Warnf("[ %s ] Client no longer exists, message lost: %s", msgEnvelope.Client, string(messageBy))
	}
}
