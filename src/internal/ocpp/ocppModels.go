package ocpp

import (
	"encoding/json"
	"time"
)

// [3,"a5663aa99f9645988a7a41b53c81a780",{"currentTime":"2023-01-13T09:58:14.920Z"}]
type OcppMessage struct {
	Direction   int             `json:"direction,omitempty"`
	MsgId       string          `json:"msgId,omitempty"`
	MessageType string          `json:"messageType,omitempty"`
	MessageBody json.RawMessage `json:"messageBody,omitempty"`
}

type OcppMessageResponse struct {
	Direction   int             `json:"direction,omitempty"`
	MsgId       string          `json:"msgId,omitempty"`
	MessageBody json.RawMessage `json:"messageBody,omitempty"`
}

// TODO move to abstraction
type ActionResponse struct {
	StatusCode  int             `json:"statusCode,omitempty"`
	MsgId       string          `json:"msgId,omitempty"`
	MessageBody json.RawMessage `json:"messageBody,omitempty"`
}

type OcppCurrentTime struct {
	CurrentTime time.Time `json:"currentTime,omitempty"`
}

type OcppBootNotification struct {
	ChargePointVendor       string `json:"chargePointVendor,omitempty"`
	ChargePointModel        string `json:"chargePointModel,omitempty"`
	ChargePointSerialNumber string `json:"chargePointSerialNumber,omitempty"`
	FirmwareVersion         string `json:"firmwareVersion,omitempty"`
	MeterType               string `json:"meterType,omitempty"`
	MeterSerialNumber       string `json:"meterSerialNumber,omitempty"`
	Iccid                   string `json:"iccid,omitempty"`
}

type OcppBootNotificationResponse struct {
	Status      string `json:"status,omitempty"`
	CurrentTime string `json:"currentTime,omitempty"`
	Interval    int    `json:"interval,omitempty"`
}

const (
	OcppDirection_ServerClient = 1
	OcppDirection_ClientServer = 2
	OcppDirection_Reply        = 3
)

const (
	BootStatus_Accepted = "Accepted"
	BootStatus_Rejected = "Rejected"
	BootStatus_Invalid  = "Invalid"
)

type OcppAuthorize struct {
	IdTag string `json:"idTag,omitempty"`
}

type OcppHeartBeatAck struct {
	CurrentTime string `json:"currentTime,omitempty"`
}

type OcppChangeAvailability struct {
	ConnectorId int    `json:"connectorId,omitempty"`
	Type        string `json:"type,omitempty"`
}

// OCPP MessageType
const (
	MsgType_ClientToServer       = 2
	MsgType_ServerToClientResult = 3
	MsgType_Error                = 4
)

// GenericResponseStatus
const (
	Respond_Error      = -1
	Respond_NoResponse = 0
	Respond_Accepted   = 1
	Respond_Rejected   = 2
)

// --- Configuration ---

type OcppGetConfiguration struct {
	Key []string `json:"key,omitempty"`
}

type OcppGetConfigurationResponse struct {
	ConfigurationKey []OcppKeyValue `json:"configurationKey,omitempty"`
	UnknownKey       []string       `json:"unknownKey,omitempty"`
}

type OcppKeyValue struct {
	Key      string `json:"key,omitempty"`
	Readonly bool   `json:"readonly,omitempty"`
	Value    string `json:"value,omitempty"`
}

// ConfigurationStatus
const (
	Config_Error          = -1
	Config_NoResponse     = 0
	Config_Accepted       = 1
	Config_Rejected       = 2
	Config_RebootRequired = 3
	Config_NotSupported   = 4
)

// MeterValues
/* TODO
type OcppMeterValue struct {
	Timestamp string `json:"timestamp,omitempty"`
	SampledValue []SampledValue `json:"sampledValue,omitempty"`
}

type OcppMeterValues struct {
	ConnectorId int `json:"connectorId,omitempty"`
	TransactionId int `json:"transactionId,omitempty"`
	MeterValue []MeterValue `json:"meterValue,omitempty"`
}*/

// SecurityEventNotification
type OcppSecurityEventNotification struct {
	Timestamp string `json:"timestamp,omitempty"`
	Type      string `json:"type,omitempty"`
}

const (
	SecurityEvent_StartupOfTheDevice = "StartupOfTheDevice"
	SecurityEvent_SettingSystemTime  = "SettingSystemTime"
)

// StatusNotification
type OcppStatusNotification struct {
	ConnectorId int    `json:"connectorId,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
	Type        string `json:"type,omitempty"`

	ErrorCode string `json:"errorCode,omitempty"`
	Status    string `json:"status,omitempty"`

	Info            string `json:"info,omitempty"`
	VendorId        string `json:"vendorId,omitempty"`
	VendorErrorCode string `json:"vendorErrorCode,omitempty"`
}

const (
	Status_Available     = "Available"
	Status_Preparing     = "Preparing"
	Status_Charging      = "Charging"
	Status_Finishing     = "Finishing"
	Status_SuspendedEvse = "SuspendedEVSE"
	Status_Unavailable   = "Unavailable"
	Status_Faulted       = "Faulted"
)

const (
	StatusError_ConnectorLockFailure = "ConnectorLockFailure"
	StatusError_EVCommunicationError = "EVCommunicationError"
	StatusError_GroundFailure        = "GroundFailure"
	StatusError_HighTemperature      = "HighTemperature"
	StatusError_InternalError        = "InternalError"
	StatusError_LocalListConflict    = "LocalListConflict"
	StatusError_NoError              = "NoError"
	StatusError_OtherError           = "OtherError"
	StatusError_OverCurrentFailure   = "OverCurrentFailure"
	StatusError_OverVoltage          = "OverVoltage"
	StatusError_PowerMeterFailure    = "PowerMeterFailure"
	StatusError_PowerSwitchFailure   = "PowerSwitchFailure"
	StatusError_ReaderFailure        = "ReaderFailure"
	StatusError_ResetFailure         = "ResetFailure"
	StatusError_UnderVoltage         = "UnderVoltage"
	StatusError_WeakSignal           = "WeakSignal"
)

const (
	MsgType_DataTransfer           = "DataTransfer"
	MsgType_SetChargingProfile     = "SetChargingProfile"
	MsgType_RemoteStartTransaction = "RemoteStartTransaction"
	MsgType_RemoteStopTransaction  = "RemoteStopTransaction"
	MsgType_ClearChargingProfile   = "ClearChargingProfile"
	MsgType_UnlockConnector        = "UnlockConnector"
	MsgType_Reset                  = "Reset"
	MsgType_GetDiagnostics         = "GetDiagnostics"
	MsgType_GetConfiguration       = "GetConfiguration"
	MsgType_ChangeAvailability     = "ChangeAvailability"
	MsgType_ChangeConfiguration    = "ChangeConfiguration"
	MsgType_TriggerMessage         = "TriggerMessage"
)

// Transactions
type OcppStartTransaction struct {
	Timestamp   string `json:"timestamp,omitempty"`
	ConnectorId int    `json:"connectorId,omitempty"`

	IdTag      string `json:"idTag,omitempty"`
	MeterStart int    `json:"meterStart,omitempty"`
}

type OcppStopTransaction struct {
	Timestamp     string `json:"timestamp,omitempty"`
	TransactionId int    `json:"transactionId,omitempty"`
	IdTag         string `json:"idTag,omitempty"`

	MeterStop int    `json:"meterStop,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type IdTagInfo struct {
	Status string `json:"status,omitempty"`
}

type OcppTransactionResponse struct {
	TransactionId int64     `json:"transactionId,omitempty"`
	IdTagInfo     IdTagInfo `json:"idTagInfo,omitempty"`
}

// ChargingProfiles
// TODO

// ChargerActions
// TODO

type OcppDataTransfer struct {
	VendorId  string `json:"vendorId,omitempty"`
	MessageId string `json:"messageId,omitempty"`
	Data      string `json:"data,omitempty"`
}
