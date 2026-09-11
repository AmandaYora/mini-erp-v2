package domain

import "testing"

func TestApplyRule(t *testing.T) {
	cases := []struct {
		name           string
		base           int64
		dir, typ, mode string
		value, step    float64
		want           int64
	}{
		{"minus 10%", 10000, DirMinus, AdjustPercent, RoundNone, 10, 0, 9000},
		{"plus nominal", 10000, DirPlus, AdjustNominal, RoundNone, 500, 0, 10500},
		{"percent fraction rounds", 9999, DirMinus, AdjustPercent, RoundNone, 10, 0, 8999}, // 8999.1
		{"round_100", 10000, DirMinus, AdjustPercent, Round100, 3, 0, 9700},                // 9700 exact
		{"round_100 nearest", 10000, DirMinus, AdjustPercent, Round100, 4, 0, 9600},
		{"round_500", 10000, DirMinus, AdjustPercent, Round500, 12, 0, 9000}, // 8800→9000
		{"round_1000", 15500, DirMinus, AdjustNominal, Round1000, 600, 0, 15000},
		{"floor", 9870, DirMinus, AdjustNominal, RoundFloor, 0, 100, 9800},
		{"ceil", 9810, DirMinus, AdjustNominal, RoundCeil, 0, 100, 9900},
		{"none keeps integer", 9871, DirPlus, AdjustNominal, RoundNone, 0, 0, 9871},
	}
	for _, tc := range cases {
		if got := ApplyRule(tc.base, tc.dir, tc.typ, tc.value, tc.mode, tc.step); got != tc.want {
			t.Errorf("%s = %d, want %d", tc.name, got, tc.want)
		}
	}
}
