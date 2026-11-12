package backup

import (
	"strings"
	"testing"
	"time"

	"github.com/gorhill/cronexpr"
)

// The schedule is evaluated in the time zone of the given moment (#42)
func Test_FindPrevious_usesTimezoneOfMoment(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}

	cron := cronexpr.MustParse("0 2 * * *")
	moment := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

	inUTC := FindPrevious(cron, moment)
	if expected := time.Date(2026, 7, 1, 2, 0, 0, 0, time.UTC); !inUTC.Equal(expected) {
		t.Errorf("UTC: got %s, want %s", inUTC, expected)
	}

	// 02:00 in Berlin is 00:00 UTC during daylight saving time
	inBerlin := FindPrevious(cron, moment.In(berlin))
	if expected := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC); !inBerlin.Equal(expected) {
		t.Errorf("Europe/Berlin: got %s, want %s", inBerlin, expected)
	}
}

// Without daylight saving time, 02:00 in Berlin is 01:00 UTC
func Test_FindPrevious_followsDaylightSavingTime(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}

	cron := cronexpr.MustParse("0 2 * * *")
	moment := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	result := FindPrevious(cron, moment.In(berlin))
	if expected := time.Date(2026, 1, 15, 1, 0, 0, 0, time.UTC); !result.Equal(expected) {
		t.Errorf("got %s, want %s", result, expected)
	}
}

func Test_FindPrevious_withoutSchedule_returnsZeroTime(t *testing.T) {
	if result := FindPrevious(nil, time.Now()); !result.IsZero() {
		t.Errorf("got %s, want the zero time", result)
	}
}

func Test_ParseDefinition_timezone(t *testing.T) {
	def, err := ParseDefinition(strings.NewReader("timezone: Europe/Berlin\n"))
	if err != nil {
		t.Fatal(err)
	}

	if def.Location == nil || def.Location.String() != "Europe/Berlin" {
		t.Errorf("got location %v, want Europe/Berlin", def.Location)
	}
}

func Test_ParseDefinition_withoutTimezone_usesProcessTimezone(t *testing.T) {
	def, err := ParseDefinition(strings.NewReader("quota: 1GiB\n"))
	if err != nil {
		t.Fatal(err)
	}

	if def.Location != nil {
		t.Errorf("got location %v, want none", def.Location)
	}
}

func Test_ParseDefinition_invalidTimezone_returnsError(t *testing.T) {
	if _, err := ParseDefinition(strings.NewReader("timezone: Mars/Olympus_Mons\n")); err == nil {
		t.Error("expected an error for an unknown time zone")
	}
}
