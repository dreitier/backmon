package main

import (
	"flag"
	"os"
	"os/signal"
	"strings"
	"sync"
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

var (
	// reloadMutex serializes reloads triggered by SIGHUP and the reload key
	reloadMutex sync.Mutex
	// updateTicker schedules the periodic disk updates; a reload resets it if `update_interval` has changed
	updateTicker *time.Ticker
)

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
	metrics.GetApplicationMetrics().ConfigReloaded(true)

	storage.InitializeConfiguration()
	scheduleDiskUpdates()

	err := web.StartServer()

	// #12: restore the terminal before printing the error. termbox.Close() clears the screen
	// (https://github.com/nsf/termbox-go/issues/233), so a message printed earlier would be lost,
	// and without Close() the console stays scrambled.
	// termbox is only initialized in interactive mode; on Windows, Close() does not check that itself
	if termbox.IsInit {
		termbox.Close()
	}
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

// handleReloadKey reloads the configuration and forces an update of all disks
func handleReloadKey() {
	log.Printf("Forcing reload...")
	reload()
}

// reload reads the configuration file again and updates all disks. An invalid configuration file is
// rejected: backmon keeps running with the configuration it has, and the disks are updated nevertheless.
func reload() {
	reloadMutex.Lock()
	defer reloadMutex.Unlock()

	if reloadConfiguration() {
		log.Info("Configuration reloaded")
	}

	storage.UpdateDiskInfo()
}

// reloadConfiguration applies everything of a changed configuration file which doesn't need a restart
// and returns whether the file could be loaded
func reloadConfiguration() bool {
	previous, current, err := config.Reload()
	metrics.GetApplicationMetrics().ConfigReloaded(err == nil)

	if err != nil {
		log.Errorf("Could not reload configuration, keeping the active one: %v", err)
		return false
	}

	if !config.HasGlobalDebugEnabled() {
		log.SetLevel(current.Global().LogLevel())
	}

	if restart := config.RequiresRestart(previous, current); len(restart) > 0 {
		log.Warnf("Changed configuration of %s is only applied after a restart", strings.Join(restart, ", "))
	}

	storage.ApplyConfiguration(current.Environments())
	metrics.GetApplicationMetrics().EnvironmentsTotal.Set(current.TotalEnvironments())

	if updateTicker != nil && previous.Global().UpdateInterval() != current.Global().UpdateInterval() {
		log.Infof("Updating disks every %s from now on", current.Global().UpdateInterval())
		updateTicker.Reset(current.Global().UpdateInterval())
	}

	return true
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
			reload()
		}
	}()
}

func scheduleDiskUpdates() {
	updateInterval := config.GetInstance().Global().UpdateInterval()

	// a SIGHUP may already be handled while backmon is starting
	reloadMutex.Lock()
	updateTicker = time.NewTicker(updateInterval)
	ticker := updateTicker
	reloadMutex.Unlock()

	go func() {
		storage.UpdateDiskInfo()
		for range ticker.C {
			storage.UpdateDiskInfo()
		}
	}()
}
