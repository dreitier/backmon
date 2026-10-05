package storage

import (
	"regexp"
	"testing"
	"time"

	"github.com/dreitier/backmon/backup"
	fs "github.com/dreitier/backmon/storage/fs"
	"github.com/stretchr/testify/assert"
)

// collectMatchingFiles with an all-internal mapping keeps files whose name
// matches the filter and skips those that don't, assigning an interpolated
// timestamp to the keepers.
func TestCollectMatchingFiles_filtersByPattern(t *testing.T) {
	assertion := assert.New(t)

	modified := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	files := []*fs.FileInfo{
		{Name: "backup-db.tar", ModifiedAt: modified},
		{Name: "unrelated.txt", ModifiedAt: modified},
	}

	fileDef := &backup.FileDefinition{
		Filter: regexp.MustCompile(`^backup-(.*)\.tar$`),
		// capture 0 (whole) + capture 1, both internal, no parser
		VariableMapping: []backup.VariableReference{{Offset: 0}, {Offset: 0}},
		SortBy:          backup.SortByInterpolation,
	}

	folderTime := backup.Timestamp{}
	matches := collectMatchingFiles(files, fileDef, nil, &folderTime, nil)

	assertion.Len(matches, 1)
	assertion.Equal("backup-db.tar", matches[0].File.Name)
	assertion.NotNil(matches[0].File.InterpolatedTimestamp)
}

// A capture group mapped to a user-defined variable must equal that
// variable's value, otherwise the file is excluded.
func TestCollectMatchingFiles_userVariableMustMatch(t *testing.T) {
	assertion := assert.New(t)

	files := []*fs.FileInfo{
		{Name: "backup-db.tar"},
		{Name: "backup-web.tar"},
	}

	fileDef := &backup.FileDefinition{
		Filter: regexp.MustCompile(`^backup-(.*)\.tar$`),
		// capture 1 refers to user variable at vars[0]
		VariableMapping: []backup.VariableReference{{Offset: 0}, {Offset: 1}},
		SortBy:          backup.SortByInterpolation,
	}

	folderTime := backup.Timestamp{}
	matches := collectMatchingFiles(files, fileDef, []string{"db"}, &folderTime, nil)

	assertion.Len(matches, 1)
	assertion.Equal("backup-db.tar", matches[0].File.Name)
}

// Conversion is applied to the user-variable value before comparison.
func TestCollectMatchingFiles_appliesConversion(t *testing.T) {
	assertion := assert.New(t)

	files := []*fs.FileInfo{{Name: "backup-DB.tar"}}

	fileDef := &backup.FileDefinition{
		Filter:          regexp.MustCompile(`^backup-(.*)\.tar$`),
		VariableMapping: []backup.VariableReference{{Offset: 0}, {Offset: 1, Conversion: func(s string) string { return "DB" }}},
		SortBy:          backup.SortByInterpolation,
	}

	folderTime := backup.Timestamp{}
	matches := collectMatchingFiles(files, fileDef, []string{"db"}, &folderTime, nil)

	assertion.Len(matches, 1)
	assertion.Equal("backup-DB.tar", matches[0].File.Name)
}

// SortByBornAt selects the file's BornAt as the temporal sort key.
func TestCollectMatchingFiles_sortByBornAt(t *testing.T) {
	assertion := assert.New(t)

	born := time.Date(2020, 5, 6, 7, 8, 9, 0, time.UTC)
	files := []*fs.FileInfo{{Name: "backup-db.tar", BornAt: born}}

	fileDef := &backup.FileDefinition{
		Filter:          regexp.MustCompile(`^backup-(.*)\.tar$`),
		VariableMapping: []backup.VariableReference{{Offset: 0}, {Offset: 0}},
		SortBy:          backup.SortByBornAt,
	}

	folderTime := backup.Timestamp{}
	matches := collectMatchingFiles(files, fileDef, nil, &folderTime, nil)

	assertion.Len(matches, 1)
	assertion.Equal(born, matches[0].Time)
}

// findMatchingFiles produces one FileGroup per file definition in the directory.
func TestFindMatchingFiles_oneGroupPerDefinition(t *testing.T) {
	assertion := assert.New(t)

	dir := &fs.DirectoryInfo{
		Name: "root",
		Files: []*fs.FileInfo{
			{Name: "db.sql"},
			{Name: "web.tar"},
		},
	}

	dirDef := &backup.Directory{
		Filter: backup.DirectoryFilter{},
		Files: []*backup.FileDefinition{
			{Filter: regexp.MustCompile(`^db\.sql$`), VariableMapping: []backup.VariableReference{{Offset: 0}}},
			{Filter: regexp.MustCompile(`^web\.tar$`), VariableMapping: []backup.VariableReference{{Offset: 0}}},
		},
	}

	groups := findMatchingFiles(dir, dirDef, nil)

	assertion.Len(groups, 2)
	assertion.Len(groups[0], 1)
	assertion.Equal("db.sql", groups[0][0].File.Name)
	assertion.Len(groups[1], 1)
	assertion.Equal("web.tar", groups[1][0].File.Name)
}

// findMatchingDirs descends through directory layers and collects files in
// the matched leaf directory, keyed by the assembled template path.
func TestFindMatchingDirs_descendsAndCollects(t *testing.T) {
	assertion := assert.New(t)

	leaf := &fs.DirectoryInfo{
		Name:  "01",
		Files: []*fs.FileInfo{{Name: "dump.sql"}},
	}
	root := &fs.DirectoryInfo{
		Name:    "db",
		SubDirs: map[string]*fs.DirectoryInfo{"01": leaf},
	}

	dirDef := &backup.Directory{
		Filter: backup.DirectoryFilter{
			Template:  []string{"", ""},
			Variables: []backup.VariableDefinition{{Name: "instance"}},
			Layers:    []*regexp.Regexp{regexp.MustCompile(`^(\d+)$`)},
		},
		Files: []*backup.FileDefinition{
			{Filter: regexp.MustCompile(`^dump\.sql$`), VariableMapping: []backup.VariableReference{{Offset: 0}}},
		},
	}

	fileGroups := make(FileLookup)
	vars := make([]string, 1)
	findMatchingDirs(root, dirDef, 0, 0, vars, fileGroups)

	// one captured directory named "01" -> path assembled from template
	group, exists := fileGroups["01"]
	assertion.True(exists)
	assertion.Len(group, 1)
	assertion.Len(group[0], 1)
	assertion.Equal("dump.sql", group[0][0].File.Name)
}

// findMatchingDirs ignores subdirectories that don't match the layer pattern.
func TestFindMatchingDirs_skipsNonMatching(t *testing.T) {
	assertion := assert.New(t)

	root := &fs.DirectoryInfo{
		Name: "db",
		SubDirs: map[string]*fs.DirectoryInfo{
			"not-a-number": {Name: "not-a-number", Files: []*fs.FileInfo{{Name: "dump.sql"}}},
		},
	}

	dirDef := &backup.Directory{
		Filter: backup.DirectoryFilter{
			Template:  []string{"", ""},
			Variables: []backup.VariableDefinition{{Name: "instance"}},
			Layers:    []*regexp.Regexp{regexp.MustCompile(`^(\d+)$`)},
		},
		Files: []*backup.FileDefinition{
			{Filter: regexp.MustCompile(`^dump\.sql$`), VariableMapping: []backup.VariableReference{{Offset: 0}}},
		},
	}

	fileGroups := make(FileLookup)
	vars := make([]string, 1)
	findMatchingDirs(root, dirDef, 0, 0, vars, fileGroups)

	assertion.Empty(fileGroups)
}
