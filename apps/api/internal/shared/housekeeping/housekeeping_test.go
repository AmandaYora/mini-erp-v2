package housekeeping

import (
	"testing"
	"time"

	"mini-erp/internal/shared/timeutil"
)

func TestNextRun(t *testing.T) {
	// 01:00 WIB -> 02:00 WIB the same day.
	now := time.Date(2026, 10, 9, 1, 0, 0, 0, timeutil.Jakarta)
	if got := NextRun(now, 2).In(timeutil.Jakarta); got.Hour() != 2 || got.Day() != 9 {
		t.Fatalf("before = %v", got)
	}
	// Exactly 02:00 or later -> tomorrow.
	now = time.Date(2026, 10, 9, 2, 0, 0, 0, timeutil.Jakarta)
	if got := NextRun(now, 2).In(timeutil.Jakarta); got.Day() != 10 || got.Hour() != 2 {
		t.Fatalf("at = %v", got)
	}
	now = time.Date(2026, 10, 9, 23, 30, 0, 0, timeutil.Jakarta)
	if got := NextRun(now, 2).In(timeutil.Jakarta); got.Day() != 10 {
		t.Fatalf("after = %v", got)
	}
}

func TestRetentionDefaults(t *testing.T) {
	r := Retention{AuditDays: 365}.Normalized()
	if r.AuditDays != 365 || r.SessionDays != 30 || r.ReservationDays != 7 || r.AssistantDays != 90 || r.IdleThreadDays != 180 {
		t.Fatalf("retention = %+v", r)
	}
	c := Cutoff(time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC), 7)
	if c.Day() != 2 {
		t.Fatalf("cutoff = %v", c)
	}
}
