// Package domain holds the dashboard's pure data-health rules (ADR-0025).
package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Snooze is a branch-wide pause of one data-health alert. The alert itself is
// never stored; the snooze remembers how big the problem was when it was
// snoozed so that a growing problem breaks through.
type Snooze struct {
	ID        int64
	BranchID  int64
	AlertID   string
	Reason    string
	Days      int
	Until     time.Time
	CountAt   int64
	AmountAt  *int64
	CreatedAt time.Time
	UpdatedAt time.Time
	UpdatedBy int64
}

// Snooze durations and limits (data-health spec §4).
const (
	MaxDaysCritical = 7
	MaxDaysOther    = 14
	ReasonMin       = 5
	ReasonMax       = 255
	// GrowthTolerance lets the rupiah figure creep up 10 % before a snooze
	// lapses; any increase in count lapses it at once.
	GrowthTolerance = 1.10
)

// ValidDays reports whether d is one of the offered durations.
func ValidDays(d int) bool {
	return d == 1 || d == 3 || d == 7 || d == 14
}

// MaxDays is the longest snooze an alert of this severity may get.
func MaxDays(severity string) int {
	if severity == "critical" {
		return MaxDaysCritical
	}
	return MaxDaysOther
}

// ReasonProblem returns the user-facing problem with a snooze reason (trimmed
// before counting), or "" when it is acceptable.
func ReasonProblem(reason string) string {
	n := utf8.RuneCountInString(strings.TrimSpace(reason))
	switch {
	case n == 0:
		return "Alasan wajib diisi"
	case n < ReasonMin:
		return "Alasan minimal 5 karakter"
	case n > ReasonMax:
		return "Alasan maksimal 255 karakter"
	}
	return ""
}

// Active reports whether the snooze still runs at now.
func (s Snooze) Active(now time.Time) bool {
	return s.Until.After(now)
}

// Grew reports whether the problem got worse since it was snoozed: more rows,
// or rupiah above the tolerance.
func (s Snooze) Grew(count int64, amount *int64) bool {
	if count > s.CountAt {
		return true
	}
	if amount != nil && s.AmountAt != nil && float64(*amount) > float64(*s.AmountAt)*GrowthTolerance {
		return true
	}
	return false
}
