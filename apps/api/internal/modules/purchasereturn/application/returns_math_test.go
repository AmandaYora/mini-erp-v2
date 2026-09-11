package application

import (
	"testing"
)

// TestLineEconomicsTaxTypes pins the purchase-return money math per tax
// type (B2 mirror of salesreturn): discount before tax, include extracts
// the base, exclude stacks on top. 2 × 11100, pct 10 → gross 22200,
// net 19980.
func TestLineEconomicsTaxTypes(t *testing.T) {
	net, base, tax := lineEconomics(2, 11100, 10, 0, "none", 0)
	if net != 19980 || base != 19980 || tax != 0 {
		t.Fatalf("none = %d/%d/%d, want 19980/19980/0", net, base, tax)
	}
	net, base, tax = lineEconomics(2, 11100, 10, 0, "exclude", 11)
	if net != 19980 || base != 19980 || tax != 2198 {
		t.Fatalf("exclude = %d/%d/%d, want 19980/19980/2198", net, base, tax)
	}
	net, base, tax = lineEconomics(2, 11100, 10, 0, "include", 11)
	if net != 19980 || base != 18000 || tax != 1980 {
		t.Fatalf("include = %d/%d/%d, want 19980/18000/1980", net, base, tax)
	}
	if base+tax != net {
		t.Fatalf("include invariant base+tax = %d, want net %d", base+tax, net)
	}
}

// TestProrate: partial returns scale the source economics; zero source
// qty returns 0 instead of dividing by zero.
func TestProrate(t *testing.T) {
	if got := prorate(1000, 2, 5); got != 400 {
		t.Fatalf("prorate = %d, want 400", got)
	}
	if got := prorate(1000, 1, 0); got != 0 {
		t.Fatalf("prorate zero whole = %d, want 0", got)
	}
}
