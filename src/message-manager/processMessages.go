package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	log "sw/ocpp/csms/internal/logging"
	mqmodels "sw/ocpp/csms/internal/models/mq"
	tablemodels "sw/ocpp/csms/internal/models/table"
	table "sw/ocpp/csms/internal/table"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
)

func ProcessRecvMessage(messageBy []byte, state any) {
	serviceState := state.(*ServiceState)

	if !serviceState.Config.Services.MessageManager.StoreMessages {
		return
	}

	// Unmarshell to a MqMessageEnvelope message
	msgEnvelope := new(mqmodels.MqMessageEnvelope)
	err := json.Unmarshal(messageBy, &msgEnvelope)
	if err != nil {
		log.Logger.Errorf("MQ Received Message, unmarshall error: %s\n", err.Error())
		return
	}

	entity := aztables.Entity{
		PartitionKey: msgEnvelope.Client,
		RowKey:       strconv.FormatInt(time.Now().UnixMilli(), 10),
		Timestamp:    aztables.EDMDateTime(time.Now()),
	}
	log.Logger.Debugf("Add message partitionKey/rowKey: %s %s\n", entity.PartitionKey, entity.RowKey)

	bodyJson, err := json.Marshal(msgEnvelope.Body)
	if err != nil {
		log.Logger.Errorf("Error: %s", err.Error())
	}

	ocppEnvelopeFields := msgEnvelope.Body.(map[string]interface{})
	Direction := fmt.Sprintf("%d", int(ocppEnvelopeFields["direction"].(float64)))

	tableEntity := tablemodels.TableMessageEntity{
		entity,
		msgEnvelope.ServerNode,
		Direction,
		msgEnvelope.MessageTime,
		string(bodyJson),
	}

	// Add MQ received message to table storage
	_, err = table.AddEntity[tablemodels.TableMessageEntity](serviceState.TableClient, tableEntity)
	if err != nil {
		log.Logger.Errorf("Error: %s", err.Error())
	}
}
