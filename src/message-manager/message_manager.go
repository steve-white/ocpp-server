package main

import (
	"os"
	"os/signal"
	"syscall"

	conf "sw/ocpp/csms/internal/config"
	helpers "sw/ocpp/csms/internal/helpers"
	"sw/ocpp/csms/internal/logging"
	svc "sw/ocpp/csms/internal/models/service"
	mq "sw/ocpp/csms/internal/mq"
	service "sw/ocpp/csms/internal/service"
	table "sw/ocpp/csms/internal/table"
	telemetry "sw/ocpp/csms/internal/telemetry"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/puzpuzpuz/xsync/v3"
)

var (
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

	mqConnection := mq.SetupMqConnection(config.Mq, "", config.Mq.MangosMq.CsmsListenUrl, "", "")

	err = mqConnection.MqConnect()
	if err != nil {
		return &ServiceState{LastError: err}
	}

	err = mqConnection.MqQueueDeclare(mq.MqChannelName_Notify)
	if err != nil {
		return &ServiceState{LastError: err}
	}

	var tableClient *aztables.Client
	if !config.Services.MessageManager.StoreMessages {
		log.Warn("Not storing messages")
	} else {
		mq.SetupMqReceiver(mqConnection, config.Mq.Type, serviceContext.HostName, mq.MqChannelName_MessagesIn)

		tableClient, err = table.GetTableClient("Messages", config.Services.MessageManager.StorageAccountName, config.Services.MessageManager.StorageAccountKey)
		if err != nil {
			return &ServiceState{LastError: err}
		}
		table.CreateTable(tableClient, "Messages")
	}

	return &ServiceState{
		Config:          config,
		MqBus:           mqConnection,
		Connections:     xsync.NewMap(),
		Context:         serviceContext,
		AppInsightsHook: telemetryHook,
		TableClient:     tableClient,
	}
}

func getServiceContext() svc.ServiceContext {
	return svc.ServiceContext{HostName: helpers.GetHostName()}
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

	log = logging.LoggingSetup(true, "messageManager") // start with debug enabled until overridden in config later

	log.Infof("--- OCPP Message Manager - v%s ---", service.Version)

	serviceState = initialise()
	if serviceState.LastError != nil {
		log.Errorf("Error in initialisation: %s", serviceState.LastError.Error())
		os.Exit(1)
	}
	config := serviceState.Config
	log = logging.LoggingSetup(config.Services.MessageManager.Debug, "messageManager")
	if len(config.Logging.AppInsightsInstrumentationKey) > 0 {
		log.AddHook(serviceState.AppInsightsHook)
	}

	go serviceState.MqBus.RunMqTopicReceiver(ProcessRecvMessage, mq.MqChannelName_MessagesIn, serviceState)

	log.Debug("block...")
	exitNotification <- struct{}{} // block until exit notification received
	log.Debug("Service closing...")
	dispose()

	os.Exit(0)
}

func dispose() {
	if serviceState.Cache != nil {
		log.Debug("Close cache")
		serviceState.Cache.Close()
	}

	if serviceState.MqBus != nil {
		log.Debug("Close MqChannel")
		serviceState.MqBus.Close()
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
