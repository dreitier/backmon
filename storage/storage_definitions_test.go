package storage

import (
	"strings"
	"testing"

	"github.com/dreitier/backmon/metrics"
	"github.com/stretchr/testify/assert"
)

const validDefinitions = `---
quota: 2GiB
directories:
  backups:
    alias: my-backups
    defaults:
      schedule: 0 2 * * *
      retention-count: 10
    files:
      dump-%Y%M%D.sql:
        alias: pgdump
        schedule: 0 1 * * *
        sort: interpolation
...
`

func TestDiskData_MarshalJSON(t *testing.T) {
	assertion := assert.New(t)
	disk := &DiskData{Name: "my-disk"}

	out, err := disk.MarshalJSON()

	assertion.NoError(err)
	assertion.JSONEq(`"my-disk"`, string(out))
}

func TestUpdateDefinitions_parsesValidDefinitions(t *testing.T) {
	assertion := assert.New(t)
	disk := &DiskData{Name: "d", metrics: metrics.NewDisk("d")}
	defer disk.metrics.Drop()

	disk.updateDefinitions(strings.NewReader(validDefinitions))

	assertion.NotNil(disk.Definition)
	assertion.Len(disk.Definition.Directories, 1)
	// groups slice is sized to the number of directories
	assertion.Len(disk.groups, 1)
}

func TestUpdateDefinitions_unchangedContentDoesNotReparse(t *testing.T) {
	assertion := assert.New(t)
	disk := &DiskData{Name: "d", metrics: metrics.NewDisk("d")}
	defer disk.metrics.Drop()

	disk.updateDefinitions(strings.NewReader(validDefinitions))
	first := disk.Definition
	assertion.NotNil(first)

	// identical content leaves the already-parsed definition untouched
	disk.updateDefinitions(strings.NewReader(validDefinitions))
	assertion.Same(first, disk.Definition)
}

func TestUpdateDefinitions_invalidYamlLeavesDefinitionUnset(t *testing.T) {
	assertion := assert.New(t)
	disk := &DiskData{Name: "d", metrics: metrics.NewDisk("d")}
	defer disk.metrics.Drop()

	// unparseable content fails parsing, so no definition is ever assigned
	disk.updateDefinitions(strings.NewReader(": not: valid: yaml\n  - broken"))

	assertion.Nil(disk.Definition)
}
