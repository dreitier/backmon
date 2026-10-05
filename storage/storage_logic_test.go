package storage

import (
	"regexp"
	"strings"
	"testing"

	"github.com/dreitier/backmon/backup"
	"github.com/stretchr/testify/assert"
)

func TestMaxDepth_nilDefinitionReturnsDefault(t *testing.T) {
	assertion := assert.New(t)
	disk := &DiskData{Definition: nil}
	assertion.Equal(maxDirDepth, disk.maxDepth())
}

func TestMaxDepth_returnsDeepestDirectory(t *testing.T) {
	assertion := assert.New(t)

	shallow := &backup.Directory{Filter: backup.DirectoryFilter{
		Layers: []*regexp.Regexp{regexp.MustCompile("^a$")},
	}}
	deep := &backup.Directory{Filter: backup.DirectoryFilter{
		Layers: []*regexp.Regexp{
			regexp.MustCompile("^a$"),
			regexp.MustCompile("^b$"),
			regexp.MustCompile("^c$"),
		},
	}}

	disk := &DiskData{Definition: &backup.Definition{
		Directories: []*backup.Directory{shallow, deep},
	}}

	assertion.Equal(uint64(3), disk.maxDepth())
}

func TestMaxDepth_noDirectoriesReturnsZero(t *testing.T) {
	assertion := assert.New(t)
	disk := &DiskData{Definition: &backup.Definition{Directories: nil}}
	assertion.Equal(uint64(0), disk.maxDepth())
}

func TestHashChanged(t *testing.T) {
	assertion := assert.New(t)
	disk := &DiskData{}

	// first content differs from the zero-valued initial hash
	changed, err := disk.hashChanged(strings.NewReader("definitions-v1"))
	assertion.NoError(err)
	assertion.True(changed)

	// identical content is detected as unchanged
	changed, err = disk.hashChanged(strings.NewReader("definitions-v1"))
	assertion.NoError(err)
	assertion.False(changed)

	// new content is detected as changed again
	changed, err = disk.hashChanged(strings.NewReader("definitions-v2"))
	assertion.NoError(err)
	assertion.True(changed)
}

func TestTimestampFromVars_appliesParsersInReverse(t *testing.T) {
	assertion := assert.New(t)

	var order []string
	record := func(name string) backup.TimeParser {
		return func(val string, ts *backup.Timestamp) {
			order = append(order, name+"="+val)
		}
	}

	varDefs := []backup.VariableDefinition{
		{Name: "a", Parser: record("a")},
		{Name: "b", Parser: nil}, // no parser -> skipped
		{Name: "c", Parser: record("c")},
	}
	vals := []string{"1", "2", "3"}

	timestampFromVars(varDefs, vals)

	// iteration is in reverse, and the nil parser is skipped
	assertion.Equal([]string{"c=3", "a=1"}, order)
}

func TestAssembleFromTemplate_emptyTemplate(t *testing.T) {
	assertion := assert.New(t)
	assertion.Equal(".", assembleFromTemplate(nil, nil, nil))
}

func TestAssembleFromTemplate_substitutesVariables(t *testing.T) {
	assertion := assert.New(t)

	template := []string{"root/", "/sub/", ""}
	varDefs := []backup.VariableDefinition{
		{Name: "service"},
		{Name: "instance"},
	}
	vars := []string{"db", "01"}

	assertion.Equal("root/db/sub/01", assembleFromTemplate(template, varDefs, vars))
}

func TestAssembleFromTemplate_fusedVariableKeepsPlaceholder(t *testing.T) {
	assertion := assert.New(t)

	template := []string{"root/", ""}
	varDefs := []backup.VariableDefinition{
		{Name: "service", Fuse: true},
	}
	vars := []string{"ignored"}

	assertion.Equal("root/{{service}}", assembleFromTemplate(template, varDefs, vars))
}
