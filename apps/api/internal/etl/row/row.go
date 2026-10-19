// Package row is the loosely typed record the ETL moves between phases.
//
// Extracted legacy values are JSON scalars: integers as numbers, DECIMAL and
// DATETIME as strings (never float, so no precision is lost), NULL as nil.
// The accessors below are forgiving about the concrete Go type because a row
// can come from json.Decoder.UseNumber (file) or be built in memory.
package row

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

// Row is one record keyed by column name.
type Row map[string]any

// Layouts the extractor emits (legacy timezone is UTC, see design F5-3).
var timeLayouts = []string{
	"2006-01-02T15:04:05.000000Z",
	"2006-01-02T15:04:05Z",
	"2006-01-02 15:04:05.999999",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

// Has reports whether the column is present and not NULL.
func (r Row) Has(k string) bool { return r[k] != nil }

// Str returns the column as text ("" for NULL).
func (r Row) Str(k string) string {
	s, _ := r.NStr(k)
	return s
}

// NStr returns the text and whether it was non-NULL.
func (r Row) NStr(k string) (string, bool) {
	switch v := r[k].(type) {
	case nil:
		return "", false
	case string:
		return v, true
	case json.Number:
		return v.String(), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case int:
		return strconv.Itoa(v), true
	case bool:
		if v {
			return "1", true
		}
		return "0", true
	default:
		return fmt.Sprint(v), true
	}
}

// NInt returns the integer value and whether it was non-NULL.
func (r Row) NInt(k string) (int64, bool) {
	s, ok := r.NStr(k)
	if !ok {
		return 0, false
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n, true
	}
	// "1.000" style values from DECIMAL columns used as flags.
	if rt, ok2 := new(big.Rat).SetString(s); ok2 {
		f, _ := rt.Float64()
		return int64(f), true
	}
	return 0, true
}

// Int returns the integer value (0 for NULL).
func (r Row) Int(k string) int64 { n, _ := r.NInt(k); return n }

// Bool treats any non-zero number or "true" as true.
func (r Row) Bool(k string) bool {
	s, ok := r.NStr(k)
	if !ok {
		return false
	}
	if s == "true" {
		return true
	}
	n, _ := strconv.ParseInt(s, 10, 64)
	return n != 0
}

// Rat returns an exact decimal (0 for NULL or unparsable text).
func (r Row) Rat(k string) *big.Rat {
	s, ok := r.NStr(k)
	if !ok {
		return new(big.Rat)
	}
	return ParseRat(s)
}

// ParseRat parses a decimal string; invalid input yields 0.
func ParseRat(s string) *big.Rat {
	v, ok := new(big.Rat).SetString(strings.TrimSpace(s))
	if !ok {
		return new(big.Rat)
	}
	return v
}

// Time parses a legacy UTC timestamp or date.
func (r Row) Time(k string) (time.Time, bool) {
	s, ok := r.NStr(k)
	if !ok || s == "" {
		return time.Time{}, false
	}
	for _, l := range timeLayouts {
		if t, err := time.ParseInLocation(l, s, time.UTC); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// Clone copies the map (shallow).
func (r Row) Clone() Row {
	out := make(Row, len(r))
	for k, v := range r {
		out[k] = v
	}
	return out
}
