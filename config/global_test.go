package config

import (
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func Test_GlobalConfiguration_accessors(t *testing.T) {
	assertion := assert.New(t)

	g := &GlobalConfiguration{
		logLevel:       log.WarnLevel,
		httpPort:       8080,
		updateInterval: 30 * time.Second,
	}

	assertion.Equal(log.WarnLevel, g.LogLevel())
	assertion.Equal(8080, g.HttpPort())
	assertion.Equal(30*time.Second, g.UpdateInterval())
}
