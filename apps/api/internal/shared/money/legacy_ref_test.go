package money

// Verbatim copies of the five pre-unification implementations, kept as
// oracles for the property test below (F8-7 step 1). DO NOT call these from
// production code — arch-check forbids these names outside shared/money,
// and these copies carry the _ref suffix so the guard never trips on them.

import "math"

func refRound(f float64) int64 { return int64(math.Round(f)) }

func refSalesCalcLine(qty float64, unit int64, pct float64, nominal int64, taxType string, rate float64) (net, base, tax, total int64) {
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

func refPurchasingCalcLine(qty float64, unit int64, pct float64, nominal int64, taxType string, rate float64) (net, base, tax, total int64) {
	return refSalesCalcLine(qty, unit, pct, nominal, taxType, rate)
}

func refFinanceLine(qty float64, unit int64, pct float64, nominal int64, taxType string, rate float64) (net, base, tax int64) {
	gross := refRound(qty * float64(unit))
	net = gross - refRound(float64(gross)*pct/100) - nominal
	switch taxType {
	case "exclude":
		base = net
		tax = refRound(float64(net) * rate / 100)
	case "include":
		if rate > 0 {
			base = refRound(float64(net) / (1 + rate/100))
		} else {
			base = net
		}
		tax = net - base
	default:
		base = net
	}
	return net, base, tax
}

func refSalesReturnLine(qty float64, unit int64, pct float64, nominal int64, taxType string, rate float64) (net, base, tax int64) {
	gross := int64(math.Round(qty * float64(unit)))
	net = gross - int64(math.Round(float64(gross)*pct/100)) - nominal
	switch taxType {
	case "exclude":
		base = net
		tax = int64(math.Round(float64(net) * rate / 100))
	case "include":
		if rate > 0 {
			base = int64(math.Round(float64(net) / (1 + rate/100)))
		} else {
			base = net
		}
		tax = net - base
	default:
		base = net
	}
	return net, base, tax
}

func refPurchaseReturnLine(qty float64, unit int64, pct float64, nominal int64, taxType string, rate float64) (net, base, tax int64) {
	return refSalesReturnLine(qty, unit, pct, nominal, taxType, rate)
}

func refProrate(amount int64, part, whole float64) int64 {
	if whole <= 0 {
		return 0
	}
	return int64(math.Round(float64(amount) * part / whole))
}
