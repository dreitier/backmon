package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dreitier/backmon/metrics"
)

const testDefinitions = `---
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
`

// useLocalDisk registers the local disk of the test configuration (see TestMain) with the given files
// (path relative to the disk root -> content) and returns the name of the disk
func useLocalDisk(t *testing.T, files map[string]string) string {
	t.Helper()

	if err := os.RemoveAll(testDiskRoot); err != nil {
		t.Fatal(err)
	}

	for name, content := range files {
		path := filepath.Join(testDiskRoot, name)

		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.MkdirAll(testDiskRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	withClients(t, map[string]*clientData{})
	InitializeConfiguration()

	// the metrics of a disk are registered globally, so they must not outlive the test
	registered := clients
	t.Cleanup(func() {
		for _, client := range registered {
			client.dropAllDisks()
		}
	})

	return testDiskRoot
}

func onlyDisk(t *testing.T) *DiskData {
	t.Helper()

	disks := GetDisks()
	if len(disks) != 1 {
		t.Fatalf("got %d disks, want 1", len(disks))
	}

	return disks[0]
}

func TestInitializeConfiguration_registersConfiguredEnvironments(t *testing.T) {
	useLocalDisk(t, nil)

	client, exists := clients["local"]
	if !exists {
		t.Fatalf("environment 'local' is not registered, got %v", clients)
	}

	if client.DefinitionFilename != "backup_definitions.yaml" || client.Client == nil {
		t.Errorf("unexpected client data: %+v", client)
	}

	if len(GetDisks()) != 0 {
		t.Error("disks must only be discovered by UpdateDiskInfo")
	}
}

func TestUpdateDiskInfo_scansFilesOfLocalDisk(t *testing.T) {
	diskName := useLocalDisk(t, map[string]string{
		"backup_definitions.yaml":   testDefinitions,
		"backups/dump-20220715.sql": "old",
		"backups/dump-20220718.sql": "newest",
		"backups/dump-20220716.sql": "older",
		"backups/unrelated.txt":     "ignored",
	})

	UpdateDiskInfo()

	disk := onlyDisk(t)
	if disk.Name != diskName {
		t.Errorf("disk name = %q, want %q", disk.Name, diskName)
	}

	if disk.Definition == nil || len(disk.Definition.Directories) != 1 {
		t.Fatalf("definitions have not been parsed: %+v", disk.Definition)
	}

	groups := GetFilenames(diskName, "my-backups", "pgdump")
	if len(groups) != 1 {
		t.Fatalf("got groups %v, want exactly one", groups)
	}

	latest := disk.groups[0][groups[0]][0]
	if latest == nil || latest.Name != "dump-20220718.sql" {
		t.Errorf("latest file = %+v, want dump-20220718.sql", latest)
	}

	if GetFilenames(diskName, "my-backups", "unknown") != nil {
		t.Error("expected no groups for an unknown file")
	}
}

func TestUpdateDiskInfo_keepsUnchangedDefinitions(t *testing.T) {
	useLocalDisk(t, map[string]string{
		"backup_definitions.yaml":   testDefinitions,
		"backups/dump-20220715.sql": "data",
	})

	UpdateDiskInfo()
	first := onlyDisk(t).Definition

	UpdateDiskInfo()

	if second := onlyDisk(t).Definition; first == nil || second != first {
		t.Error("unchanged definitions must not be parsed again")
	}
}

func TestUpdateDiskInfo_picksUpNewFiles(t *testing.T) {
	diskName := useLocalDisk(t, map[string]string{
		"backup_definitions.yaml":   testDefinitions,
		"backups/dump-20220715.sql": "data",
	})

	UpdateDiskInfo()

	newFile := filepath.Join(diskName, "backups", "dump-20220801.sql")
	if err := os.WriteFile(newFile, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	UpdateDiskInfo()

	disk := onlyDisk(t)
	groups := GetFilenames(diskName, "my-backups", "pgdump")

	if len(groups) != 1 {
		t.Fatalf("got groups %v, want exactly one", groups)
	}

	if latest := disk.groups[0][groups[0]][0]; latest == nil || latest.Name != "dump-20220801.sql" {
		t.Errorf("latest file = %+v, want dump-20220801.sql", latest)
	}
}

func TestUpdateDiskInfo_diskWithoutDefinitions(t *testing.T) {
	useLocalDisk(t, map[string]string{"backups/dump-20220715.sql": "data"})

	UpdateDiskInfo()

	if disk := onlyDisk(t); disk.Definition != nil {
		t.Errorf("expected no definitions, got %+v", disk.Definition)
	}
}

func TestUpdateDiskInfo_diskWithInvalidDefinitions(t *testing.T) {
	useLocalDisk(t, map[string]string{"backup_definitions.yaml": "directories: [this is not a map"})

	UpdateDiskInfo()

	if disk := onlyDisk(t); disk.Definition != nil {
		t.Errorf("expected no definitions, got %+v", disk.Definition)
	}
}

func TestUpdateDiskInfo_ignoresDiskWithBackmonignore(t *testing.T) {
	useLocalDisk(t, map[string]string{
		"backup_definitions.yaml": testDefinitions,
		".backmonignore":          "",
	})

	UpdateDiskInfo()

	if disks := GetDisks(); len(disks) != 0 {
		t.Errorf("got %d disks, want the disk to be ignored", len(disks))
	}
}

func TestUpdateDiskInfo_dropsDiskWhichAppearsInBackmonignoreLater(t *testing.T) {
	diskName := useLocalDisk(t, map[string]string{"backup_definitions.yaml": testDefinitions})

	UpdateDiskInfo()
	onlyDisk(t)

	if err := os.WriteFile(filepath.Join(diskName, ".backmonignore"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	UpdateDiskInfo()

	if disks := GetDisks(); len(disks) != 0 {
		t.Errorf("got %d disks, want the disk to be dropped", len(disks))
	}
}

func TestUpdateDiskInfo_dropsDisksRemovedOnTheClient(t *testing.T) {
	// mockClient reports no disks at all
	client := &clientData{
		Client: &mockClient{},
		Disks:  map[string]*DiskData{"gone": {Name: "gone", metrics: metrics.NewDisk("gone")}},
	}
	withClients(t, map[string]*clientData{"env": client})

	UpdateDiskInfo()

	if len(client.Disks) != 0 {
		t.Errorf("got disks %v, want the removed disk to be dropped", client.Disks)
	}
}
