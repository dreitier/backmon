package provider

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	fs "github.com/dreitier/backmon/storage/fs"
)

// writeFiles creates the given files (path relative to root -> content) below root
func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()

	for name, content := range files {
		path := filepath.Join(root, name)

		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func fileNames(dir *fs.DirectoryInfo) map[string]int64 {
	names := make(map[string]int64, len(dir.Files))

	for _, file := range dir.Files {
		names[file.Name] = file.Size
	}

	return names
}

func TestLocalClient_GetFileNamesRejectsUnknownDisk(t *testing.T) {
	c := &LocalClient{Directory: t.TempDir()}

	if _, err := c.GetFileNames("/some/other/disk", 10); err == nil {
		t.Error("expected an error for a disk which is not the configured directory")
	}
}

func TestLocalClient_GetFileNamesFailsForMissingDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	c := &LocalClient{Directory: root}

	if _, err := c.GetFileNames(root, 10); err == nil {
		t.Error("expected an error for a directory which does not exist")
	}
}

func TestLocalClient_GetFileNamesBuildsTree(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"a.txt":          "a",
		"sub/b.txt":      "bb",
		"sub/deep/c.txt": "ccc",
	})
	c := &LocalClient{Directory: root}

	tree, err := c.GetFileNames(root, 10)
	if err != nil {
		t.Fatal(err)
	}

	if got := fileNames(tree); len(got) != 1 || got["a.txt"] != 1 {
		t.Errorf("root files = %v, want only a.txt with size 1", got)
	}

	sub := tree.SubDirs["sub"]
	if sub == nil {
		t.Fatal("expected subdirectory 'sub'")
	}

	if got := fileNames(sub); len(got) != 1 || got["b.txt"] != 2 {
		t.Errorf("files in sub = %v, want only b.txt with size 2", got)
	}

	deep := sub.SubDirs["deep"]
	if deep == nil {
		t.Fatal("expected subdirectory 'sub/deep'")
	}

	if got := fileNames(deep); len(got) != 1 || got["c.txt"] != 3 {
		t.Errorf("files in sub/deep = %v, want only c.txt with size 3", got)
	}
}

func TestLocalClient_GetFileNamesRespectsMaxDepth(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"a.txt":          "a",
		"sub/b.txt":      "bb",
		"sub/deep/c.txt": "ccc",
	})
	c := &LocalClient{Directory: root}

	flat, err := c.GetFileNames(root, 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(flat.Files) != 1 || len(flat.SubDirs) != 0 {
		t.Errorf("depth 0: got %d files and %d subdirectories, want 1 and 0", len(flat.Files), len(flat.SubDirs))
	}

	oneLevel, err := c.GetFileNames(root, 1)
	if err != nil {
		t.Fatal(err)
	}

	sub := oneLevel.SubDirs["sub"]
	if sub == nil {
		t.Fatal("depth 1: expected subdirectory 'sub'")
	}

	if len(sub.SubDirs) != 0 {
		t.Errorf("depth 1: 'sub/deep' must not be scanned, got %v", sub.SubDirs)
	}
}

func TestLocalClient_GetFileNamesAppliesDotStatFiles(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"sub/dump.sql":      "data",
		"sub/dump.sql.stat": "born_at: 1000000\nmodified_at: 2000000\narchived_at: 3000000\n",
		"sub/other.sql":     "data",
	})
	c := &LocalClient{Directory: root}

	tree, err := c.GetFileNames(root, 10)
	if err != nil {
		t.Fatal(err)
	}

	sub := tree.SubDirs["sub"]
	if sub == nil {
		t.Fatal("expected subdirectory 'sub'")
	}

	if len(sub.Files) != 2 {
		t.Fatalf("got %d files, want 2: the .stat file itself must not be listed", len(sub.Files))
	}

	for _, file := range sub.Files {
		if file.Name != "dump.sql" {
			continue
		}

		if file.BornAt.Unix() != 1000000 || file.ModifiedAt.Unix() != 2000000 || file.ArchivedAt.Unix() != 3000000 {
			t.Errorf("timestamps of dump.sql = %d/%d/%d, want the values of its .stat file",
				file.BornAt.Unix(), file.ModifiedAt.Unix(), file.ArchivedAt.Unix())
		}
	}
}

func TestLocalClient_Download(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"a.txt": "hello"})
	c := &LocalClient{Directory: root}

	data, length, _, err := c.Download(root, &fs.FileInfo{Name: "a.txt"})
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = data.Close() }()

	content, _ := io.ReadAll(data)
	if string(content) != "hello" || length != 5 {
		t.Errorf("got %q with length %d, want \"hello\" with length 5", content, length)
	}
}

func TestLocalClient_DownloadErrors(t *testing.T) {
	root := t.TempDir()
	c := &LocalClient{Directory: root}

	if _, _, _, err := c.Download("/some/other/disk", &fs.FileInfo{Name: "a.txt"}); err == nil {
		t.Error("expected an error for an unknown disk")
	}

	if _, _, _, err := c.Download(root, &fs.FileInfo{Name: "missing.txt"}); err == nil {
		t.Error("expected an error for a missing file")
	}
}

func TestLocalClient_DeleteRemovesFileAndItsDotStatFile(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"dump.sql":      "data",
		"dump.sql.stat": "born_at: 1000000\n",
		"keep.sql":      "data",
	})
	c := &LocalClient{Directory: root}

	if err := c.Delete(root, &fs.FileInfo{Name: "dump.sql"}); err != nil {
		t.Fatal(err)
	}

	remaining, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(remaining) != 1 || remaining[0].Name() != "keep.sql" {
		t.Errorf("remaining entries = %v, want only keep.sql", remaining)
	}
}

func TestLocalClient_DeleteErrors(t *testing.T) {
	root := t.TempDir()
	c := &LocalClient{Directory: root}

	if err := c.Delete("/some/other/disk", &fs.FileInfo{Name: "a.txt"}); err == nil {
		t.Error("expected an error for an unknown disk")
	}

	if err := c.Delete(root, &fs.FileInfo{Name: "missing.txt"}); err == nil {
		t.Error("expected an error for a missing file")
	}
}
