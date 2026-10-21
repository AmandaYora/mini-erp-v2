package integration

import (
	"testing"

	paymentapp "mini-erp/internal/modules/payment/application"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	purchasereturnapp "mini-erp/internal/modules/purchasereturn/application"
	salesreturnapp "mini-erp/internal/modules/salesreturn/application"
)

func refundAlloc(retType string, retID, amount int64) []paymentapp.AllocationInput {
	return []paymentapp.AllocationInput{{OrderType: retType, OrderID: retID, Amount: amount}}
}

// N-40: the part of a return kept as customer balance can no longer be taken a
// second time through payments, neither as cash refund nor as a credit offset.
func TestCreditedReturnCannotBeUsedTwiceSales(t *testing.T) {
	freshDB(t)
	fx, _, retID, total := creditSalesFixture(t)
	m := testMods
	half := total / 2
	if _, err := m.salesreturn.Service().AddSettlement(ctx(), 0, fx.branchID, retID,
		salesreturnapp.SettlementInput{Type: "customer_credit", Amount: half}); err != nil {
		t.Fatalf("credit half: %v", err)
	}
	// The projection payments read carries what was credited.
	proj, err := m.salesreturn.Service().GetReturn(ctx(), retID)
	if err != nil || proj.CreditSettled != half {
		t.Fatalf("projection credit = %+v err %v, want %d", proj, err, half)
	}

	// Refund beyond the uncredited remainder is refused, the remainder is fine.
	if _, err := m.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", total-half+1,
		refundAlloc(paymentcontracts.OrderSalesReturn, retID, total-half+1)); err == nil {
		t.Fatal("refund over the uncredited remainder must be refused")
	}
	// Credit offset against a new bill: same cap.
	so2 := sellFixture(t, ctx(), fx, 2)
	if _, err := m.payment.Service().SettleCredit(ctx(), 0, fx.branchID, fx.customerID, paymentcontracts.OrderSalesReturn, retID,
		paymentcontracts.OrderSales, so2, total-half+1, ""); err == nil {
		t.Fatal("credit offset over the uncredited remainder must be refused")
	}
	if _, err := m.payment.Service().SettleCredit(ctx(), 0, fx.branchID, fx.customerID, paymentcontracts.OrderSalesReturn, retID,
		paymentcontracts.OrderSales, so2, total-half, ""); err != nil {
		t.Fatalf("offset of the uncredited remainder: %v", err)
	}
	// Nothing left for a refund now.
	if _, err := m.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", 1,
		refundAlloc(paymentcontracts.OrderSalesReturn, retID, 1)); err == nil {
		t.Fatal("a fully taken return must not be refundable again")
	}
}

// N-40 other direction: money already refunded through payments cannot be
// credited again as a balance.
func TestRefundedReturnCannotBeCreditedSales(t *testing.T) {
	freshDB(t)
	fx, _, retID, total := creditSalesFixture(t)
	m := testMods
	half := total / 2
	if _, err := m.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", half,
		refundAlloc(paymentcontracts.OrderSalesReturn, retID, half)); err != nil {
		t.Fatalf("refund half: %v", err)
	}
	if _, err := m.salesreturn.Service().AddSettlement(ctx(), 0, fx.branchID, retID,
		salesreturnapp.SettlementInput{Type: "customer_credit", Amount: total - half + 1}); err == nil {
		t.Fatal("credit over what is still unrefunded must be refused")
	}
	if _, err := m.salesreturn.Service().AddSettlement(ctx(), 0, fx.branchID, retID,
		salesreturnapp.SettlementInput{Type: "customer_credit", Amount: total - half}); err != nil {
		t.Fatalf("credit the unrefunded rest: %v", err)
	}
}

func TestCreditedReturnCannotBeUsedTwicePurchase(t *testing.T) {
	freshDB(t)
	fx := purchaseProduct(t, newShop(t, ctx()))
	m := testMods
	poID := purchaseFixture(t, ctx(), fx, 10000, 2)
	po, err := m.purchasing.Service().GetByID(ctx(), poID)
	if err != nil || po == nil {
		t.Fatalf("get PO: %v", err)
	}
	if _, err := m.payment.Service().Create(ctx(), 0, fx.branchID, po.PartyID, "cash", "", "", po.GrandTotal,
		[]paymentapp.AllocationInput{{OrderType: paymentcontracts.OrderPurchase, OrderID: poID, Amount: po.GrandTotal}}); err != nil {
		t.Fatalf("pay PO: %v", err)
	}
	ret, err := m.purchasereturn.Service().Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "",
		[]purchasereturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 2}})
	if err != nil {
		t.Fatalf("create return: %v", err)
	}
	if _, err := m.purchasereturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	half := ret.Total / 2
	if _, err := m.purchasereturn.Service().AddSettlement(ctx(), 0, fx.branchID, ret.ID,
		purchasereturnapp.SettlementInput{Type: "supplier_credit", Amount: half}); err != nil {
		t.Fatalf("credit half: %v", err)
	}
	rest := ret.Total - half
	if _, err := m.payment.Service().Create(ctx(), 0, fx.branchID, po.PartyID, "cash", "", "", rest+1,
		refundAlloc(paymentcontracts.OrderPurchaseReturn, ret.ID, rest+1)); err == nil {
		t.Fatal("supplier refund over the uncredited remainder must be refused")
	}
	if _, err := m.payment.Service().Create(ctx(), 0, fx.branchID, po.PartyID, "cash", "", "", rest,
		refundAlloc(paymentcontracts.OrderPurchaseReturn, ret.ID, rest)); err != nil {
		t.Fatalf("supplier refund of the remainder: %v", err)
	}
	// Refunded money cannot be credited again.
	if _, err := m.purchasereturn.Service().AddSettlement(ctx(), 0, fx.branchID, ret.ID,
		purchasereturnapp.SettlementInput{Type: "supplier_credit", Amount: 1}); err == nil {
		t.Fatal("credit after the remainder was refunded must be refused")
	}
}
