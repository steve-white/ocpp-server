// Provides OCPP auth mechanism using auth records stored in redis
package main

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	telemetry "sw/ocpp/csms/internal/telemetry"

	"github.com/go-redis/redis"
)

var (
	PREFIX_CONNSTATE string = "CS_"
	PREFIX_CPAUTH    string = "CP_"
)

func AuthConnection(rw http.ResponseWriter, req *http.Request, serviceState *ServiceState) (bool, string) {
	networkId, err := toNetworkId(req.URL.Path)
	if err != nil {
		_log.Warn("Invalid networkId passed, return 404...: ", err.Error())
		rw.WriteHeader(http.StatusNotFound)
		return false, ""
	}
	_log.Debug("networkId: ", networkId)

	if !serviceState.Config.Services.CsmsServer.EnableAuth {
		_log.Debug("networkId OK, auth is disabled...")
		return true, networkId
	}

	isValid := authNetworkId(networkId, serviceState)
	if !isValid {
		_log.Warn("networkId invalid, return 404...")
		rw.WriteHeader(http.StatusNotFound)
		telemetry.TrackAuthenticationEvent(networkId, req.RemoteAddr, "401")
		return false, ""
	}
	_log.Debug("networkId OK")
	telemetry.TrackAuthenticationEvent(networkId, req.RemoteAddr, "200")

	return true, networkId
}

func authNetworkId(networkId string, serviceState *ServiceState) bool {
	key := PREFIX_CPAUTH + networkId

	cache := serviceState.Cache

	response := cache.Get(key)
	if response.Err() == redis.Nil {
		_log.Error("Not found: ", key)
		return false
	}
	if response.Err() != nil {
		_log.Error("Cache error: ", response.Err())
		return false
	}
	val := response.Val()

	_log.Debug("Found auth = ", val)

	/*wasSet := cache.MSet(PREFIX_CONNSTATE+networkId, "", 10*time.Second)
	if wasSet.Val() {
		return true
	}*/

	return true
}

func toNetworkId(urlPath string) (string, error) {
	idx := strings.LastIndex(urlPath, "/")
	if idx+1 >= len(urlPath) {
		return "", errors.New("no networkId")
	}

	networkId := urlPath[idx+1:]

	networkId = truncateText(networkId, NETWORKID_MAXLEN)
	if !validNetworkIdString(networkId) {
		return "", errors.New("invalid characters in networkId")
	}
	return networkId, nil
}

func validNetworkIdString(networkId string) bool {
	isValid := regexp.MustCompile(`^[A-Za-z0-9\-]+$`).MatchString
	return isValid(networkId)
}

func truncateText(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}
