// Provides cache interaction functions
package cache

import (
	log "sw/ocpp/csms/internal/logging"

	"github.com/go-redis/redis"
)

func ConnectRedis(hostIp string, password string, dbId int) (*redis.Client, error) {
	log.Logger.Info("Connect to redis: ", hostIp)

	client := redis.NewClient(&redis.Options{
		Addr:     hostIp,
		Password: password,
		DB:       dbId,
	})

	pong, err := client.Ping().Result()
	if err != nil {
		log.Logger.Error("Error in redis connection: ", err.Error())
		return nil, err
	}
	log.Logger.Info("Connected to redis...")
	log.Logger.Info(pong, err)
	return client, nil
}
