package config

import (
	log "github.com/sirupsen/logrus"
	"time"
)

type GlobalConfiguration struct {
	logLevel       log.Level
	httpPort       int
	updateInterval time.Duration
}

func (self *GlobalConfiguration) LogLevel() log.Level {
	return self.logLevel
}

func (self *GlobalConfiguration) HttpPort() int {
	return self.httpPort
}

func (self *GlobalConfiguration) UpdateInterval() time.Duration {
	return self.updateInterval
}