package table

import "github.com/Azure/azure-sdk-for-go/sdk/data/aztables"

// Table Storage Messages entity
type TableMessageEntity struct {
	aztables.Entity
	ServerNode  string `json:"serverNode"`
	Direction   string `json:"direction"`
	MessageTime string `json:"messageTime"`
	Body        string `json:"body"`
}
