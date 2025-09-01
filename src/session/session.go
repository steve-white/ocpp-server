package main

import (
	"os"
	"os/signal"
	"syscall"

	conf "sw/ocpp/csms/internal/config"
	db "sw/ocpp/csms/internal/db"
	helpers "sw/ocpp/csms/internal/helpers"
	log "sw/ocpp/csms/internal/logging"
	svc "sw/ocpp/csms/internal/models/service"
	mq "sw/ocpp/csms/internal/mq"
	service "sw/ocpp/csms/internal/service"
	telemetry "sw/ocpp/csms/internal/telemetry"

	"github.com/puzpuzpuz/xsync/v3"
)

var (
	// Set by build LDFLAGS
	Version        = "dev"
	CommitHash     = "n/a"
	BuildTimestamp = "n/a"

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

	mqConnection := mq.SetupMqConnection(config.Mq, "", config.Mq.MangosMq.CsmsListenUrl, "", config.Mq.MangosMq.CsmsListenRequestUrl)

	err = mqConnection.MqConnect()
	if err != nil {
		return &ServiceState{LastError: err}
	}

	// TODO is this needed?
	err = mqConnection.MqQueueDeclare(mq.MqChannelName_Notify)
	if err != nil {
		return &ServiceState{LastError: err}
	}

	mq.SetupMqReceiver(mqConnection, config.Mq.Type, serviceContext.HostName, mq.MqChannelName_MessagesIn)

	return &ServiceState{
		Config:          config,
		MqBus:           mqConnection,
		Connections:     xsync.NewMap(),
		Context:         serviceContext,
		AppInsightsHook: telemetryHook,
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

	_log = log.LoggingSetup(true, "session") // start with debug enabled until overridden in config later

	_log.Infof("--- OCPP Session - v%s ---", service.Version)

	_serviceState = initialise()
	if _serviceState.LastError != nil {
		_log.Errorf("Error in initialisation: %s", _serviceState.LastError.Error())
		os.Exit(1)
	}

	_log = log.LoggingSetup(_serviceState.Config.Services.Session.Debug, "session")
	if len(_serviceState.Config.Logging.AppInsightsInstrumentationKey) > 0 {
		_log.AddHook(_serviceState.AppInsightsHook)
	}
	dbConfig := _serviceState.Config.DbConfig
	err := db.ConnectDb(dbConfig.DbType, dbConfig.DbConnectionString)
	if err != nil {
		_log.Errorf("Error in DB connection: %s", err.Error())
		os.Exit(1)
	}
	err = db.CreateTables()
	if err != nil {
		_log.Errorf("Error in DB table create: %s", err.Error())
		os.Exit(1)
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
