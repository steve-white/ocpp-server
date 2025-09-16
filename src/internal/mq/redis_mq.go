package mq

import (
	"fmt"
	"os"
	log "sw/ocpp/csms/internal/logging"
	svc "sw/ocpp/csms/internal/models/service"
	"time"

	"errors"

	"github.com/go-redis/redis"
)

type RedisMqConnection struct {
	HostIp   string
	Password string
	DbId     int

	clientRedis   *redis.Client
	topicReceiver *redis.PubSub
}

func (r *RedisMqConnection) Close() error {
	log.Logger.Info("Close redis MQ: ", r.HostIp)

	return r.clientRedis.Close()
}

func (r *RedisMqConnection) MqConnect() error {
	log.Logger.Info(fmt.Sprintf("Connecting to redis MQ: %s DbId: %d Password: %s", r.HostIp, r.DbId, r.Password))

	client := redis.NewClient(&redis.Options{
		Addr:     r.HostIp,
		Password: r.Password,
		DB:       r.DbId,
	})

	pong, err := client.Ping().Result()
	if err != nil {
		log.Logger.Error("Error in redis connection: ", err.Error())
		return err
	}
	log.Logger.Info("Connected to redis...")
	log.Logger.Info(pong, err)
	r.clientRedis = client

	return nil
}

func (r *RedisMqConnection) MqMessagePublish(channelName string, json string) error {
	return r.clientRedis.Publish(channelName, json).Err()
}

func (r *RedisMqConnection) MqQueueMessagePublish(channelName string, json string) error {
	return r.clientRedis.LPush(channelName, json).Err()
}

func (r *RedisMqConnection) SetupMqTopicReceiver(channelName string, routingKey string) error {
	r.topicReceiver = r.clientRedis.Subscribe(channelName)
	return nil
}

func (r *RedisMqConnection) RunMqTopicReceiver(ProcessRecvMqMessage func(messageBy []byte, state any), topicName string, state any) error {
	for {
		// TODO use a channel instead of ReceiveMessage
		msg, err := r.topicReceiver.ReceiveMessage()
		if err != nil {
			log.Logger.Errorf("Error receiving message: %s\n", err.Error())
		}
		//log.Logger.Debugf("msg: %s\n", msg.Payload)

		ProcessRecvMqMessage([]byte(msg.Payload), state)
	}
}

func (r *RedisMqConnection) RunMqQueueReceiver(ProcessRecvMqMessage func(messageBy []byte, state any), topicName string, state any) error {
	for {
		timeout := time.Duration(5 * float64(time.Second))
		pipe := r.clientRedis.Pipeline()
		var result *redis.StringSliceCmd
		for {
			result = pipe.BRPop(timeout, topicName)
			_, err := pipe.Exec()
			if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
				log.Logger.Errorf("Error receiving message: %s\n", result.Err().Error())
			}
			if len(result.Val()) > 0 {
				break
			}
		}

		//log.Logger.Debugf("msg: %s\n", result.Val()[1])

		ProcessRecvMqMessage([]byte(result.Val()[1]), state)
	}
}

func (r *RedisMqConnection) MqSendClientMessageRetry(hostName string, connInfo *svc.ConnectionInfo, body any) error {

	json, err := MqCreateMessageEnvelope(hostName, connInfo.NetworkId, body)
	if err != nil {
		return err
	}

	return r.MqMessagePublishRetry(MqChannelName_MessagesIn, json)
}

func (r *RedisMqConnection) MqMessagePublishRetry(channel string, json string) error {
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

func (r RedisMqConnection) MqQueueDeclare(queueName string) error {
	// NOOP
	return nil
}
