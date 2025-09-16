package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	conf "sw/ocpp/csms/internal/config"
	db "sw/ocpp/csms/internal/db"
	helpers "sw/ocpp/csms/internal/helpers"
	httplistener "sw/ocpp/csms/internal/http"
	"sw/ocpp/csms/internal/logging"
	svc "sw/ocpp/csms/internal/models/service"
	mq "sw/ocpp/csms/internal/mq"
	ocppmodels "sw/ocpp/csms/internal/ocpp"
	service "sw/ocpp/csms/internal/service"
	telemetry "sw/ocpp/csms/internal/telemetry"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/puzpuzpuz/xsync/v3"

	"github.com/go-chi/render"
)

var (
	// Set by build LDFLAGS
	Version        = "dev"
	CommitHash     = "n/a"
	BuildTimestamp = "n/a"

	// Globals
	exitNotification  chan T
	log               = logging.Logger
	serviceState      *ServiceState
	actionTimeoutSecs time.Duration = 5 // TODO make this configurable - move to config
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
		Context:         serviceContext,
		AppInsightsHook: telemetryHook,
		MessagesWaiting: xsync.NewMap(),
	}
}

func getServiceContext() svc.ServiceContext {
	return svc.ServiceContext{HostName: helpers.GetHostName()}
}

func createSendActionHandler(msgType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		action_SendPayloadToCharger(w, r, msgType)
	}
}

func setupRestApi(serviceState *ServiceState, config conf.HttpConfig) error {

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)
	//router.Use(mwLogger.New(log))
	router.Use(middleware.Recoverer)
	router.Use(middleware.URLFormat)

	router.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})
	router.Route("/", func(r chi.Router) {
		r.Use(middleware.BasicAuth("device-manager", map[string]string{
			config.HttpUser: config.HttpPassword,
		}))

		r.Route("/actions", func(r chi.Router) {
			r.Route("/datatransfer/{networkid}", func(r chi.Router) {
				r.Use(NetworkIdCtx)
				r.Post("/", action_dataTransfer)
				//r.Post("/restart", action_dataTransfer)
			})
			r.Route("/setchargingprofile/{networkid}", func(r chi.Router) {
				r.Use(NetworkIdCtx)
				r.Post("/", createSendActionHandler(ocppmodels.MsgType_SetChargingProfile))
			})
			r.Route("/clearchargingprofile/{networkid}", func(r chi.Router) {
				r.Use(NetworkIdCtx)
				r.Post("/", createSendActionHandler(ocppmodels.MsgType_ClearChargingProfile))
			})
			r.Route("/remotestarttransaction/{networkid}", func(r chi.Router) {
				r.Use(NetworkIdCtx)
				r.Post("/", createSendActionHandler(ocppmodels.MsgType_RemoteStartTransaction))
			})
			r.Route("/remotestoptransaction/{networkid}", func(r chi.Router) {
				r.Use(NetworkIdCtx)
				r.Post("/", createSendActionHandler(ocppmodels.MsgType_RemoteStopTransaction))
			})
			r.Route("/unlockconnector/{networkid}", func(r chi.Router) {
				r.Use(NetworkIdCtx)
				r.Post("/", createSendActionHandler(ocppmodels.MsgType_UnlockConnector))
			})
			r.Route("/reset/{networkid}", func(r chi.Router) {
				r.Use(NetworkIdCtx)
				r.Post("/", createSendActionHandler(ocppmodels.MsgType_Reset))
			})
			r.Route("/getdiagnostics/{networkid}", func(r chi.Router) {
				r.Use(NetworkIdCtx)
				r.Post("/", createSendActionHandler(ocppmodels.MsgType_GetDiagnostics))
			})
			r.Route("/getconfiguration/{networkid}", func(r chi.Router) {
				r.Use(NetworkIdCtx)
				r.Post("/", createSendActionHandler(ocppmodels.MsgType_GetConfiguration))
			})
			r.Route("/changeavailability/{networkid}", func(r chi.Router) {
				r.Use(NetworkIdCtx)
				r.Post("/", createSendActionHandler(ocppmodels.MsgType_ChangeAvailability))
			})
			r.Route("/changeconfiguration/{networkid}", func(r chi.Router) {
				r.Use(NetworkIdCtx)
				r.Post("/", createSendActionHandler(ocppmodels.MsgType_ChangeConfiguration))
			})
			r.Route("/triggermessage/{networkid}", func(r chi.Router) {
				r.Use(NetworkIdCtx)
				r.Post("/", createSendActionHandler(ocppmodels.MsgType_TriggerMessage))
			})
		})
	})

	log.Info("Starting REST API Server")
	listenNetPort := fmt.Sprintf("%s:%d", config.ListenAddress, config.ListenPort)
	log.Info("REST API listening on: ", listenNetPort)

	_ioCloser, err := httplistener.ListenAndServeWithClose(listenNetPort, router)
	if err != nil {
		log.Error("Failed to start REST API server")
		return err
	}
	serviceState.IoCloser = &_ioCloser

	log.Info("REST server started")
	return nil
}

func StreamToByte(stream io.Reader) []byte {
	buf := new(bytes.Buffer)
	buf.ReadFrom(stream)
	return buf.Bytes()
}

func action_SendPayloadToCharger(w http.ResponseWriter, r *http.Request, msgType string) {
	log.Info("Path: " + r.URL.Path)

	device := r.Context().Value("device").(*Device)
	var response *ocppmodels.ActionResponse

	var buf bytes.Buffer
	_, err := io.Copy(&buf, r.Body)
	if err != nil {
		log.Errorf("Error streaming response: %s", err.Error())
		render.JSON(w, r, createActionResponse("Error streaming response"))
		return
	}
	log.Debugf("Read: %s", buf.String())

	msgId := ocppmodels.GenerateUniqueId()
	profileRaw := json.RawMessage(buf.Bytes())
	ocppMessageJson, err := CreateCsmsToDeviceRequest(msgId, device.ServerNode, device.NetworkId, msgType, profileRaw)
	if err != nil {
		log.Errorf("Error in CreateCsmsToDeviceRequest: %s", err.Error())
		render.JSON(w, r, createActionResponse("Error in serialisation"))
		return
		// TODO log to appinsights
	}

	waitMessage := &svc.DeviceWaitingMessage{}
	waitMessage.CreatedTimestamp = time.Now()
	waitMessage.Notify = make(chan int)

	serviceState.MessagesWaiting.Store(msgId, waitMessage)
	mqErr := serviceState.MqBus.MqMessagePublishRetry(mq.MqChannelName_MessagesOut, ocppMessageJson)
	if mqErr != nil {
		log.Errorf("Error sending reply to MQ, msg lost: %s", mqErr.Error())
		render.JSON(w, r, createActionResponse("Error sending reply to MQ, msg lost"))
		return
		// TODO log to appinsights
	}

	responseRaw, ok := WithTimeout(func() interface{} { return <-waitMessage.Notify }, time.Second*actionTimeoutSecs)
	if !ok {
		log.Errorf("Timed out waiting for response")
		response = createActionResponse("Timed out waiting for response")
	} else {
		if responseRaw != nil {
			responseStr := string(waitMessage.Response.MessageBody)
			response = createActionResponse(responseStr)

			log.Info("Response: " + responseStr)
			w.WriteHeader(http.StatusOK)
		} else {
			log.Warn("Nil response for action")
			response = createActionResponse("Nil response")
			w.WriteHeader(http.StatusNotFound)
		}
	}
	render.JSON(w, r, response)
	serviceState.MessagesWaiting.Delete(msgId)
}

func action_dataTransfer(w http.ResponseWriter, r *http.Request) {
	device := r.Context().Value("device").(*Device)
	log.Info("DataTransfer request")
	dataTransferMessage := &ocppmodels.OcppDataTransfer{}
	err := json.Unmarshal(StreamToByte(r.Body), dataTransferMessage)
	if err != nil {
		log.Errorf("Error unmarshalling DataTransfer: %s", err.Error())
		return
	}

	dataTransferBy, err := json.Marshal(&dataTransferMessage)
	if err != nil {
		log.Errorf("Error marshalling response: %s", err.Error())
		return
	}

	msgId := ocppmodels.GenerateUniqueId()
	dataTransferRaw := json.RawMessage(dataTransferBy)
	ocppMessageJson, _ := CreateCsmsToDeviceRequest(msgId, device.ServerNode, device.NetworkId, ocppmodels.MsgType_DataTransfer, dataTransferRaw)

	waitMessage := &svc.DeviceWaitingMessage{}
	waitMessage.CreatedTimestamp = time.Now()
	waitMessage.Notify = make(chan int)

	//err := serviceState.MqBus.MqRegisterCallback(mq.MqChannelName_MessagesIn)
	serviceState.MessagesWaiting.Store(msgId, waitMessage)
	mqErr := serviceState.MqBus.MqMessagePublishRetry(mq.MqChannelName_MessagesOut, ocppMessageJson)
	if mqErr != nil {
		log.Errorf("Error sending reply to MQ, msg lost: %s", mqErr.Error())
		//return mqErr // transient MQ error unrecoverable, close connection to CP
		// TODO log to appinsights
	}

	responseRaw, ok := WithTimeout(func() interface{} { return <-waitMessage.Notify }, time.Second*actionTimeoutSecs)
	var response *ocppmodels.ActionResponse
	if !ok {
		log.Errorf("Timed out waiting for response")
		response = createActionResponse("Timed out waiting for response")
	} else {
		if responseRaw != nil {
			responseStr := string(waitMessage.Response.MessageBody)
			response = createActionResponse(responseStr)
			log.Info("Response: " + responseStr)
			w.WriteHeader(http.StatusOK)
		} else {
			log.Warn("Nil response for data transfer action")
			response = createActionResponse("Nil response")
			w.WriteHeader(http.StatusNotFound)
		}
	}
	render.JSON(w, r, response)
	serviceState.MessagesWaiting.Delete(msgId)
}

func createActionResponse(message string) *ocppmodels.ActionResponse {
	msgData, err := json.Marshal(message)
	if err != nil {
		fmt.Println("Error marshaling to JSON:", err)
		return nil
	}

	response := ocppmodels.ActionResponse{MessageBody: json.RawMessage(msgData)}

	return &response
}

type ActionResponse struct {
}

func ActionResponseRender(rd *ocppmodels.ActionResponse) *ActionResponse {
	actionResponse := new(ActionResponse)

	return actionResponse
}

func (rd *ActionResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

type ErrResponse struct {
	Err            error `json:"-"` // low-level runtime error
	HTTPStatusCode int   `json:"-"` // http response status code

	StatusText string `json:"status"`          // user-level status message
	AppCode    int64  `json:"code,omitempty"`  // application-specific error code
	ErrorText  string `json:"error,omitempty"` // application-level error message, for debugging
}

func (e *ErrResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

func ErrRender(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: 422,
		StatusText:     "Error rendering response.",
		ErrorText:      err.Error(),
	}
}

func WithTimeout(delegate func() interface{}, timeout time.Duration) (ret interface{}, ok bool) {
	ch := make(chan interface{}, 1) // buffered
	go func() { ch <- delegate() }()
	select {
	case ret = <-ch:
		return ret, true
	case <-time.After(timeout):
	}
	return nil, false
}

func CreateCsmsToDeviceRequest(msgId string, serverNode string, client string, messageType string, rawMessage json.RawMessage) (string, error) {
	ocppResponse := new(ocppmodels.OcppMessage)
	ocppResponse.MsgId = msgId
	ocppResponse.MessageType = messageType
	ocppResponse.Direction = ocppmodels.OcppDirection_ClientServer
	ocppResponse.MessageBody = rawMessage

	/*jsonData, err := json.Marshal(ocppResponse)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return "", err
	}*/

	return mq.MqCreateMessageEnvelope(serverNode, client, ocppResponse)
}

// NetworkIdCtx middleware is used to load a device object from
// the URL parameters passed through as the request. In case
// the it could not be found, we stop here and return a 404.
func NetworkIdCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var device *Device
		var err error

		if networkid := chi.URLParam(r, "networkid"); networkid != "" {
			device, err = dbGetDevice(networkid)
		} else {
			//render.Render(w, r, ErrNotFound)
			return
		}
		if err != nil {
			//render.Render(w, r, ErrNotFound)
			return
		}

		ctx := context.WithValue(r.Context(), "device", device)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func dbGetDevice(networkid string) (*Device, error) {
	return &Device{NetworkId: networkid, ServerNode: "Node1"}, nil
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

	log = logging.LoggingSetup(true, "device-manager") // start with debug enabled until overridden in config later

	log.Infof("--- OCPP Device Manager - v%s ---", service.Version)

	serviceState = initialise()
	if serviceState.LastError != nil {
		log.Errorf("Error in initialisation: %s", serviceState.LastError.Error())
		os.Exit(1)
	}
	config := serviceState.Config.Services.DeviceManager

	log = logging.LoggingSetup(config.Debug, "device-manager")
	if len(serviceState.Config.Logging.AppInsightsInstrumentationKey) > 0 {
		log.AddHook(serviceState.AppInsightsHook)
	}
	dbConfig := serviceState.Config.DbConfig
	err := db.ConnectDb(dbConfig.DbType, dbConfig.DbConnectionString)
	if err != nil {
		log.Errorf("Error in DB connection: %s", err.Error())
		os.Exit(1)
	}
	err = db.CreateDeviceTables()
	if err != nil {
		log.Errorf("Error in DB table create: %s", err.Error())
		os.Exit(1)
	}

	go serviceState.MqBus.RunMqTopicReceiver(ProcessRecvMessage, mq.MqChannelName_MessagesIn, serviceState)

	setupRestApi(serviceState, config.HttpConfig)

	log.Debug("block...")
	exitNotification <- struct{}{} // block until exit notification received
	log.Debug("Service closing...")

	dispose()

	os.Exit(0)
}

func dispose() {
	// TODO: move timeout to config
	/*ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := serviceState.HttpServer.Shutdown(ctx); err != nil {
		log.Error("Failed to stop REST server")
	}*/
	if serviceState.IoCloser != nil {
		log.Debug("Close websocket listener")
		(*serviceState.IoCloser).Close()
	}

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
