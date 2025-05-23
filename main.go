package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dreitier/backmon/config"
	"github.com/dreitier/backmon/metrics"
	"github.com/dreitier/backmon/storage"
	"github.com/dreitier/backmon/web"
	termbox "github.com/nsf/termbox-go"
	log "github.com/sirupsen/logrus"
)

const app = "backmon"

var gitRepo = "dreitier/backmon"
var gitCommit = "unknown"
var gitTag = "unknown"

func printVersion() {
	if gitTag == "" {
		gitTag = "err-no-git-tag"
	}

	log.Printf("%s (dist=%s; version=%s; commit=%s)", app, gitRepo, gitTag, gitCommit)
}

func main() {
	flag.Parse()
	configureLogger()
	configureTerminal()
	configureSignals()
	printVersion()

	if !config.HasGlobalDebugEnabled() {
		logLevel := config.GetInstance().Global().LogLevel()
		log.SetLevel(logLevel)
	}

	// #13: update number of total environments
	metrics.GetApplicationMetrics().EnvironmentsTotal.Set(config.GetInstance().TotalEnvironments())

	storage.InitializeConfiguration()
	scheduleDiskUpdates()

	// #12: in case of an error during webserver startup (e.g. missing certificate or privat key), the console output gets scrambled.
	// this is because of @see https://github.com/nsf/termbox-go/issues/233. If we use a `defer termbox.Close()`, the whole output would be swallowed.
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
					// handle exiting
				} else if current == `"\x1b"` /* ESC */ || current == `"q"` || current == `"\x03"` {
					log.Printf("Exiting...")
					termbox.Close()
					os.Exit(0)
				}
			case termbox.EventError:
				panic(ev.Err)
			default:
				log.Tracef("unhandled default case %v", ev)
			}
		}
	}()
}

func configureLogger() {
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = time.RFC3339
	customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)
}

func configureSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)

	go func() {
		for range c {
			log.Printf("Got HUP signal, reloading ...")
			storage.UpdateDiskInfo()
		}
	}()
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
