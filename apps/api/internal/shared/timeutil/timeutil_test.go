package timeutil

import (
	"testing"
	"time"
)

func TestDayBoundsWIB(t *testing.T) {
	day, err := ParseDateInput("2026-08-17")
	if err != nil {
		t.Fatal(err)
	}
	start, end := StartOfDayWIB(day), EndOfDayWIB(day)
	if !start.Before(end) {
		t.Fatal("start must precede end")
	}
	// WIB has no DST: bounds are exactly one day apart minus a nanosecond.
	if end.Sub(start) != 24*time.Hour-time.Nanosecond {
		t.Fatalf("day span = %v", end.Sub(start))
	}
	// 12:00 UTC on the 17th is 19:00 WIB — still the same Jakarta day.
	mid := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	if StartOfDayWIB(mid) != start || EndOfDayWIB(mid) != end {
		t.Fatal("bounds must follow the Jakarta calendar day, not UTC")
	}
	// A UTC instant just past Jakarta midnight belongs to the next day.
	next := time.Date(2026, 8, 17, 17, 0, 0, 1, time.UTC) // 00:00:00.000000001 WIB 18th
	if !StartOfDayWIB(next).After(start) {
		t.Fatal("post-midnight WIB instant must roll to the next day")
	}
	if _, err := ParseDateInput("17-08-2026"); err == nil {
		t.Fatal("non-ISO input must be rejected")
	}
}

func TestDateOrToday(t *testing.T) {
	if got := DateOrToday("  2026-03-04 "); got != "2026-03-04" {
		t.Fatalf("got %q", got)
	}
	today := NowUTC().In(Jakarta).Format("2006-01-02")
	if got := DateOrToday("   "); got != today && got != NowUTC().In(Jakarta).Format("2006-01-02") {
		t.Fatalf("blank must default to today, got %q", got)
	}
}

func TestStampConversionsUseWIB(t *testing.T) {
	// 2026-09-30 18:00 UTC is 2026-10-01 01:00 WIB: the WIB day, not the UTC one.
	for _, in := range []string{"2026-09-30 18:00:00", "2026-09-30 18:00:00.123456", "2026-09-30T18:00:00Z", "2026-09-30T18:00:00.5Z"} {
		if got := WIBDay(in); got != "2026-10-01" {
			t.Errorf("WIBDay(%q) = %q, want 2026-10-01", in, got)
		}
	}
	if got := FormatWIB("2026-09-30T18:00:00Z"); got != "2026-10-01 01:00:00" {
		t.Errorf("FormatWIB = %q", got)
	}
	if WIBDay("") != "" || FormatWIB("bukan waktu") != "" {
		t.Error("empty or malformed stamps must render empty")
	}
	if WireTimeWIB(nil) != nil {
		t.Error("nil time must stay null")
	}
	ts := time.Date(2026, 9, 30, 18, 0, 0, 0, time.UTC)
	if got := WireTimeWIB(&ts); got != "2026-10-01 01:00:00" {
		t.Errorf("WireTimeWIB = %v", got)
	}
}
