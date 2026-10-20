package config

import (
	"time"

	log "github.com/sirupsen/logrus"
)

type GlobalConfiguration struct {
	logLevel       log.Level
	httpPort       int
	updateInterval time.Duration
}

func (config *GlobalConfiguration) LogLevel() log.Level {
	return config.logLevel
}

func (config *GlobalConfiguration) HttpPort() int {
	return config.httpPort
}

func (config *GlobalConfiguration) UpdateInterval() time.Duration {
	return config.updateInterval
}
