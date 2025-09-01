// Provides a HTTP Websocket Server
package main

import (
	"encoding/json"
	"errors"
	"sw/ocpp/csms/internal/helpers"
	svc "sw/ocpp/csms/internal/models/service"
	mq "sw/ocpp/csms/internal/mq"
	"sw/ocpp/csms/internal/ocpp"
	telemetry "sw/ocpp/csms/internal/telemetry"
	"time"

	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	logrus "github.com/sirupsen/logrus"
)

type OcppStatusNotification ocpp.OcppStatusNotification
type OcppSecurityEventNotification ocpp.OcppSecurityEventNotification
type OcppBootNotificationResponse ocpp.OcppBootNotificationResponse
type OcppMessage ocpp.OcppMessage
type OcppHeartBeatAck ocpp.OcppHeartBeatAck
type OcppDataTransfer ocpp.OcppDataTransfer

const NETWORKID_MAXLEN = 32
const MAX_MSG_SIZE = 8192

var (
	// DefaultUpgrader specifies the parameters for upgrading an HTTP
	// connection to a WebSocket connection.
	DefaultUpgrader = &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	// DefaultDialer is a dialer with all fields set to the default zero values.
	DefaultDialer = websocket.DefaultDialer
)

type Websocket struct {
	// Upgrader specifies the parameters for upgrading a incoming HTTP
	// connection to a WebSocket connection. If nil, DefaultUpgrader is used.
	Upgrader *websocket.Upgrader

	Delegate     HandleMessageDelegate
	serviceState *ServiceState
}

func HttpHandler(serviceState *ServiceState) *Websocket {

	return &Websocket{serviceState: serviceState}
}

func addConnection(serviceState *ServiceState, connState *svc.ConnectionState) {
	conns := serviceState.Connections

	// TODO switch to extId
	conns.Store(connState.Info.NetworkId, connState)
}

func removeConnection(serviceState *ServiceState, networkId string) {
	conns := serviceState.Connections
	conns.Delete(networkId)
}

// ServeHTTP implements the http.Handler that proxies WebSocket connections.
func (w *Websocket) ServeHTTP(rw http.ResponseWriter, req *http.Request) {

	_log.Debug("Client connected to : ", req.Host, " path:", req.URL.Path, ", client: ", req.RemoteAddr)
	authenticated, networkId := AuthConnection(rw, req, w.serviceState)
	if !authenticated {
		return
	}

	remoteAddrStr := req.RemoteAddr
	connInfo := svc.ConnectionInfo{NetworkId: networkId, RemoteAddr: remoteAddrStr}
	connectionState := svc.ConnectionState{
		Info:        &connInfo,
		HttpRequest: req,
	}

	addConnection(w.serviceState, &connectionState)
	if !w.serviceState.Config.Services.CsmsServer.StandaloneMode {
		err := mq.MqNotifyClientConnected(w.serviceState.MqBus, w.serviceState.Context.HostName, connectionState.Info)
		if err != nil {
			_log.Errorf("%s : websocket: problem sending MQ notify connected %s", remoteAddrStr, err)
			return
		}
	}

	upgrader := DefaultUpgrader
	upgradeHeader := http.Header{}

	// Upgrade the existing incoming request to a WebSocket connection.
	connPub, err := upgrader.Upgrade(rw, req, upgradeHeader)
	if err != nil {
		_log.Errorf("%s : websocket: couldn't upgrade %s", remoteAddrStr, err)
		return
	}
	connectionState.WebSocket = connPub
	defer connPub.Close()

	errClient := make(chan error, 1)

	handleClientWebsocket := func(src *websocket.Conn, errc chan error) {

		/*if (_log.IsLevelEnabled(logrus.DebugLevel)) {
			_log.Debugf("MaxMsgSize: %d", MAX_MSG_SIZE)
		}*/
		src.SetReadLimit(MAX_MSG_SIZE)

		for {

			msgType, msg, err := src.ReadMessage()
			if err != nil {
				/*m := websocket.FormatCloseMessage(websocket.CloseNormalClosure, fmt.Sprintf("%v", err))
				if e, ok := err.(*websocket.CloseError); ok {
					if e.Code != websocket.CloseNoStatusReceived {
						m = websocket.FormatCloseMessage(e.Code, e.Text)
					}
				}*/
				errc <- err
				//disposeClient(w.serviceState, &connectionState)
				_log.Warnf("%s : Client disconnected(read): %s", remoteAddrStr, err)
				break
			}

			err = HandleMessage(msgType, msg, w.serviceState, &connectionState)
			if err != nil {
				errc <- err
				_log.Warnf("%s : Error: %s", remoteAddrStr, err)
				break
			}
		}
	}

	// TODO use netpoll instead:
	// https://www.freecodecamp.org/news/million-websockets-and-go-cc58418460bb/
	// https://github.com/gobwas/ws
	go handleClientWebsocket(connPub, errClient)

	//cancelChan := make(chan bool)
	//go runPingWebsocket(connPub, errClient, cancelChan)

	var message string
	if err == <-errClient {
		message = "websocket: Error when copying from client: %v"
	}
	_log.Warnf("%s : Wait return, close", remoteAddrStr)
	if e, ok := err.(*websocket.CloseError); !ok || e.Code == websocket.CloseAbnormalClosure {
		_log.Errorf(message, err)
	}
	disposeClient(w.serviceState, &connectionState)
	_log.Warnf("%s : Return", remoteAddrStr)
}

func disposeClient(serviceState *ServiceState, connState *svc.ConnectionState) {
	if !serviceState.Config.Services.CsmsServer.StandaloneMode {
		mq.MqNotifyClientDisconnected(serviceState.MqBus, serviceState.Context.HostName, connState.Info)
	}

	connState.WebSocket.Close()
	removeConnection(serviceState, connState.Info.NetworkId)
}

/*
func runPingWebsocket(src *websocket.Conn, errc chan error, cancel chan bool) {

		for {
			select {
			case <-cancel:
				break
			case <-time.After(60 * time.Second):
				msg := GetHeatBeatAck()
				_log.Debugf("%s: Send Heartbeat: %s", src.RemoteAddr().String(), msg)
				err := src.WriteMessage(websocket.TextMessage, []byte(msg))
				if err != nil {
					errc <- err
					_log.Warnf("Client disconnected: %s", src.RemoteAddr().String())
					break
				}
				break
			}
		}
	}
*/
type HandleMessageDelegate interface {
	HandleMessage(isServer bool, msg []byte) []byte
}

func HandleMessage(msgType int, msgBytes []byte, serviceState *ServiceState, connectionState *svc.ConnectionState) error {
	msgStr := string(msgBytes)

	_log.Debug("RecvClient->: ", msgStr)

	var msgEnvelope OcppMessage
	err := msgEnvelope.UnmarshalOcppJson(msgBytes)

	if !serviceState.Config.Services.CsmsServer.StandaloneMode {
		connectionUrl := fmt.Sprintf("%s:%d:%s", serviceState.Context.HostName,
			serviceState.Config.Services.CsmsServer.ListenPort,
			connectionState.HttpRequest.URL)
		telemetry.TrackConnectionRequest(connectionUrl, 1)
	}

	var msgSendBy []byte = nil

	skipAck := false
	sendToMq := true
	standaloneMode := serviceState.Config.Services.CsmsServer.StandaloneMode
	if err != nil {
		_log.Warnf("Unable to parse ocpp envelope: %s, for message: %s", err, msgStr)
		// TODO envelope bad messages and send to other storage?

		return nil // Ignore malformed message
	} else {
		if msgEnvelope.Direction == ocpp.MsgType_ServerToClientResult {
			_, ok := serviceState.MessagesWaiting.Load(msgEnvelope.MsgId)
			if ok {
				//response := ocpp.OcppMessage(msgEnvelope)
				//msg := val.(*svc.WaitingMessage)
				//msg.Response = &response
				//msg.CreatedTimestamp = time.Now()
				_log.Debugf("Valid message response: %s", msgEnvelope.MsgId)

				mqErr := serviceState.MqBus.MqSendClientMessageRetry(serviceState.Context.HostName, connectionState.Info, msgEnvelope)
				if mqErr != nil {
					_log.Errorf("Error sending to MQ: %s", mqErr.Error())
					return mqErr // transient MQ error unrecoverable, close connection to CP
				}

				serviceState.MessagesWaiting.Delete(msgEnvelope.MsgId)
				return nil
			} else {
				_log.Warnf("No waiting messages for message: %s", msgStr)
			}
		} else if msgEnvelope.Direction == ocpp.MsgType_ClientToServer {
			switch msgEnvelope.MessageType {
			case "StatusNotification", "MeterValues", "SecurityEventNotification":
				if standaloneMode {
					sendToMq = false
				}
			case "BootNotification":
				if standaloneMode {
					sendToMq = false
				}

				_log.Debugf("Received BootNotification: %s", msgStr)
				bootResponse := OcppBootNotificationResponse{Status: ocpp.BootStatus_Accepted, CurrentTime: helpers.GenerateDateNow(), Interval: 60}
				msgSendBy, _ = MarshalOcppJsonResponse(ocpp.MsgType_ServerToClientResult, msgEnvelope.MsgId, &bootResponse)

			case "Heartbeat":
				if standaloneMode {
					sendToMq = false
				}
				_log.Debugf("Received Heartbeat: %s", msgStr)
				msg := ocpp.GetHeatBeatAck(msgEnvelope.MsgId)
				msgSendBy = []byte(msg)
			case "StartTransaction":
				sendToMq = true
				skipAck = true
			case "StopTransaction":
				sendToMq = true
				skipAck = false
			case "DataTransfer":
				skipAck = false
				msgSendBy = []byte(fmt.Sprintf("[%d,\"%s\",{\"status\":\"UnknownVendorId\"}]", ocpp.MsgType_ServerToClientResult, msgEnvelope.MsgId))
			}
		} else {
			_log.Errorf("Unhandled OCPP direction: %d", msgEnvelope.Direction)
		}
	}

	if sendToMq {
		mqErr := serviceState.MqBus.MqSendClientMessageRetry(serviceState.Context.HostName, connectionState.Info, msgEnvelope)
		if mqErr != nil {
			_log.Errorf("Error sending to MQ: %s", mqErr.Error())
			return mqErr // transient MQ error unrecoverable, close connection to CP
		}
	}

	if !skipAck && msgSendBy == nil { // ACK message once it's send to MQ
		msgSendBy = []byte(getSimpleAckMsg(msgEnvelope.MsgId))
	}
	if !standaloneMode {
		telemetry.TrackOcppRequest(connectionState.Info.NetworkId, connectionState.Info.RemoteAddr,
			msgEnvelope.MsgId, msgEnvelope.MessageType, "200", 1*time.Millisecond)
	}

	if msgSendBy != nil {
		if _log.IsLevelEnabled(logrus.DebugLevel) {
			_log.Debug("<-SendClient: ", string(msgSendBy))
		}
		connectionState.WebSocketMutex.Lock()
		err = connectionState.WebSocket.WriteMessage(msgType, msgSendBy)
		connectionState.WebSocketMutex.Unlock()
		if err != nil {
			_log.Warnf("%s : Client disconnected(write): %s", connectionState.Info.RemoteAddr, err)
			return err
		}
	}

	return nil
}

func getSimpleAckMsg(msgId string) string {
	return fmt.Sprintf("[%d,\"%s\",{}]", ocpp.MsgType_ServerToClientResult, msgId)
}

func GetOcppDirection(buf []byte) (int, error) {
	var parsedData []interface{}
	err := json.Unmarshal(buf, &parsedData)
	if err != nil {
		_log.Errorf("Error parsing JSON: %s", err)
		return -1, err
	}

	if len(parsedData) > 0 {
		if firstValue, ok := parsedData[0].(float64); ok { // JSON numbers are parsed as float64
			direction := int(firstValue)
			return direction, nil
		} else {
			_log.Errorf("First value is not a number")
			return -1, errors.New("First value is not a number")
		}
	} else {
		_log.Errorf("JSON array is empty")
		return -1, errors.New("JSON array is empty")
	}
}

func (n *OcppMessage) UnmarshalOcppJson(buf []byte) error {

	direction, err := GetOcppDirection(buf)
	if err != nil {
		return err
	}

	if direction == ocpp.MsgType_ClientToServer {

		tmp := []interface{}{&n.Direction, &n.MsgId, &n.MessageType, &n.MessageBody}

		if err := json.Unmarshal(buf, &tmp); err != nil {
			return err
		}
	} else if direction == ocpp.MsgType_ServerToClientResult {

		tmp := []interface{}{&n.Direction, &n.MsgId, &n.MessageBody}

		if err := json.Unmarshal(buf, &tmp); err != nil {
			return err
		}
	}
	return nil
}

func MarshalOcppJsonResponse(direction int, msgId string, messageBody *OcppBootNotificationResponse) ([]byte, error) {
	messageBodyJson, err := json.Marshal(&messageBody)
	if err != nil {
		return nil, err
	}

	messageBodyRaw := json.RawMessage(messageBodyJson)
	ocppEnvelope := []interface{}{direction, msgId, messageBodyRaw}

	output, err := json.Marshal(ocppEnvelope)
	return output, err
}
