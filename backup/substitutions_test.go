package backup

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ExpandSubstitutions should leave an input without a substitution marker
// untouched and report no captures.
func TestExpandSubstitutions_NoMarker(t *testing.T) {
	expanded, captures := ExpandSubstitutions("plain-text-no-marker")

	assert.Equal(t, "plain-text-no-marker", expanded)
	assert.Nil(t, captures)
}

// The timestamp substitutions each expand to a named capture group and attach
// the matching TimeParser to the recorded VariableDefinition.
func TestExpandSubstitutions_TimestampParsers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expanded string
	}{
		{"year", "%Y", "(?P<year>[0-9]{4})"},
		{"year_short", "%y", "(?P<year_short>[0-9]{2})"},
		{"month", "%M", "(?P<month>0[1-9]|1[0-2])"},
		{"day", "%D", "(?P<day>0[1-9]|[1,2][0-9]|3[0,1])"},
		{"hour", "%h", "(?P<hour>[0,1][0-9]|2[0-3])"},
		{"minute", "%m", "(?P<minute>[0-5][0-9])"},
		{"second", "%s", "(?P<second>[0-5][0-9])"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expanded, captures := ExpandSubstitutions(tc.input)

			assert.Equal(t, tc.expanded, expanded)
			assert.Len(t, captures, 1)
			assert.Equal(t, tc.input, captures[0].Name)
			assert.NotNil(t, captures[0].Parser, "a timestamp substitution must carry a parser")
		})
	}
}

// The non-timestamp substitutions expand to a capture group but do not attach a
// parser; the capture is still recorded.
func TestExpandSubstitutions_NonTimestampSubstitutions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expanded string
	}{
		{"unsigned_int", "%i", "(0|[1-9][0-9]*)"},
		{"int", "%I", "([0-9]+)"},
		{"hex_lower", "%x", "([0-9a-f]+)"},
		{"hex_upper", "%X", "([0-9A-F]+)"},
		{"word", "%w", `(\w+)`},
		{"variable_value", "%v", `([^\\./]+?)`},
		{"any", "%A", "(.+?)"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expanded, captures := ExpandSubstitutions(tc.input)

			assert.Equal(t, tc.expanded, expanded)
			assert.Len(t, captures, 1)
			assert.Nil(t, captures[0].Parser, "a non-timestamp substitution has no parser")
		})
	}
}

// A doubled marker is an escaped literal '%' in the output.
func TestExpandSubstitutions_EscapedMarker(t *testing.T) {
	expanded, captures := ExpandSubstitutions("a%%b")

	assert.Equal(t, "a%b", expanded)
	assert.Len(t, captures, 1)
}

// An unrecognized substitution character is dropped and records no capture.
func TestExpandSubstitutions_InvalidSubstitution(t *testing.T) {
	expanded, captures := ExpandSubstitutions("a%zb")

	assert.Equal(t, "ab", expanded)
	assert.Nil(t, captures)
}

// A lone trailing marker is emitted verbatim. This branch is only reachable via
// ExpandSubstitutionsInto, since ExpandSubstitutions short-circuits a trailing
// marker as "nothing to substitute".
func TestExpandSubstitutionsInto_TrailingMarker(t *testing.T) {
	var text strings.Builder
	captures := ExpandSubstitutionsInto("a%", &text)

	assert.Equal(t, "a%", text.String())
	assert.Nil(t, captures)
}

// appendToTemplate splits the fragment around each substitution's name,
// appending the literal pieces between them.
func TestAppendToTemplate_SplitsAroundSubstitutions(t *testing.T) {
	template := appendToTemplate(nil, "backup-%Y-%M-end", []VariableDefinition{
		{Name: "%Y"},
		{Name: "%M"},
	})

	assert.Equal(t, []string{"backup-", "-", "-end"}, template)
}

// With no substitutions the whole fragment is appended as a single literal.
func TestAppendToTemplate_NoSubstitutions(t *testing.T) {
	template := appendToTemplate([]string{"prefix"}, "literal", nil)

	assert.Equal(t, []string{"prefix", "literal"}, template)
}

// parseVariables resolves an operation-prefixed capture against the offset map
// and wires up the conversion function.
func TestParseVariables_OperationAndOffset(t *testing.T) {
	pattern := regexp.MustCompile(`(?P<upper_myvar>x)`)
	offsets := map[string]uint{"myvar": 3}

	variables, err := parseVariables(pattern, offsets)

	assert.NoError(t, err)
	// index 0 is the whole-match unnamed group
	assert.Len(t, variables, 2)
	assert.Equal(t, uint(3), variables[1].Offset)
	assert.Equal(t, "AB", variables[1].Conversion("ab"))
}

// parseVariables errors when an operation-prefixed capture names a variable that
// was never defined.
func TestParseVariables_UndefinedVariable(t *testing.T) {
	pattern := regexp.MustCompile(`(?P<op_undefined>x)`)

	variables, err := parseVariables(pattern, map[string]uint{})

	assert.Nil(t, variables)
	assert.ErrorContains(t, err, "use of undefined variable 'undefined'")
}

// A bare named capture (no operation prefix, no underscore) is treated as a
// timestamp-extraction reference.
func TestParseVariables_BareTimestampCapture(t *testing.T) {
	pattern := regexp.MustCompile(`(?P<year>[0-9]{4})`)

	variables, err := parseVariables(pattern, map[string]uint{})

	assert.NoError(t, err)
	assert.Len(t, variables, 2)
	assert.NotNil(t, variables[1].Parser)
}
