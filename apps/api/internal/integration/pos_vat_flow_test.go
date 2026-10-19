package integration

import (
	"testing"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	financeapp "mini-erp/internal/modules/finance/application"
	salesapp "mini-erp/internal/modules/sales/application"
)

// TestPOSOrderVATFlowReachesTaxSummary (K-03, POS): a POS-channel order with the
// cashier's optional VAT (exclude and include) gets the same totals the checkout
// estimate shows (money.LineAmounts: exclude adds, include splits DPP/PPN), and
// after delivery + posting its PPN lands in the output-VAT figure; the default
// (no tax) POS order adds nothing.
func TestPOSOrderVATFlowReachesTaxSummary(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	const date = "2026-10-12"

	sell := func(ref, taxType string, rate float64, wantGrand, wantTax int64) {
		t.Helper()
		so, err := m.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
			PartyID: 0, Channel: "pos", PaymentTerms: "cod", TaxType: taxType, TaxRate: rate,
			OrderDate: date, ClientRef: ref,
			Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 2}},
		})
		if err != nil {
			t.Fatalf("create POS order %s: %v", taxType, err)
		}
		// The checkout estimate (client) must equal the server total.
		if so.GrandTotal != wantGrand || so.TaxTotal != wantTax {
			t.Fatalf("POS %s: grand %d tax %d, want %d/%d", taxType, so.GrandTotal, so.TaxTotal, wantGrand, wantTax)
		}
		if _, err := m.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
			t.Fatalf("confirm %s: %v", taxType, err)
		}
		dn, err := m.delivery.Service().Create(ctx(), 0, fx.branchID, so.ID, date, "", deliveryapp.DocInput{},
			[]deliveryapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 2}})
		if err != nil {
			t.Fatalf("delivery %s: %v", taxType, err)
		}
		if _, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); err != nil {
			t.Fatalf("confirm delivery %s: %v", taxType, err)
		}
		if _, err := m.finance.Service().Post(ctx(), 0, fx.branchID, financeapp.SourceDelivery, dn.ID); err != nil {
			t.Fatalf("post %s: %v", taxType, err)
		}
	}

	// 2 x 15.000 = 30.000. Default: no tax. Exclude 11%: +3.300. Include 11%: DPP 27.027, PPN 2.973.
	sell("pos-vat-none", "none", 0, 30000, 0)
	sell("pos-vat-excl", "exclude", 11, 33300, 3300)
	sell("pos-vat-incl", "include", 11, 30000, 2973)

	sum, err := m.finance.Service().TaxSummary(ctx(), fx.branchID, 2026, 10)
	if err != nil {
		t.Fatalf("tax summary: %v", err)
	}
	if want := int64(3300 + 2973); sum.PpnOut != want {
		t.Fatalf("PPN keluaran = %d, want %d", sum.PpnOut, want)
	}
}
