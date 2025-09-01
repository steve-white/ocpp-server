package config

import (
	"fmt"
	"os"
	"path/filepath"

	log "sw/ocpp/csms/internal/logging"

	"github.com/spf13/viper"
)

func LogCwd() {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	cwd := filepath.Dir(ex)

	log.Logger.Info("CWD: " + cwd)
}

func ReadConfig() *Configuration {

	LogCwd()
	viper.SetConfigFile("../cfg/conf.yaml")
	viper.SetConfigType("yaml")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Logger.Error("No config file: ", err.Error()) //ignore, it can be either cli params, or conf file
	}

	var config Configuration
	if err := viper.Unmarshal(&config); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return &config
}
