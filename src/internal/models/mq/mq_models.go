package mq

type MqMessageEnvelope struct {
	ServerNode  string `json:"serverNode"`
	Client      string `json:"client"`
	MessageTime string `json:"messageTime"`
	Body        any    `json:"body"`
}

type MqNotifyConnectionChange struct {
	QueuedTime string `json:"queuedTime"`
	ServerNode string `json:"serverNode"`
	NotifyType string `json:"notifyType"`
	RemoteAddr string `json:"remoteAddr,omitempty"`
	NetworkId  string `json:"networkId,omitempty"`
}
