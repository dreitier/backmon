package backup

import (
	"io"
	"os"
	"testing"
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
	const bucketNamePattern = "/backup.to/{{service}}/inst_{{instance}}"
	const bucketNameMatch = "/backup.to/test#3/inst_a~1/"
	const bucketNameFail = "myapp_z/erg_production-2019-06-24_02-45-00.sql"

	regex, err := ParseDirectoryPattern(bucketNamePattern)
	if err != nil {
		t.Fatal(err)
	}

	if !regex.MatchString(bucketNameMatch) {
		t.Error("Regex did not match a correct patten:", bucketNameMatch)
	}

	if regex.MatchString(bucketNameFail) {
		t.Error("Regex matched an incorrect pattern:", bucketNameFail)
	}

	captures := make(map[string]string)
	match := regex.FindStringSubmatch(bucketNameMatch)

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
	const bucketNamePattern = "saas/backup/{{instance}}"
	const bucketNameMatch = "saas/backup/inst1"
	const bucketNameFail = "saas/bla/inst_x"

	regex, err := ParseDirectoryPattern(bucketNamePattern)
	if err != nil {
		t.Fatal(err)
	}

	if !regex.MatchString(bucketNameMatch) {
		t.Error("Regex did not match a correct patten:", bucketNameMatch)
	}

	if regex.MatchString(bucketNameFail) {
		t.Error("Regex matched an incorrect pattern:", bucketNameFail)
	}

	captures := make(map[string]string)
	match := regex.FindStringSubmatch(bucketNameMatch)

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
