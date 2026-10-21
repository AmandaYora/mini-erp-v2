package money

// Catalog, member and minimum selling prices are kept per SALES unit. An order
// line may use another unit of the product (base, purchase, sales); these
// helpers price it by base-quantity ratio so a carton of 36 never sells at
// the price of one piece.

// PriceForUnit converts a per-sales-unit price to a line unit holding factor
// base units, where the sales unit holds salesFactor base units.
func PriceForUnit(salesUnitPrice int64, factor, salesFactor float64) int64 {
	if salesFactor <= 0 || factor <= 0 || factor == salesFactor {
		return salesUnitPrice
	}
	return Round(float64(salesUnitPrice) * factor / salesFactor)
}

// MinLineNet is the lowest net a line of qtyBase base units may carry under a
// per-sales-unit minimum price (0 = no minimum).
func MinLineNet(minPerSalesUnit int64, qtyBase, salesFactor float64) int64 {
	if minPerSalesUnit <= 0 {
		return 0
	}
	if salesFactor <= 0 {
		salesFactor = 1
	}
	return Round(float64(minPerSalesUnit) * qtyBase / salesFactor)
}
