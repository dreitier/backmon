package config

import (
	"code.cloudfoundry.org/bytefmt"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	Second = time.Second
	Minute = time.Minute
	Hour   = time.Hour
	Day    = 24 * Hour
	Week   = 7 * Day
	Month  = 30 * Day
	Year   = 365 * Day
)

var (
	durationExpr = regexp.MustCompile(`^` +
		`(?:(?P<year>[0-9]+)Y)?\s*` +
		`(?:(?P<month>[0-9]+)M)?\s*` +
		`(?:(?P<week>[0-9]+)[wW])?\s*` +
		`(?:(?P<day>[0-9]+)[dD])?\s*` +
		`(?:(?P<hour>[0-9]+)h)?\s*` +
		`(?:(?P<minute>[0-9]+)m)?\s*` +
		`(?:(?P<second>[0-9]+)s)?$`)
)

type Raw map[string]interface{}

// ParseFromString Provide a YAML string and unmarshal it
func ParseFromString(content string) (Raw, error) {
	var out map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &out); err != nil {
		return nil, err
	}
	return out, nil

}

func Parse(reader io.Reader) (Raw, error) {
	var out map[string]interface{}
	if err := yaml.NewDecoder(reader).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c Raw) Sub(key string) Raw {
	val := c[key]
	if val == nil {
		return nil
	}
	if reflect.TypeOf(val).Kind() == reflect.Map {
		switch v := val.(type) {
		case map[interface{}]interface{}:
			var sub = map[string]interface{}{}
			for key, elem := range v {
				if s, ok := key.(string); ok {
					sub[s] = elem
				}
			}
			return sub
		case map[string]interface{}:
			return v
		}
	}
	return nil
}

func (c Raw) Has(key string) bool {
	_, exists := c[key]
	return exists
}

func (c Raw) String(key string) string {
	return interpolate(asString(c[key]))
}

func (c Raw) StringSlice(key string) []string {
	val := c[key]
	if val == nil {
		return nil
	}
	if s, ok := val.([]string); ok {
		return s
	}
	if s, ok := val.([]interface{}); ok {
		slice := make([]string, 0, len(s))
		for _, raw := range s {
			if elem := asString(raw); elem != "" {
				slice = append(slice, elem)
			}
		}
		return slice
	}
	return nil
}

func (c Raw) Bool(key string) bool {
	return asBool(c[key])
}

func (c Raw) Uint64(key string) uint64 {
	return asUint64(c[key])
}

func (c Raw) Int64(key string) int64 {
	return asInt64(c[key])
}

func (c Raw) Bytes(key string) uint64 {
	val := c[key]
	if val == nil {
		return 0
	}
	s, ok := val.(string)
	if !ok {
		return asUint64(val)
	}

	s = strings.ToUpper(s)
	i := strings.IndexFunc(s, unicode.IsLetter)

	if i >= 0 {
		s = strings.Replace(s, " ", "", -1)
		bytes, err := bytefmt.ToBytes(s)
		if err == nil {
			return bytes
		}
	}
	parsed, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func (c Raw) Duration(key string) time.Duration {
	val := c[key]
	if val == nil {
		return 0
	}
	s, ok := val.(string)
	if !ok {
		return Day * time.Duration(asUint64(val))
	}

	match := durationExpr.FindStringSubmatch(s)
	if match == nil {
		return 0
	}

	duration := time.Duration(0)
	// match[0] is the match for the whole regex
	duration += Year * asDuration(match[1])
	duration += Month * asDuration(match[2])
	duration += Week * asDuration(match[3])
	duration += Day * asDuration(match[4])
	duration += Hour * asDuration(match[5])
	duration += Minute * asDuration(match[6])
	duration += Second * asDuration(match[7])

	return duration
}

func asDuration(val string) time.Duration {
	i, err := strconv.ParseUint(val, 10, 63)
	if err != nil {
		return 0
	}
	return time.Duration(i)
}

func asString(val interface{}) string {
	if val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	if s, ok := val.(fmt.Stringer); ok && s != nil {
		return s.String()
	}
	return fmt.Sprintf("%v", val)
}

func asUint64(val interface{}) uint64 {
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case int8:
		return uint64(v)
	case int16:
		return uint64(v)
	case int32:
		return uint64(v)
	case int64:
		return uint64(v)
	case int:
		return uint64(v)
	case uint8:
		return uint64(v)
	case uint16:
		return uint64(v)
	case uint32:
		return uint64(v)
	case uint64:
		return v
	case uint:
		return uint64(v)
	case string:
		i, err := strconv.ParseUint(v, 10, 64)
		if err == nil {
			return i
		}
	}
	return 0
}

func asInt64(val interface{}) int64 {
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case uint8:
		return int64(v)
	case uint16:
		return int64(v)
	case uint32:
		return int64(v)
	case uint64:
		return int64(v)
	case uint:
		return int64(v)
	case string:
		i, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			return i
		}
	}
	return 0
}

func asBool(val interface{}) bool {
	if val == nil {
		return false
	}
	if b, ok := val.(bool); ok {
		return b
	}
	if s, ok := val.(string); ok {
		b, err := strconv.ParseBool(s)
		if err == nil {
			return b
		}
	}
	return false
}

func interpolate(s string) string {
	r := regexp.MustCompile(`^__\${(\w+)}__$`)
	m := r.FindStringSubmatch(s)

	if len(m) <= 1 {
		return s
	}

	v := os.Getenv(m[1])

	return v
}
