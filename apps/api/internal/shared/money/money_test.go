package money

import "testing"

func TestRupiahString(t *testing.T) {
	cases := []struct {
		in   Rupiah
		want string
	}{
		{0, "Rp0"},
		{500, "Rp500"},
		{1000, "Rp1.000"},
		{1234567, "Rp1.234.567"},
		{-500, "-Rp500"},
	}
	for _, tc := range cases {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("Rupiah(%d) = %q, want %q", int64(tc.in), got, tc.want)
		}
	}
}
