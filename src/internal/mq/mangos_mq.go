package mq

import (
	"fmt"
	"strings"
	log "sw/ocpp/csms/internal/logging"
	svc "sw/ocpp/csms/internal/models/service"
	"time"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/pub"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	"go.nanomsg.org/mangos/v3/protocol/req"
	"go.nanomsg.org/mangos/v3/protocol/sub"

	// register transports
	_ "go.nanomsg.org/mangos/v3/transport/tcp"
)

type MangosMqConnection struct {
	PublisherListenUrl  string
	SubscriberClientUrl string
	RequestListenUrl    string
	RequestClientUrl    string

	SockPubListener     mangos.Socket
	SockSubClient       mangos.Socket
	SockRequestListener mangos.Socket
	SockRequestClient   mangos.Socket
}

func (r *MangosMqConnection) Close() error {
	log.Logger.Info("Close mangos_mq")

	if r.SockPubListener != nil {
		r.SockPubListener.Close()
	}
	if r.SockSubClient != nil {
		r.SockSubClient.Close()
	}
	if r.SockRequestListener != nil {
		r.SockRequestListener.Close()
	}
	if r.SockRequestClient != nil {
		r.SockRequestClient.Close()
	}
	return nil
}

func (r *MangosMqConnection) MqConnect() error {
	var err error

	// Publisher listen socket. Listen for subscribers...
	if r.PublisherListenUrl != "" {
		log.Logger.Info(fmt.Sprintf("Listening on MQ URL for subscribers: %s", r.PublisherListenUrl))
		var pubSock mangos.Socket
		if pubSock, err = pub.NewSocket(); err != nil {
			log.Logger.Errorf("can't get new pub socket: %s", err)
			return err
		}

		err = pubSock.Listen(r.PublisherListenUrl)
		if err != nil {
			log.Logger.Errorf("can't listen on pub socket: %s", err.Error())
			return err
		}
		r.SockPubListener = pubSock
	}

	// Subscriber client socket. Connect to a publisher socket...
	if r.SubscriberClientUrl != "" {

		log.Logger.Info(fmt.Sprintf("Connecting to MQ publisher URL: %s", r.SubscriberClientUrl))
		var subSock mangos.Socket
		if subSock, err = sub.NewSocket(); err != nil {
			log.Logger.Errorf("can't get new pub socket: %s", err)
			return err
		}

		for {
			err := subSock.Dial(r.SubscriberClientUrl)
			if err != nil {
				log.Logger.Errorf("can't dial on sub socket: %s", err.Error())
			} else {
				log.Logger.Info(fmt.Sprintf("Connected to MQ URL: %s", r.SubscriberClientUrl))
				break
			}

			time.Sleep(1000 * time.Millisecond)
		}

		r.SockSubClient = subSock
	}

	// Request listener socket. Listen for clients...
	if r.RequestListenUrl != "" {
		log.Logger.Info(fmt.Sprintf("Listening on MQ URL for requests: %s", r.RequestListenUrl))
		var requestListenSock mangos.Socket
		if requestListenSock, err = rep.NewSocket(); err != nil {
			log.Logger.Errorf("can't get new reqSock socket: %s", err)
			return err
		}

		err = requestListenSock.Listen(r.RequestListenUrl)
		if err != nil {
			log.Logger.Errorf("can't listen on reqSock socket: %s", err.Error())
			return err
		}
		r.SockRequestListener = requestListenSock
	}

	// Request client socket. Connect to a requests socket...
	if r.RequestClientUrl != "" {
		log.Logger.Info(fmt.Sprintf("Connecting to MQ requests URL: %s", r.RequestClientUrl))
		var requestClientSock mangos.Socket
		if requestClientSock, err = req.NewSocket(); err != nil {
			log.Logger.Errorf("can't get new requests socket: %s", err)
			return err
		}

		for {
			err := requestClientSock.Dial(r.RequestClientUrl)
			if err != nil {
				log.Logger.Errorf("can't dial on requests socket: %s", err.Error())
			} else {
				log.Logger.Info(fmt.Sprintf("Connected to requests URL: %s", r.RequestClientUrl))
				break
			}

			time.Sleep(1000 * time.Millisecond)
		}

		r.SockRequestClient = requestClientSock
	}
	return nil
}

func (r *MangosMqConnection) MqMessagePublish(channelName string, json string) error {
	log.Logger.Debugf(fmt.Sprintf("MQ[%s] send: %s", channelName, json))
	if r.SockPubListener != nil {
		if err := r.SockPubListener.Send([]byte(channelName + "|" + json)); err != nil {
			log.Logger.Errorf("Failed publishing to pub socket: %s", err.Error())
			return err
		}
	}

	if r.SockRequestClient != nil {
		if err := r.SockRequestClient.Send([]byte(channelName + "|" + json)); err != nil {
			log.Logger.Errorf("Failed publishing to requests client socket: %s", err.Error())
			return err
		}
	}

	return nil
}

func (r *MangosMqConnection) MqQueueMessagePublish(channelName string, json string) error {
	// not implemented
	return nil
}

func (r *MangosMqConnection) SetupMqTopicReceiver(channelName string, routingKey string) error {
	if r.SockSubClient == nil {
		return nil
	}
	log.Logger.Debugf("MQ subscribe: %s", channelName)

	err := r.SockSubClient.SetOption(mangos.OptionSubscribe, []byte(channelName))
	if err != nil {
		log.Logger.Errorf("cannot subscribe: %s", err.Error())
		return err
	}

	return nil
}

func (r *MangosMqConnection) RunMqTopicReceiver(ProcessRecvMqMessage func(messageBy []byte, state any), topicName string, state any) error {
	if r.SockSubClient != nil {
		go ReceiveMqTopicMessages(r.SockSubClient, ProcessRecvMqMessage, topicName, state)
	}
	if r.SockRequestListener != nil {
		go ReceiveMqTopicMessages(r.SockRequestListener, ProcessRecvMqMessage, topicName, state)
	}
	return nil
}

func ReceiveMqTopicMessages(socket mangos.Socket, ProcessRecvMqMessage func(messageBy []byte, state any), topicName string, state any) error {
	for {
		var msgBy []byte
		var err error
		if msgBy, err = socket.Recv(); err != nil {
			log.Logger.Errorf("cannot receive: %s", err.Error())
			return err
		}

		// Remove MQ pre-amble e.g: channelName|{"message"}
		str := string(msgBy)
		idx := strings.Index(str, "|")
		if idx > -1 {
			newStr := str[idx+1:]
			log.Logger.Debugf("MQ[%s] recv: %s", topicName, newStr)
			ProcessRecvMqMessage([]byte(newStr), state)
		}
		_ = socket.Send([]byte{}) // ACK for REQ/REP
	}
}

func (r *MangosMqConnection) RunMqQueueReceiver(ProcessRecvMqMessage func(messageBy []byte, state any), topicName string, state any) error {
	// Not implemented
	return nil
}

func (r *MangosMqConnection) MqSendClientMessageRetry(hostName string, connInfo *svc.ConnectionInfo, body any) error {

	json, err := MqCreateMessageEnvelope(hostName, connInfo.NetworkId, body)
	if err != nil {
		return err
	}

	return r.MqMessagePublishRetry(MqChannelName_MessagesIn, json)
}

func (r *MangosMqConnection) MqMessagePublishRetry(channel string, json string) error {
	var mqErr error
	retry := 0

	// Try recover from transient errors
	for {
		mqErr = r.MqMessagePublish(channel, json)
		if mqErr == nil {
			break
		} else {
			if retry == MqChannel_SendMaxRetries {
				log.Logger.Errorf("MQ[%s] error, failed to send message after %d retries. Error: %s, message: %s", channel, MqChannel_SendMaxRetries, mqErr.Error(), json)
				return mqErr
			}
			retry++
			log.Logger.Warnf("MQ[%s] problem, wait: %dms, retry %d/%d, error: %s", channel, MqChannel_SendRetryWaitMs, retry, MqChannel_SendMaxRetries, mqErr.Error())
			time.Sleep(MqChannel_SendRetryWaitMs * time.Millisecond)
		}
	}
	return nil
}

func (r MangosMqConnection) MqQueueDeclare(queueName string) error {
	// NOOP
	return nil
}
