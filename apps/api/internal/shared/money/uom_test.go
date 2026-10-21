package money

import "testing"

func TestPriceForUnit(t *testing.T) {
	// 1 DUS = 36 BH, sales unit BH (factor 1) at 25.000.
	if got := PriceForUnit(25000, 36, 1); got != 900000 {
		t.Fatalf("carton price = %d, want 900000", got)
	}
	if got := PriceForUnit(25000, 1, 1); got != 25000 {
		t.Fatalf("sales unit price = %d", got)
	}
	// Sales unit is a pack of 12; base unit sold alone: 120.000 / 12.
	if got := PriceForUnit(120000, 1, 12); got != 10000 {
		t.Fatalf("base unit price = %d, want 10000", got)
	}
	if got := PriceForUnit(25000, 36, 0); got != 25000 {
		t.Fatalf("unknown sales factor keeps the price, got %d", got)
	}
}

func TestMinLineNet(t *testing.T) {
	if got := MinLineNet(20000, 36, 1); got != 720000 {
		t.Fatalf("carton minimum = %d, want 720000", got)
	}
	if MinLineNet(0, 36, 1) != 0 {
		t.Fatal("no minimum must be 0")
	}
}
