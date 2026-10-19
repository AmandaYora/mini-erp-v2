package pagination

import "testing"

func TestParseGuards(t *testing.T) {
	cases := []struct {
		pageRaw, limitRaw string
		wantPage, wantLim int
	}{
		{"", "", 1, 20},        // missing → defaults
		{"0", "0", 1, 20},      // zero → defaults, never empty list (KI-26)
		{"-5", "-5", 1, 20},    // negative → defaults, never a crash
		{"abc", "abc", 1, 20},  // non-numeric → defaults, never a 500
		{"2", "500", 2, 100},   // over max → clamped
		{"3", "10", 3, 10},     // sane values pass through
		{" 2 ", " 10 ", 1, 20}, // Atoi rejects spaces → defaults
	}
	for _, tc := range cases {
		if page, limit := Parse(tc.pageRaw, tc.limitRaw); page != tc.wantPage || limit != tc.wantLim {
			t.Errorf("Parse(%q,%q) = (%d,%d), want (%d,%d)",
				tc.pageRaw, tc.limitRaw, page, limit, tc.wantPage, tc.wantLim)
		}
	}
}

func TestOffsetTotalPages(t *testing.T) {
	if Offset(1, 20) != 0 || Offset(3, 20) != 40 {
		t.Fatal("bad offset")
	}
	if TotalPages(0, 20) != 0 || TotalPages(40, 20) != 2 || TotalPages(41, 20) != 3 {
		t.Fatal("bad total pages")
	}
}

func TestParseDisplayLimit(t *testing.T) {
	cases := []struct {
		raw      string
		def, max int
		want     int
	}{
		{"", 0, 100, 0},
		{"abc", 5, 100, 5},
		{"0", 5, 100, 5},
		{"-3", 5, 100, 5},
		{"7", 0, 100, 7},
		{"100", 0, 100, 100},
		{"101", 0, 100, 100},
		{"99999999999", 0, 100, 100},
		{"500", 0, 0, 500}, // max 0 = unbounded
	}
	for _, c := range cases {
		if got := ParseDisplayLimit(c.raw, c.def, c.max); got != c.want {
			t.Errorf("ParseDisplayLimit(%q,%d,%d)=%d want %d", c.raw, c.def, c.max, got, c.want)
		}
	}
}
