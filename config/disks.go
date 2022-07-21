package config

import (
	log "github.com/sirupsen/logrus"
)

// behaviour enum
const (
	DISKS_BEHAVIOUR_INCLUDE = iota
	DISKS_BEHAVIOUR_EXCLUDE = iota
)

// applied policy enum
const (
	DISKS_POLICY_INCLUDE = "explicit_include_policy"
	DISKS_POLICY_EXCLUDE = "explicit_exclude_policy"
	DISKS_POLICY_CONFLICTING = "unallowed_define_in_include_and_exclude_policy"
	DISKS_POLICY_NO_MATCH_FALLBACK = "not_matching_fallback_to_all_others"
)

// this is the transformed outcome of the `disks:` section
type DisksConfiguration struct {
	// simple disknames can be identified through a lookup table
	include					map[string]SingleDiskConfiguration
	// for regexps we cannot use a lookup table but have execute each regex
	includeRegExps			[]SingleDiskConfiguration
	exclude					map[string]SingleDiskConfiguration
	excludeRegExps			[]SingleDiskConfiguration
	// fallback to that behaviour if a policy does not match or is conflicting
	behaviourForAllOthers	int
}

type SingleDiskConfiguration struct {
	Name                string
	// pro forma
	IsRegularExpression bool
}

// Return true if the given disk is defined as "included" through some policy
func (self *DisksConfiguration) IsDiskIncluded(diskName string) bool {
	status, appliedPolicy := GetDiskStatus(diskName, self)

	if (status == DISKS_BEHAVIOUR_EXCLUDE) {
		log.Debugf("Disk %s is excluded (%s)", diskName, appliedPolicy)
		return false
	}

	return true
}

// Calculate the disk's status based upon the defined policies
func GetDiskStatus(diskName string, disksConfiguration *DisksConfiguration) (/* status */int, /* appliedPolicy */ string) {
	_, isExplicitlyIncluded := disksConfiguration.include[diskName]
	_, isExplicitlyExcluded := disksConfiguration.exclude[diskName]

	// include
	if (isExplicitlyIncluded && !isExplicitlyExcluded) {
		return DISKS_BEHAVIOUR_INCLUDE, DISKS_POLICY_INCLUDE
	}

	// exclude
	if (isExplicitlyExcluded && !isExplicitlyIncluded) {
		return DISKS_BEHAVIOUR_EXCLUDE, DISKS_POLICY_EXCLUDE
	}

	// disk has been defined in both `include` and `exclude` sections
	if (isExplicitlyIncluded && isExplicitlyExcluded) {
		return disksConfiguration.behaviourForAllOthers, DISKS_POLICY_CONFLICTING
	}

	// this applies to each other disk
	return disksConfiguration.behaviourForAllOthers, DISKS_POLICY_NO_MATCH_FALLBACK
}