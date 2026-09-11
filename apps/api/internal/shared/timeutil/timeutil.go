package timeutil

import (
	"time"
)

// Jakarta is Asia/Jakarta (WIB, UTC+7, no DST). It falls back to a fixed
// +7 zone when the IANA database is unavailable, so behavior never depends
// on container tzdata.
var Jakarta = loadJakarta()

func loadJakarta() *time.Location {
	if loc, err := time.LoadLocation("Asia/Jakarta"); err == nil {
		return loc
	}
	return time.FixedZone("WIB", 7*3600)
}

// NowUTC returns the current time in UTC for storage. Timestamps are always
// stored UTC and converted to Jakarta only for display/filtering.
func NowUTC() time.Time {
	return time.Now().UTC()
}

// StartOfDayWIB returns 00:00:00 of the given Jakarta calendar day as UTC —
// the inclusive lower bound for day-range queries.
func StartOfDayWIB(day time.Time) time.Time {
	y, m, d := day.In(Jakarta).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, Jakarta).UTC()
}

// EndOfDayWIB returns the last nanosecond of the given Jakarta calendar day
// as UTC — the inclusive upper bound. This fixes the OQ-A32 class of bugs
// where `date_to` silently dropped the final day's rows.
func EndOfDayWIB(day time.Time) time.Time {
	y, m, d := day.In(Jakarta).Date()
	return time.Date(y, m, d, 23, 59, 59, int(time.Second-time.Nanosecond), Jakarta).UTC()
}

// ParseDateInput parses "2006-01-02" as a Jakarta calendar day.
func ParseDateInput(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", s, Jakarta)
}
