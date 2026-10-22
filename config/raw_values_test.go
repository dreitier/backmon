package config

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Raw_Sub_returnsNestedMap(t *testing.T) {
	assertion := assert.New(t)

	// map[string]interface{} branch
	raw := Raw{"outer": map[string]interface{}{"inner": "value"}}
	sub := raw.Sub("outer")
	assertion.NotNil(sub)
	assertion.Equal("value", sub.String("inner"))
}

func Test_Raw_Sub_convertsMapInterfaceKeys(t *testing.T) {
	assertion := assert.New(t)

	// map[interface{}]interface{} branch (as produced by some YAML decoders)
	raw := Raw{"outer": map[interface{}]interface{}{"inner": "value", 42: "ignored"}}
	sub := raw.Sub("outer")
	assertion.NotNil(sub)
	assertion.Equal("value", sub.String("inner"))
}

func Test_Raw_Sub_missingOrNonMapReturnsNil(t *testing.T) {
	assertion := assert.New(t)

	raw := Raw{"scalar": "not a map"}
	assertion.Nil(raw.Sub("absent"))
	assertion.Nil(raw.Sub("scalar"))
}

func Test_Raw_Has(t *testing.T) {
	assertion := assert.New(t)

	raw := Raw{"present": nil}
	assertion.True(raw.Has("present"))
	assertion.False(raw.Has("absent"))
}

func Test_Raw_StringSlice(t *testing.T) {
	assertion := assert.New(t)

	// []string branch
	raw := Raw{"items": []string{"a", "b"}}
	assertion.Equal([]string{"a", "b"}, raw.StringSlice("items"))

	// []interface{} branch, empty strings dropped
	raw = Raw{"items": []interface{}{"a", "", 7}}
	assertion.Equal([]string{"a", "7"}, raw.StringSlice("items"))

	// missing key
	assertion.Nil(Raw{}.StringSlice("absent"))
}

func Test_Raw_Bool(t *testing.T) {
	assertion := assert.New(t)

	assertion.True(Raw{"k": true}.Bool("k"))
	assertion.True(Raw{"k": "true"}.Bool("k"))
	assertion.False(Raw{"k": "false"}.Bool("k"))
	assertion.False(Raw{"k": "not-a-bool"}.Bool("k"))
	assertion.False(Raw{}.Bool("absent"))
}

func Test_Raw_Uint64(t *testing.T) {
	assertion := assert.New(t)

	assertion.Equal(uint64(5), Raw{"k": 5}.Uint64("k"))
	assertion.Equal(uint64(42), Raw{"k": "42"}.Uint64("k"))
	assertion.Equal(uint64(0), Raw{"k": "nope"}.Uint64("k"))
	assertion.Equal(uint64(0), Raw{}.Uint64("absent"))
}

func Test_Raw_Int64(t *testing.T) {
	assertion := assert.New(t)

	assertion.Equal(int64(-3), Raw{"k": -3}.Int64("k"))
	assertion.Equal(int64(99), Raw{"k": "99"}.Int64("k"))
	assertion.Equal(int64(0), Raw{"k": "nope"}.Int64("k"))
}

func Test_Raw_Bytes(t *testing.T) {
	assertion := assert.New(t)

	// human-readable unit suffix
	assertion.Equal(uint64(1024), Raw{"k": "1K"}.Bytes("k"))
	assertion.Equal(uint64(2*1024*1024), Raw{"k": "2 MB"}.Bytes("k"))
	// plain numeric string
	assertion.Equal(uint64(500), Raw{"k": "500"}.Bytes("k"))
	// non-string falls back to asUint64
	assertion.Equal(uint64(7), Raw{"k": 7}.Bytes("k"))
	// nil / unparseable
	assertion.Equal(uint64(0), Raw{}.Bytes("absent"))
	assertion.Equal(uint64(0), Raw{"k": "not-bytes"}.Bytes("k"))
}

func Test_Raw_Duration_fromString(t *testing.T) {
	assertion := assert.New(t)

	assertion.Equal(Day, Raw{"k": "1d"}.Duration("k"))
	assertion.Equal(90*time.Minute, Raw{"k": "1h30m"}.Duration("k"))
	assertion.Equal(Year+Month+Week+Day+Hour+Minute+Second, Raw{"k": "1Y1M1w1d1h1m1s"}.Duration("k"))
	// unparseable string -> 0
	assertion.Equal(time.Duration(0), Raw{"k": "garbage"}.Duration("k"))
}

func Test_Raw_Duration_fromNumberIsDays(t *testing.T) {
	assertion := assert.New(t)

	// non-string numeric is interpreted as a number of days
	assertion.Equal(3*Day, Raw{"k": 3}.Duration("k"))
	// missing key
	assertion.Equal(time.Duration(0), Raw{}.Duration("absent"))
}

func Test_asDuration(t *testing.T) {
	assertion := assert.New(t)

	assertion.Equal(time.Duration(12), asDuration("12"))
	assertion.Equal(time.Duration(0), asDuration(""))
	assertion.Equal(time.Duration(0), asDuration("x"))
}

func Test_asString(t *testing.T) {
	assertion := assert.New(t)

	assertion.Equal("", asString(nil))
	assertion.Equal("hello", asString("hello"))
	assertion.Equal("42", asString(42))
	// fmt.Stringer branch
	assertion.Equal("1s", asString(time.Second))
}

func Test_asUint64_coversNumericKinds(t *testing.T) {
	assertion := assert.New(t)

	assertion.Equal(uint64(0), asUint64(nil))
	assertion.Equal(uint64(1), asUint64(int8(1)))
	assertion.Equal(uint64(2), asUint64(int16(2)))
	assertion.Equal(uint64(3), asUint64(int32(3)))
	assertion.Equal(uint64(4), asUint64(int64(4)))
	assertion.Equal(uint64(5), asUint64(int(5)))
	assertion.Equal(uint64(6), asUint64(uint8(6)))
	assertion.Equal(uint64(7), asUint64(uint16(7)))
	assertion.Equal(uint64(8), asUint64(uint32(8)))
	assertion.Equal(uint64(9), asUint64(uint64(9)))
	assertion.Equal(uint64(10), asUint64(uint(10)))
	assertion.Equal(uint64(11), asUint64("11"))
	assertion.Equal(uint64(0), asUint64("nope"))
	assertion.Equal(uint64(0), asUint64(1.5))
}

func Test_asInt64_coversNumericKinds(t *testing.T) {
	assertion := assert.New(t)

	assertion.Equal(int64(0), asInt64(nil))
	assertion.Equal(int64(1), asInt64(int8(1)))
	assertion.Equal(int64(2), asInt64(int16(2)))
	assertion.Equal(int64(3), asInt64(int32(3)))
	assertion.Equal(int64(4), asInt64(int64(4)))
	assertion.Equal(int64(5), asInt64(int(5)))
	assertion.Equal(int64(6), asInt64(uint8(6)))
	assertion.Equal(int64(7), asInt64(uint16(7)))
	assertion.Equal(int64(8), asInt64(uint32(8)))
	assertion.Equal(int64(9), asInt64(uint64(9)))
	assertion.Equal(int64(10), asInt64(uint(10)))
	assertion.Equal(int64(-11), asInt64("-11"))
	assertion.Equal(int64(0), asInt64("nope"))
	assertion.Equal(int64(0), asInt64(1.5))
}

func Test_asBool(t *testing.T) {
	assertion := assert.New(t)

	assertion.False(asBool(nil))
	assertion.True(asBool(true))
	assertion.False(asBool(false))
	assertion.True(asBool("1"))
	assertion.False(asBool("0"))
	assertion.False(asBool("maybe"))
	assertion.False(asBool(123))
}

func Test_ParseFromString(t *testing.T) {
	assertion := assert.New(t)

	raw, err := ParseFromString("key: value\nnested:\n  inner: 1\n")
	assertion.NoError(err)
	assertion.Equal("value", raw.String("key"))
	assertion.Equal(uint64(1), raw.Sub("nested").Uint64("inner"))
}

func Test_Parse_fromReader(t *testing.T) {
	assertion := assert.New(t)

	raw, err := Parse(strings.NewReader("key: value\n"))
	assertion.NoError(err)
	assertion.Equal("value", raw.String("key"))

	// invalid YAML surfaces an error
	_, err = Parse(strings.NewReader("key: : :\n"))
	assertion.Error(err)
}
