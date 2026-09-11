package domain

// ToBase converts qty in a transaction UOM to the stock base UOM.
func ToBase(qty, factor float64) float64 {
	return qty * factor
}

// FromBase converts a base-UOM quantity back to the transaction UOM.
func FromBase(baseQty, factor float64) float64 {
	if factor == 0 {
		return 0
	}
	return baseQty / factor
}

// ValidFactor reports whether a UOM factor is usable (strictly positive).
func ValidFactor(f float64) bool {
	return f > 0
}
