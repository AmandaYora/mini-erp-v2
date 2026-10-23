package application

import (
	productcontracts "mini-erp/internal/modules/product/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
)

// itemProductIDs lists the distinct products of the order lines (one batch read).
func itemProductIDs(items []*salescontracts.Item) []int64 {
	seen := map[int64]bool{}
	out := make([]int64, 0, len(items))
	for _, it := range items {
		if !seen[it.ProductID] {
			seen[it.ProductID] = true
			out = append(out, it.ProductID)
		}
	}
	return out
}

// returnUOMs is the product's base unit and every unit a return line may use
// (base, sales, purchase — plus the order line's own unit), each with its base
// factor. Without the product (archived/unknown) only the line unit is offered.
func returnUOMs(p *productcontracts.Product, lineUOM string, lineFactor float64) (string, []map[string]any) {
	type u struct {
		uom    string
		factor float64
	}
	var list []u
	add := func(uom string, factor float64) {
		if uom == "" || factor <= 0 {
			return
		}
		for _, x := range list {
			if x.uom == uom {
				return
			}
		}
		list = append(list, u{uom, factor})
	}
	base := lineUOM
	add(lineUOM, lineFactor)
	if p != nil {
		base = p.BaseUOM
		add(p.BaseUOM, 1)
		add(p.SalesUOM, p.SalesFactor)
		add(p.PurchaseUOM, p.PurchaseFactor)
	}
	out := make([]map[string]any, 0, len(list))
	for _, x := range list {
		out = append(out, map[string]any{"uom": x.uom, "factor": x.factor})
	}
	return base, out
}
