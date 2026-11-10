package web

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dreitier/backmon/storage"
)

const (
	testUsername = "alice"
	testPassword = "changeme"
)

// testDiskName is the name (= directory) of the local disk served by the API during the tests
var testDiskName string

var testDiskFiles = map[string]string{
	"backup_definitions.yaml": `---
directories:
  backups:
    alias: my-backups
    defaults:
      schedule: 0 2 * * *
      retention-count: 10
      retention-age: 7d
    files:
      dump-%Y%M%D.sql:
        alias: pgdump
        sort: interpolation
`,
	"backups/dump-20220715.sql": "old dump",
	"backups/dump-20220718.sql": "newest dump",
}

// TestMain provides a configuration file and one scanned local disk: config.GetInstance() reads ./config.yaml
// exactly once per process, so the working directory has to point to it before the router is created.
func TestMain(m *testing.M) {
	os.Exit(runWithConfiguration(m))
}

func runWithConfiguration(m *testing.M) int {
	dir, err := os.MkdirTemp("", "backmon-web-test")
	if err != nil {
		panic(err)
	}

	defer func() { _ = os.RemoveAll(dir) }()

	testDiskName = filepath.Join(dir, "disk")

	cfg := fmt.Sprintf(`http:
  basic_auth:
    username: %s
    password: %s
downloads:
  enabled: true
environments:
  local:
    path: %s
`, testUsername, testPassword, testDiskName)

	files := map[string]string{"config.yaml": cfg}
	for name, content := range testDiskFiles {
		files[filepath.Join("disk", name)] = content
	}

	for name, content := range files {
		path := filepath.Join(dir, name)

		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			panic(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			panic(err)
		}
	}

	if err := os.Chdir(dir); err != nil {
		panic(err)
	}

	storage.InitializeConfiguration()
	storage.UpdateDiskInfo()

	return m.Run()
}
