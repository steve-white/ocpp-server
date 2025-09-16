package main

import (
	"os"
	"os/signal"
	"syscall"

	conf "sw/ocpp/csms/internal/config"
	db "sw/ocpp/csms/internal/db"
	helpers "sw/ocpp/csms/internal/helpers"
	"sw/ocpp/csms/internal/logging"
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

	mqConnection := mq.SetupMqConnection(config.Mq, "", config.Mq.MangosMq.CsmsListenUrl, "", config.Mq.MangosMq.CsmsListenRequestUrl)

	err = mqConnection.MqConnect()
	if err != nil {
		return &ServiceState{LastError: err}
	}

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

	exitNotification = make(chan T)

	go func() {
		sig := <-sigchnl
		multiSignalHandler(sig)
		log.Debug("Caught close...")
		<-exitNotification // send notification to unblock and exit
	}()

	log = logging.LoggingSetup(true, "session") // start with debug enabled until overridden in config later

	log.Infof("--- OCPP Session - v%s ---", service.Version)

	serviceState = initialise()
	if serviceState.LastError != nil {
		log.Errorf("Error in initialisation: %s", serviceState.LastError.Error())
		os.Exit(1)
	}

	log = logging.LoggingSetup(serviceState.Config.Services.Session.Debug, "session")
	if len(serviceState.Config.Logging.AppInsightsInstrumentationKey) > 0 {
		log.AddHook(serviceState.AppInsightsHook)
	}
	dbConfig := serviceState.Config.DbConfig
	err := db.ConnectDb(dbConfig.DbType, dbConfig.DbConnectionString)
	if err != nil {
		log.Errorf("Error in DB connection: %s", err.Error())
		os.Exit(1)
	}
	err = db.CreateTables()
	if err != nil {
		log.Errorf("Error in DB table create: %s", err.Error())
		os.Exit(1)
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
