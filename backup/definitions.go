package backup

import (
	"bytes"
	"github.com/dreitier/cloudmon/config"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorhill/cronexpr"
	log "github.com/sirupsen/logrus"
	"io"
	"regexp"
	"strings"
	"time"
)

const (
	variableValueSyntax = `[^\\./]+?`
	// The substitution marker must be an ASCII character and
	// must not be a regex meta character
	SubstitutionMarker = '%'
)

const (
	SORT_BY_INTERPOLATION = iota
	SORT_BY_BORN_AT = iota
	SORT_BY_MODIFIED_AT = iota
	SORT_BY_ARCHIVED_AT = iota
)

//noinspection RegExpRedundantEscape
var (
	variableDefExp    = regexp.MustCompile(`\\\{\\\{(?P<var>\w+)\\\}\\\}`)
	variableDefExpRaw = regexp.MustCompile(`\{\{(?P<var>\w+)\}\}`)
	variableExp       = regexp.MustCompile(`\\\$\\\{(?P<var>\w+)(?:\:(?P<op>[a-zA-Z]*))?\\\}`)
)

func ParseDefinition(definitionsReader io.Reader) (Definition, error) {
	raw, err := ParseRawDefinitions(definitionsReader)

	if err != nil {
		return nil, err
	}
	
	return parseDirectories(raw)
}

func parseDirectories(raw RawDefinition) ([]*Directory, error) {
	r := make([]*Directory, 0, len(raw))
	aliases := make(map[string]empty)

	for rawPattern, rawDir := range raw {
		filter, variableOffsets := ParsePathPattern(rawPattern)

		if err := applyFusion(filter.Variables, rawDir.FuseVars); err != nil {
			return nil, err
		}

		var alias string
		var safeAlias string
		
		if rawDir.Alias != "" {
			alias = rawDir.Alias
			var legal bool
			safeAlias, legal = MakeLegalAlias(alias)

			if !legal {
				log.Warnf("The directory alias '%s' contained non-url characters, its name will be '%s' in urls", rawDir.Alias, safeAlias)
			}
		} else {
			alias = rawPattern
			safeAlias, _ = MakeLegalAlias(alias)
		}

		if _, exists := aliases[alias]; exists {
			return nil, errors.New(fmt.Sprintf(
				"Cannot have multiple directory definitions with the alias: '%s'", alias))
		} else {
			aliases[alias] = empty{}
		}

		r = append(r, &Directory{
			Alias:     alias,
			SafeAlias: safeAlias,
			Filter:    filter,
			Files:     parseFiles(rawDir.Files, variableOffsets),
		})
	}

	return r, nil
}

func applyFusion(variables []VariableDefinition, fuseVars []string) error {
	for _, fuseVar := range fuseVars {
		match := false

		for i := 0; i < len(variables); i++ {
			if variables[i].Name == fuseVar {
				variables[i].Fuse = true
				match = true
			}
		}

		if !match {
			return errors.New(fmt.Sprintf("cannot fuse values of undefined variable '%s'", fuseVar))
		}
	}

	return nil
}

func parseFiles(raw map[string]*RawFile, variableOffsets map[string]uint) []*BackupFileDefinition {
	files := make([]*BackupFileDefinition, 0, len(raw))
	aliases := make(map[string]empty)
	
	for rawPattern, rawFile := range raw {
		pattern, err := ParseFilePattern(rawPattern)

		if err != nil {
			log.Errorf("Could not parse File pattern '%s': %v.", rawPattern, err)
			continue
		}

		variables, err := parseVariables(pattern, variableOffsets)

		if err != nil {
			log.Errorf("Could not parse File pattern '%s': %v.", rawPattern, err)
			continue
		}

		retentionCount, retentionAge := retentionOrDefault(rawFile)

		sortBy := parseSortBy(rawFile.Sort)

		var alias string
		var safeAlias string

		if rawFile.Alias != "" {
			alias = rawFile.Alias
			var legal bool
			safeAlias, legal = MakeLegalAlias(alias)

			if !legal {
				log.Warnf("The file alias '%s' contained non-url characters, its name will be '%s' in urls", rawFile.Alias, safeAlias)
			}
		} else {
			alias = rawPattern
			safeAlias, _ = MakeLegalAlias(alias)
		}

		if _, exists := aliases[alias]; exists {
			log.Errorf("Cannot have multiple file definitions with the alias: '%s'", alias)
			continue
		}

		aliases[alias] = empty{}

		file := &BackupFileDefinition{
			Pattern:         rawPattern,
			Filter:          pattern,
			VariableMapping: variables,
			Alias:           alias,
			SafeAlias:       safeAlias,
			Schedule:        rawFile.Schedule,
			SortBy:	         sortBy,
			Purge:           rawFile.Purge,
			RetentionCount:  retentionCount,
			RetentionAge:    retentionAge,
		}

		files = append(files, file)
	}

	return files
}

func parseVariables(pattern *regexp.Regexp, variableOffsets map[string]uint) ([]VariableReference, error) {
	subexpNames := pattern.SubexpNames()
	variables := make([]VariableReference, len(subexpNames))

	for i, capture := range subexpNames {
		op := ""

		if !strings.HasPrefix(capture, "_") {
			split := strings.Index(capture, "_")

			if split == -1 {
				variables[i] = VariableReference{
					Parser: parseTimestampExtraction(capture),
				}

				continue
			}

			op = capture[:split]
			capture = capture[split+1:]
		}

		offset, exists := variableOffsets[capture]

		if !exists {
			return nil, fmt.Errorf("use of undefined variable '%s'", capture[:])
		}

		variables[i] = VariableReference{
			Offset:     offset,
			Conversion: parseVariableOperation(op),
		}
	}

	return variables, nil
}

func parseSortBy(op string) int {
	switch op {
	case "born_at":
		return SORT_BY_BORN_AT
	case "modified_at":
		return SORT_BY_MODIFIED_AT
	case "archived_at":
		return SORT_BY_ARCHIVED_AT
	case "interpolation":
		return SORT_BY_INTERPOLATION
	case "":
		return SORT_BY_INTERPOLATION
	default: 
		log.Warnf("Unknown 'sort' parameter '%s', defaulting to 'interpolation'", op)
		return SORT_BY_INTERPOLATION
	}
}

func parseVariableOperation(op string) func(string) string {
	switch op {
	case "lower":
		return strings.ToLower
	case "upper":
		return strings.ToUpper
	case "":
		return nil
	default:
		log.Warnf("Unknown operation '%s', defaulting to no op.", op)
		return nil
	}
}

func parseTimestampExtraction(op string) TimeParser {
	switch op {
	case "year":
		return extractYear
	case "month":
		return extractMonth
	case "day":
		return extractDay
	case "hour":
		return extractHour
	case "minute":
		return extractMinute
	case "second":
		return extractSecond
	default:
		return nil
	}
}

func retentionOrDefault(file *RawFile) (uint64, time.Duration) {
	if !file.Purge {
		return file.RetentionCount, file.RetentionAge
	}

	if file.RetentionCount > 0 {
		if file.RetentionAge > 0 {
			return file.RetentionCount, file.RetentionAge
		}

		return file.RetentionCount, config.Week
	}

	if file.RetentionAge > 0 {
		return 3, file.RetentionAge
	}

	log.Warn("Purge is enabled, but no retention is specified; defaulting to 'count: 3' and 'age: 7d'")

	return 3, config.Week
}

func ParseDirectoryPattern(pattern string) (*regexp.Regexp, error) {
	pattern = regexp.QuoteMeta(pattern)

	if strings.HasSuffix(pattern, "/") {
		pattern = strings.TrimSuffix(pattern, "/")
		pattern = pattern + "?/"
	}

	//Make sure to always match the whole string
	pattern = "^" + pattern + "$"

	expression := variableDefExp.ReplaceAllString(pattern, `(?P<_${var}>`+variableValueSyntax+`)`)

	return regexp.Compile(expression)
}

func ParsePathPattern(pattern string) (filter DirectoryFilter, variableOffsets map[string]uint) {
	normalized := strings.Trim(pattern, `/`)

	if len(normalized) == 0 || normalized == "." {
		// The pattern refers to the disk root
		normalized = "."
		filter = DirectoryFilter{
			Pattern:   normalized,
			Template:  []string{ normalized },
			Layers:    nil,
			Variables: nil,
		}

		return filter, nil
	}

	if strings.HasPrefix(normalized, "./") {
		normalized = normalized[len("./"):]
	}

	captures, leftovers := splitPattern(normalized)

	variableOffsets = make(map[string]uint)
	filters := make([]*regexp.Regexp, 0, strings.Count(normalized, "/")+1)
	var template []string
	var variableDefinitions []VariableDefinition
	offset := uint(1)
	expr := strings.Builder{}

	expr.WriteString("^")

	for i, literal := range leftovers {
		segment := literal
		varCount := len(variableDefinitions)
		slash := strings.Index(segment, "/")

		for ; slash >= 0; slash = strings.Index(segment, "/") {
			vars := ExpandSubstitutionsInto(regexp.QuoteMeta(segment[:slash]), &expr)
			variableDefinitions = append(variableDefinitions, vars...)
			offset += uint(len(vars))

			expr.WriteString("$")
			filters = append(filters, regexp.MustCompile(expr.String()))

			expr.Reset()
			expr.WriteString("^")
			segment = segment[slash+1:]
		}

		vars := ExpandSubstitutionsInto(regexp.QuoteMeta(segment), &expr)
		variableDefinitions = append(variableDefinitions, vars...)
		template = appendToTemplate(template, literal, variableDefinitions[varCount:])
		offset += uint(len(vars))

		if i < len(captures) {
			variableDefinitions = append(variableDefinitions, VariableDefinition{
				Name: captures[i],
			})
			variableOffsets[captures[i]] = offset
			offset++

			expr.WriteString(`(?P<_`)
			expr.WriteString(captures[i])
			expr.WriteString(`>` + variableValueSyntax + `)`)
		}
	}

	expr.WriteString("$")
	filters = append(filters, regexp.MustCompile(expr.String()))

	filter = DirectoryFilter{
		Pattern:   normalized,
		Template:  template,
		Layers:    filters,
		Variables: variableDefinitions,
	}

	return filter, variableOffsets
}

func splitPattern(pattern string) (captures []string, leftovers []string) {
	if pattern == "" {
		return nil, nil
	}

	matches := variableDefExpRaw.FindAllStringSubmatchIndex(pattern, -1)
	captures = make([]string, len(matches))
	leftovers = make([]string, len(matches)+1)
	last := 0

	for i, match := range matches {
		leftovers[i] = pattern[last:match[0]]
		last = match[1]
		captures[i] = pattern[match[2]:match[3]]
	}

	leftovers[len(matches)] = pattern[last:]

	return captures, leftovers
}

func appendToTemplate(template []string, fragment string, substitutions []VariableDefinition) []string {
	offset := 0

	for _, sub := range substitutions {
		offset = strings.Index(fragment, sub.Name)
		template = append(template, fragment[:offset])
		fragment = fragment[offset+len(sub.Name):]
	}

	template = append(template, fragment)

	return template
}

func ExpandSubstitutions(input string) (expanded string, captures []VariableDefinition) {
	var i int

	for i = 0; i < len(input); i++ {
		if input[i] == SubstitutionMarker {
			break
		}
	}

	i++

	if i >= len(input) {
		//Nothing to substitute, just return the input as is
		return input, nil
	}

	text := strings.Builder{}
	captures = ExpandSubstitutionsInto(input, &text)

	return text.String(), captures
}

func ExpandSubstitutionsInto(input string, text *strings.Builder) (captures []VariableDefinition) {
	text.Grow(len(input))
	substitute := false

	for i := 0; i < len(input); i++ {
		if substitute {
			substitute = false
			captureAdded, parser := writeSubstitutionInto(input[i], text)
			if captureAdded {
				captures = append(captures, VariableDefinition{
					Name:   input[i-1 : i+1],
					Parser: parser,
				})
			}
		} else if input[i] == SubstitutionMarker {
			substitute = true
		} else {
			//TODO: write unsubstituted characters in batches
			text.WriteByte(input[i])
		}
	}

	if substitute {
		//The last character was a single '%'
		text.WriteByte(SubstitutionMarker)
	}

	return captures
}

func writeSubstitutionInto(character byte, to *strings.Builder) (captureAdded bool, parser TimeParser) {
	//All these cases need to be characters whose encoding is a single byte in UTF-8,
	// this means they need to be ASCII characters (codepoint <= 0x80)
	switch character {
	case SubstitutionMarker:
		to.WriteByte(SubstitutionMarker)
	case 'Y':
		to.WriteString("(?P<year>[0-9]{4})")
		parser = extractYear
	case 'y':
		to.WriteString("(?P<year>[0-9]{2})")
		parser = extractYear
	case 'M':
		to.WriteString("(?P<month>0[1-9]|1[0-2])")
		parser = extractMonth
	case 'D':
		to.WriteString("(?P<day>0[1-9]|[1,2][0-9]|3[0,1])")
		parser = extractDay
	case 'h':
		to.WriteString("(?P<hour>[0,1][0-9]|2[0-3])")
		parser = extractHour
	case 'm':
		to.WriteString("(?P<minute>[0-5][0-9])")
		parser = extractMinute
	case 's':
		to.WriteString("(?P<second>[0-5][0-9])")
		parser = extractSecond
	case 'i':
		to.WriteString("(0|[1-9][0-9]*)")
	case 'I':
		to.WriteString("([0-9]+)")
	case 'x':
		to.WriteString("([0-9a-f]+)")
	case 'X':
		to.WriteString("([0-9A-F]+)")
	case 'w':
		to.WriteString("(\\w+)")
	case 'v':
		to.WriteString("(" + variableValueSyntax + ")")
	case '?':
		to.WriteString("(.+?)")
	default:
		//The given character is not a valid substitute,
		// emit a warning and output nothing
		log.Warnf("'%%%s' is not a valid substitution, ignoring it", string(character))
		return false, nil
	}
	return true, parser
}

func ParseFilePattern(pattern string) (*regexp.Regexp, error) {
	pattern = regexp.QuoteMeta(pattern)

	//Make sure to always match the whole string
	pattern = "^" + pattern + "$"

	expression := variableExp.ReplaceAllString(pattern, `(?P<${op}_${var}>`+variableValueSyntax+`)`)
	//TODO: make use of our local knowledge of the variables
	expression, _ = ExpandSubstitutions(expression)

	return regexp.Compile(expression)
}

type Definition []*Directory

type Directory struct {
	Alias        string
	SafeAlias    string
	Filter       DirectoryFilter
	Files        []*BackupFileDefinition
	ActiveGroups []string
}

func (dir *Directory) MarshalJSON() ([]byte, error) {
	return json.Marshal(dir.Alias)
}

type DirectoryFilter struct {
	Pattern   string
	Template  []string
	Layers    []*regexp.Regexp
	Variables []VariableDefinition
}

func (filter *DirectoryFilter) MarshalJSON() ([]byte, error) {
	text := bytes.Buffer{}
	text.WriteByte('[')
	first := true

	for _, v := range filter.Variables {
		if v.Fuse {
			continue
		}

		if first {
			first = false
		} else {
			text.WriteByte(',')
		}

		data, _ := json.Marshal(v.Name)
		text.Write(data)
	}

	text.WriteByte(']')
	
	return text.Bytes(), nil
}

type VariableDefinition struct {
	Name   string
	Parser TimeParser
	Fuse   bool
}

type BackupFileDefinition struct {
	Pattern         string
	Filter          *regexp.Regexp
	VariableMapping []VariableReference
	Alias           string
	SafeAlias       string
	Schedule        *cronexpr.Expression
	SortBy          int
	Purge           bool
	RetentionCount  uint64
	RetentionAge    time.Duration
}

func (file *BackupFileDefinition) MarshalJSON() ([]byte, error) {
	return json.Marshal(file.Alias)
}

type VariableReference struct {
	Offset     uint
	Conversion func(string) string
	Parser     TimeParser
}

type empty struct{}
