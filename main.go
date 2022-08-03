package main

import (
	"github.com/dreitier/cloudmon/config"
	"github.com/dreitier/cloudmon/storage"
	"github.com/dreitier/cloudmon/web"
	log "github.com/sirupsen/logrus"
	"time"
	"fmt"
	"os"
	termbox "github.com/nsf/termbox-go"
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
	configureLogrus()
	configureTerminal()
	printVersion()

	if !config.HasGlobalDebugEnabled() {
		logLevel := config.GetInstance().Global().LogLevel()
		log.SetLevel(logLevel)
	}

	storage.InitializeConfiguration()
	scheduleDiskUpdates()

	web.StartServer()
}

func configureTerminal() {
	if config.IsRunningInBackgroundForced() {
		return
	}

	// #7: allow manual refreshing of disks
	// set up termbox, @see https://github.com/nsf/termbox-go/blob/master/_demos/raw_input.go
	err := termbox.Init()

	if err != nil {
		log.Warnf("Unable to run in interactive mode: %s", err)
		return
	}

	// start goroutine to continuously poll the keyboard
	go func() {
		for {
			var current string
			var data [64]byte

			// we have to poll the raw events; normal events don't include escape sequences
			switch ev := termbox.PollRawEvent(data[:]); ev.Type {
			case termbox.EventRaw:
				d := data[:ev.N]
				current = fmt.Sprintf("%q", d)

				// handle disk refresh
				if current == `"\x12"` /* Ctrl+R */ || current == `"r"` {
					log.Printf("Forcing reload...")
					storage.UpdateDiskInfo()
				// handlq exiting
				} else if current == `"\x1b"` /* ESC */ || current == `"q"` || current == `"\x03"` {
					log.Printf("Exiting...")
					termbox.Close()
					os.Exit(0)
				}
			case termbox.EventError:
				panic(ev.Err)
			}
		}
	}()
}

func configureLogrus() {
	customFormatter := new(log.TextFormatter)
    customFormatter.TimestampFormat = "2022-08-02 20:22:05"
    customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)
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
