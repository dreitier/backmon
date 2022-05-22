package backup

import (
	"github.com/dreitier/cloudmon/config"
	"github.com/gorhill/cronexpr"
	"time"
)

func FindPrevious(cron *cronexpr.Expression, moment time.Time) time.Time {
	if cron == nil {
		return time.Time{}
	}
	next := cron.Next(moment)
	high := next
	if next.IsZero() {
		high = moment.Add(time.Second)
	}
	duration := -2 * config.Day
	low := moment.Add(duration)
	mid := cron.Next(low)
	if mid.IsZero() || !mid.Before(high) {
		for {
			duration *= 2
			low = moment.Add(duration)
			mid = cron.Next(low)
			if !mid.IsZero() && mid.Before(high) {
				break
			}
			if duration < -200 * config.Year {
				return time.Time{}
			}
		}
	}
	if mid.IsZero() {
		return mid
	}
	return findPreviousInRange(cron, mid, moment, high)
}

func findPreviousInRange(cron *cronexpr.Expression, low time.Time, high time.Time, next time.Time) time.Time {
	diff := high.Sub(low)
	halfDiff := time.Duration(int64(diff) / 2)
	median := low.Add(halfDiff)
	nextAfterMedian := cron.Next(median)
	if nextAfterMedian.Before(next) {
		if diff < time.Minute {
			return nextAfterMedian
		} else {
			return findPreviousInRange(cron, nextAfterMedian, high, next)
		}
	} else {
		if diff < time.Minute {
			return low
		} else {
			return findPreviousInRange(cron, low, median, next)
		}
	}
}