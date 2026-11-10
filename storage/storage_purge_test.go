package storage

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/dreitier/backmon/backup"
	fs "github.com/dreitier/backmon/storage/fs"
)

// mockClient records the files passed to Delete and can be configured to fail
// deleting specific file names.
type mockClient struct {
	deleted []string
	failOn  map[string]bool
}

func (m *mockClient) GetDiskNames() ([]string, error) { return nil, nil }
func (m *mockClient) GetFileNames(disk string, maxDepth uint64) (*fs.DirectoryInfo, error) {
	return nil, nil
}
func (m *mockClient) Download(disk string, file *fs.FileInfo) (io.ReadCloser, int64, string, error) {
	return nil, 0, "", nil
}
func (m *mockClient) Delete(disk string, file *fs.FileInfo) error {
	if m.failOn != nil && m.failOn[file.Name] {
		return errors.New("delete failed")
	}
	m.deleted = append(m.deleted, file.Name)
	return nil
}

func newFileGroup(n int) FileGroup {
	base := time.Now().UTC()
	group := make(FileGroup, n)
	for i := 0; i < n; i++ {
		// newest first: index 0 is the most recent
		group[i] = TemporalFile{
			Time: base.Add(-time.Duration(i) * time.Hour),
			File: &fs.FileInfo{Name: "file-" + string(rune('a'+i))},
		}
	}
	return group
}

// When purging is disabled, Purge must return the list untouched regardless of
// how many files exceed the retention count.
func TestPurgeDisabledKeepsEverything(t *testing.T) {
	client := &mockClient{}
	list := newFileGroup(5)
	def := &backup.FileDefinition{Purge: false, RetentionCount: 1, RetentionAge: time.Hour}

	remainder, _ := list.Purge(def, "grp", "disk", client)

	if len(remainder) != 5 {
		t.Errorf("remainder = %d, want 5 (purge disabled)", len(remainder))
	}
	if len(client.deleted) != 0 {
		t.Errorf("deleted %d files, want 0", len(client.deleted))
	}
}

// With purging enabled and more files than the retention count allows, the
// oldest excess files are deleted and dropped from the remainder.
func TestPurgeRemovesExcess(t *testing.T) {
	client := &mockClient{}
	list := newFileGroup(5)
	// retention age far in the past so no file counts as "young"
	def := &backup.FileDefinition{Purge: true, RetentionCount: 2, RetentionAge: time.Nanosecond}

	remainder, young := list.Purge(def, "grp", "disk", client)

	if young != 0 {
		t.Errorf("young = %d, want 0", young)
	}
	if len(remainder) != 2 {
		t.Errorf("remainder = %d, want 2", len(remainder))
	}
	if len(client.deleted) != 3 {
		t.Errorf("deleted = %d, want 3", len(client.deleted))
	}
}

// Young files (within the retention age) must be kept even when they exceed the
// retention count.
func TestPurgeKeepsYoungFiles(t *testing.T) {
	client := &mockClient{}
	list := newFileGroup(4)
	// retention age large enough that all files are younger than the threshold
	def := &backup.FileDefinition{Purge: true, RetentionCount: 1, RetentionAge: 24 * time.Hour}

	remainder, young := list.Purge(def, "grp", "disk", client)

	if young != 4 {
		t.Errorf("young = %d, want 4 (all files within retention age)", young)
	}
	if len(remainder) != 4 {
		t.Errorf("remainder = %d, want 4", len(remainder))
	}
	if len(client.deleted) != 0 {
		t.Errorf("deleted = %d, want 0", len(client.deleted))
	}
}

// A file whose deletion fails must be retained in the remainder.
func TestPurgeKeepsFileWhenDeleteFails(t *testing.T) {
	list := newFileGroup(4)
	// the 4th file (index 3) is the oldest; it is in the excess set
	failing := list[3].File.Name
	client := &mockClient{failOn: map[string]bool{failing: true}}
	def := &backup.FileDefinition{Purge: true, RetentionCount: 2, RetentionAge: time.Nanosecond}

	remainder, _ := list.Purge(def, "grp", "disk", client)

	// 4 files, keep 2, so 2 are excess; one deletion fails and is kept back
	if len(remainder) != 3 {
		t.Errorf("remainder = %d, want 3 (one delete failed)", len(remainder))
	}
	found := false
	for _, f := range remainder {
		if f.File.Name == failing {
			found = true
		}
	}
	if !found {
		t.Errorf("file %q whose delete failed should remain in the list", failing)
	}
}
