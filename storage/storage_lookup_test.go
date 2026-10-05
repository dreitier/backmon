package storage

import (
	"testing"

	"github.com/dreitier/backmon/backup"
	fs "github.com/dreitier/backmon/storage/fs"
)

// withClients swaps the package-level clients map for the duration of a test
// and restores it afterwards so tests do not leak global state.
func withClients(t *testing.T, replacement map[string]*clientData) {
	t.Helper()
	original := clients
	clients = replacement
	t.Cleanup(func() { clients = original })
}

func sampleDisk() *DiskData {
	fileDef := &backup.FileDefinition{Alias: "file"}
	dir := &backup.Directory{
		Alias: "dir",
		Files: []*backup.FileDefinition{fileDef},
	}
	return &DiskData{
		Name:     "disk1",
		SafeName: "disk1",
		Definition: &backup.Definition{
			Directories: []*backup.Directory{dir},
		},
		groups: []map[string][]*fs.FileInfo{
			{
				"groupA": {{Name: "backup-a.tar"}},
			},
		},
	}
}

func TestFindDisk(t *testing.T) {
	disk := sampleDisk()
	withClients(t, map[string]*clientData{
		"env": {Disks: map[string]*DiskData{"disk1": disk}},
	})

	if got := FindDisk("disk1"); got != disk {
		t.Errorf("FindDisk(disk1) = %v, want the sample disk", got)
	}
	if got := FindDisk("missing"); got != nil {
		t.Errorf("FindDisk(missing) = %v, want nil", got)
	}
}

func TestGetDisks(t *testing.T) {
	withClients(t, map[string]*clientData{
		"env": {Disks: map[string]*DiskData{"disk1": sampleDisk()}},
	})

	disks := GetDisks()
	if len(disks) != 1 {
		t.Fatalf("GetDisks() returned %d disks, want 1", len(disks))
	}
	if disks[0].Name != "disk1" {
		t.Errorf("disk name = %q, want disk1", disks[0].Name)
	}
}

func TestGetDisksEmpty(t *testing.T) {
	withClients(t, map[string]*clientData{})
	if disks := GetDisks(); len(disks) != 0 {
		t.Errorf("GetDisks() = %d, want 0", len(disks))
	}
}

func TestGetFilenames(t *testing.T) {
	withClients(t, map[string]*clientData{
		"env": {Disks: map[string]*DiskData{"disk1": sampleDisk()}},
	})

	names := GetFilenames("disk1", "dir", "file")
	if len(names) != 1 || names[0] != "groupA" {
		t.Errorf("GetFilenames = %v, want [groupA]", names)
	}
}

func TestGetFilenamesUnknown(t *testing.T) {
	withClients(t, map[string]*clientData{
		"env": {Disks: map[string]*DiskData{"disk1": sampleDisk()}},
	})

	cases := []struct {
		name            string
		disk, dir, file string
	}{
		{"unknown disk", "nope", "dir", "file"},
		{"unknown directory", "disk1", "nope", "file"},
		{"unknown file", "disk1", "dir", "nope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if names := GetFilenames(tc.disk, tc.dir, tc.file); names != nil {
				t.Errorf("GetFilenames = %v, want nil", names)
			}
		})
	}
}

// Download must report an error when the requested file cannot be resolved.
func TestDownloadUnknownFile(t *testing.T) {
	withClients(t, map[string]*clientData{
		"env": {Disks: map[string]*DiskData{"disk1": sampleDisk()}},
	})

	_, length, _, err := Download("disk1", "dir", "nope", "groupA")
	if err == nil {
		t.Error("expected an error for an unknown file")
	}
	if length != -1 {
		t.Errorf("length = %d, want -1 on error", length)
	}
}
