package storage

import (
	"os"
	"testing"
)

func TestLocalClient_GetBucketNames(t *testing.T) {
	envName := "test"
	directory := os.TempDir()
	c := LocalClient{EnvName: envName, Directory: directory}
	names, _ := c.GetBucketNames()

	if len(names) != 1 {
		t.Error("wrong number of bucket returned")
	}

	if names[0] != directory {
		t.Errorf("BucketInfo name returned: %s, BucketInfo name exptected: %s", names[0], directory)
	}

}
