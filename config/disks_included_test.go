package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DisksConfiguration_GetIncludedDisks(t *testing.T) {
	assertion := assert.New(t)
	raw, _ := ParseFromString(
		`
include:
- bucket-1
- bucket-2
`)
	cfg := ParseDisksSection(raw)

	included := cfg.GetIncludedDisks()
	assertion.Len(included, 2)
	assertion.Contains(included, "bucket-1")
	assertion.Contains(included, "bucket-2")
}

func Test_DisksConfiguration_IsDiskIncluded(t *testing.T) {
	assertion := assert.New(t)
	raw, _ := ParseFromString(
		`
include:
- bucket-2
exclude:
- bucket-1
all_others: include
`)
	cfg := ParseDisksSection(raw)

	assertion.True(cfg.IsDiskIncluded("bucket-2"))
	assertion.False(cfg.IsDiskIncluded("bucket-1"))
	// a disk matched by no policy falls back to the default (include) behaviour
	assertion.True(cfg.IsDiskIncluded("bucket-unknown"))
}
