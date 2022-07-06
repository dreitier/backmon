package main

import (
	"github.com/dreitier/cloudmon/config"
	"github.com/dreitier/cloudmon/storage"
	"github.com/dreitier/cloudmon/web"
	log "github.com/sirupsen/logrus"
	"time"
)

const app = "cloudmon"
var gitRepo = "dreitier/cloudmon"
var gitCommit = "unknown"
var gitTag = "unknown"

func printVersion() {
	if gitTag == "" {
		gitTag = "err-no-git-tag"
	}

	log.Printf("%s (dist=%s; version=%s; commit=%s)", app, gitRepo, gitTag, gitCommit)
}

func main() {
	printVersion()
	
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
