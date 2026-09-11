package domain

import "testing"

func TestCalcLine(t *testing.T) {
	cases := []struct {
		name                  string
		qty                   float64
		unit                  int64
		pct                   float64
		nominal               int64
		taxType               string
		rate                  float64
		net, base, tax, total int64
	}{
		{"plain", 2, 10000, 0, 0, "none", 0, 20000, 20000, 0, 20000},
		{"exclude 11%", 1, 10000, 0, 0, "exclude", 11, 10000, 10000, 1100, 11100},
		{"include 11%", 1, 11100, 0, 0, "include", 11, 11100, 10000, 1100, 11100},
	}
	for _, tc := range cases {
		net, base, tax, total := CalcLine(tc.qty, tc.unit, tc.pct, tc.nominal, tc.taxType, tc.rate)
		if net != tc.net || base != tc.base || tax != tc.tax || total != tc.total {
			t.Errorf("%s = (%d,%d,%d,%d), want (%d,%d,%d,%d)",
				tc.name, net, base, tax, total, tc.net, tc.base, tc.tax, tc.total)
		}
	}
}
