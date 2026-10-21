package timeutil

import (
	"strings"
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

// DateOrToday trims raw and returns it, or today's Jakarta calendar day as
// YYYY-MM-DD when raw is blank. It does not validate the date.
func DateOrToday(raw string) string {
	if t := strings.TrimSpace(raw); t != "" {
		return t
	}
	return NowUTC().In(Jakarta).Format("2006-01-02")
}

// stampLayouts are the shapes a stored UTC instant arrives in: RFC3339 from a
// DATETIME scanned into a string with parseTime, or the naive DATETIME text.
var stampLayouts = []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05", "2006-01-02"}

// ParseStampUTC reads a stored UTC instant (system stamps: created_at,
// moved_at, completed_at...). ok is false for empty or malformed text.
func ParseStampUTC(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, l := range stampLayouts {
		if t, err := time.ParseInLocation(l, s, time.UTC); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// WIBDay returns the Jakarta calendar day (YYYY-MM-DD) of a stored UTC
// instant — never its UTC date, which is the day before for 00:00–06:59 WIB.
// Empty or malformed input returns "".
func WIBDay(stamp string) string {
	t, ok := ParseStampUTC(stamp)
	if !ok {
		return ""
	}
	return t.In(Jakarta).Format("2006-01-02")
}

// FormatWIB renders a stored UTC instant as a naive Jakarta wall clock
// ("2006-01-02 15:04:05") — the single wire form for every datetime the API
// returns, so clients never have to guess a column's zone. Empty or malformed
// input returns "".
func FormatWIB(stamp string) string {
	t, ok := ParseStampUTC(stamp)
	if !ok {
		return ""
	}
	return t.In(Jakarta).Format("2006-01-02 15:04:05")
}

// WireTimeWIB is FormatWIB for an optional time.Time: nil stays nil (JSON
// null), anything else becomes the naive Jakarta wall clock.
func WireTimeWIB(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.UTC().In(Jakarta).Format("2006-01-02 15:04:05")
}
