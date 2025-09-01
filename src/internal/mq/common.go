package mq

import (
	"encoding/json"
	"os"
	conf "sw/ocpp/csms/internal/config"
	"sw/ocpp/csms/internal/helpers"
	log "sw/ocpp/csms/internal/logging"
	mqmodels "sw/ocpp/csms/internal/models/mq"
	svc "sw/ocpp/csms/internal/models/service"
)

type MqBus interface {
	MqConnect() error
	Close() error
	MqQueueDeclare(queueName string) error
	MqMessagePublish(queueName string, json string) error
	MqSendClientMessageRetry(hostName string, connState *svc.ConnectionInfo, body any) error
	MqMessagePublishRetry(channel string, json string) error
	RunMqTopicReceiver(ProcessRecvMqMessage func(messageBy []byte, state any), topicName string, state any) error
	SetupMqTopicReceiver(channelName string, routingKey string) error
}

func SetupMqReceiver(mqConnection MqBus, mqType, hostname string, channelName string) {
	// TODO tidy this up
	if mqType == "mangos_mq" {
		routingKey := hostname
		mqConnection.SetupMqTopicReceiver(channelName, routingKey)
	} else if mqType == "rabbit_mq" {
		routingKey := hostname
		mqConnection.SetupMqTopicReceiver(channelName, routingKey)
	} else if mqType == "redis_mq" {
		mqConnection.SetupMqTopicReceiver(channelName, "")
	}
}

func SetupMqConnection(config conf.MqConfig, publisherListenUrl string, subscriberConnectUrl string, requestListenUrl string, requestConnectUrl string) MqBus {
	log.Logger.Info("MqType = " + config.Type)
	var mqConnection MqBus
	if config.Type == "mangos_mq" {
		mangosMq := &MangosMqConnection{PublisherListenUrl: publisherListenUrl, SubscriberClientUrl: subscriberConnectUrl,
			RequestListenUrl: requestListenUrl, RequestClientUrl: requestConnectUrl}
		mqConnection = mangosMq
	} else if config.Type == "rabbit_mq" {
		rabbitMq := &RabbitMqConnection{AmqpServerURL: config.RabbitMq.ServerUrl}
		mqConnection = rabbitMq
	} else if config.Type == "redis_mq" {
		redisMq := &RedisMqConnection{HostIp: config.RedisMq.HostPort, DbId: config.RedisMq.DbId, Password: config.RedisMq.Password}
		mqConnection = redisMq
	} else {
		log.Logger.Errorf("Invalid mqtype: %s", config.Type)
		os.Exit(1)
	}
	return mqConnection
}

func MqNotifyNodeConnected(m MqBus, hostName string) error {
	notify := GetMqNotifyNodeConnectionChange_Message(hostName, NotifyMsg_NodeConnected)
	jsonString, _ := JsonMarshallString(notify)

	return m.MqMessagePublishRetry(MqChannelName_Notify, jsonString)
}

func MqNotifyNodeDisconnected(m MqBus, hostName string) error {
	notify := GetMqNotifyNodeConnectionChange_Message(hostName, NotifyMsg_NodeDisconnected)
	jsonString, _ := JsonMarshallString(notify)

	return m.MqMessagePublishRetry(MqChannelName_Notify, jsonString)
}

func MqNotifyClientConnected(m MqBus, hostName string, connInfo *svc.ConnectionInfo) error {
	notify := GetMqNotifyClientConnectionChange_Message(hostName, connInfo, NotifyMsg_ClientConnected)
	jsonString, _ := JsonMarshallString(notify)

	return m.MqMessagePublishRetry(MqChannelName_Notify, jsonString)
}

func MqNotifyClientDisconnected(m MqBus, hostName string, connInfo *svc.ConnectionInfo) error {
	notify := GetMqNotifyClientConnectionChange_Message(hostName, connInfo, NotifyMsg_ClientDisconnected)
	jsonString, _ := JsonMarshallString(notify)

	return m.MqMessagePublishRetry(MqChannelName_Notify, jsonString)
}

func GetMqNotifyNodeConnectionChange_Message(hostName string, notifyType string) mqmodels.MqNotifyConnectionChange {
	return mqmodels.MqNotifyConnectionChange{
		QueuedTime: helpers.GenerateDateNowMs(),
		ServerNode: hostName,
		NotifyType: notifyType,
		//RemoteAddr: ""/*serviceState.Context.LocalAddress*/,
		//NetworkId: "",
	}
}

func GetMqNotifyClientConnectionChange_Message(hostName string, connInfo *svc.ConnectionInfo, notifyType string) mqmodels.MqNotifyConnectionChange {
	return mqmodels.MqNotifyConnectionChange{
		QueuedTime: helpers.GenerateDateNowMs(),
		ServerNode: hostName,
		NotifyType: notifyType,
		RemoteAddr: connInfo.RemoteAddr,
		NetworkId:  connInfo.NetworkId,
	}
}

func JsonMarshallString(body any) (string, error) {
	jsonBy, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return string(jsonBy), nil
}

func MqCreateMessageEnvelope(hostName string, networkId string, body any) (string, error) {
	mqMsgEnvelope := mqmodels.MqMessageEnvelope{
		MessageTime: helpers.GenerateDateNowMs(),
		ServerNode:  hostName,
		Client:      networkId,
		Body:        body,
	}
	jsonString, err := JsonMarshallString(mqMsgEnvelope)

	return jsonString, err
}
