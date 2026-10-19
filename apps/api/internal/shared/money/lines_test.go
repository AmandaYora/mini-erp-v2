package money

import (
	"math/rand"
	"testing"
)

// TestLineAmountsAgainstLegacyOracles (F8-7): 1M seeded random cases must
// match all five verbatim pre-unification copies, and the tax invariants
// must hold for every case.
func TestLineAmountsAgainstLegacyOracles(t *testing.T) {
	rng := rand.New(rand.NewSource(20261009))
	taxTypes := []string{"none", "exclude", "include", ""}
	rates := []float64{0, 11, 12, 0.5, 10}
	for i := 0; i < 1000000; i++ {
		qty := 0.0001 + rng.Float64()*10000
		if i%7 == 0 {
			qty += 0.5
		}
		unit := rng.Int63n(5e9 + 1)
		var pct float64
		switch i % 5 {
		case 0:
			pct = 0
		case 1:
			pct = 0.5
		case 2:
			pct = 33.333
		case 3:
			pct = 100
		default:
			pct = rng.Float64() * 100
		}
		gross := Round(qty * float64(unit))
		nominal := rng.Int63n(gross + 1)
		taxType := taxTypes[rng.Intn(len(taxTypes))]
		rate := rates[rng.Intn(len(rates))]

		got := LineAmounts(qty, unit, pct, nominal, taxType, rate)

		n1, b1, x1, t1 := refSalesCalcLine(qty, unit, pct, nominal, taxType, rate)
		n2, b2, x2, t2 := refPurchasingCalcLine(qty, unit, pct, nominal, taxType, rate)
		if (n1 != n2) || (b1 != b2) || (x1 != x2) || (t1 != t2) {
			t.Fatalf("case %d: sales and purchasing oracles disagree (pre-existing drift)", i)
		}
		n3, b3, x3 := refFinanceLine(qty, unit, pct, nominal, taxType, rate)
		n4, b4, x4 := refSalesReturnLine(qty, unit, pct, nominal, taxType, rate)
		n5, b5, x5 := refPurchaseReturnLine(qty, unit, pct, nominal, taxType, rate)
		if got.Net != n1 || got.Base != b1 || got.Tax != x1 || got.Total != t1 ||
			got.Net != n2 || got.Base != b2 || got.Tax != x2 || got.Total != t2 ||
			got.Net != n3 || got.Base != b3 || got.Tax != x3 ||
			got.Net != n4 || got.Base != b4 || got.Tax != x4 ||
			got.Net != n5 || got.Base != b5 || got.Tax != x5 {
			t.Fatalf("case %d: got %+v want sales(%d,%d,%d,%d) finance(%d,%d,%d)",
				i, got, n1, b1, x1, t1, n3, b3, x3)
		}
		switch taxType {
		case "exclude":
			if got.Base != got.Net || got.Total != got.Net+got.Tax {
				t.Fatalf("case %d exclude invariant: %+v", i, got)
			}
		case "include":
			if got.Base+got.Tax != got.Net || got.Total != got.Net {
				t.Fatalf("case %d include invariant: %+v", i, got)
			}
		default:
			if got.Tax != 0 || got.Total != got.Net || got.Base != got.Net {
				t.Fatalf("case %d none invariant: %+v", i, got)
			}
		}
		if gotP := Prorate(got.Total, 3, 7); gotP != refProrate(got.Total, 3, 7) {
			t.Fatalf("case %d prorate: got %d want %d", i, gotP, refProrate(got.Total, 3, 7))
		}
	}
}

// TestProrateEdges pins the zero-base and identity rules.
func TestProrateEdges(t *testing.T) {
	if got := Prorate(1000, 5, 0); got != 0 {
		t.Fatalf("whole<=0: got %d want 0", got)
	}
	if got := Prorate(1000, 5, -2); got != 0 {
		t.Fatalf("whole<0: got %d want 0", got)
	}
	if got := Prorate(1234, 7, 7); got != 1234 {
		t.Fatalf("part==whole: got %d want 1234", got)
	}
	// Monotonic in part.
	prev := Prorate(100000, 0, 100)
	for part := 1.0; part <= 100; part++ {
		cur := Prorate(100000, part, 100)
		if cur < prev {
			t.Fatalf("not monotone at part=%v: %d < %d", part, cur, prev)
		}
		prev = cur
	}
}

// TestRoundHalfAway pins the rounding policy (ADR-0005).
func TestRoundHalfAway(t *testing.T) {
	for in, want := range map[float64]int64{2.5: 3, -2.5: -3, 2.4: 2, 2.6: 3, 0.5: 1, -0.5: -1} {
		if got := Round(in); got != want {
			t.Fatalf("Round(%v) = %d want %d", in, got, want)
		}
	}
}
