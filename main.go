package main

import (
	"github.com/dreitier/cloudmon/config"
	"github.com/dreitier/cloudmon/storage"
	"github.com/dreitier/cloudmon/web"
	log "github.com/sirupsen/logrus"
	"time"
)

func main() {
	logLevel := config.GetInstance().Global().LogLevel()

	log.SetLevel(logLevel)

	scheduleBucketUpdates()
	web.StartServer()
}

func scheduleBucketUpdates() {
	updateInterval := config.GetInstance().Global().UpdateInterval()

	ticker := time.NewTicker(updateInterval)
	go func() {
		storage.UpdateBucketInfo()
		for range ticker.C {
			storage.UpdateBucketInfo()
		}
	}()
}
