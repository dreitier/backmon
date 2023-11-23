package config

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_NewSingleDiskConfiguration_GH6_detectsRegex(t *testing.T) {
	assertion := assert.New(t)

	sut, _ := NewSingleDiskConfiguration("/mystaticregex/")

	assertion.True(sut.IsRegularExpression)
	assertion.Equal("mystaticregex", sut.Name)
}

func Test_NewSingleDiskConfiguration_GH6_detectsRegexError(t *testing.T) {
	assertion := assert.New(t)

	sut, err := NewSingleDiskConfiguration("/notendingregex(.*/")

	assertion.Nil(sut)
	assertion.NotNil(err)
}

func Test_GetDiskStatus_GH5_UC1_simpleExclude(t *testing.T) {
	assertion := assert.New(t)
	raw, _ := ParseFromString(
		`
exclude:
- "a"
`)
	cfg := ParseDisksSection(raw)

	status, appliedPolicy := GetDiskStatus("a", cfg)

	assertion.Equal(DISKS_POLICY_EXCLUDE, appliedPolicy)
	assertion.Equal(DISKS_BEHAVIOUR_EXCLUDE, status)
}

func Test_GetDiskStatus_GH5_UC2_1_allOthersExplicitlyIncluded(t *testing.T) {
	assertion := assert.New(t)
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
	assertion.Equal(DISKS_BEHAVIOUR_EXCLUDE, status)

	status, _ = GetDiskStatus("bucket-2", cfg)
	assertion.Equal(DISKS_BEHAVIOUR_INCLUDE, status)

	status, _ = GetDiskStatus("bucket-3", cfg)
	assertion.Equal(DISKS_BEHAVIOUR_INCLUDE, status)
}

func Test_GetDiskStatus_GH5_UC2_2_allOthersImplicitlyIncluded(t *testing.T) {
	assertion := assert.New(t)
	raw, _ := ParseFromString(
		`
exclude:
- bucket-1
include:
- bucket-2
`)
	cfg := ParseDisksSection(raw)

	status, _ := GetDiskStatus("bucket-1", cfg)
	assertion.Equal(DISKS_BEHAVIOUR_EXCLUDE, status)

	status, _ = GetDiskStatus("bucket-2", cfg)
	assertion.Equal(DISKS_BEHAVIOUR_INCLUDE, status)

	status, _ = GetDiskStatus("bucket-3", cfg)
	assertion.Equal(DISKS_BEHAVIOUR_INCLUDE, status)
}

func Test_GetDiskStatus_GH5_UC3_allOthersExplicitlyExcluded(t *testing.T) {
	assertion := assert.New(t)
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
	assertion.Equal(DISKS_BEHAVIOUR_EXCLUDE, status)

	status, _ = GetDiskStatus("bucket-2", cfg)
	assertion.Equal(DISKS_BEHAVIOUR_INCLUDE, status)

	status, _ = GetDiskStatus("bucket-3", cfg)
	assertion.Equal(DISKS_BEHAVIOUR_EXCLUDE, status)

	spew.Dump(raw)
}

func Test_GetDiskStatus_GH5_UC4_conflictingStatement(t *testing.T) {
	assertion := assert.New(t)
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
	assertion.Equal(DISKS_BEHAVIOUR_EXCLUDE, status)
	assertion.Equal(DISKS_POLICY_CONFLICTING, appliedPolicy)
}

func Test_GetDiskStatus_GH6_UC1_excludedByRegex(t *testing.T) {
	assertion := assert.New(t)
	raw, _ := ParseFromString(
		`
exclude:
- "/ab.*ef/"
`)
	cfg := ParseDisksSection(raw)

	status, appliedPolicy := GetDiskStatus("abcdef", cfg)

	assertion.Equal(DISKS_POLICY_EXCLUDE_BY_REGEX, appliedPolicy)
	assertion.Equal(DISKS_BEHAVIOUR_EXCLUDE, status)
}

func Test_GetDiskStatus_GH6_UC1_includedByRegex(t *testing.T) {
	assertion := assert.New(t)
	raw, _ := ParseFromString(
		`
include:
- "/ab.*ef/"
`)
	cfg := ParseDisksSection(raw)

	status, appliedPolicy := GetDiskStatus("abcdef", cfg)

	assertion.Equal(DISKS_POLICY_INCLUDE_BY_REGEX, appliedPolicy)
	assertion.Equal(DISKS_BEHAVIOUR_INCLUDE, status)
}

func Test_GetDiskStatus_GH6_UC1_whenRegExpsForInclude_AND_excludeMatches_itFallsBackToDefaultBehaviour(t *testing.T) {
	assertion := assert.New(t)
	raw, _ := ParseFromString(
		`
include:
- "/ab.*ef/"
exclude:
- "/ab.*ef/"
`)
	cfg := ParseDisksSection(raw)

	status, appliedPolicy := GetDiskStatus("abcdef", cfg)

	assertion.Equal(DISKS_POLICY_CONFLICTING_BY_REGEX, appliedPolicy)
	assertion.Equal(DISKS_BEHAVIOUR_INCLUDE /* default behaviour */, status)
}
