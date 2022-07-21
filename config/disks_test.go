package config

import (
	"testing"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
)

func Test_GetDiskStatus_GH5_UC1_simpleExclude(t *testing.T) {
	assert := assert.New(t)
	raw, _ := ParseFromString(
`
exclude:
- "a"
`)
	cfg := ParseDisksSection(raw)

	status, appliedPolicy := GetDiskStatus("a", cfg)

	assert.Equal(DISKS_POLICY_EXCLUDE, appliedPolicy)
	assert.Equal(DISKS_BEHAVIOUR_EXCLUDE, status)
}

func Test_GetDiskStatus_GH5_UC2_1_allOthersExplicitlyIncluded(t *testing.T) {
	assert := assert.New(t)
	raw, _ := ParseFromString(
`
exclude:
- bucket-1
include:
- bucket-2
# all_others is by default set to "include" as this is default behaviour at the moment
all_others: include
`)
	cfg := ParseDisksSection(raw)

	status, _ := GetDiskStatus("bucket-1", cfg)
	assert.Equal(DISKS_BEHAVIOUR_EXCLUDE, status)

	status, _ = GetDiskStatus("bucket-2", cfg)
	assert.Equal(DISKS_BEHAVIOUR_INCLUDE, status)

	status, _ = GetDiskStatus("bucket-3", cfg)
	assert.Equal(DISKS_BEHAVIOUR_INCLUDE, status)
}

func Test_GetDiskStatus_GH5_UC2_2_allOthersImplicitlyIncluded(t *testing.T) {
	assert := assert.New(t)
	raw, _ := ParseFromString(
`
exclude:
- bucket-1
include:
- bucket-2
`)
	cfg := ParseDisksSection(raw)

	status, _ := GetDiskStatus("bucket-1", cfg)
	assert.Equal(DISKS_BEHAVIOUR_EXCLUDE, status)

	status, _ = GetDiskStatus("bucket-2", cfg)
	assert.Equal(DISKS_BEHAVIOUR_INCLUDE, status)

	status, _ = GetDiskStatus("bucket-3", cfg)
	assert.Equal(DISKS_BEHAVIOUR_INCLUDE, status)
}

func Test_GetDiskStatus_GH5_UC3_allOthersExplicitlyExcluded(t *testing.T) {
	assert := assert.New(t)
	raw, _ := ParseFromString(
`
exclude:
- bucket-1
include:
- bucket-2
all_others: exclude
`)
	cfg := ParseDisksSection(raw)

	status, _ := GetDiskStatus("bucket-1", cfg)
	assert.Equal(DISKS_BEHAVIOUR_EXCLUDE, status)

	status, _ = GetDiskStatus("bucket-2", cfg)
	assert.Equal(DISKS_BEHAVIOUR_INCLUDE, status)

	status, _ = GetDiskStatus("bucket-3", cfg)
	assert.Equal(DISKS_BEHAVIOUR_EXCLUDE, status)

	spew.Dump(raw)
}

func Test_GetDiskStatus_GH5_UC4_conflictingStatement(t *testing.T) {
	assert := assert.New(t)
	raw, _ := ParseFromString(
`
exclude:
- bucket-2
include:
- bucket-1
- bucket-2
all_others: exclude
`)
	cfg := ParseDisksSection(raw)

	status, appliedPolicy := GetDiskStatus("bucket-2", cfg)
	assert.Equal(DISKS_BEHAVIOUR_EXCLUDE, status)
	assert.Equal(DISKS_POLICY_CONFLICTING, appliedPolicy)
}