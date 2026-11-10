package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// testDiskRoot is the directory of the local disk configured for the tests of UpdateDiskInfo
var testDiskRoot string

// TestMain provides a configuration file: config.GetInstance() reads ./config.yaml exactly once per process,
// so the working directory has to point to it before the first test touches the configuration.
func TestMain(m *testing.M) {
	os.Exit(runWithConfiguration(m))
}

func runWithConfiguration(m *testing.M) int {
	dir, err := os.MkdirTemp("", "backmon-storage-test")
	if err != nil {
		panic(err)
	}

	defer func() { _ = os.RemoveAll(dir) }()

	testDiskRoot = filepath.Join(dir, "disk")
	cfg := fmt.Sprintf("environments:\n  local:\n    path: %s\n", testDiskRoot)

	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(cfg), 0o600); err != nil {
		panic(err)
	}

	if err := os.Chdir(dir); err != nil {
		panic(err)
	}

	return m.Run()
}
