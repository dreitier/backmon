package backup

import (
	"strconv"
	"strings"
	"time"
)

const (
	yearFlag = 1 << iota
	monthFlag
	dayFlag
	hourFlag
	minuteFlag
	secondFlag
)

type Timestamp struct {
	year   uint16
	month  uint8
	day    uint8
	hour   uint8
	minute uint8
	second uint8
	flags  uint8
}

func (t Timestamp) ToTime() time.Time {
	return time.Date(
		int(t.year),
		time.Month(t.month),
		int(t.day),
		int(t.hour),
		int(t.minute),
		int(t.second),
		0,
		time.UTC)
}

func (t Timestamp) TimeWithDefaults(defaults time.Time) time.Time {
	defaults = defaults.UTC()
	if (t.flags & yearFlag) == 0 {
		t.year = uint16(defaults.Year())
	}
	if (t.flags & monthFlag) == 0 {
		t.month = uint8(defaults.Month())
	}
	if (t.flags & dayFlag) == 0 {
		t.day = uint8(defaults.Day())
	}
	if (t.flags & hourFlag) == 0 {
		t.hour = uint8(defaults.Hour())
	}
	if (t.flags & minuteFlag) == 0 {
		t.minute = uint8(defaults.Minute())
	}
	if (t.flags & secondFlag) == 0 {
		t.second = uint8(defaults.Second())
	}
	return time.Date(
		int(t.year),
		time.Month(t.month),
		int(t.day),
		int(t.hour),
		int(t.minute),
		int(t.second),
		0,
		time.UTC)
}

func (t Timestamp) String() string {
	str := strings.Builder{}
	str.WriteString(strconv.FormatUint(uint64(t.day), 10))
	str.WriteString(".")
	str.WriteString(strconv.FormatUint(uint64(t.month), 10))
	str.WriteString(".")
	str.WriteString(strconv.FormatUint(uint64(t.year), 10))
	str.WriteString("-")
	str.WriteString(strconv.FormatUint(uint64(t.hour), 10))
	str.WriteString(":")
	str.WriteString(strconv.FormatUint(uint64(t.minute), 10))
	str.WriteString(":")
	str.WriteString(strconv.FormatUint(uint64(t.second), 10))
	return str.String()
}

type TimeParser func(string, *Timestamp)

func extractYear(year string, timestamp *Timestamp) {
	val, _ := strconv.ParseUint(year, 10, 16)
	timestamp.year = uint16(val)
	timestamp.flags |= yearFlag
}

func extractMonth(month string, timestamp *Timestamp) {
	val, _ := strconv.ParseUint(month, 10, 8)
	timestamp.month = uint8(val)
	timestamp.flags |= monthFlag
}

func extractDay(day string, timestamp *Timestamp) {
	val, _ := strconv.ParseUint(day, 10, 8)
	timestamp.day = uint8(val)
	timestamp.flags |= dayFlag
}

func extractHour(hour string, timestamp *Timestamp) {
	val, _ := strconv.ParseUint(hour, 10, 8)
	timestamp.hour = uint8(val)
	timestamp.flags |= hourFlag
}

func extractMinute(minute string, timestamp *Timestamp) {
	val, _ := strconv.ParseUint(minute, 10, 8)
	timestamp.minute = uint8(val)
	timestamp.flags |= minuteFlag
}

func extractSecond(second string, timestamp *Timestamp) {
	val, _ := strconv.ParseUint(second, 10, 8)
	timestamp.second = uint8(val)
	timestamp.flags |= secondFlag
}
