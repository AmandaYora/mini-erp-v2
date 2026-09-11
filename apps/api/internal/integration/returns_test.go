package integration

import (
	"context"
	"testing"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	paymentapp "mini-erp/internal/modules/payment/application"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	productapp "mini-erp/internal/modules/product/application"
	salesapp "mini-erp/internal/modules/sales/application"
	salesreturnapp "mini-erp/internal/modules/salesreturn/application"
	salesreturncontracts "mini-erp/internal/modules/salesreturn/contracts"
)

// deliverAll confirms an SO in full through one delivery.
func deliverAll(t *testing.T, ctx context.Context, fx shopFixture, soID int64, qty float64) {
	t.Helper()
	m := testMods
	dn, err := m.delivery.Service().Create(ctx, 0, fx.branchID, soID, "2026-09-10", "", deliveryapp.DocInput{},
		[]deliveryapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: qty,
		}})
	if err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	if _, err := m.delivery.Service().Confirm(ctx, 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{}); err != nil {
		t.Fatalf("confirm delivery: %v", err)
	}
}

// createSalesReturn drafts a full-line return for qty units.
func createSalesReturn(t *testing.T, fx shopFixture, soID int64, qty float64) *salesreturncontracts.SalesReturn {
	t.Helper()
	ret, err := testMods.salesreturn.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", "return_only",
		[]salesreturnapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: qty,
		}}, nil)
	if err != nil {
		t.Fatalf("create return: %v", err)
	}
	return ret
}

// TestReturnFullDiscountTaxToZero is the D1 money test: a fully returned
// discounted + taxed order must zero the receivable, with PPN exactly
// reversed through 2200 (SPT must not report tax on returned sales).
func TestReturnFullDiscountTaxToZero(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	so, err := m.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod",
		TaxType: "exclude", TaxRate: 11,
		Items: []salesapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs",
			Qty: 2, DiscountPct: 10, DiscountNominal: 1000,
		}},
	})
	if err != nil {
		t.Fatalf("create SO: %v", err)
	}
	// gross 30000 − pct 3000 − nominal 1000 = net 26000; tax 11% = 2860;
	// total 28860 (server-side pricing from catalog @15000).
	if so.GrandTotal != 28860 {
		t.Fatalf("SO total = %d, want 28860", so.GrandTotal)
	}
	if _, err := m.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
		t.Fatalf("confirm SO: %v", err)
	}
	deliverAll(t, ctx(), fx, so.ID, 2)

	pay, err := m.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID,
		"cash", "", "", so.GrandTotal,
		[]paymentapp.AllocationInput{{OrderType: paymentcontracts.OrderSales, OrderID: so.ID, Amount: so.GrandTotal}})
	if err != nil {
		t.Fatalf("pay SO: %v", err)
	}
	_ = pay

	ret := createSalesReturn(t, fx, so.ID, 2)
	if ret.Subtotal != 26000 || ret.DiscountTotal != 4000 || ret.TaxTotal != 2860 || ret.Total != 28860 {
		t.Fatalf("return header = subtotal %d discount %d tax %d total %d, want 26000/4000/2860/28860",
			ret.Subtotal, ret.DiscountTotal, ret.TaxTotal, ret.Total)
	}
	if len(ret.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(ret.Items))
	}
	it := ret.Items[0]
	if it.TaxBase != 26000 || it.TaxAmount != 2860 || it.LineTotal != 28860 ||
		it.DiscountPct != 10 || it.DiscountNominal != 1000 {
		t.Fatalf("return line = pct %v base %d tax %d total %d nominal %d",
			it.DiscountPct, it.TaxBase, it.TaxAmount, it.LineTotal, it.DiscountNominal)
	}
	if _, err := testMods.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm return: %v", err)
	}

	entry, err := m.finance.Service().Post(ctx(), 0, fx.branchID, "sales_return", ret.ID)
	if err != nil {
		t.Fatalf("post return: %v", err)
	}
	var dr4100, dr2200, cr1200 int64
	for _, l := range entry.Lines {
		switch l.AccountCode {
		case "4100":
			dr4100 += l.Debit
		case "2200":
			dr2200 += l.Debit
		case "1200":
			cr1200 += l.Credit
		}
	}
	if dr4100 != 26000 {
		t.Fatalf("4100 debit = %d, want 26000 (subtotal, not total)", dr4100)
	}
	if dr2200 != 2860 {
		t.Fatalf("2200 debit = %d, want 2860 (exact PPN reversal)", dr2200)
	}
	if cr1200 != 28860 {
		t.Fatalf("1200 credit = %d, want 28860", cr1200)
	}

	refund, err := m.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID,
		"cash", "", "", 28860,
		[]paymentapp.AllocationInput{{OrderType: paymentcontracts.OrderSalesReturn, OrderID: ret.ID, Amount: 28860}})
	if err != nil {
		t.Fatalf("refund: %v", err)
	}
	_ = refund
	bal, err := m.payment.Service().GetPartyBalance(ctx(), fx.branchID, fx.customerID)
	if err != nil {
		t.Fatalf("party balance: %v", err)
	}
	if bal.Outstanding != 0 {
		t.Fatalf("outstanding = %d, want 0 (billed=%d paid=%d returned=%d refunded=%d)",
			bal.Outstanding, bal.TotalBilled, bal.TotalPaid, bal.TotalReturned, bal.TotalRefunded)
	}
}

// TestReturnPartialProRata: 40% qty back means exactly 40% of subtotal and
// 40% of tax (clean numbers, no rounding slack allowed).
func TestReturnPartialProRata(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	so, err := m.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod",
		TaxType: "exclude", TaxRate: 10,
		Items: []salesapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs",
			Qty: 10, DiscountPct: 10,
		}},
	})
	if err != nil {
		t.Fatalf("create SO: %v", err)
	}
	// gross 150000 − disc 15000 = 135000; tax 10% = 13500; total 148500
	// (server prices from catalog: 10 × 15000).
	if so.GrandTotal != 148500 {
		t.Fatalf("SO total = %d, want 148500", so.GrandTotal)
	}
	if _, err := m.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
		t.Fatalf("confirm SO: %v", err)
	}
	deliverAll(t, ctx(), fx, so.ID, 10)

	ret := createSalesReturn(t, fx, so.ID, 4)
	if ret.Subtotal != 54000 || ret.TaxTotal != 5400 || ret.Total != 59400 {
		t.Fatalf("partial = subtotal %d tax %d total %d, want 54000/5400/59400",
			ret.Subtotal, ret.TaxTotal, ret.Total)
	}
}

// TestDeliveryEmptiesStockBooksFullCOGS guards the emptying edge: when a
// delivery takes the last units, COGS must be the recorded movement value,
// not the live average (which reads zero once the position is empty).
// Stock comes from a receipt (exact 10000 cost), never from zero-cost
// opening adjustments.
func TestDeliveryEmptiesStockBooksFullCOGS(t *testing.T) {
	freshDB(t)
	fx := costedProduct(t, newShop(t, ctx()), "TST-P3", "Barang Uji Kirim")
	m := testMods
	receivePO(t, fx, 10000, 5)

	so, err := m.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod",
		TaxType: "none",
		Items: []salesapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 5,
		}},
	})
	if err != nil {
		t.Fatalf("create SO: %v", err)
	}
	if _, err := m.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
		t.Fatalf("confirm SO: %v", err)
	}
	dn, err := m.delivery.Service().Create(ctx(), 0, fx.branchID, so.ID, "2026-09-10", "", deliveryapp.DocInput{
		DriverName: "Budi", VehiclePlate: "B1234CD", WarehouseStaffName: "Gudang",
		DropLocationNote: "Titip satpam",
	},
		[]deliveryapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: 5,
		}})
	if err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	confirmed, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{
		RecipientName: "Ani", RecipientSignatureStatus: "signed",
	})
	if err != nil {
		t.Fatalf("confirm delivery: %v", err)
	}
	if confirmed.DriverName != "Budi" || confirmed.VehiclePlate != "B1234CD" ||
		confirmed.WarehouseStaffName != "Gudang" || confirmed.DropLocationNote != "Titip satpam" {
		t.Fatalf("doc fields not persisted: %+v", confirmed)
	}
	if confirmed.RecipientName != "Ani" || confirmed.RecipientSignatureStatus != "signed" {
		t.Fatalf("receipt fields not persisted: %+v", confirmed)
	}
	if confirmed.DispatchedAt == "" || confirmed.ConfirmedAt == "" {
		t.Fatalf("dispatch stamps missing: %+v", confirmed)
	}
	entry, err := m.finance.Service().Post(ctx(), 0, fx.branchID, "delivery", dn.ID)
	if err != nil {
		t.Fatalf("post delivery: %v", err)
	}
	var cogs, relief int64
	for _, l := range entry.Lines {
		switch l.AccountCode {
		case "5000":
			cogs += l.Debit
		case "1300":
			relief += l.Credit
		}
	}
	// 5 units received @10000, all shipped: full relief, not zero.
	if cogs != 50000 || relief != 50000 {
		t.Fatalf("cogs=%d relief=%d, want 50000/50000", cogs, relief)
	}
}

// TestSalesReturnRestockUsesRecordedCost locks restock valuation to the
// return's own recorded in-movement.
func TestSalesReturnRestockUsesRecordedCost(t *testing.T) {
	freshDB(t)
	fx := costedProduct(t, newShop(t, ctx()), "TST-P4", "Barang Uji Restock")
	m := testMods
	receivePO(t, fx, 10000, 5)

	so, err := m.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod",
		TaxType: "none",
		Items: []salesapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 5,
		}},
	})
	if err != nil {
		t.Fatalf("create SO: %v", err)
	}
	if _, err := m.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
		t.Fatalf("confirm SO: %v", err)
	}
	dn, err := m.delivery.Service().Create(ctx(), 0, fx.branchID, so.ID, "2026-09-10", "", deliveryapp.DocInput{},
		[]deliveryapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: 3,
		}})
	if err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	if _, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{}); err != nil {
		t.Fatalf("confirm delivery: %v", err)
	}
	ret := createSalesReturn(t, fx, so.ID, 2)
	if _, err := testMods.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm return: %v", err)
	}
	entry, err := m.finance.Service().Post(ctx(), 0, fx.branchID, "sales_return", ret.ID)
	if err != nil {
		t.Fatalf("post return: %v", err)
	}
	var restock int64
	for _, l := range entry.Lines {
		if l.AccountCode == "1300" {
			restock += l.Debit
		}
	}
	// 2 units back at the 10000 moving average.
	if restock != 20000 {
		t.Fatalf("restock Dr1300 = %d, want 20000", restock)
	}
}

// TestReturnUnmatchedLineRejected locks the B2 failure mode: a line that
// cannot match a source line is rejected, never priced from live catalog.
func TestReturnUnmatchedLineRejected(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	other, err := m.product.Service().CreateProduct(ctx(), 0, productapp.ProductInput{
		Code: "TST-002", Name: "Barang Asing", Type: "barang", Tracked: true,
		BaseUOM: "pcs", PurchaseUOM: "pcs", SalesUOM: "pcs",
		PurchaseFactor: 1, SalesFactor: 1, PurchasePrice: 5000, SellingPrice: 7000,
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	soID := sellFixture(t, ctx(), fx, 2)
	deliverAll(t, ctx(), fx, soID, 2)

	_, err = testMods.salesreturn.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", "return_only",
		[]salesreturnapp.LineInput{{
			ProductID: other.ID, VariantID: other.Variants[0].ID,
			LocationID: fx.locationID, UOM: "pcs", Qty: 1,
		}}, nil)
	if err == nil {
		t.Fatal("expected rejection for unmatched line, got nil")
	}
}
