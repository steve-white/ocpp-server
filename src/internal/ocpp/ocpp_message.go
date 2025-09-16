// Provides functions to get OCPP messages by type
package ocpp

import (
	"encoding/json"
	"fmt"

	helpers "sw/ocpp/csms/internal/helpers"

	"github.com/google/uuid"
)

func GetAck(correlationId string) string {
	json, _ := WrapEvent(MsgType_ServerToClientResult, correlationId, "")
	return json
}

func GetHeatBeatAck(eventId string) string {
	hbEvent := OcppHeartBeatAck{CurrentTime: helpers.GenerateDateNowMs()}
	json, _ := WrapEvent(MsgType_ServerToClientResult, eventId, hbEvent)

	return json
}

func GenerateUniqueId() string {
	return uuid.New().String()
}

func WrapEvent(direction int, eventId string, obj any) (string, error) {
	jsonBy, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("[%d, \"%s\", %s]", direction, eventId, string(jsonBy)), nil
}
