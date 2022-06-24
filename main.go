package main

import (
	"github.com/dreitier/cloudmon/config"
	"github.com/dreitier/cloudmon/storage"
	"github.com/dreitier/cloudmon/web"
	log "github.com/sirupsen/logrus"
	"time"
)

func main() {
	if !config.HasGlobalDebugEnabled() {
		logLevel := config.GetInstance().Global().LogLevel()

		log.SetLevel(logLevel)
	}

	scheduleDiskUpdates()
	web.StartServer()
}

func scheduleDiskUpdates() {
	updateInterval := config.GetInstance().Global().UpdateInterval()

	ticker := time.NewTicker(updateInterval)
	go func() {
		storage.UpdateDiskInfo()
		for range ticker.C {
			storage.UpdateDiskInfo()
		}
	}()
}
