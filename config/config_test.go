package config

import (
	//	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_GH29_PR31_NewConfigurationInstance_canDetectRegionForFirstEnvironment(t *testing.T) {
	assertion := assert.New(t)

	raw, _ := ParseFromString(
		`
port: 8080
http:
environments:
  default:
    region: eu-central-2
`)
	sut := NewConfigurationInstance(raw)

	assertion.NotNil(sut)
	assertion.Equal("eu-central-2", sut.Environments()[0].Client.Region)
}

func Test_GH29_PR31_MissingDisksSectionInEnvironment_isOkForAutoDiscover(t *testing.T) {
	assertion := assert.New(t)

	raw, _ := ParseFromString(
		`
port: 8080
http:
environments:
  default:
    disks:
    s3:
      auto_discover_disks: false
`)
	sut := NewConfigurationInstance(raw)

	assertion.NotNil(sut)
	assertion.Equal(1, len(sut.Environments()))
	assertion.False(sut.Environments()[0].Client.AutoDiscoverDisks)

	diskCfg := sut.Environments()[0].Client.Disks

	assertion.Equal(0, len(diskCfg.include))
	assertion.Equal(0, len(diskCfg.exclude))
	assertion.Equal(DisksBehaviourInclude, diskCfg.behaviourForAllOthers)
}

func Test_GH29_PR31_DisksSectionInEnvironment_isAlwaysParsed(t *testing.T) {
	assertion := assert.New(t)

	raw, _ := ParseFromString(
		`
port: 8080

environments:
  default:
    disks:
      include:
        - included-1
      exclude:
        - excluded-1  
`)
	sut := NewConfigurationInstance(raw)

	assertion.NotNil(sut)
	assertion.Equal(1, len(sut.Environments()))

	var diskCfg *DisksConfiguration = sut.Environments()[0].Client.Disks
	assertion.NotNil(diskCfg)

	assertion.Equal(1, len(diskCfg.include))
	assertion.Contains(diskCfg.include, "included-1")
	assertion.Equal(1, len(diskCfg.exclude))
	assertion.Contains(diskCfg.exclude, "excluded-1")
}
