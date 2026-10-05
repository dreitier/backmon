package backup

import (
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/dreitier/backmon/config"
	"github.com/stretchr/testify/assert"
)

func Test_parseQuota(t *testing.T) {
	assertion := assert.New(t)

	// empty quota -> zero, no error
	q, err := parseQuota(&RawDefinition{quota: ""})
	assertion.NoError(err)
	assertion.Equal(uint64(0), q)

	// valid quota -> parsed bytes
	q, err = parseQuota(&RawDefinition{quota: "2GiB"})
	assertion.NoError(err)
	assertion.Equal(uint64(2*1024*1024*1024), q)

	// invalid quota -> error and zero
	q, err = parseQuota(&RawDefinition{quota: "not-a-size"})
	assertion.Error(err)
	assertion.Equal(uint64(0), q)
}

func Test_parseSortBy(t *testing.T) {
	assertion := assert.New(t)

	assertion.Equal(SortByBornAt, parseSortBy("born_at"))
	assertion.Equal(SortByModifiedAt, parseSortBy("modified_at"))
	assertion.Equal(SortByArchivedAt, parseSortBy("archived_at"))
	assertion.Equal(SortByInterpolation, parseSortBy("interpolation"))
	assertion.Equal(SortByInterpolation, parseSortBy(""))
	// unknown falls back to interpolation
	assertion.Equal(SortByInterpolation, parseSortBy("bogus"))
}

func Test_parseVariableOperation(t *testing.T) {
	assertion := assert.New(t)

	lower := parseVariableOperation("lower")
	assertion.NotNil(lower)
	assertion.Equal("abc", lower("ABC"))

	upper := parseVariableOperation("upper")
	assertion.NotNil(upper)
	assertion.Equal("ABC", upper("abc"))

	assertion.Nil(parseVariableOperation(""))
	assertion.Nil(parseVariableOperation("unknown"))
}

func Test_parseTimestampExtraction(t *testing.T) {
	assertion := assert.New(t)

	cases := []struct {
		op    string
		input string
		check func(*Timestamp) bool
	}{
		{"year", "2023", func(ts *Timestamp) bool { return ts.year == 2023 && ts.flags&yearFlag != 0 }},
		{"month", "07", func(ts *Timestamp) bool { return ts.month == 7 && ts.flags&monthFlag != 0 }},
		{"day", "15", func(ts *Timestamp) bool { return ts.day == 15 && ts.flags&dayFlag != 0 }},
		{"hour", "09", func(ts *Timestamp) bool { return ts.hour == 9 && ts.flags&hourFlag != 0 }},
		{"minute", "45", func(ts *Timestamp) bool { return ts.minute == 45 && ts.flags&minuteFlag != 0 }},
		{"second", "30", func(ts *Timestamp) bool { return ts.second == 30 && ts.flags&secondFlag != 0 }},
	}

	for _, c := range cases {
		parser := parseTimestampExtraction(c.op)
		assertion.NotNil(parser, "parser for %q should not be nil", c.op)
		ts := &Timestamp{}
		parser(c.input, ts)
		assertion.True(c.check(ts), "parser for %q did not set the expected field", c.op)
	}

	assertion.Nil(parseTimestampExtraction("unknown"))
	assertion.Nil(parseTimestampExtraction(""))
}

func Test_retentionOrDefault(t *testing.T) {
	assertion := assert.New(t)

	// purge disabled -> values returned as-is
	count, age := retentionOrDefault(&RawFile{Purge: false, RetentionCount: 5, RetentionAge: 2 * time.Hour})
	assertion.Equal(uint64(5), count)
	assertion.Equal(2*time.Hour, age)

	// purge enabled, both count and age set -> returned as-is
	count, age = retentionOrDefault(&RawFile{Purge: true, RetentionCount: 4, RetentionAge: 3 * time.Hour})
	assertion.Equal(uint64(4), count)
	assertion.Equal(3*time.Hour, age)

	// purge enabled, only count set -> default age of a week
	count, age = retentionOrDefault(&RawFile{Purge: true, RetentionCount: 7})
	assertion.Equal(uint64(7), count)
	assertion.Equal(config.Week, age)

	// purge enabled, only age set -> default count of 3
	count, age = retentionOrDefault(&RawFile{Purge: true, RetentionAge: 5 * time.Hour})
	assertion.Equal(uint64(3), count)
	assertion.Equal(5*time.Hour, age)

	// purge enabled, nothing set -> defaults for both
	count, age = retentionOrDefault(&RawFile{Purge: true})
	assertion.Equal(uint64(3), count)
	assertion.Equal(config.Week, age)
}

func Test_Directory_MarshalJSON(t *testing.T) {
	assertion := assert.New(t)

	dir := &Directory{Alias: "my-backups"}
	data, err := json.Marshal(dir)
	assertion.NoError(err)
	assertion.JSONEq(`"my-backups"`, string(data))
}

func Test_FileDefinition_MarshalJSON(t *testing.T) {
	assertion := assert.New(t)

	file := &FileDefinition{Alias: "pgdump"}
	data, err := json.Marshal(file)
	assertion.NoError(err)
	assertion.JSONEq(`"pgdump"`, string(data))
}

func Test_DirectoryFilter_MarshalJSON(t *testing.T) {
	assertion := assert.New(t)

	filter := &DirectoryFilter{
		Variables: []VariableDefinition{
			{Name: "service"},
			{Name: "fused", Fuse: true},
			{Name: "instance"},
		},
		Layers: []*regexp.Regexp{},
	}

	data, err := json.Marshal(filter)
	assertion.NoError(err)
	// fused variables are omitted from the JSON output
	assertion.JSONEq(`["service","instance"]`, string(data))
}
