package main

import (
	"encoding/json"
	db "sw/ocpp/csms/internal/db"
	log "sw/ocpp/csms/internal/logging"
	mqmodels "sw/ocpp/csms/internal/models/mq"
	mq "sw/ocpp/csms/internal/mq"
	ocppmodels "sw/ocpp/csms/internal/ocpp"
	"time"
)

func ProcessRecvMessage(messageBy []byte, state any) {
	// Unmarshell to a MqMessageEnvelope message
	msgEnvelope := new(mqmodels.MqMessageEnvelope)
	err := json.Unmarshal(messageBy, &msgEnvelope)
	if err != nil {
		log.Logger.Errorf("MQ Received Message, unmarshall error: %s\n", err.Error())
		return
	}

	// e.g {"serverNode":"dell1234","client":"charger-id1","messageTime":"2024-03-15T10:48:10.637Z",
	// "body":{"direction":2,"msgId":"008633423e9f415cb98dcbcc0d0a63ff","messageType":"SecurityEventNotification",
	// "messageBody":{"timestamp":"2024-03-15T10:48:10Z","type":"SettingSystemTime"}}}
	//log.Logger.Infof("MQ Received Message: %s\n", msgEnvelope.Body)

	if msgEnvelope.Body == nil {
		return
	}
	ocppEnvelopeFields := msgEnvelope.Body.(map[string]interface{})
	direction := int(ocppEnvelopeFields["direction"].(float64))
	if direction != ocppmodels.OcppDirection_ClientServer {
		return
	}
	msgType := ocppEnvelopeFields["messageType"].(string)

	// TODO StopTransaction
	if msgType != "StartTransaction" {
		return
	}

	log.Logger.Debugf("MQ Received MessagesOut: %s\n", string(messageBy))

	msgId := ocppEnvelopeFields["msgId"].(string)

	timeStarted, err := time.Parse("2006-01-02T15:04:05.000Z", msgEnvelope.MessageTime)
	if err != nil {
		log.Logger.Errorf("Unable to parse message time: {%s} - {%s}", msgEnvelope.MessageTime, err.Error())
	}

	transResponse := new(ocppmodels.OcppTransactionResponse)
	transactionId, err := db.InsertNextTransaction(msgEnvelope.Client, timeStarted)
	if err != nil {
		_log.Errorf("Error inserting transation: %s", err.Error())
		transResponse.IdTagInfo.Status = "error"
	} else {
		transResponse.TransactionId = *transactionId
		transResponse.IdTagInfo.Status = "accepted"
	}

	ocppResponse := new(ocppmodels.OcppMessageResponse)
	ocppResponse.MsgId = msgId
	ocppResponse.Direction = ocppmodels.OcppDirection_Reply

	transResponseBy, err := json.Marshal(&transResponse)
	if err != nil {
		_log.Errorf("Error marshalling response: %s", err.Error())
		return
	}
	transResponseRaw := json.RawMessage(transResponseBy)
	ocppResponse.MessageBody = transResponseRaw

	json, _ := mq.MqCreateMessageEnvelope(msgEnvelope.ServerNode, msgEnvelope.Client, ocppResponse)

	mqErr := _serviceState.MqBus.MqMessagePublishRetry(mq.MqChannelName_MessagesOut, json)
	if mqErr != nil {
		_log.Errorf("Error sending reply to MQ, msg lost: %s", mqErr.Error())
		//return mqErr // transient MQ error unrecoverable, close connection to CP
		// TODO log to appinsights
	}
}
