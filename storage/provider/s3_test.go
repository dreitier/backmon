package provider

import (
	"testing"

	fs "github.com/dreitier/backmon/storage/fs"
)

func TestObjectKey(t *testing.T) {
	cases := map[string]*fs.FileInfo{
		"dump.sql":           {Name: "dump.sql", Parent: ""},
		"backups/dump.sql":   {Name: "dump.sql", Parent: "backups"},
		"backups/a/dump.sql": {Name: "dump.sql", Parent: "backups/a/"},
	}

	for expected, file := range cases {
		if key := objectKey(file); key != expected {
			t.Errorf("expected key %q, got %q", expected, key)
		}
	}
}
