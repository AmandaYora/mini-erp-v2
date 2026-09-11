package money

import (
	"strconv"
	"strings"
)

// Rupiah is whole rupiah as int64. Money is never float and never has cents
// (ADR-0005) — every amount in the system is this type.
type Rupiah int64

// String renders "Rp1.234.567" (dot thousands separator, no decimals).
// Negative values render "-Rp500".
func (r Rupiah) String() string {
	neg := r < 0
	n := int64(r)
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var groups []string
	for len(s) > 3 {
		groups = append([]string{s[len(s)-3:]}, groups...)
		s = s[:len(s)-3]
	}
	groups = append([]string{s}, groups...)
	out := "Rp" + strings.Join(groups, ".")
	if neg {
		return "-" + out
	}
	return out
}
