package main

import (
	"io"

	conf "sw/ocpp/csms/internal/config"
	mq "sw/ocpp/csms/internal/mq"

	"github.com/go-redis/redis"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/sirupsen/logrus"
)

type ServiceState struct {
	Config          *conf.Configuration
	IoCloser        *io.Closer
	Cache           *redis.Client
	MqBus           mq.MqBus
	Connections     *xsync.Map
	LastError       error
	Context         ServiceContext
	AppInsightsHook logrus.Hook
	MessagesWaiting *xsync.Map
}

type ServiceContext struct {
	HostName string
}
