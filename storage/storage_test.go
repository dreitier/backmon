package storage

import (
	"errors"
	"io"
	"testing"

	"github.com/dreitier/backmon/backup"
	"github.com/dreitier/backmon/metrics"
	fs "github.com/dreitier/backmon/storage/fs"
)

type fakeClient struct{}

func (f *fakeClient) GetDiskNames() ([]string, error) { return nil, nil }

func (f *fakeClient) GetFileNames(string, uint64) (*fs.DirectoryInfo, error) { return nil, nil }

func (f *fakeClient) Download(string, *fs.FileInfo) (io.ReadCloser, int64, string, error) {
	return nil, -1, "", errors.New("not implemented")
}

func (f *fakeClient) Delete(string, *fs.FileInfo) error { return nil }

func withDisk(t *testing.T, disk *DiskData) {
	clients["test-env"] = &clientData{
		Client: &fakeClient{},
		Disks:  map[string]*DiskData{disk.Name: disk},
	}

	t.Cleanup(func() {
		disk.metrics.Drop()
		delete(clients, "test-env")
	})
}

func TestApiAccess_diskWithoutDefinitions_doesNotPanic(t *testing.T) {
	withDisk(t, &DiskData{Name: "no-definitions", metrics: metrics.NewDisk("no-definitions")})

	definition, found := GetDefinition("no-definitions")

	if !found || definition != nil {
		t.Errorf("expected disk to be found without definitions, got found=%t definition=%v", found, definition)
	}

	if names := GetFilenames("no-definitions", "dir", "file"); names != nil {
		t.Errorf("expected no file names, got %v", names)
	}

	if _, _, _, err := Download("no-definitions", "dir", "file", "group"); err == nil {
		t.Error("expected an error when downloading from a disk without definitions")
	}
}

func TestDownload_unknownGroup_returnsError(t *testing.T) {
	file := &backup.FileDefinition{Alias: "file"}
	disk := &DiskData{
		Name:    "with-definitions",
		metrics: metrics.NewDisk("with-definitions"),
		Definition: &backup.Definition{
			Directories: []*backup.Directory{{Alias: "dir", Files: []*backup.FileDefinition{file}}},
		},
		groups: []map[string][]*fs.FileInfo{
			{"known-group": {&fs.FileInfo{Name: "dump.sql"}}},
		},
	}
	withDisk(t, disk)

	if _, _, _, err := Download("with-definitions", "dir", "file", "unknown-group"); err == nil {
		t.Error("expected an error when downloading an unknown group")
	}

	client, fileInfo := findDownload("with-definitions", "dir", "file", "known-group")

	if client == nil || fileInfo == nil || fileInfo.Name != "dump.sql" {
		t.Errorf("expected known group to resolve, got client=%v file=%v", client, fileInfo)
	}
}
