package domain

import "testing"

func TestUOMConversion(t *testing.T) {
	if got := ToBase(2, 12); got != 24 {
		t.Fatalf("ToBase = %v", got)
	}
	if got := FromBase(24, 12); got != 2 {
		t.Fatalf("FromBase = %v", got)
	}
	if FromBase(24, 0) != 0 {
		t.Fatal("zero factor must not divide")
	}
	if !ValidFactor(0.5) || ValidFactor(0) || ValidFactor(-1) {
		t.Fatal("factor guard wrong")
	}
}
