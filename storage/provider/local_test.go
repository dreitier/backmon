package provider

import (
	"os"
	"testing"
	"github.com/dreitier/cloudmon/storage/abstraction"
)

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
