package integration

import (
	"reflect"
	"testing"

	paymentapp "mini-erp/internal/modules/payment/application"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	purchasereturnapp "mini-erp/internal/modules/purchasereturn/application"
	salesreturnapp "mini-erp/internal/modules/salesreturn/application"
)

// K-04d: the preview shows the settlement outlook confirmation will arrive at.
// Same builder, same offset helper: the numbers cannot drift.
func TestSalesPreviewSettlementOutlook(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	lines := func(qty float64) []salesreturnapp.LineInput {
		return []salesreturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: qty}}
	}

	// Unpaid order: the return is absorbed by the open bill, nothing external.
	soID := sellFixture(t, ctx(), fx, 4)
	deliverAll(t, ctx(), fx, soID, 4)
	prev, err := m.salesreturn.Service().PreviewReturn(ctx(), fx.branchID, soID, "return_only", lines(1), nil)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if prev.OwedToCustomer != prev.Total || prev.ReceivableReduction != prev.Total || prev.ExternalRemaining != 0 ||
		len(prev.AllowedSettlementTypes) != 0 || prev.OrderOutstanding != orderTotal(t, soID) || !prev.HasParty {
		t.Fatalf("unpaid-order outlook = %+v", prev)
	}
	created := createSalesReturn(t, fx, soID, 1)
	if _, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, created.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	after, _ := m.salesreturn.Service().GetByID(ctx(), created.ID)
	if after.ReceivableReduction != prev.ReceivableReduction || after.ExternalRemaining != prev.ExternalRemaining ||
		!reflect.DeepEqual(after.AllowedSettlementTypes, prev.AllowedSettlementTypes) {
		t.Fatalf("confirmed %d/%d/%v != preview %d/%d/%v", after.ReceivableReduction, after.ExternalRemaining, after.AllowedSettlementTypes,
			prev.ReceivableReduction, prev.ExternalRemaining, prev.AllowedSettlementTypes)
	}

	// Fully paid order: the whole return is external, refund or customer balance.
	so2 := sellFixture(t, ctx(), fx, 2)
	deliverAll(t, ctx(), fx, so2, 2)
	total := orderTotal(t, so2)
	if _, err := m.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", total, payAll(so2, total)); err != nil {
		t.Fatalf("pay: %v", err)
	}
	prev2, err := m.salesreturn.Service().PreviewReturn(ctx(), fx.branchID, so2, "return_only", lines(2), nil)
	if err != nil {
		t.Fatalf("preview 2: %v", err)
	}
	want := []string{"refund", "customer_credit"}
	if prev2.OrderOutstanding != 0 || prev2.ReceivableReduction != 0 || prev2.ExternalRemaining != prev2.Total || !reflect.DeepEqual(prev2.AllowedSettlementTypes, want) {
		t.Fatalf("paid-order outlook = %+v, want external %d allowed %v", prev2, prev2.Total, want)
	}

	// Exchange: replacement dearer -> surcharge to collect, nothing owed back.
	ex, err := m.salesreturn.Service().PreviewReturn(ctx(), fx.branchID, soID, "exchange", lines(1),
		[]salesreturnapp.ReplacementInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 2}})
	if err != nil {
		t.Fatalf("exchange preview: %v", err)
	}
	if ex.Difference <= 0 || ex.ChargeToCustomer != ex.Difference || ex.OwedToCustomer != 0 || ex.ExternalRemaining != 0 {
		t.Fatalf("exchange outlook = %+v, want a surcharge only", ex)
	}
}

func TestPurchasePreviewSettlementOutlook(t *testing.T) {
	freshDB(t)
	fx := purchaseProduct(t, newShop(t, ctx()))
	m := testMods
	lines := []purchasereturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 2}}

	// Unpaid PO: everything reduces what we owe.
	poID := purchaseFixture(t, ctx(), fx, 10000, 4)
	prev, err := m.purchasereturn.Service().PreviewReturn(ctx(), fx.branchID, poID, lines)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if prev.PayableReduction != prev.Total || prev.ExternalRemaining != 0 || len(prev.AllowedSettlementTypes) != 0 || prev.OrderOutstanding == 0 {
		t.Fatalf("unpaid outlook = %+v", prev)
	}
	created, err := m.purchasereturn.Service().Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "", lines)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := m.purchasereturn.Service().Confirm(ctx(), 0, fx.branchID, created.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	after, _ := m.purchasereturn.Service().GetByID(ctx(), created.ID)
	if after.PayableReduction != prev.PayableReduction || after.ExternalRemaining != prev.ExternalRemaining {
		t.Fatalf("confirmed %d/%d != preview %d/%d", after.PayableReduction, after.ExternalRemaining, prev.PayableReduction, prev.ExternalRemaining)
	}

	// Paid PO: the excess can be refunded or kept as supplier balance.
	po2 := purchaseFixture(t, ctx(), fx, 10000, 2)
	po, _ := m.purchasing.Service().GetByID(ctx(), po2)
	if _, err := m.payment.Service().Create(ctx(), 0, fx.branchID, po.PartyID, "cash", "", "", po.GrandTotal,
		[]paymentapp.AllocationInput{{OrderType: paymentcontracts.OrderPurchase, OrderID: po2, Amount: po.GrandTotal}}); err != nil {
		t.Fatalf("pay: %v", err)
	}
	prev2, err := m.purchasereturn.Service().PreviewReturn(ctx(), fx.branchID, po2, lines)
	if err != nil {
		t.Fatalf("preview 2: %v", err)
	}
	want := []string{"refund", "supplier_credit"}
	if prev2.OrderOutstanding != 0 || prev2.PayableReduction != 0 || prev2.ExternalRemaining != prev2.Total || !reflect.DeepEqual(prev2.AllowedSettlementTypes, want) {
		t.Fatalf("paid outlook = %+v, want external %d allowed %v", prev2, prev2.Total, want)
	}
}
