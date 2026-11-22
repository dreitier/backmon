package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/dreitier/backmon/config"
	"github.com/dreitier/backmon/metrics"
	"github.com/dreitier/backmon/storage"
)

const testDefinitions = `---
directories:
  backups:
    files:
      dump-%Y%M%D.sql:
        sort: interpolation
`

// testDir contains ./config.yaml and the local disks of the tests
var testDir string

// TestMain provides a configuration file: config.GetInstance() searches for ./config.yaml on first use
func TestMain(m *testing.M) {
	os.Exit(runWithConfiguration(m))
}

func runWithConfiguration(m *testing.M) int {
	dir, err := os.MkdirTemp("", "backmon-main-test")
	if err != nil {
		panic(err)
	}

	defer func() { _ = os.RemoveAll(dir) }()

	testDir = dir

	for _, disk := range []string{"first", "second"} {
		path := filepath.Join(dir, disk, "backups")

		if err := os.MkdirAll(path, 0o755); err != nil {
			panic(err)
		}

		if err := os.WriteFile(filepath.Join(dir, disk, "backup_definitions.yaml"), []byte(testDefinitions), 0o600); err != nil {
			panic(err)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, config.CfgFileName), []byte(testConfiguration("first")), 0o600); err != nil {
		panic(err)
	}

	if err := os.Chdir(dir); err != nil {
		panic(err)
	}

	storage.InitializeConfiguration()

	return m.Run()
}

// testConfiguration returns a configuration file with one environment for each of the given local disks
func testConfiguration(disks ...string) string {
	cfg := "update_interval: 1h\nenvironments:\n"

	for _, disk := range disks {
		cfg += fmt.Sprintf("  %s:\n    path: %s\n", disk, filepath.Join(testDir, disk))
	}

	return cfg
}

// writeConfiguration replaces ./config.yaml; the original configuration is reloaded when the test is done
func writeConfiguration(t *testing.T, content string) {
	t.Helper()

	write := func(content string) {
		if err := os.WriteFile(config.CfgFileName, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Cleanup(func() {
		write(testConfiguration("first"))
		reload()
		log.SetLevel(log.InfoLevel)
	})

	write(content)
}

// metric returns the line of the given series as it is served by /metrics
func metric(t *testing.T, series string) string {
	t.Helper()

	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if strings.HasPrefix(line, series+" ") {
			return line
		}
	}

	t.Fatalf("series %s is missing in the metrics:\n%s", series, rec.Body.String())

	return ""
}

func TestReload_appliesChangedConfiguration(t *testing.T) {
	updateTicker = time.NewTicker(time.Hour)
	defer updateTicker.Stop()

	reload()

	if got := len(storage.GetDisks()); got != 1 {
		t.Fatalf("got %d disks, want 1", got)
	}

	writeConfiguration(t, "log_level: warn\n"+strings.Replace(testConfiguration("first", "second"), "1h", "5m", 1))

	reload()

	if got := len(storage.GetDisks()); got != 2 {
		t.Errorf("got %d disks, want the disk of the added environment to be monitored", got)
	}

	if got := metric(t, "backmon_environments_total"); got != "backmon_environments_total 2" {
		t.Errorf("got %q, want 2 environments", got)
	}

	if got := metric(t, "backmon_config_last_reload_successful"); got != "backmon_config_last_reload_successful 1" {
		t.Errorf("got %q, want a successful reload", got)
	}

	if log.GetLevel() != log.WarnLevel {
		t.Errorf("log level = %s, want warn", log.GetLevel())
	}

	if got := config.GetInstance().Global().UpdateInterval(); got != 5*time.Minute {
		t.Errorf("update interval = %s, want 5m", got)
	}
}

func TestReload_removesEnvironment(t *testing.T) {
	writeConfiguration(t, testConfiguration("first", "second"))
	reload()

	writeConfiguration(t, testConfiguration("second"))
	reload()

	disks := storage.GetDisks()

	if len(disks) != 1 || disks[0].Name != filepath.Join(testDir, "second") {
		t.Errorf("got disks %v, want only the disk of the remaining environment", disks)
	}
}

func TestReload_keepsRunningWithActiveConfigurationIfFileIsInvalid(t *testing.T) {
	reload()

	writeConfiguration(t, "environments: [this is not a map\n")

	reload()

	if got := metric(t, "backmon_config_last_reload_successful"); got != "backmon_config_last_reload_successful 0" {
		t.Errorf("got %q, want a failed reload", got)
	}

	if got := len(storage.GetDisks()); got != 1 {
		t.Errorf("got %d disks, want the disk of the active configuration to stay monitored", got)
	}

	if got := metric(t, "backmon_environments_total"); got != "backmon_environments_total 1" {
		t.Errorf("got %q, want 1 environment", got)
	}
}
