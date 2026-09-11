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
		{"percent", 2, 10000, 10, 0, "none", 0, 18000, 18000, 0, 18000},
		{"nominal", 1, 10000, 0, 1500, "none", 0, 8500, 8500, 0, 8500},
		{"both", 1, 10000, 10, 500, "none", 0, 8500, 8500, 0, 8500},
		{"exclude 11%", 1, 10000, 0, 0, "exclude", 11, 10000, 10000, 1100, 11100},
		{"include 11%", 1, 11100, 0, 0, "include", 11, 11100, 10000, 1100, 11100},
		{"fractional qty", 1.5, 10000, 0, 0, "none", 0, 15000, 15000, 0, 15000},
	}
	for _, tc := range cases {
		net, base, tax, total := CalcLine(tc.qty, tc.unit, tc.pct, tc.nominal, tc.taxType, tc.rate)
		if net != tc.net || base != tc.base || tax != tc.tax || total != tc.total {
			t.Errorf("%s = (%d,%d,%d,%d), want (%d,%d,%d,%d)",
				tc.name, net, base, tax, total, tc.net, tc.base, tc.tax, tc.total)
		}
	}
}
