package service

import (
	"net/http"
	mqModels "sw/ocpp/csms/internal/models/mq"
	ocppModels "sw/ocpp/csms/internal/ocpp"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type ServiceContext struct {
	HostName string
}

type ConnectionState struct {
	Info           *ConnectionInfo
	HttpRequest    *http.Request
	WebSocket      *websocket.Conn
	WebSocketMutex sync.Mutex
}

type ConnectionInfo struct {
	NetworkId  string
	RemoteAddr string
}

type WaitingMessage struct {
	Notify           chan int
	Response         *ocppModels.OcppMessage
	CreatedTimestamp time.Time
}

type DeviceWaitingMessage struct {
	Envelope         *mqModels.MqMessageEnvelope
	Notify           chan int
	Response         *ocppModels.OcppMessage
	CreatedTimestamp time.Time
}
