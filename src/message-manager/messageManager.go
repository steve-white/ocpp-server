package main

import (
	"os"
	"os/signal"
	"syscall"

	conf "sw/ocpp/csms/internal/config"
	helpers "sw/ocpp/csms/internal/helpers"
	log "sw/ocpp/csms/internal/logging"
	svc "sw/ocpp/csms/internal/models/service"
	mq "sw/ocpp/csms/internal/mq"
	service "sw/ocpp/csms/internal/service"
	table "sw/ocpp/csms/internal/table"
	telemetry "sw/ocpp/csms/internal/telemetry"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/puzpuzpuz/xsync/v3"
)

var (
	// Globals
	_exitNotification chan T
	_log              = log.Logger
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
		log.Logger.Warn("Not storing messages")
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

	_exitNotification = make(chan T)

	go func() {
		sig := <-sigchnl
		multiSignalHandler(sig)
		_log.Debug("Caught close...")
		<-_exitNotification // send notification to unblock and exit
	}()

	_log = log.LoggingSetup(true, "messageManager") // start with debug enabled until overridden in config later

	_log.Infof("--- OCPP Message Manager - v%s ---", service.Version)

	_serviceState = initialise()
	if _serviceState.LastError != nil {
		_log.Errorf("Error in initialisation: %s", _serviceState.LastError.Error())
		os.Exit(1)
	}
	config := _serviceState.Config
	_log = log.LoggingSetup(config.Services.MessageManager.Debug, "messageManager")
	if len(config.Logging.AppInsightsInstrumentationKey) > 0 {
		_log.AddHook(_serviceState.AppInsightsHook)
	}

	go _serviceState.MqBus.RunMqTopicReceiver(ProcessRecvMessage, mq.MqChannelName_MessagesIn, _serviceState)

	_log.Debug("block...")
	_exitNotification <- struct{}{} // block until exit notification received
	_log.Debug("Service closing...")
	dispose()

	os.Exit(0)
}

func dispose() {
	if _serviceState.Cache != nil {
		_log.Debug("Close cache")
		_serviceState.Cache.Close()
	}

	if _serviceState.MqBus != nil {
		_log.Debug("Close MqChannel")
		_serviceState.MqBus.Close()
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
