package domain

import (
	"errors"
	"math"
)

// Price bases for member pricing.
const (
	BasisSellingPrice    = "selling_price"
	BasisMinSellingPrice = "min_selling_price"
	BasisPurchasePrice   = "purchase_price"
)

// Rounding modes (legacy vocabulary, F3b-5). A rounding mode needs an
// explicit step (kelipatan); a zero step locks the rule to "none" (KI-73).
const (
	RoundNone    = "none"
	RoundNearest = "nearest"
	RoundUp      = "up"
	RoundDown    = "down"
)

// Pre-parity mode names. They are still ACCEPTED on input (and normalised by
// NormalizeRounding) so existing clients keep working; they are never stored
// or returned any more.
const (
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

// MaxPercent is the technical ceiling of a percent adjustment. Legacy only
// requires >= 0 (markups above 100% are legal); DECIMAL(18,4) stays safe.
const MaxPercent = 100000

// ErrBasePrice reports a basis price that is not positive.
var ErrBasePrice = errors.New("harga basis kosong")

// NormalizeRounding maps any accepted mode spelling to the canonical
// none|nearest|up|down plus the step it implies. Pre-parity names carry a
// default step (round_100 -> nearest/100) that applies only when the caller
// gave none. ok is false for an unknown mode.
func NormalizeRounding(mode string, step float64) (string, float64, bool) {
	switch mode {
	case "", RoundNone:
		return RoundNone, step, true
	case RoundNearest, RoundUp, RoundDown:
		return mode, step, true
	case Round100, Round500, Round1000:
		def := map[string]float64{Round100: 100, Round500: 500, Round1000: 1000}[mode]
		if step <= 0 {
			step = def
		}
		return RoundNearest, step, true
	case RoundFloor, RoundCeil:
		if step <= 0 {
			step = 100
		}
		if mode == RoundFloor {
			return RoundDown, step, true
		}
		return RoundUp, step, true
	}
	return "", 0, false
}

// BasePrice derives the per-sales-unit price a member rule starts from.
// Selling and minimum prices are already per sales unit. The purchase price
// is per PURCHASE unit, so it is divided by the purchase factor (base units
// per purchase unit) and multiplied by the sales factor (legacy parity: a
// 100.000/dus, 12-pcs dus costs 8.333,33 per pcs).
func BasePrice(basis string, selling, minSelling, purchase int64, purchaseFactor, salesFactor float64) (float64, error) {
	switch basis {
	case BasisMinSellingPrice:
		if minSelling <= 0 {
			return 0, ErrBasePrice
		}
		return float64(minSelling), nil
	case BasisPurchasePrice:
		if purchase <= 0 {
			return 0, ErrBasePrice
		}
		if purchaseFactor <= 0 || salesFactor <= 0 {
			return 0, errors.New("konversi satuan tidak valid")
		}
		return float64(purchase) / purchaseFactor * salesFactor, nil
	default:
		if selling <= 0 {
			return 0, ErrBasePrice
		}
		return float64(selling), nil
	}
}

// ApplyRule computes the member price. `base` is the rule's basis price per
// sales unit (may carry fractions: the purchase basis divides by a factor).
// Order (legacy): adjust, round to the step, then whole rupiah.
func ApplyRule(base float64, direction, adjType string, value float64, mode string, step float64) int64 {
	adjusted := base
	switch adjType {
	case AdjustPercent:
		delta := base * value / 100
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
	nm, ns, ok := NormalizeRounding(mode, step)
	if ok {
		adjusted = applyRounding(adjusted, nm, ns)
	}
	return int64(math.Round(adjusted)) // half away from zero
}

func applyRounding(amount float64, mode string, step float64) float64 {
	if step <= 0 {
		return amount
	}
	switch mode {
	case RoundDown:
		return math.Floor(amount/step) * step
	case RoundUp:
		return math.Ceil(amount/step) * step
	case RoundNearest:
		return math.Round(amount/step) * step
	}
	return amount
}
