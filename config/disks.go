package config

import (
	log "github.com/sirupsen/logrus"
	"regexp"
	"strings"
)

// behaviour enum
const (
	DisksBehaviourInclude = iota
	DisksBehaviourExclude = iota
)

// applied policy enum
const (
	DisksPolicyInclude            = "explicit_include_policy"
	DisksPolicyIncludeByRegex     = "explicit_include_by_regex_policy"
	DisksPolicyExclude            = "explicit_exclude_policy"
	DisksPolicyExcludeByRegex     = "explicit_exclude_by_regex_policy"
	DisksPolicyConflicting        = "unallowed_definition_in_include_and_exclude_policy"
	DisksPolicyConflictingByRegex = "unallowed_definition_in_include_or_exclude_and_contradicting_regexp"
	DisksPolicyNoMatchFallback    = "not_matching_fallback_to_all_others"
)

// DisksConfiguration this is the transformed outcome of the `disks:` section
type DisksConfiguration struct {
	// simple disk names can be identified through a lookup table
	include map[string]SingleDiskConfiguration
	// for regexps, we cannot use a lookup table but have execute each regex
	includeRegExps []SingleDiskConfiguration
	exclude        map[string]SingleDiskConfiguration
	excludeRegExps []SingleDiskConfiguration
	// fallback to that behaviour if a policy does not match or is conflicting
	behaviourForAllOthers int
}

type SingleDiskConfiguration struct {
	// either the name of the disk or the regular expression
	Name string
	// pro forma
	IsRegularExpression bool
}

// NewSingleDiskConfiguration Create a new SingleDiskConfiguration
// @return error if the diskNameOrRegExp is based upon a regex ('/someregex/')
func NewSingleDiskConfiguration(diskNameOrRegExp string) (*SingleDiskConfiguration, error) {
	// we are checking if the given diskName to include or exclude is a regular expression like "/.*/"
	isRegEx := strings.HasPrefix(diskNameOrRegExp, "/") && strings.HasSuffix(diskNameOrRegExp, "/")

	if isRegEx {
		diskNameOrRegExp = strings.TrimSuffix(strings.TrimPrefix(diskNameOrRegExp, "/"), "/")
		_, err := regexp.MatchString(diskNameOrRegExp, "ignoreme")

		if err != nil {
			return nil, err
		}
	}

	r := &SingleDiskConfiguration{
		Name:                diskNameOrRegExp,
		IsRegularExpression: isRegEx,
	}

	return r, nil
}

func (diskConfig *DisksConfiguration) GetIncludedDisks() map[string]SingleDiskConfiguration {
	return diskConfig.include
}

// IsDiskIncluded Return true if the given disk is defined as "included" through some policy
func (diskConfig *DisksConfiguration) IsDiskIncluded(diskName string) bool {
	status, appliedPolicy := GetDiskStatus(diskName, diskConfig)

	if status == DisksBehaviourExclude {
		log.Debugf("Disk %s is excluded (%s)", diskName, appliedPolicy)

		return false
	}

	return true
}

// Check if at least one of the regexps matches
// @return true if at least one of the Configuration regexps matches the given diskName
// @return false if no regexp matches
func hasAtLeastOneMatch(diskName string, possibleConfigsWithRegExps []SingleDiskConfiguration) bool {
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

// GetDiskStatus Calculate the disk's status based upon the defined policies
// @return (status, appliedPolicy)
func GetDiskStatus(diskName string, disksConfiguration *DisksConfiguration) ( /* status */ int /* appliedPolicy */, string) {
	_, isExplicitlyIncluded := disksConfiguration.include[diskName]
	isIncludedByRegex := hasAtLeastOneMatch(diskName, disksConfiguration.includeRegExps)
	isIncluded := isExplicitlyIncluded || isIncludedByRegex

	_, isExplicitlyExcluded := disksConfiguration.exclude[diskName]
	isExcludedByRegex := hasAtLeastOneMatch(diskName, disksConfiguration.excludeRegExps)
	isExcluded := isExplicitlyExcluded || isExcludedByRegex

	// include
	if isExplicitlyIncluded && !isExcluded {
		return DisksBehaviourInclude, DisksPolicyInclude
	}

	// included by regex
	if isIncludedByRegex && !isExcluded {
		return DisksBehaviourInclude, DisksPolicyIncludeByRegex
	}

	// exclude
	if isExplicitlyExcluded && !isIncluded {
		return DisksBehaviourExclude, DisksPolicyExclude
	}

	// excluded by regex
	if isExcludedByRegex && !isIncluded {
		return DisksBehaviourExclude, DisksPolicyExcludeByRegex
	}

	if isExplicitlyIncluded && isExplicitlyExcluded {
		return disksConfiguration.behaviourForAllOthers, DisksPolicyConflicting
	}

	// disk has been defined in both `include` and `exclude` sections
	if isIncluded && isExcluded {
		return disksConfiguration.behaviourForAllOthers, DisksPolicyConflictingByRegex
	}

	// this applies to a disk which has no match
	return disksConfiguration.behaviourForAllOthers, DisksPolicyNoMatchFallback
}
