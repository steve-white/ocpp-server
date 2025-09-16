package mq

import (
	"fmt"
	log "sw/ocpp/csms/internal/logging"
	svc "sw/ocpp/csms/internal/models/service"
	"time"

	"github.com/streadway/amqp"
)

type RabbitMqConnection struct {
	AmqpServerURL   string
	ChannelRabbitMQ *amqp.Channel
}

const (
	MqChannel_SendMaxRetries  = 3
	MqChannel_SendRetryWaitMs = 2000
	MqChannel_PollWaitMs      = 1000
)

func (r *RabbitMqConnection) Close() error {
	log.Logger.Info("Close RabbitMQ: ", r.AmqpServerURL)

	return r.ChannelRabbitMQ.Close()
}

func (r *RabbitMqConnection) MqConnect() error {

	log.Logger.Infof("Connect to RabbitMQ: %s", r.AmqpServerURL)

	// Create a new RabbitMQ connection.
	connectRabbitMQ, err := amqp.Dial(r.AmqpServerURL)
	if err != nil {
		log.Logger.Errorf("Error connecting to RabbitMQ: %s %s", r.AmqpServerURL, error(err))
		return err
	}
	//defer connectRabbitMQ.Close()

	// Open a channel to RabbitMQ
	channelRabbitMQ, err := connectRabbitMQ.Channel()
	if err != nil {
		return err
	}
	//defer channelRabbitMQ.Close()

	log.Logger.Debug("Connected to RabbitMQ")
	r.ChannelRabbitMQ = channelRabbitMQ
	return nil
}

func (r *RabbitMqConnection) MqQueueDeclare(queueName string) error {
	log.Logger.Debugf("Declare queue: %s", queueName)

	// Declare queue that we can pub/sub to
	_, err := r.ChannelRabbitMQ.QueueDeclare(
		queueName, // queue name
		true,      // durable
		false,     // auto delete
		false,     // exclusive
		false,     // no wait
		nil,       // arguments
	)
	if err != nil {
		return err
	}
	return nil
}

func (r *RabbitMqConnection) SetupMqTopicReceiver(topicName string, routingKey string) error {
	channel := r.ChannelRabbitMQ
	errExchange := channel.ExchangeDeclare(
		topicName, // name
		"topic",   // type
		false,     // durable
		false,     // auto-deleted
		false,     // internal
		false,     // no-wait
		nil,       // arguments
	)

	if errExchange != nil {
		log.Logger.Errorf("MQ[%s] Error in ExchangeDeclare: %s\n", topicName, errExchange)
		return errExchange
	}
	log.Logger.Debugf("MQ[%s] init routingKey=%s, exchange=%s\n", topicName, routingKey, topicName)

	q, errDeclare := channel.QueueDeclare(
		"",    // name
		false, // durable
		false, // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,   // arguments
	)

	if errDeclare != nil {
		log.Logger.Errorf("MQ[%s] Error in QueueDeclare: %s\n", topicName, errDeclare)
		return errDeclare
	}

	errBind := channel.QueueBind(
		q.Name,     // queue name
		routingKey, // routing key
		topicName,  // exchange
		false,
		nil)

	if errBind != nil {
		log.Logger.Errorf("MQ[%s] Error in QueueBind: %s\n", topicName, errBind)
		return errBind
	}
	return nil
}

func (r *RabbitMqConnection) MqTopicConsume(queueName string) (<-chan amqp.Delivery, error) {

	messages, err := r.ChannelRabbitMQ.Consume(
		queueName, // queue name
		"",        // consumer
		true,      // auto-ack
		false,     // exclusive
		false,     // no local
		false,     // no wait
		nil,       // arguments
	)

	return messages, err
}

func (m RabbitMqConnection) MqMessagePublish(queueName string, json string) error {
	message := amqp.Publishing{
		ContentType: "application/json",
		Body:        []byte(json),
	}

	log.Logger.Debugf(fmt.Sprintf("MQ[%s] send: %s", queueName, json))
	if err := m.ChannelRabbitMQ.Publish(
		"",        // exchange
		queueName, // queue name
		false,     // mandatory
		false,     // immediate
		message,   // message to publish
	); err != nil {
		return err
	}

	return nil
}

func (m *RabbitMqConnection) MqSendClientMessageRetry(hostName string, connInfo *svc.ConnectionInfo, body any) error {

	json, err := MqCreateMessageEnvelope(hostName, connInfo.NetworkId, body)
	if err != nil {
		return err
	}

	return m.MqMessagePublishRetry(MqChannelName_MessagesIn, json)
}

func (m *RabbitMqConnection) MqMessagePublishRetry(queueName string, json string) error {
	var mqErr error
	retry := 0

	// Try recover from transient errors
	for {
		mqErr = m.MqMessagePublish(queueName, json)
		if mqErr == nil {
			break
		} else {
			if retry == MqChannel_SendMaxRetries {
				log.Logger.Errorf("MQ[%s] error, failed to send message after %d retries. Error: %s, message: %s", queueName, MqChannel_SendMaxRetries, mqErr.Error(), json)
				return mqErr
			}
			retry++

			log.Logger.Warnf("MQ[%s] problem, wait: %dms, retry %d/%d, error: %s", queueName, MqChannel_SendRetryWaitMs, retry, MqChannel_SendMaxRetries, mqErr.Error())
			time.Sleep(MqChannel_SendRetryWaitMs * time.Millisecond)

			m.Close()
			m.MqConnect()
		}
	}
	return nil
}

func (m *RabbitMqConnection) RunMqTopicReceiver(ProcessRecvMqMessage func(messageBy []byte, state any), topicName string, state any) error {
	for {
		messages, topicErr := m.MqTopicConsume(topicName)
		if topicErr != nil {
			log.Logger.Errorf("MQ[%s] Error in Consume: %s\n", topicName, topicErr)
			return topicErr
		}

		for message := range messages {
			log.Logger.Debugf(fmt.Sprintf("MQ[%s] recv: %s", topicName, message.Body))
			ProcessRecvMqMessage(message.Body, state)
		}
		time.Sleep(MqChannel_PollWaitMs * time.Millisecond) // TODO improve this
	}
	// TODO improve error handling. This is called as a goroutine. If there are MQ issues, we need to try reconnect
}
