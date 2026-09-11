package application

import "testing"

// The invariant every posting builder leans on: `base` is what belongs in
// Persediaan/Penjualan and `base + tax` is what the counterparty owes.
// Booking a tax-inclusive line at `net` instead of `base` double-counts the
// tax — once inside the asset/revenue and again as PPN.
func TestLineEconomics(t *testing.T) {
	cases := []struct {
		name           string
		qty            float64
		unit           int64
		pct            float64
		nominal        int64
		taxType        string
		rate           float64
		net, base, tax int64
	}{
		{"tanpa pajak", 2, 10_000, 0, 0, "none", 0, 20_000, 20_000, 0},
		{"pajak di luar 11%", 1, 10_000, 0, 0, "exclude", 11, 10_000, 10_000, 1_100},
		{"pajak di dalam 11%", 1, 11_100, 0, 0, "include", 11, 11_100, 10_000, 1_100},
		{"diskon persen lalu pajak luar", 2, 10_000, 10, 0, "exclude", 11, 18_000, 18_000, 1_980},
		{"diskon nominal lalu pajak dalam", 1, 11_100, 0, 1_100, "include", 11, 10_000, 9_009, 991},
		{"include tarif nol", 1, 10_000, 0, 0, "include", 0, 10_000, 10_000, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			net, base, tax := lineEconomics(tc.qty, tc.unit, tc.pct, tc.nominal, tc.taxType, tc.rate)
			if net != tc.net || base != tc.base || tax != tc.tax {
				t.Fatalf("= (net %d, base %d, tax %d), mau (net %d, base %d, tax %d)",
					net, base, tax, tc.net, tc.base, tc.tax)
			}
			// The balance guarantee, asserted rather than assumed: whatever
			// the tax type, base + tax is exactly what the party owes.
			if want := owed(tc.taxType, net, tax); base+tax != want {
				t.Fatalf("base+tax = %d, mau %d (%s)", base+tax, want, tc.taxType)
			}
		})
	}
}

// owed restates the counterparty amount independently of lineEconomics so
// the assertion above is a real check, not a restatement of the code.
func owed(taxType string, net, tax int64) int64 {
	if taxType == "exclude" {
		return net + tax
	}
	return net
}

func TestProrate(t *testing.T) {
	cases := []struct {
		name        string
		amount      int64
		part, whole float64
		want        int64
	}{
		{"seluruhnya", 10_000, 5, 5, 10_000},
		{"separuh", 10_000, 2.5, 5, 5_000},
		{"sepertiga dibulatkan", 10_000, 1, 3, 3_333},
		{"qty sumber nol tidak membagi nol", 10_000, 1, 0, 0},
		{"qty sumber negatif tidak membagi", 10_000, 1, -1, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := prorate(tc.amount, tc.part, tc.whole); got != tc.want {
				t.Fatalf("= %d, mau %d", got, tc.want)
			}
		})
	}
}

// A surat jalan for part of an order must book only that part. The bug this
// guards: booking the whole order on every note, so a two-shipment order
// billed the books twice.
func TestPartialDeliveryBooksOnlyItsShare(t *testing.T) {
	// SO line: 10 pcs × 10.000, PPN 11% di luar → base 100.000, tax 11.000.
	_, base, tax := lineEconomics(10, 10_000, 0, 0, "exclude", 11)

	var revenue, taxTotal int64
	for _, shipped := range []float64{4, 6} { // dua surat jalan
		revenue += prorate(base, shipped, 10)
		taxTotal += prorate(tax, shipped, 10)
	}
	if revenue != base {
		t.Fatalf("penjualan terbukukan %d, mau %d", revenue, base)
	}
	if taxTotal != tax {
		t.Fatalf("PPN terbukukan %d, mau %d", taxTotal, tax)
	}
	if got := revenue + taxTotal; got != base+tax {
		t.Fatalf("piutang terbukukan %d, mau %d", got, base+tax)
	}
}
