package integration

import (
	"testing"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	financeapp "mini-erp/internal/modules/finance/application"
	salesapp "mini-erp/internal/modules/sales/application"
)

// TestSalesOrderVATFlowReachesTaxSummary (F-01/K-03): orders created with VAT
// excluded and included, shipped and posted, land in the output-VAT figure the
// way the form's estimate says (money.LineAmounts), and an order without tax
// adds nothing.
func TestSalesOrderVATFlowReachesTaxSummary(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	const date = "2026-10-12"

	sell := func(taxType string, rate float64) int64 {
		t.Helper()
		so, err := m.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
			PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod",
			TaxType: taxType, TaxRate: rate, OrderDate: date,
			Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 2}},
		})
		if err != nil {
			t.Fatalf("create SO %s: %v", taxType, err)
		}
		if _, err := m.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
			t.Fatalf("confirm SO %s: %v", taxType, err)
		}
		dn, err := m.delivery.Service().Create(ctx(), 0, fx.branchID, so.ID, date, "", deliveryapp.DocInput{},
			[]deliveryapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 2}})
		if err != nil {
			t.Fatalf("create delivery %s: %v", taxType, err)
		}
		if _, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); err != nil {
			t.Fatalf("confirm delivery %s: %v", taxType, err)
		}
		if _, err := m.finance.Service().Post(ctx(), 0, fx.branchID, financeapp.SourceDelivery, dn.ID); err != nil {
			t.Fatalf("post delivery %s: %v", taxType, err)
		}
		return so.ID
	}

	sell("none", 0)
	// 2 x 15.000 = 30.000 net. Exclude 11%: PPN 3.300. Include 11%: DPP 27.027, PPN 2.973.
	sell("exclude", 11)
	sell("include", 11)

	sum, err := m.finance.Service().TaxSummary(ctx(), fx.branchID, 2026, 10)
	if err != nil {
		t.Fatalf("tax summary: %v", err)
	}
	if want := int64(3300 + 2973); sum.PpnOut != want {
		t.Fatalf("PPN keluaran = %d, want %d", sum.PpnOut, want)
	}

	rows, err := m.finance.Service().TaxDetail(ctx(), fx.branchID, 2026, 10)
	if err != nil {
		t.Fatalf("tax detail: %v", err)
	}
	var withPpn int
	var ppnTotal int64
	for _, r := range rows {
		if r.Ppn > 0 {
			withPpn++
		}
		ppnTotal += r.Ppn
	}
	if withPpn != 2 || ppnTotal != 3300+2973 {
		t.Fatalf("tax detail ppn rows=%d total=%d, want 2 rows totalling 6273", withPpn, ppnTotal)
	}
}
