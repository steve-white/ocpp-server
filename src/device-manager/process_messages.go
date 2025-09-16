package main

import (
	"encoding/json"
	"fmt"

	mqmodels "sw/ocpp/csms/internal/models/mq"
	svc "sw/ocpp/csms/internal/models/service"
	ocppmodels "sw/ocpp/csms/internal/ocpp"
)

func ProcessRecvMessage(messageBy []byte, state any) {
	// Unmarshall to a MqMessageEnvelope message
	msgEnvelope := new(mqmodels.MqMessageEnvelope)
	err := json.Unmarshal(messageBy, &msgEnvelope)
	if err != nil {
		log.Errorf("MQ Received Message, unmarshall error: %s\n", err.Error())
		return
	}

	ocppMessage, err := UnmarshallOcppMessage(msgEnvelope.Body)
	if err != nil {
		log.Errorf("Error unmarshalling ocpp message: %s\n", err.Error())
		return
	}

	err = json.Unmarshal(messageBy, &msgEnvelope)
	if err != nil {
		log.Errorf("MQ Received Message, unmarshall body error: %s\n", err.Error())
		return
	}
	log.Debugf("OcppMessage Response, Direction: %d, Id: %s\n", ocppMessage.Direction, ocppMessage.MsgId)
	if ocppMessage.Direction != 3 {
		return
	}

	val, ok := serviceState.MessagesWaiting.Load(ocppMessage.MsgId)
	if ok {
		msg := val.(*svc.DeviceWaitingMessage)
		msg.Envelope = msgEnvelope
		msg.Response = ocppMessage
		msg.Notify <- 1
	} else {
		log.Errorf("Cannot find MsgId: %s\n", ocppMessage.MsgId)
	}
}

func UnmarshallOcppMessage(msgEnvelopeBody any) (*ocppmodels.OcppMessage, error) {
	jsonData, err := json.Marshal(msgEnvelopeBody)
	if err != nil {
		fmt.Println("Error marshalling struct:", err)
		return nil, err
	}

	msgBody := new(ocppmodels.OcppMessage)
	err = json.Unmarshal(jsonData, &msgBody)
	if err != nil {
		log.Errorf("MQ Received Message, unmarshall error: %s\n", err.Error())
		return nil, err
	}
	return msgBody, nil
}
