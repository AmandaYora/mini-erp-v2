package domain

import "testing"

func TestApplyRule(t *testing.T) {
	cases := []struct {
		name           string
		base           float64
		dir, typ, mode string
		value, step    float64
		want           int64
	}{
		{"minus 10%", 10000, DirMinus, AdjustPercent, RoundNone, 10, 0, 9000},
		{"plus nominal", 10000, DirPlus, AdjustNominal, RoundNone, 500, 0, 10500},
		{"percent fraction rounds", 9999, DirMinus, AdjustPercent, RoundNone, 10, 0, 8999}, // 8999.1
		{"legacy round_100", 10000, DirMinus, AdjustPercent, Round100, 3, 0, 9700},
		{"legacy round_100 nearest", 10000, DirMinus, AdjustPercent, Round100, 4, 0, 9600},
		{"legacy round_500", 10000, DirMinus, AdjustPercent, Round500, 12, 0, 9000}, // 8800 -> 9000
		{"legacy round_1000", 15500, DirMinus, AdjustNominal, Round1000, 600, 0, 15000},
		{"legacy floor", 9870, DirMinus, AdjustNominal, RoundFloor, 0, 100, 9800},
		{"legacy ceil", 9810, DirMinus, AdjustNominal, RoundCeil, 0, 100, 9900},
		{"none keeps integer", 9871, DirPlus, AdjustNominal, RoundNone, 0, 0, 9871},
		{"nearest step 250", 10000, DirMinus, AdjustPercent, RoundNearest, 1, 250, 10000}, // 9900 -> 10000
		{"nearest step 250 down", 10000, DirMinus, AdjustPercent, RoundNearest, 4, 250, 9500},
		{"down step 250", 10300, DirPlus, AdjustNominal, RoundDown, 0, 250, 10250},
		{"up step 250", 10300, DirPlus, AdjustNominal, RoundUp, 0, 250, 10500},
		{"markup over 100 percent", 10000, DirPlus, AdjustPercent, RoundNone, 150, 0, 25000},
		{"purchase basis with factor", 100000.0 / 12 * 1, DirPlus, AdjustPercent, RoundNone, 10, 0, 9167},
	}
	for _, tc := range cases {
		if got := ApplyRule(tc.base, tc.dir, tc.typ, tc.value, tc.mode, tc.step); got != tc.want {
			t.Errorf("%s = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestBasePrice(t *testing.T) {
	// Purchase price is per dus (12 pcs); the member rule prices per pcs.
	got, err := BasePrice(BasisPurchasePrice, 20000, 15000, 100000, 12, 1)
	if err != nil || ApplyRule(got, DirPlus, AdjustPercent, 10, RoundNone, 0) != 9167 {
		t.Fatalf("purchase basis = %v, %v", got, err)
	}
	// Selling per dus of 12 factor stays per sales unit (no conversion).
	if got, _ := BasePrice(BasisSellingPrice, 20000, 0, 0, 12, 12); got != 20000 {
		t.Fatalf("selling basis = %v", got)
	}
	for _, basis := range []string{BasisSellingPrice, BasisMinSellingPrice, BasisPurchasePrice} {
		if _, err := BasePrice(basis, 0, 0, 0, 1, 1); err != ErrBasePrice {
			t.Errorf("%s empty basis err = %v", basis, err)
		}
	}
	if _, err := BasePrice(BasisPurchasePrice, 0, 0, 100, 0, 1); err == nil {
		t.Error("zero factor must fail")
	}
}

func TestNormalizeRounding(t *testing.T) {
	cases := []struct {
		in   string
		step float64
		mode string
		want float64
		ok   bool
	}{
		{"", 0, RoundNone, 0, true},
		{"nearest", 250, RoundNearest, 250, true},
		{"round_500", 0, RoundNearest, 500, true},
		{"round_100", 250, RoundNearest, 250, true},
		{"floor", 0, RoundDown, 100, true},
		{"ceil", 50, RoundUp, 50, true},
		{"banana", 0, "", 0, false},
	}
	for _, tc := range cases {
		m, s, ok := NormalizeRounding(tc.in, tc.step)
		if m != tc.mode || s != tc.want || ok != tc.ok {
			t.Errorf("%q/%v = %q %v %v", tc.in, tc.step, m, s, ok)
		}
	}
}
