package provider

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	fs "github.com/dreitier/backmon/storage/fs"
)

func TestLocalClient_GetFileNamesRejectsUnknownDisk(t *testing.T) {
	c := LocalClient{EnvName: "test", Directory: "/mnt/backup"}

	if _, err := c.GetFileNames("/some/other/disk", 1); err == nil {
		t.Error("expected error for a disk that does not match the configured directory")
	}
}

func TestLocalClient_DownloadRejectsUnknownDisk(t *testing.T) {
	c := LocalClient{EnvName: "test", Directory: "/mnt/backup"}

	_, _, _, err := c.Download("/wrong", &fs.FileInfo{Name: "x", Parent: ""})
	if err == nil {
		t.Error("expected error when downloading from an unknown disk")
	}
}

func TestLocalClient_DownloadReturnsErrorForMissingFile(t *testing.T) {
	root := t.TempDir()
	c := LocalClient{EnvName: "test", Directory: root}

	_, _, _, err := c.Download(root, &fs.FileInfo{Name: "does-not-exist", Parent: ""})
	if err == nil {
		t.Error("expected error when downloading a file that does not exist")
	}
}

func TestLocalClient_DeleteRejectsUnknownDisk(t *testing.T) {
	c := LocalClient{EnvName: "test", Directory: "/mnt/backup"}

	if err := c.Delete("/wrong", &fs.FileInfo{Name: "x", Parent: ""}); err == nil {
		t.Error("expected error when deleting from an unknown disk")
	}
}

func TestLocalClient_GetDiskNames(t *testing.T) {
	envName := "test"
	directory := os.TempDir()
	c := LocalClient{EnvName: envName, Directory: directory}
	names, _ := c.GetDiskNames()

	if len(names) != 1 {
		t.Error("wrong number of disk returned")
	}

	if names[0] != directory {
		t.Errorf("DiskInfo name returned: %s, DiskInfo name exptected: %s", names[0], directory)
	}

}

func TestLocalClient_DownloadAndDeleteFileInSubdirectory(t *testing.T) {
	root := t.TempDir()
	subDir := filepath.Join(root, "backups", "db")

	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	backupPath := filepath.Join(subDir, "dump.sql")
	statPath := backupPath + ".stat"

	if err := os.WriteFile(backupPath, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(statPath, []byte("born_at: 1000\nmodified_at: 2000\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := LocalClient{EnvName: "test", Directory: root}
	dir, err := c.GetFileNames(root, 10)

	if err != nil {
		t.Fatal(err)
	}

	files := dir.SubDirs["backups"].SubDirs["db"].Files

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	file := files[0]

	if file.Parent != filepath.Join("backups", "db") {
		t.Errorf("expected parent relative to disk root, got %s", file.Parent)
	}

	if file.BornAt.Unix() != 1000 || file.ModifiedAt.Unix() != 2000 {
		t.Errorf(".stat file has not been applied: born_at=%d modified_at=%d", file.BornAt.Unix(), file.ModifiedAt.Unix())
	}

	reader, length, _, err := c.Download(root, file)

	if err != nil {
		t.Fatalf("download failed: %s", err)
	}

	content, _ := io.ReadAll(reader)
	_ = reader.Close()

	if string(content) != "content" || length != int64(len("content")) {
		t.Errorf("unexpected download content %q with length %d", content, length)
	}

	if err := c.Delete(root, file); err != nil {
		t.Fatalf("delete failed: %s", err)
	}

	for _, path := range []string{backupPath, statPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("expected %s to be deleted", path)
		}
	}
}
