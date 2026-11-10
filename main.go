package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	// embed the time zone database: `timezone` in backup definitions must also work on Windows and in minimal images
	_ "time/tzdata"

	termbox "github.com/nsf/termbox-go"
	log "github.com/sirupsen/logrus"

	"github.com/dreitier/backmon/config"
	"github.com/dreitier/backmon/metrics"
	"github.com/dreitier/backmon/storage"
	"github.com/dreitier/backmon/web"
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

	err := web.StartServer()

	// #12: restore the terminal before printing the error. termbox.Close() clears the screen
	// (https://github.com/nsf/termbox-go/issues/233), so a message printed earlier would be lost,
	// and without Close() the console stays scrambled.
	termbox.Close()
	log.Fatalf("Webserver stopped: %v", err)
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

	// start goroutine to continuously poll the keyboard (platform-specific, see terminal_*.go)
	go pollKeyboard()
}

// handleReloadKey forces an update of all disks
func handleReloadKey() {
	log.Printf("Forcing reload...")
	storage.UpdateDiskInfo()
}

// handleExitKey restores the terminal and exits
func handleExitKey() {
	log.Printf("Exiting...")
	termbox.Close()
	os.Exit(0)
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
