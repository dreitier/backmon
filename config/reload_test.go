package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

const reloadTestConfig = `port: 8000
update_interval: 1h
environments:
  first:
    path: /mnt/first
`

// useConfigurationFile makes the given content the configuration loaded at startup and returns the path of
// the file, so that a test can change it before calling Reload
func useConfigurationFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), CfgFileName)
	writeConfigurationFile(t, path, content)

	raw, err := ParseFromString(content)
	if err != nil {
		t.Fatal(err)
	}

	// GetInstance must not search for a configuration file of its own
	once.Do(func() {})

	originalInstance, originalPath := instance.Load(), loadedConfigPath
	t.Cleanup(func() {
		instance.Store(originalInstance)
		loadedConfigPath = originalPath
	})

	instance.Store(NewConfigurationInstance(raw))
	loadedConfigPath = path

	return path
}

func writeConfigurationFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func Test_Reload_activatesChangedConfiguration(t *testing.T) {
	assertion := assert.New(t)
	path := useConfigurationFile(t, reloadTestConfig)

	writeConfigurationFile(t, path, `port: 8000
update_interval: 5m
log_level: warn
http:
  basic_auth:
    username: alice
    password: changeme
downloads:
  enabled: true
environments:
  first:
    path: /mnt/first
  second:
    s3:
      region: eu-west-1
`)

	previous, current, err := Reload()

	assertion.NoError(err)
	assertion.NotSame(previous, current)
	assertion.Same(current, GetInstance())

	assertion.Equal(time.Hour, previous.Global().UpdateInterval())
	assertion.Equal(float64(1), previous.TotalEnvironments())

	assertion.Equal(5*time.Minute, current.Global().UpdateInterval())
	assertion.Equal(log.WarnLevel, current.Global().LogLevel())
	assertion.NotNil(current.Http().BasicAuth)
	assertion.True(current.Downloads().Enabled)
	assertion.Equal(float64(2), current.TotalEnvironments())
	assertion.Empty(RequiresRestart(previous, current))
}

func Test_Reload_keepsActiveConfigurationIfFileIsRejected(t *testing.T) {
	cases := map[string]string{
		"invalid YAML":            "environments:\n  first:\n  path: [unterminated\n",
		"empty file":              "",
		"no environments":         "port: 8000\n",
		"one invalid environment": "environments:\n  first:\n    path: /mnt/first\n  broken:\n    s3:\n      proxy: not-a-url\n",
	}

	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			assertion := assert.New(t)
			path := useConfigurationFile(t, reloadTestConfig)
			active := GetInstance()

			writeConfigurationFile(t, path, content)

			previous, current, err := Reload()

			assertion.Error(err)
			assertion.Same(active, previous)
			assertion.Same(active, current)
			assertion.Same(active, GetInstance())
		})
	}
}

func Test_Reload_keepsActiveConfigurationIfFileIsMissing(t *testing.T) {
	assertion := assert.New(t)
	path := useConfigurationFile(t, reloadTestConfig)
	active := GetInstance()

	assertion.NoError(os.Remove(path))

	_, _, err := Reload()

	assertion.Error(err)
	assertion.Same(active, GetInstance())
}

func Test_RequiresRestart_listsSettingsOfTheListener(t *testing.T) {
	assertion := assert.New(t)
	path := useConfigurationFile(t, reloadTestConfig)

	writeConfigurationFile(t, path, `port: 9000
http:
  tls:
    certificate: /etc/backmon/tls.crt
    key: /etc/backmon/tls.key
environments:
  first:
    path: /mnt/first
`)

	previous, current, err := Reload()

	assertion.NoError(err)
	assertion.Equal([]string{"port", "http.tls"}, RequiresRestart(previous, current))
	// the changed values are active nevertheless, they describe the desired state
	assertion.Equal(9000, GetInstance().Global().HttpPort())
}
