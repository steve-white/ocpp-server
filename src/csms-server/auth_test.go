package main

import (
	"github.com/stretchr/testify/mock"
)

// Mocking the ServiceState and its dependencies
type MockServiceState struct {
	Config MockConfig
}

type MockConfig struct {
	Services MockServices
}

type MockServices struct {
	CsmsServer MockCsmsServer
}

type MockCsmsServer struct {
	EnableAuth bool
}

// Mocking the telemetry
type MockTelemetry struct {
	mock.Mock
}

func (m *MockTelemetry) TrackAuthenticationEvent(networkId, remoteAddr, status string) {
	m.Called(networkId, remoteAddr, status)
}

var telemetryT = &MockTelemetry{}

/* TODO
func TestAuthConnection(t *testing.T) {
	// Arrange
	// Mock telemetry
	telemetryT.On("TrackAuthenticationEvent", mock.Anything, mock.Anything, mock.Anything).Return()

	serviceState := &MockServiceState{
		Config: MockConfig{
			Services: MockServices{
				CsmsServer: MockCsmsServer{
					EnableAuth: true,
				},
			},
		},
	}

	// Act
	result, networkId := AuthConnection(rw, req, serviceState)

	// Assert
	assert.Equal(t, tt.expectedResult, result)
	assert.Equal(t, tt.expectedId, networkId)
	assert.Equal(t, tt.expectedStatus, rw.Result().StatusCode)
}*/
