package money

import "testing"

func TestLineEconomicsExemptLineCarriesNoTax(t *testing.T) {
	// Exempt line in a taxed order: stored base 15.000, tax 0.
	var e LineEconomics
	e.Add(3, 5000, 0, 0, 15000, 0)
	gross, net, base, tax := e.Part(1)
	if gross != 5000 || net != 5000 || base != 5000 || tax != 0 {
		t.Fatalf("part = %d %d %d %d, want 5000 5000 5000 0", gross, net, base, tax)
	}
}

func TestLineEconomicsDuplicateLinesAverage(t *testing.T) {
	// 100 @ 1.193.000 plus 5 @ 1 (legacy duplicate lines): one unit is worth
	// the weighted average, never the last line's price of 1.
	var e LineEconomics
	e.Add(100, 1193000, 0, 0, 119300000, 0)
	e.Add(5, 1, 0, 0, 5, 0)
	_, _, base, _ := e.Part(10)
	if base != 11361905 {
		t.Fatalf("10 units base = %d, want 11361905", base)
	}
	if e.UnitPrice() != 1136191 || e.DiscountPct() != 0 {
		t.Fatalf("display = %d / %v", e.UnitPrice(), e.DiscountPct())
	}
	if _, _, all, _ := e.Part(105); all != 119300005 {
		t.Fatalf("full return = %d, want the stored total", all)
	}
}

func TestLineEconomicsDiscountAndTax(t *testing.T) {
	// 2 @ 100.000, 10% off, PPN 11% exclusive: net 180.000, base 180.000, tax 19.800.
	var e LineEconomics
	e.Add(2, 100000, 10, 0, 180000, 19800)
	gross, net, base, tax := e.Part(1)
	if gross != 100000 || net != 90000 || base != 90000 || tax != 9900 {
		t.Fatalf("part = %d %d %d %d", gross, net, base, tax)
	}
}
