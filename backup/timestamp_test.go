package backup

import (
	"testing"
	"time"
)

func TestTimestamp_ToTime(t *testing.T) {
	ts := Timestamp{year: 2023, month: 4, day: 5, hour: 6, minute: 7, second: 8}

	got := ts.ToTime()
	want := time.Date(2023, time.April, 5, 6, 7, 8, 0, time.UTC)

	if !got.Equal(want) {
		t.Errorf("ToTime() = %v, want %v", got, want)
	}
}

func TestTimestamp_TimeWithDefaults_FillsUnsetFields(t *testing.T) {
	defaults := time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)

	// Only the year is explicitly set; every other field must fall back to the default.
	ts := Timestamp{}
	extractYear("2023", &ts)

	got := ts.TimeWithDefaults(defaults)
	want := time.Date(2023, time.January, 2, 3, 4, 5, 0, time.UTC)

	if !got.Equal(want) {
		t.Errorf("TimeWithDefaults() = %v, want %v", got, want)
	}
}

func TestTimestamp_TimeWithDefaults_PrefersSetFields(t *testing.T) {
	defaults := time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)

	ts := Timestamp{}
	extractYear("2023", &ts)
	extractMonth("11", &ts)
	extractDay("22", &ts)
	extractHour("13", &ts)
	extractMinute("44", &ts)
	extractSecond("55", &ts)

	got := ts.TimeWithDefaults(defaults)
	want := time.Date(2023, time.November, 22, 13, 44, 55, 0, time.UTC)

	if !got.Equal(want) {
		t.Errorf("TimeWithDefaults() = %v, want %v", got, want)
	}
}

func TestTimestamp_TimeWithDefaults_NormalizesDefaultsToUTC(t *testing.T) {
	// A default in a non-UTC zone must be normalized so the extracted
	// components correspond to the UTC wall-clock time.
	loc := time.FixedZone("UTC+5", 5*60*60)
	defaults := time.Date(2020, time.January, 2, 3, 4, 5, 0, loc)

	ts := Timestamp{}
	extractYear("2023", &ts)

	got := ts.TimeWithDefaults(defaults)
	// 03:04:05 at UTC+5 is 22:04:05 on the previous day in UTC.
	want := time.Date(2023, time.January, 1, 22, 4, 5, 0, time.UTC)

	if !got.Equal(want) {
		t.Errorf("TimeWithDefaults() = %v, want %v", got, want)
	}
}

func TestTimestamp_Extractors_SetFlags(t *testing.T) {
	ts := Timestamp{}

	extractYear("2023", &ts)
	extractMonth("4", &ts)
	extractDay("5", &ts)
	extractHour("6", &ts)
	extractMinute("7", &ts)
	extractSecond("8", &ts)

	if ts.year != 2023 || ts.month != 4 || ts.day != 5 || ts.hour != 6 || ts.minute != 7 || ts.second != 8 {
		t.Errorf("extractors stored wrong values: %+v", ts)
	}

	allFlags := uint8(yearFlag | monthFlag | dayFlag | hourFlag | minuteFlag | secondFlag)
	if ts.flags != allFlags {
		t.Errorf("flags = %b, want %b", ts.flags, allFlags)
	}
}

func TestTimestamp_String(t *testing.T) {
	ts := Timestamp{year: 2023, month: 4, day: 5, hour: 6, minute: 7, second: 8}

	if got, want := ts.String(), "5.4.2023-6:7:8"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
