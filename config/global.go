package config

import (
	log "github.com/sirupsen/logrus"
	"time"
)

type global struct {
	logLevel       log.Level
	httpPort       int
	updateInterval time.Duration
	ignored        map[string]struct{}
}

func (g *global) LogLevel() log.Level {
	return g.logLevel
}

func (g *global) HttpPort() int {
	return g.httpPort
}

func (g *global) UpdateInterval() time.Duration {
	return g.updateInterval
}

func (g *global) IgnoreBucket(bucket string) bool {
	_, ignored := g.ignored[bucket]
	return ignored
}
