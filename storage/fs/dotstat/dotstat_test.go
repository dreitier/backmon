package fs

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	stat "github.com/dreitier/backmon/storage/fs"
)

func TestIsStatFile(t *testing.T) {
	cases := map[string]bool{
		"dump.sql.stat":       true,
		"dump.stat":           true,
		".stat":               true,
		"dump.sql":            false,
		"stat":                false,
		"dump.stat.gz":        false,
		"backups/db.sql.stat": true,
	}

	for name, expected := range cases {
		if got := IsStatFile(name); got != expected {
			t.Errorf("IsStatFile(%q) = %v, want %v", name, got, expected)
		}
	}
}

func TestToDotStatPath(t *testing.T) {
	if got := ToDotStatPath("backups/dump.sql"); got != "backups/dump.sql.stat" {
		t.Errorf("ToDotStatPath = %q, want %q", got, "backups/dump.sql.stat")
	}
}

func TestRemoveDotStatSuffix(t *testing.T) {
	if got := RemoveDotStatSuffix("dump.sql.stat"); got != "dump.sql" {
		t.Errorf("RemoveDotStatSuffix(with suffix) = %q, want %q", got, "dump.sql")
	}

	// A path without the suffix must be returned unchanged.
	if got := RemoveDotStatSuffix("dump.sql"); got != "dump.sql" {
		t.Errorf("RemoveDotStatSuffix(without suffix) = %q, want %q", got, "dump.sql")
	}
}

// writeStatFile creates a .stat file in a temp dir and returns its path.
func writeStatFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "dump.sql.stat")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestApplyDotStatValues_AllFields(t *testing.T) {
	statPath := writeStatFile(t, "born_at: 1000\nmodified_at: 2000\narchived_at: 3000\n")

	file := &stat.FileInfo{Name: "dump.sql", Parent: "backups/db"}
	sources := map[string]string{"backups/db/dump.sql": statPath}

	ApplyDotStatValues(sources, []*stat.FileInfo{file})

	if file.BornAt.Unix() != 1000 {
		t.Errorf("BornAt = %d, want 1000", file.BornAt.Unix())
	}
	if file.ModifiedAt.Unix() != 2000 {
		t.Errorf("ModifiedAt = %d, want 2000", file.ModifiedAt.Unix())
	}
	if file.ArchivedAt.Unix() != 3000 {
		t.Errorf("ArchivedAt = %d, want 3000", file.ArchivedAt.Unix())
	}
}

func TestApplyDotStatValues_PartialFieldsLeaveOthersUntouched(t *testing.T) {
	statPath := writeStatFile(t, "modified_at: 2000\n")

	original := time.Unix(42, 0)
	file := &stat.FileInfo{Name: "dump.sql", Parent: "backups", BornAt: original, ArchivedAt: original}
	sources := map[string]string{"backups/dump.sql": statPath}

	ApplyDotStatValues(sources, []*stat.FileInfo{file})

	if file.ModifiedAt.Unix() != 2000 {
		t.Errorf("ModifiedAt = %d, want 2000", file.ModifiedAt.Unix())
	}
	// Fields not present in the .stat file must keep their original values.
	if !file.BornAt.Equal(original) {
		t.Errorf("BornAt = %v, want untouched %v", file.BornAt, original)
	}
	if !file.ArchivedAt.Equal(original) {
		t.Errorf("ArchivedAt = %v, want untouched %v", file.ArchivedAt, original)
	}
}

func TestApplyDotStatValues_NoMatchingSourceLeavesFileUntouched(t *testing.T) {
	original := time.Unix(42, 0)
	file := &stat.FileInfo{Name: "dump.sql", Parent: "backups", BornAt: original}

	// map key does not match Parent + "/" + Name
	ApplyDotStatValues(map[string]string{"other/file.sql": "/does/not/exist.stat"}, []*stat.FileInfo{file})

	if !file.BornAt.Equal(original) {
		t.Errorf("BornAt = %v, want untouched %v", file.BornAt, original)
	}
}

func TestApplyDotStatValues_InvalidTimestampIsIgnored(t *testing.T) {
	statPath := writeStatFile(t, "born_at: not-a-number\n")

	original := time.Unix(42, 0)
	file := &stat.FileInfo{Name: "dump.sql", Parent: "backups", BornAt: original}
	sources := map[string]string{"backups/dump.sql": statPath}

	ApplyDotStatValues(sources, []*stat.FileInfo{file})

	// Unparseable values are ignored and the original timestamp is preserved.
	if !file.BornAt.Equal(original) {
		t.Errorf("BornAt = %v, want untouched %v after unparseable value", file.BornAt, original)
	}
}

func TestApplyDotStatValuesRecursively(t *testing.T) {
	rootStat := writeStatFile(t, "born_at: 1000\n")
	childStat := writeStatFile(t, "born_at: 5000\n")

	rootFile := &stat.FileInfo{Name: "root.sql", Parent: "."}
	childFile := &stat.FileInfo{Name: "child.sql", Parent: "sub"}

	dir := &stat.DirectoryInfo{
		Name:  "root",
		Files: []*stat.FileInfo{rootFile},
		SubDirs: map[string]*stat.DirectoryInfo{
			"sub": {Name: "sub", Files: []*stat.FileInfo{childFile}},
		},
	}

	sources := map[string]string{
		"./root.sql":    rootStat,
		"sub/child.sql": childStat,
	}

	ApplyDotStatValuesRecursively(sources, dir)

	if rootFile.BornAt.Unix() != 1000 {
		t.Errorf("root BornAt = %d, want 1000", rootFile.BornAt.Unix())
	}
	if childFile.BornAt.Unix() != 5000 {
		t.Errorf("child BornAt = %d, want 5000 (subdirectories must be applied recursively)", childFile.BornAt.Unix())
	}
}
