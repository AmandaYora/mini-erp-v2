package integration

import (
	"testing"

	paymentapp "mini-erp/internal/modules/payment/application"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
)

// TestPaymentOverpayRejected (A2): money cannot leak in — paying more than
// the open bills is a conflict, both with and without explicit allocations.
func TestPaymentOverpayRejected(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	soID := sellFixture(t, ctx(), fx, 2)
	so, err := m.sales.Service().GetByID(ctx(), soID)
	if err != nil {
		t.Fatalf("get SO: %v", err)
	}
	if _, err := m.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID,
		"cash", "", "", so.GrandTotal+1000, nil); err == nil {
		t.Fatal("expected overpay rejection, got nil")
	}
	if _, err := m.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID,
		"cash", "", "", so.GrandTotal+1000,
		[]paymentapp.AllocationInput{{OrderType: paymentcontracts.OrderSales, OrderID: soID, Amount: so.GrandTotal + 1000}}); err == nil {
		t.Fatal("expected over-allocation rejection, got nil")
	}
	bal, err := m.payment.Service().GetPartyBalance(ctx(), fx.branchID, fx.customerID)
	if err != nil {
		t.Fatalf("party balance: %v", err)
	}
	if bal.Outstanding != so.GrandTotal {
		t.Fatalf("outstanding = %d, want %d (nothing paid)", bal.Outstanding, so.GrandTotal)
	}
}

// TestPaymentCancelRecomputes (A2): cancelling a payment reopens the bills
// — outstanding returns to the full total.
func TestPaymentCancelRecomputes(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	soID := sellFixture(t, ctx(), fx, 2)
	so, err := m.sales.Service().GetByID(ctx(), soID)
	if err != nil {
		t.Fatalf("get SO: %v", err)
	}
	pay, err := m.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID,
		"cash", "", "", so.GrandTotal,
		[]paymentapp.AllocationInput{{OrderType: paymentcontracts.OrderSales, OrderID: soID, Amount: so.GrandTotal}})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	bal, err := m.payment.Service().GetPartyBalance(ctx(), fx.branchID, fx.customerID)
	if err != nil {
		t.Fatalf("party balance: %v", err)
	}
	if bal.Outstanding != 0 {
		t.Fatalf("outstanding = %d, want 0 after full pay", bal.Outstanding)
	}
	if _, err := m.payment.Service().Cancel(ctx(), 0, fx.branchID, pay.ID); err != nil {
		t.Fatalf("cancel payment: %v", err)
	}
	again, err := m.payment.Service().GetPartyBalance(ctx(), fx.branchID, fx.customerID)
	if err != nil {
		t.Fatalf("party balance: %v", err)
	}
	if again.Outstanding != so.GrandTotal {
		t.Fatalf("outstanding = %d, want %d after cancel", again.Outstanding, so.GrandTotal)
	}
}
