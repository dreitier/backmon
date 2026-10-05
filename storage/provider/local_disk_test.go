package provider

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestLocalClient_GetDiskName(t *testing.T) {
	cases := []struct {
		name      string
		directory string
		expected  string
	}{
		{"leading slash trimmed", "/mnt/backup", "mnt_backup"},
		{"nested path", "/a/b/c", "a_b_c"},
		{"no leading slash", "mnt/backup", "mnt_backup"},
		{"root only", "/", ""},
		{"single segment", "/data", "data"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := LocalClient{EnvName: "test", Directory: tc.directory}
			if got := c.getDiskName(); got != tc.expected {
				t.Errorf("getDiskName(%q) = %q, want %q", tc.directory, got, tc.expected)
			}
		})
	}
}

func TestLocalClient_FindDiskMatchesConfiguredDirectory(t *testing.T) {
	c := LocalClient{EnvName: "test", Directory: "/mnt/backup"}

	diskName := "/mnt/backup"
	got, err := c.findDisk(&diskName)

	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if got == nil || *got != "/mnt/backup" {
		t.Errorf("expected /mnt/backup, got %v", got)
	}
}

func TestLocalClient_FindDiskNormalizesMissingLeadingSeparator(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("leading separator normalization is skipped on windows")
	}

	c := LocalClient{EnvName: "test", Directory: "/mnt/backup"}

	// no leading separator; findDisk must prepend one before matching
	diskName := "mnt/backup"
	got, err := c.findDisk(&diskName)

	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if got == nil || *got != "/mnt/backup" {
		t.Errorf("expected normalized /mnt/backup, got %v", got)
	}

	// the normalization also mutates the caller's string
	if diskName != "/mnt/backup" {
		t.Errorf("expected diskName to be normalized in place, got %q", diskName)
	}
}

func TestLocalClient_FindDiskRejectsUnknownDisk(t *testing.T) {
	c := LocalClient{EnvName: "test", Directory: "/mnt/backup"}

	diskName := string(os.PathSeparator) + "does-not-exist"
	got, err := c.findDisk(&diskName)

	if err == nil {
		t.Error("expected error for a disk that is not configured")
	}

	if got != nil {
		t.Errorf("expected nil result on error, got %v", got)
	}

	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("expected an 'unknown' error, got %s", err)
	}
}
