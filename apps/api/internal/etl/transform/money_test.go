package transform

import (
	"math"
	"math/big"
	"math/rand"
	"testing"

	"mini-erp/internal/etl/report"
	"mini-erp/internal/etl/row"
	"mini-erp/internal/shared/money"
)

func TestRoundHalfAway(t *testing.T) {
	cases := map[string]int64{
		"0": 0, "0.49": 0, "0.5": 1, "0.50": 1, "1.4999": 1, "1.5": 2, "2.5": 3, "-0.5": -1, "-0.49": 0, "-1.5": -2, "-2.5": -3,
		"0.005": 0, "999999999999999.50": 1000000000000000, "-999999999999999.50": -1000000000000000, "12345678901234567.4": 12345678901234567,
	}
	for in, want := range cases {
		if got := RoundHalfAway(row.ParseRat(in)); got != want {
			t.Errorf("Round(%s) = %d, want %d", in, got, want)
		}
	}
}

// Property: odd symmetry, and agreement with the kernel the revamp itself uses.
func TestRoundProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 10000; i++ {
		cents := rng.Int63n(2_000_000_000_00) - 1_000_000_000_00
		r := new(big.Rat).SetFrac64(cents, 100)
		neg := new(big.Rat).Neg(r)
		if RoundHalfAway(r) != -RoundHalfAway(neg) {
			t.Fatalf("asimetri pada %v", r)
		}
		f, _ := r.Float64()
		if math.Abs(f) < 1e12 { // float is exact enough here; compare with shared/money.Round
			if want := money.Round(f); want != RoundHalfAway(r) {
				t.Fatalf("Round(%v)=%d, money.Round=%d", r, RoundHalfAway(r), want)
			}
		}
	}
}

func TestRounderLogsEveryChange(t *testing.T) {
	log := &report.Log{}
	m := &Rounder{Log: log}
	var before, after int64
	var sumExact = new(big.Rat)
	rng := rand.New(rand.NewSource(11))
	for i := 0; i < 10000; i++ {
		r := new(big.Rat).SetFrac64(rng.Int63n(100_000_000), 100)
		sumExact.Add(sumExact, r)
		after += m.Round("x", int64(i), "amount", r)
		before++
	}
	// Σ(after) − Σ(exact) must equal the Σ of the logged deltas (nothing changed silently).
	var logged = new(big.Rat)
	for _, a := range log.Adjustments {
		logged.Add(logged, new(big.Rat).Sub(row.ParseRat(a.After), row.ParseRat(a.Before)))
	}
	diff := new(big.Rat).Sub(new(big.Rat).SetInt64(after), sumExact)
	// the log stores 4-decimal strings, so allow that precision
	if new(big.Rat).Sub(diff, logged).Cmp(big.NewRat(1, 100)) > 0 || new(big.Rat).Sub(logged, diff).Cmp(big.NewRat(1, 100)) > 0 {
		t.Fatalf("log tidak menjelaskan selisih: diff=%v logged=%v", diff.FloatString(4), logged.FloatString(4))
	}
	_ = before
}

func TestBalanceEntry(t *testing.T) {
	lines := []Line{{Debit: 101}, {Credit: 50}, {Credit: 50}}
	idx, delta, side, err := BalanceEntry(lines, 5)
	if err != nil || delta != 1 || side != "credit" || idx != 1 {
		t.Fatalf("idx=%d delta=%d side=%s err=%v", idx, delta, side, err)
	}
	if lines[1].Credit != 51 {
		t.Fatalf("baris terbesar harus menyerap: %+v", lines)
	}
	// debit short
	l2 := []Line{{Debit: 99}, {Debit: 1}, {Credit: 101}}
	if _, d, s, err := BalanceEntry(l2, 5); err != nil || d != 1 || s != "debit" || l2[0].Debit != 100 {
		t.Fatalf("%v %d %s %+v", err, d, s, l2)
	}
	// balanced: nothing changes
	if idx, d, _, err := BalanceEntry([]Line{{Debit: 5}, {Credit: 5}}, 5); err != nil || idx != -1 || d != 0 {
		t.Fatal("jurnal seimbang tidak boleh diubah")
	}
	// too large: refused, not rounded away
	if _, _, _, err := BalanceEntry([]Line{{Debit: 100}, {Credit: 10}}, 5); err == nil {
		t.Fatal("selisih besar harus ditolak")
	}
}

func TestStatusFromGroup(t *testing.T) {
	for g, want := range map[string]string{"pending": "draft", "active": "confirmed", "completed": "completed", "cancelled": "cancelled"} {
		if got, err := statusFromGroup(g); err != nil || got != want {
			t.Errorf("%s -> %s,%v", g, got, err)
		}
	}
	if _, err := statusFromGroup("weird"); err == nil {
		t.Fatal("status asing harus ditolak, bukan jatuh ke draft")
	}
}

func TestWIBBoundary(t *testing.T) {
	// 2025-12-31 17:30 UTC is already 2026-01-01 in Jakarta: current year, not opening-covered.
	tm, _ := row.Row{"t": "2025-12-31T17:30:00.000000Z"}.Time("t")
	if d := wibDate(tm); d != "2026-01-01" {
		t.Fatalf("wibDate = %s", d)
	}
	tm2, _ := row.Row{"t": "2025-12-31T16:59:59.000000Z"}.Time("t")
	if d := wibDate(tm2); d != "2025-12-31" {
		t.Fatalf("wibDate = %s", d)
	}
}

func TestTaxType(t *testing.T) {
	cases := []struct {
		taxable, incl int
		want          string
	}{{0, 0, "none"}, {0, 1, "none"}, {1, 0, "exclude"}, {1, 1, "include"}}
	for _, c := range cases {
		if got := taxType(row.Row{"taxable": c.taxable, "price_includes_tax": c.incl}); got != c.want {
			t.Errorf("%+v -> %s", c, got)
		}
	}
}

func TestSplitByQty(t *testing.T) {
	q := func(s ...string) []*big.Rat {
		out := make([]*big.Rat, len(s))
		for i, v := range s {
			out[i] = row.ParseRat(v)
		}
		return out
	}
	cases := []struct {
		total int64
		qtys  []*big.Rat
		want  []int64
	}{
		{300001, q("20", "5"), []int64{240001, 60000}},
		{100, q("1", "1", "1"), []int64{34, 33, 33}},
		{-100, q("1", "1", "1"), []int64{-34, -33, -33}},
		{10, q("0.5", "0.25", "0.25"), []int64{5, 3, 2}},
		{7, q("0", "0"), []int64{7, 0}},
		{0, q("3", "4"), []int64{0, 0}},
	}
	for _, c := range cases {
		got := SplitByQty(c.total, c.qtys)
		var sum int64
		for i := range got {
			sum += got[i]
			if got[i] != c.want[i] {
				t.Errorf("SplitByQty(%d, %v) = %v, want %v", c.total, c.qtys, got, c.want)
				break
			}
		}
		if sum != c.total {
			t.Errorf("SplitByQty(%d) sums to %d", c.total, sum)
		}
	}
	// Property: the parts always add back to the total.
	rng := rand.New(rand.NewSource(11))
	for i := 0; i < 2000; i++ {
		n := 1 + rng.Intn(70)
		qs := make([]*big.Rat, n)
		for j := range qs {
			qs[j] = big.NewRat(rng.Int63n(1_000_000)+1, 1000)
		}
		total := rng.Int63n(10_000_000_000)
		var sum int64
		for _, v := range SplitByQty(total, qs) {
			sum += v
		}
		if sum != total {
			t.Fatalf("Σ split = %d, want %d", sum, total)
		}
	}
}
