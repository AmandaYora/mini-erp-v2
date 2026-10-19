package money

import "math"

// Arithmetic kernel for order-line pricing (ADR-0010, F8-7): the single
// implementation every money module calls. Pure numerics over the locked
// ADR-0005 policy (integer rupiah, half away from zero per step) — tax
// types, rates and discounts stay the caller's business inputs.
//
// Formula: gross = round(qty × unit); disc = round(gross × pct/100);
// net = gross − disc − nominal; then tax by type:
//   - none/"": total = net (base = net, tax = 0)
//   - exclude: total = net + round(net × rate/100) (base = net)
//   - include: total = net (base = round(net/(1+rate/100)), tax = net − base)
//
// Base is always the amount that belongs in Persediaan/Penjualan, and
// base+tax is always what the counterparty owes.

// Round maps a float to whole rupiah, half away from zero (math.Round).
func Round(f float64) int64 {
	return int64(math.Round(f))
}

// Prorate scales an amount by part/whole (delivered/received/returned share
// of a line). Returns 0 for a zero base rather than dividing by zero.
func Prorate(amount int64, part, whole float64) int64 {
	if whole <= 0 {
		return 0
	}
	return Round(float64(amount) * part / whole)
}

// Line is the priced result of one order line, all in whole rupiah.
type Line struct {
	Net   int64
	Base  int64
	Tax   int64
	Total int64
}

// Split returns the priced parts in the historical (net, base, tax, total)
// order, for call sites migrated from the per-module copies.
func (l Line) Split() (net, base, tax, total int64) {
	return l.Net, l.Base, l.Tax, l.Total
}

// LineAmounts prices one order line. Unknown tax types behave as "none".
func LineAmounts(qty float64, unit int64, discountPct float64, discountNominal int64, taxType string, taxRate float64) Line {
	gross := Round(qty * float64(unit))
	net := gross - Round(float64(gross)*discountPct/100) - discountNominal
	out := Line{Net: net, Base: net}
	switch taxType {
	case "exclude":
		out.Tax = Round(float64(net) * taxRate / 100)
		out.Total = net + out.Tax
	case "include":
		if taxRate > 0 {
			out.Base = Round(float64(net) / (1 + taxRate/100))
		}
		out.Tax = net - out.Base
		out.Total = net
	default:
		out.Total = net
	}
	return out
}
