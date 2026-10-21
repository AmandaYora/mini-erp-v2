package validator

import (
	"strings"
	"testing"
)

func TestReason(t *testing.T) {
	if got, err := Reason("  salah pilih gudang  ", true); err != nil || got != "salah pilih gudang" {
		t.Fatalf("trimmed reason: got %q, %v", got, err)
	}
	if _, err := Reason("   ", true); err == nil {
		t.Fatal("blank mandatory reason must fail")
	}
	if got, err := Reason("", false); err != nil || got != "" {
		t.Fatalf("optional empty reason: got %q, %v", got, err)
	}
	if _, err := Reason(strings.Repeat("é", ReasonMaxRunes), true); err != nil {
		t.Fatalf("255 runes must pass: %v", err)
	}
	if _, err := Reason(strings.Repeat("a", ReasonMaxRunes+1), false); err == nil {
		t.Fatal("256 runes must fail")
	}
}
