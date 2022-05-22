package backup

import (
	"github.com/gorhill/cronexpr"
	"testing"
	"time"
)

func Test_FindPrevious(t *testing.T) {
	cron := cronexpr.MustParse("30 23 * * *")
	loc := time.Local
	timestamp := time.Date(2019, 4, 25, 8, 42, 55, 0, loc)

	result := FindPrevious(cron, timestamp)

	expected := time.Date(2019, 4, 24, 23, 30, 0, 0, loc)

	if !result.Equal(expected) {
		t.Errorf("Calculated execution time does not match expected execution time.\n"+
			"Expected: [%s], Actual: [%s]", expected.String(), result.String())
	}
}

func Test_FindPreviousWithWeekDays(t *testing.T) {
	cron := cronexpr.MustParse("30 23 * * MON-FRI")
	loc := time.Local
	timestamp := time.Date(2019, 4, 29, 8, 42, 55, 0, loc)

	result := FindPrevious(cron, timestamp)

	expected := time.Date(2019, 4, 26, 23, 30, 0, 0, loc)

	if !result.Equal(expected) {
		t.Errorf("Calculated execution time does not match expected execution time.\n"+
			"Expected: [%s], Actual: [%s]", expected.String(), result.String())
	}
}
