package domain

import "math"

// Pricing math for order lines, pure and integer-safe. See purchasing/domain
// for the formula — duplicated intentionally (each module owns its math, no
// shared domain logic per the monorepo standard).
func CalcLine(qty float64, unit int64, pct float64, nominal int64, taxType string, rate float64) (net, base, tax, total int64) {
	gross := int64(math.Round(qty * float64(unit)))
	disc := int64(math.Round(float64(gross) * pct / 100))
	net = gross - disc - nominal
	switch taxType {
	case "exclude":
		tax = int64(math.Round(float64(net) * rate / 100))
		base = net
		total = net + tax
	case "include":
		if rate > 0 {
			base = int64(math.Round(float64(net) / (1 + rate/100)))
		} else {
			base = net
		}
		tax = net - base
		total = net
	default:
		base = net
		total = net
	}
	return net, base, tax, total
}
