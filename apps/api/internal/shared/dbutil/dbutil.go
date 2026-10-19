// Package dbutil holds the tiny, purely technical conversions between Go
// values and database/sql values that every infrastructure package needs.
// No domain logic belongs here (see .claude/rules/backend-modular-monolith.md).
package dbutil

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// timeLayout is the wire format repositories hand to contracts.
const timeLayout = "2006-01-02 15:04:05"

// NullIfEmpty maps the empty string to SQL NULL.
func NullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// NullIfZero maps a zero reference id / amount to SQL NULL.
func NullIfZero(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

// NullPtr maps a nil pointer to SQL NULL and otherwise passes the value.
func NullPtr(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

// NullInt unwraps a sql.NullInt64 into a bindable argument.
func NullInt(v sql.NullInt64) any {
	if v.Valid {
		return v.Int64
	}
	return nil
}

// NullInt64 maps zero to an invalid sql.NullInt64.
func NullInt64(v int64) sql.NullInt64 {
	if v == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: v, Valid: true}
}

// NullString maps the empty string to an invalid sql.NullString.
func NullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// Bool maps a bool to the 0/1 TINYINT the schema uses.
func Bool(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ParseDecimal reads a DECIMAL column scanned as text. Unparsable input
// reads as 0, matching the repositories' previous lenient behaviour.
func ParseDecimal(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

// FormatNullTime renders a nullable timestamp as "2006-01-02 15:04:05",
// or the empty string when NULL.
func FormatNullTime(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format(timeLayout)
}

// dateTimeLayouts are the textual forms callers hand to repositories: a bare
// calendar day, a day with time (optionally fractional), or RFC 3339.
var dateTimeLayouts = []string{"2006-01-02 15:04:05", "2006-01-02", time.RFC3339Nano}

// ParseDateTime reads a "YYYY-MM-DD[ HH:MM:SS[.ffffff]]" (or RFC 3339) string
// as the wall-clock instant the DATETIME column stores (UTC, no conversion),
// so a generated query can bind it as time.Time where it used to bind text.
func ParseDateTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range dateTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("tanggal/waktu tidak valid: %q", s)
}

// NullDateTime maps the empty string to NULL and otherwise parses like
// ParseDateTime.
func NullDateTime(s string) (sql.NullTime, error) {
	if strings.TrimSpace(s) == "" {
		return sql.NullTime{}, nil
	}
	t, err := ParseDateTime(s)
	if err != nil {
		return sql.NullTime{}, err
	}
	return sql.NullTime{Time: t, Valid: true}, nil
}
