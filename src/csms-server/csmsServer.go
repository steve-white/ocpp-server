package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	redisManage "sw/ocpp/csms/internal/cache"
	conf "sw/ocpp/csms/internal/config"
	helpers "sw/ocpp/csms/internal/helpers"
	httplistener "sw/ocpp/csms/internal/http"
	LOG "sw/ocpp/csms/internal/logging"
	mqmodels "sw/ocpp/csms/internal/models/mq"
	svc "sw/ocpp/csms/internal/models/service"
	svcmodels "sw/ocpp/csms/internal/models/service"
	mq "sw/ocpp/csms/internal/mq"
	service "sw/ocpp/csms/internal/service"
	telemetry "sw/ocpp/csms/internal/telemetry"

	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"
	"github.com/puzpuzpuz/xsync/v3"
)

var (
	// Globals
	_exitNotification chan T
	_log              = LOG.Logger
	_serviceState     *ServiceState
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
		_log.Warn("Not connecting to redis auth cache")
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

	_exitNotification = make(chan T)

	go func() {
		sig := <-sigchnl
		multiSignalHandler(sig)
		_log.Debug("Caught close...")
		<-_exitNotification // send notification to unblock and exit
	}()

	_log = LOG.LoggingSetup(true, "csms-server") // start with debug enabled until overridden in config later

	_log.Infof("--- CSMS OCPP Server - v%s ---", service.Version)

	_serviceState = initialise()
	if _serviceState.LastError != nil {
		_log.Errorf("Error in initialisation: %s", _serviceState.LastError.Error())
		os.Exit(1)
	}
	config := _serviceState.Config.Services.CsmsServer

	_log = LOG.LoggingSetup(config.Debug, "csms-server")
	if len(_serviceState.Config.Logging.AppInsightsInstrumentationKey) > 0 {
		_log.AddHook(_serviceState.AppInsightsHook)
	}

	_log.Debugf("standalone_mode: %t", _serviceState.Config.Services.CsmsServer.StandaloneMode)
	_log.Debugf("enable_auth: %t", _serviceState.Config.Services.CsmsServer.EnableAuth)

	//go expungeOldWaitingMessages(_serviceState.MessagesWaiting, 1*time.Minute) // Expunge every 1 minute

	go _serviceState.MqBus.RunMqTopicReceiver(ProcessRecvMqMessage, mq.MqChannelName_MessagesOut, _serviceState)

	var err error
	listenNetPort := fmt.Sprintf("%s:%d", config.ListenAddress, config.ListenPort)
	_log.Info("OCPP listening on: ", listenNetPort)

	hostName := _serviceState.Context.HostName
	mq.MqNotifyNodeConnected(_serviceState.MqBus, hostName)

	_ioCloser, err := httplistener.ListenAndServeWithClose(listenNetPort, HttpHandler(_serviceState))
	_serviceState.IoCloser = &_ioCloser

	if err != nil {
		log.Fatalln(err)
		os.Exit(1)
	}
	_log.Debug("block...")
	_exitNotification <- struct{}{} // block until exit notification received
	_log.Debug("Service closing...")
	mq.MqNotifyNodeDisconnected(_serviceState.MqBus, hostName)

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
				_log.Infof("Deleted key: %s\n", key)
			}
			return true // Continue iteration
		})
	}
}

func dispose() {
	if _serviceState.IoCloser != nil {
		_log.Debug("Close websocket listener")
		(*_serviceState.IoCloser).Close()
	}
	if _serviceState.MqBus != nil {
		_log.Debug("Close MQ")
		_serviceState.MqBus.Close()
	}
	if _serviceState.Cache != nil {
		_log.Debug("Close cache")
		_serviceState.Cache.Close()
	}
}

func multiSignalHandler(signal os.Signal) {

	switch signal {
	case syscall.SIGHUP:
		_log.Debug("Signal: ", signal.String())
	case syscall.SIGINT:
		_log.Debug("Signal: ", signal.String())
	case syscall.SIGTERM:
		_log.Debug("Signal: ", signal.String())
	case syscall.SIGQUIT:
		_log.Debug("Signal: ", signal.String())
	default:
		_log.Warnf("Unhandled/unknown signal %s", signal.String())
	}
}

func ProcessRecvMqMessage(messageBy []byte, state any) { //message amqp.Delivery
	msgEnvelope := new(mqmodels.MqMessageEnvelope)
	err := json.Unmarshal(messageBy, &msgEnvelope)
	if err != nil {
		_log.Errorf("MQ Received Message, unmarshall error: %s\n", err.Error())
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
			_log.Errorf("[ %s ] Unable to unmarshall message envelope: %s - %s", msgEnvelope.Client, string(messageBy), err.Error())
			// TODO reply with OCPP error ?
			return
		}*/

		body, err := json.Marshal(ocppEnvelopeFields["messageBody"])
		if err != nil {
			_log.Errorf("[ %s ] Unable to marshall messageBody for envelope: %s - %s", msgEnvelope.Client, string(messageBy), err.Error())
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

		_log.Debugf("[ %s ] Reply: %s", msgEnvelope.Client, msgReply)
		connection.WebSocketMutex.Lock()
		err = connection.WebSocket.WriteMessage(websocket.TextMessage, []byte(msgReply))
		connection.WebSocketMutex.Unlock()
		if err != nil {
			_log.Errorf("[ %s ] Error writing msg to client, msg: %s - %s", msgEnvelope.Client, string(msgReply), err.Error())
		}
	} else {
		_log.Warnf("[ %s ] Client no longer exists, message lost: %s", msgEnvelope.Client, string(messageBy))
	}
}
