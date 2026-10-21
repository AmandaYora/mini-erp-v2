package application

import (
	"context"
	"testing"
)

// BalancesByKey caps the footing at the end of the asOf day, exactly like
// BalanceSheet/TrialBalanceAsOf ("<day> 23:59:59"): the dashboard capped at
// today equals the balance sheet of today, future-dated journals excluded.
func TestAsOfUpperBound(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"  ", ""},
		{"2026-10-21", "2026-10-21 23:59:59"},
		{" 2026-02-28 ", "2026-02-28 23:59:59"},
	}
	for _, c := range cases {
		got, err := asOfUpperBound(c.in)
		if err != nil || got != c.want {
			t.Fatalf("asOfUpperBound(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
	for _, bad := range []string{"21-10-2026", "2026-13-01", "today"} {
		if _, err := asOfUpperBound(bad); err == nil {
			t.Fatalf("asOfUpperBound(%q) accepted a malformed day", bad)
		}
	}
}

// A malformed asOf is rejected before any repository read (nil repo would panic).
func TestBalancesByKeyRejectsMalformedAsOf(t *testing.T) {
	s := &Service{}
	if _, err := s.BalancesByKey(context.Background(), 1, []string{"cash"}, "kemarin"); err == nil {
		t.Fatal("malformed asOf accepted")
	}
	out, err := s.BalancesByKey(context.Background(), 1, nil, "kemarin")
	if err != nil || len(out) != 0 {
		t.Fatalf("no keys = no read: %v %v", out, err)
	}
}
