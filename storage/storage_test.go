package storage

import (
	"sort"
	"testing"
	"time"

	fs "github.com/dreitier/backmon/storage/fs"
)

// gatherDirUsageStats must recurse into subdirectories and accumulate both the
// object count and the summed size across the whole tree.
func TestGatherDirUsageStats(t *testing.T) {
	root := &fs.DirectoryInfo{
		Name: "root",
		Files: []*fs.FileInfo{
			{Name: "a", Size: 100},
			{Name: "b", Size: 200},
		},
		SubDirs: map[string]*fs.DirectoryInfo{
			"sub": {
				Name: "sub",
				Files: []*fs.FileInfo{
					{Name: "c", Size: 50},
				},
				SubDirs: map[string]*fs.DirectoryInfo{
					"deep": {
						Name:  "deep",
						Files: []*fs.FileInfo{{Name: "d", Size: 1}},
					},
				},
			},
			"empty": {Name: "empty"},
		},
	}

	count, size := gatherDirUsageStats(root, 0, 0)

	if count != 4 {
		t.Errorf("object count = %d, want 4", count)
	}
	if size != 351 {
		t.Errorf("size sum = %d, want 351", size)
	}
}

func TestGatherDirUsageStatsEmptyDir(t *testing.T) {
	count, size := gatherDirUsageStats(&fs.DirectoryInfo{Name: "empty"}, 0, 0)
	if count != 0 || size != 0 {
		t.Errorf("empty dir should yield 0/0, got %d/%d", count, size)
	}
}

// FileGroup implements sort.Interface such that the most recent file sorts
// first (descending by Time).
func TestFileGroupSortsNewestFirst(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	group := FileGroup{
		{Time: base.Add(1 * time.Hour), File: &fs.FileInfo{Name: "middle"}},
		{Time: base.Add(2 * time.Hour), File: &fs.FileInfo{Name: "newest"}},
		{Time: base, File: &fs.FileInfo{Name: "oldest"}},
	}

	if group.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", group.Len())
	}

	sort.Sort(group)

	wantOrder := []string{"newest", "middle", "oldest"}
	for i, want := range wantOrder {
		if group[i].File.Name != want {
			t.Errorf("position %d = %q, want %q", i, group[i].File.Name, want)
		}
	}
}

func TestFileGroupLessAndSwap(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	group := FileGroup{
		{Time: base.Add(time.Hour), File: &fs.FileInfo{Name: "later"}},
		{Time: base, File: &fs.FileInfo{Name: "earlier"}},
	}

	// index 0 is later in time, so it should sort before index 1
	if !group.Less(0, 1) {
		t.Error("Less(0,1) = false, want true (newer sorts first)")
	}
	if group.Less(1, 0) {
		t.Error("Less(1,0) = true, want false")
	}

	group.Swap(0, 1)
	if group[0].File.Name != "earlier" || group[1].File.Name != "later" {
		t.Errorf("Swap did not exchange elements: %q, %q", group[0].File.Name, group[1].File.Name)
	}
}
