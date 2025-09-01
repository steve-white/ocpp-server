package main

import (
	"io"

	conf "sw/ocpp/csms/internal/config"
	svc "sw/ocpp/csms/internal/models/service"
	mq "sw/ocpp/csms/internal/mq"

	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"

	"github.com/puzpuzpuz/xsync/v3"
)

type ServiceState struct {
	Config          *conf.Configuration
	IoCloser        *io.Closer
	Cache           *redis.Client
	MqBus           mq.MqBus
	Connections     *xsync.Map
	LastError       error
	Context         svc.ServiceContext
	AppInsightsHook logrus.Hook
}
