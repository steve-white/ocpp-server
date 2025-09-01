package helpers

import (
	"fmt"
	"os"
)

func GetHostName() string {
	hostName, err := os.Hostname()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return hostName
}
