package dbutil

import (
	"database/sql"
	"testing"
	"time"
)

func TestNullIfEmpty(t *testing.T) {
	if NullIfEmpty("") != nil {
		t.Fatal("empty must be NULL")
	}
	if NullIfEmpty("x") != "x" {
		t.Fatal("non-empty passes through")
	}
}

func TestNullIfZero(t *testing.T) {
	if NullIfZero(0) != nil {
		t.Fatal("zero must be NULL")
	}
	if NullIfZero(7) != int64(7) {
		t.Fatal("non-zero passes through")
	}
	if NullIfZero(-1) != int64(-1) {
		t.Fatal("negative passes through")
	}
}

func TestNullPtr(t *testing.T) {
	if NullPtr(nil) != nil {
		t.Fatal("nil must be NULL")
	}
	v := int64(0)
	if NullPtr(&v) != int64(0) {
		t.Fatal("pointer to zero stays zero, not NULL")
	}
}

func TestNullInt(t *testing.T) {
	if NullInt(sql.NullInt64{}) != nil {
		t.Fatal("invalid must be NULL")
	}
	if NullInt(sql.NullInt64{Int64: 5, Valid: true}) != int64(5) {
		t.Fatal("valid passes through")
	}
}

func TestNullInt64(t *testing.T) {
	if NullInt64(0).Valid {
		t.Fatal("zero is invalid")
	}
	if n := NullInt64(9); !n.Valid || n.Int64 != 9 {
		t.Fatalf("got %+v", n)
	}
}

func TestBool(t *testing.T) {
	if Bool(true) != 1 || Bool(false) != 0 {
		t.Fatal("bool mapping")
	}
}

func TestParseDecimal(t *testing.T) {
	cases := map[string]float64{
		"":                 0,
		"abc":              0,
		" 12.5 ":           12.5,
		"-3":               -3,
		"0.000000000001":   0.000000000001,
		"123456789.123456": 123456789.123456,
	}
	for in, want := range cases {
		if got := ParseDecimal(in); got != want {
			t.Errorf("ParseDecimal(%q)=%v want %v", in, got, want)
		}
	}
}

func TestFormatNullTime(t *testing.T) {
	if FormatNullTime(sql.NullTime{}) != "" {
		t.Fatal("NULL must be empty string")
	}
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if got := FormatNullTime(sql.NullTime{Time: ts, Valid: true}); got != "2026-01-02 03:04:05" {
		t.Fatalf("got %q", got)
	}
}
