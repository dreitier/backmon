package config

import (
	log "github.com/sirupsen/logrus"
	"strings"
	"regexp"
)

// behaviour enum
const (
	DISKS_BEHAVIOUR_INCLUDE = iota
	DISKS_BEHAVIOUR_EXCLUDE = iota
)

// applied policy enum
const (
	DISKS_POLICY_INCLUDE = "explicit_include_policy"
	DISKS_POLICY_INCLUDE_BY_REGEX = "explicit_include_by_regex_policy"
	DISKS_POLICY_EXCLUDE = "explicit_exclude_policy"
	DISKS_POLICY_EXCLUDE_BY_REGEX = "explicit_exclude_by_regex_policy"
	DISKS_POLICY_CONFLICTING = "unallowed_definition_in_include_and_exclude_policy"
	DISKS_POLICY_CONFLICTING_BY_REGEX = "unallowed_definition_in_include_or_exclude_and_contradicting_regexp"
	DISKS_POLICY_NO_MATCH_FALLBACK = "not_matching_fallback_to_all_others"
)

// this is the transformed outcome of the `disks:` section
type DisksConfiguration struct {
	// simple disknames can be identified through a lookup table
	include                map[string]SingleDiskConfiguration
	// for regexps we cannot use a lookup table but have execute each regex
	includeRegExps         []SingleDiskConfiguration
	exclude                map[string]SingleDiskConfiguration
	excludeRegExps         []SingleDiskConfiguration
	// fallback to that behaviour if a policy does not match or is conflicting
	behaviourForAllOthers  int
}

type SingleDiskConfiguration struct {
	// either the name of the disk or the regular expression
	Name                string
	// pro forma
	IsRegularExpression bool
}

// Create a new SingleDiskConfiguration
// @return error if the diskNameOrRegExp is based upon a regex ('/someregex/')
func NewSingleDiskConfiguration(diskNameOrRegExp string) (*SingleDiskConfiguration, error) {
	// we are checking if the given diskName to include or exclude is a regular expression like "/.*/"
	isRegEx := strings.HasPrefix(diskNameOrRegExp, "/") && strings.HasSuffix(diskNameOrRegExp, "/")

	if (isRegEx) {
		diskNameOrRegExp = strings.TrimSuffix(strings.TrimPrefix(diskNameOrRegExp, "/"), "/")
		_, err := regexp.MatchString(diskNameOrRegExp, "ignoreme")

		if err != nil {
			return nil, err
		}
	}

	r := &SingleDiskConfiguration{
		Name: diskNameOrRegExp,
		IsRegularExpression: isRegEx,
	}

	return r, nil
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

// Check if at least one of the regexps matches
// @return true if at least one of the configuration regexps matches the given diskName
// @return false if no regexp matches
func hasAtLeastOneMatch(diskName string, possibleConfigsWithRegExps []SingleDiskConfiguration) (bool) {
	for _, config := range possibleConfigsWithRegExps {
		if !config.IsRegularExpression {
			continue
		}

		match, err := regexp.MatchString(config.Name /* contains the regexp */, diskName)
		
		if err != nil {
			continue
		}

		if match {
			return true
		}
	}

	return false
}

// Calculate the disk's status based upon the defined policies
// @return (status, appliedPolicy)
func GetDiskStatus(diskName string, disksConfiguration *DisksConfiguration) (/* status */int, /* appliedPolicy */ string) {
	_, isExplicitlyIncluded := disksConfiguration.include[diskName]
	isIncludedByRegex := hasAtLeastOneMatch(diskName, disksConfiguration.includeRegExps)
	isIncluded := isExplicitlyIncluded || isIncludedByRegex

	_, isExplicitlyExcluded := disksConfiguration.exclude[diskName]
	isExcludedByRegex := hasAtLeastOneMatch(diskName, disksConfiguration.excludeRegExps)
	isExcluded := isExplicitlyExcluded || isExcludedByRegex

	// include
	if (isExplicitlyIncluded && !isExcluded) {
		return DISKS_BEHAVIOUR_INCLUDE, DISKS_POLICY_INCLUDE
	}

	// included by regex
	if (isIncludedByRegex && !isExcluded) {
		return DISKS_BEHAVIOUR_INCLUDE, DISKS_POLICY_INCLUDE_BY_REGEX
	}

	// exclude
	if (isExplicitlyExcluded && !isIncluded) {
		return DISKS_BEHAVIOUR_EXCLUDE, DISKS_POLICY_EXCLUDE
	}

	// excluded by regex
	if (isExcludedByRegex && !isIncluded) {
		return DISKS_BEHAVIOUR_EXCLUDE, DISKS_POLICY_EXCLUDE_BY_REGEX
	}
	
	if (isExplicitlyIncluded && isExplicitlyExcluded) {
		return disksConfiguration.behaviourForAllOthers, DISKS_POLICY_CONFLICTING
	}

	// disk has been defined in both `include` and `exclude` sections
	if (isIncluded && isExcluded) {
		return disksConfiguration.behaviourForAllOthers, DISKS_POLICY_CONFLICTING_BY_REGEX
	}

	// this applies to a disk which has no match
	return disksConfiguration.behaviourForAllOthers, DISKS_POLICY_NO_MATCH_FALLBACK
}