package money

// LineEconomics accumulates the STORED money facts of the source order lines
// of one position and unit (a sales/purchase order may hold several), so a
// return values its lines from what the order actually billed — each line's
// own discount, taxable flag and rate — instead of recomputing at today's
// header tax. Duplicate lines at different prices average by quantity.
type LineEconomics struct {
	Qty   float64
	Gross int64 // qty x unit price, before discounts
	Net   int64 // after discounts, before tax split
	Base  int64 // stored tax base (DPP)
	Tax   int64 // stored tax amount
	lines int
	price int64
	pct   float64
}

// Add folds one source line in. base/tax are the line's stored split.
func (e *LineEconomics) Add(qty float64, unitPrice int64, discountPct float64, discountNominal, base, tax int64) {
	gross := Round(qty * float64(unitPrice))
	e.Qty += qty
	e.Gross += gross
	e.Net += gross - Round(float64(gross)*discountPct/100) - discountNominal
	e.Base += base
	e.Tax += tax
	e.lines++
	e.price, e.pct = unitPrice, discountPct
}

// Part is the share of qty units: gross, net, tax base and tax, each pro-rated
// from the stored totals (a full return gets them back exactly).
func (e LineEconomics) Part(qty float64) (gross, net, base, tax int64) {
	return Prorate(e.Gross, qty, e.Qty), Prorate(e.Net, qty, e.Qty), Prorate(e.Base, qty, e.Qty), Prorate(e.Tax, qty, e.Qty)
}

// UnitPrice is the source unit price (the quantity-weighted average when
// several lines differ).
func (e LineEconomics) UnitPrice() int64 {
	if e.lines <= 1 || e.Qty <= 0 {
		return e.price
	}
	return Round(float64(e.Gross) / e.Qty)
}

// DiscountPct is the source line's percent discount; several lines report 0
// and carry their whole discount as the nominal part instead.
func (e LineEconomics) DiscountPct() float64 {
	if e.lines <= 1 {
		return e.pct
	}
	return 0
}
