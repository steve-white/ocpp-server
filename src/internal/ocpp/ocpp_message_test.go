package ocpp

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// Mocking the helpers.GenerateDateNowMs function
func mockGenerateDateNowMs() string {
	return "2024-09-27T08:59:59.000Z"
}

func TestGetAck(t *testing.T) {
	correlationId := "test-correlation-id"
	expected := "[2, \"test-correlation-id\", \"\"]"

	result := GetAck(correlationId)

	assert.Equal(t, expected, result)
}

func TestGetHeatBeatAck(t *testing.T) {
	// Mock the GenerateDateNowMs function
	//originalGenerateDateNowMs := helpers.GenerateDateNowMs
	//helpers.GenerateDateNowMs = mockGenerateDateNowMs
	//defer func() { helpers.GenerateDateNowMs = originalGenerateDateNowMs }()

	eventId := "test-event-id"
	hbEvent := OcppHeartBeatAck{CurrentTime: mockGenerateDateNowMs()}
	hbEventJson, _ := json.Marshal(hbEvent)
	expected := "[2, \"test-event-id\", " + string(hbEventJson) + "]"

	result := GetHeatBeatAck(eventId)

	assert.Equal(t, expected, result)
}

func TestGenerateUniqueId(t *testing.T) {
	result := GenerateUniqueId()

	_, err := uuid.Parse(result)
	assert.NoError(t, err)
}

func TestWrapEvent(t *testing.T) {
	direction := 2
	eventId := "test-event-id"
	obj := map[string]string{"key": "value"}
	objJson, _ := json.Marshal(obj)
	expected := "[2, \"test-event-id\", " + string(objJson) + "]"

	result, err := WrapEvent(direction, eventId, obj)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}
