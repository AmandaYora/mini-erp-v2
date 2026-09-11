package domain

import "math"

// Price bases for member pricing.
const (
	BasisSellingPrice    = "selling_price"
	BasisMinSellingPrice = "min_selling_price"
	BasisPurchasePrice   = "purchase_price"
)

// Rounding modes. An explicit increment is required whenever mode != none;
// a zero/empty increment locks to "none" (KI-73).
const (
	RoundNone  = "none"
	Round100   = "round_100"
	Round500   = "round_500"
	Round1000  = "round_1000"
	RoundFloor = "floor"
	RoundCeil  = "ceil"
)

// Adjustment types and directions.
const (
	AdjustPercent = "percent"
	AdjustNominal = "nominal"
	DirMinus      = "minus"
	DirPlus       = "plus"
)

// NominalConfirmThreshold is the confirmation bar (rupiah): a nominal
// adjustment above this needs an explicit confirmed resubmit (KI-76).
const NominalConfirmThreshold = 1_000_000

// ApplyRule computes the member price. `base` is the rule's basis price
// (already selected from the product), always in whole rupiah.
func ApplyRule(base int64, direction, adjType string, value float64, mode string, step float64) int64 {
	adjusted := float64(base)
	switch adjType {
	case AdjustPercent:
		delta := float64(base) * value / 100
		if direction == DirMinus {
			adjusted -= delta
		} else {
			adjusted += delta
		}
	case AdjustNominal:
		if direction == DirMinus {
			adjusted -= value
		} else {
			adjusted += value
		}
	}
	rounded := math.Round(adjusted) // half away from zero — whole rupiah first
	return applyRounding(int64(rounded), mode, step)
}

func applyRounding(amount int64, mode string, step float64) int64 {
	inc := step
	if inc <= 0 {
		switch mode {
		case Round100:
			inc = 100
		case Round500:
			inc = 500
		case Round1000:
			inc = 1000
		case RoundFloor, RoundCeil:
			inc = 100
		default:
			return amount
		}
	}
	i := int64(inc)
	if i <= 0 {
		return amount
	}
	switch mode {
	case RoundFloor:
		return amount / i * i
	case RoundCeil:
		return (amount + i - 1) / i * i
	default: // round_100/500/1000 → nearest
		return int64(math.Round(float64(amount)/float64(i))) * i
	}
}
