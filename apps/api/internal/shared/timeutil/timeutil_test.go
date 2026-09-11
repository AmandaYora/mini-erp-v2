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
