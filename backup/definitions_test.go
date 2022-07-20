package backup

import (
	"io"
	"os"
	"testing"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
)

func Test_parseDefinitions(t *testing.T) {
	definitionsFile, err := os.Open("backup_definitions.json")
	if err != nil {
		t.Fatal(err)
	}

	defer definitionsFile.Close()

	reader := io.Reader(definitionsFile)

	defs, err := ParseRawDefinitions(reader)

	if err != nil {
		t.Error(err)
	}

	if defs == nil {
		t.Error("parsed definitions object is nil")
	}
}

func Test_parseDirectoryPattern(t *testing.T) {
	const diskNamePattern = "/backup.to/{{service}}/inst_{{instance}}"
	const diskNameMatch = "/backup.to/test#3/inst_a~1/"
	const diskNameFail = "myapp_z/erg_production-2019-06-24_02-45-00.sql"

	regex, err := ParseDirectoryPattern(diskNamePattern)
	if err != nil {
		t.Fatal(err)
	}

	if !regex.MatchString(diskNameMatch) {
		t.Error("Regex did not match a correct patten:", diskNameMatch)
	}

	if regex.MatchString(diskNameFail) {
		t.Error("Regex matched an incorrect pattern:", diskNameFail)
	}

	captures := make(map[string]string)
	match := regex.FindStringSubmatch(diskNameMatch)

	for i, name := range regex.SubexpNames() {
		// Ignore the whole regexp match and unnamed groups
		if i == 0 || name == "" {
			continue
		}

		captures[name] = match[i]
	}

	if captures["service"] != "test#3"{
		t.Error("Named capture group 'service' did not match the correct patten 'test#3'")
	}

	if captures["instance"] != "a~1"{
		t.Error("Named capture group 'instance' did not match the correct patten 'a~1'")
	}
}

func Test_parseDirectoryPatternWithNumber(t *testing.T) {
	const diskNamePattern = "saas/backup/{{instance}}"
	const diskNameMatch = "saas/backup/inst1"
	const diskNameFail = "saas/bla/inst_x"

	regex, err := ParseDirectoryPattern(diskNamePattern)
	if err != nil {
		t.Fatal(err)
	}

	if !regex.MatchString(diskNameMatch) {
		t.Error("Regex did not match a correct patten:", diskNameMatch)
	}

	if regex.MatchString(diskNameFail) {
		t.Error("Regex matched an incorrect pattern:", diskNameFail)
	}

	captures := make(map[string]string)
	match := regex.FindStringSubmatch(diskNameMatch)

	for i, name := range regex.SubexpNames() {
		// Ignore the whole regexp match and unnamed groups
		if i == 0 || name == "" {
			continue
		}

		captures[name] = match[i]
	}

	if captures["instance"] != "inst1"{
		t.Error("Named capture group 'instance' did not match the correct patten 'inst1'")
	}
}

func Test_parseFilenamePattern(t *testing.T) {
	const fileNamePattern = "myapp_${instance:lower}_production-%Y-%m-%d_%H-%M-%S.sql"
	const fileNameMatch = "myapp_zerg_production-2019-06-24_02-45-00.sql"
	const fileNameFail = "myapp_z/erg_production-2019-06-24_02-45-00.sql"

	regex, err := ParseFilePattern(fileNamePattern)
	if err != nil {
		t.Fatal(err)
	}

	if !regex.MatchString(fileNameMatch) {
		t.Error("Regex did not match a correct patten:", fileNameMatch)
	}

	if regex.MatchString(fileNameFail) {
		t.Error("Regex matched an incorrect pattern:", fileNameFail)
	}

	captures := make(map[string]string)
	match := regex.FindStringSubmatch(fileNameMatch)

	for i, name := range regex.SubexpNames() {
		// Ignore the whole regexp match and unnamed groups
		if i == 0 || name == "" {
			continue
		}

		captures[name] = match[i]
	}

	if captures["lower_instance"] != "zerg"{
		t.Error("Named capture group 'lower_instance' did not match the correct patten 'zerg'")
	}
}

func TestSplitPattern_extractsToVariables(t *testing.T) {
	assert := assert.New(t)

	captures, leftovers := splitPattern("{{a}}/{{b}}")

	assert.Equal("a", captures[0])
	assert.Equal("b", captures[1])

	assert.Equal("", leftovers[0])
	assert.Equal("/", leftovers[1])
	assert.Equal("", leftovers[2])
}

func TestSplitPattern_extractsVariableAndPathSegment(t *testing.T) {
	assert := assert.New(t)

	captures, leftovers := splitPattern("root/{{a}}")

	assert.Equal("a", captures[0])
	assert.Equal(1, len(captures))

	assert.Equal(2, len(leftovers))
	assert.Equal("root/", leftovers[0])
	assert.Equal("", leftovers[1])
}

func TestParsePathPattern(t *testing.T) {
	assert := assert.New(t)
	filter, variableOffsets := ParsePathPattern("root/{{var1}}/subdir/{{var2}}")

	spew.Dump(filter)
	spew.Dump(variableOffsets)

	assert.Equal(3, len(filter.Template))
	assert.Equal("root/", filter.Template[0])
	assert.Equal("/subdir/", filter.Template[1])

	assert.Equal(4, len(filter.Layers))
	assert.Equal("^root$", filter.Layers[0].String())
	assert.Equal("^(?P<_var1>[^\\\\./]+?)$", filter.Layers[1].String())
	assert.Equal("^subdir$", filter.Layers[2].String())
	assert.Equal("^(?P<_var2>[^\\\\./]+?)$", filter.Layers[3].String())

	assert.Equal(2, len(filter.Variables))
	assert.Equal("var1", filter.Variables[0].Name)
	assert.Equal(false, filter.Variables[0].Fuse)
	assert.Equal("var2", filter.Variables[1].Name)
	assert.Equal(false, filter.Variables[1].Fuse)

	assert.Equal(2, len(variableOffsets))
	assert.Equal(uint(1), variableOffsets["var1"])
	assert.Equal(uint(2), variableOffsets["var2"])
}

func Test_applyFusion(t *testing.T) {
	assert := assert.New(t)
	filter, _ := ParsePathPattern("root/{{var1}}/subdir/{{var2}}/{{var4}}")

	fuses := []string{"var1", "var2"}

	assert.Equal(nil, applyFusion(filter.Variables, fuses))
}

func Test_applyFusion_returnsError_ifFuseReferencesAnUnknownVariable(t *testing.T) {
	assert := assert.New(t)
	filter, _ := ParsePathPattern("root/{{var1}}/subdir/{{var2}}/{{var4}}")

	fuses := []string{"var1", "var2", "var3"}

	// error
	assert.NotNil(applyFusion(filter.Variables, fuses))
}