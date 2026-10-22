package stat

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsFilePathValid_ExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	valid, err := IsFilePathValid(path)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !valid {
		t.Error("expected existing file to be reported as valid")
	}
}

func TestIsFilePathValid_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.txt")

	valid, err := IsFilePathValid(path)
	if err != nil {
		t.Fatalf("a missing file must not produce an error, got: %s", err)
	}
	if valid {
		t.Error("expected missing file to be reported as invalid")
	}
}

func TestIsFilePathValid_ExistingDirectory(t *testing.T) {
	// A directory exists on disk, so os.Stat succeeds and the path is valid.
	valid, err := IsFilePathValid(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !valid {
		t.Error("expected existing directory to be reported as valid")
	}
}
