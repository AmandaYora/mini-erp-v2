package integration

import (
	"context"
	"testing"

	"mini-erp/internal/shared/apperror"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	financeapp "mini-erp/internal/modules/finance/application"
	paymentapp "mini-erp/internal/modules/payment/application"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
)

func ctx() context.Context { return context.Background() }

func mustAppErr(t *testing.T, err error) *apperror.AppError {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	appErr, ok := err.(*apperror.AppError)
	if !ok {
		t.Fatalf("expected *apperror.AppError, got %T (%v)", err, err)
	}
	return appErr
}

// TestFinanceManualRejectsUnbalanced guards the books against one-sided
// entries at the service boundary (P4 is enforced here, not in SQL).
func TestFinanceManualRejectsUnbalanced(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	_, err := m.finance.Service().Manual(ctx(), 0, fx.branchID, "2026-09-10", "uji pincang",
		[]financeapp.ManualLine{
			{AccountCode: "1100", Debit: 1000},
			{AccountCode: "4000", Credit: 999},
		})
	appErr := mustAppErr(t, err)
	if appErr.Code != apperror.CodeValidation {
		t.Fatalf("expected validation, got %s (%s)", appErr.Code, appErr.Message)
	}

	entry, err := m.finance.Service().Manual(ctx(), 0, fx.branchID, "2026-09-10", "uji seimbang",
		[]financeapp.ManualLine{
			{AccountCode: "1100", Debit: 1000},
			{AccountCode: "4000", Credit: 1000},
		})
	if err != nil {
		t.Fatalf("balanced manual: %v", err)
	}
	var debit, credit int64
	for _, l := range entry.Lines {
		debit += l.Debit
		credit += l.Credit
	}
	if debit != 1000 || credit != 1000 {
		t.Fatalf("unbalanced entry persisted: debit=%d credit=%d", debit, credit)
	}
}

// TestFinancePostIdempotent posts one delivery twice: the second call must
// return the same entry without double-booking (L7 idempotency contract).
func TestFinancePostIdempotent(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	soID := sellFixture(t, ctx(), fx, 2)
	dn, err := m.delivery.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", deliveryapp.DocInput{},
		[]deliveryapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: 2,
		}})
	if err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	if _, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{}); err != nil {
		t.Fatalf("confirm delivery: %v", err)
	}

	first, err := m.finance.Service().Post(ctx(), 0, fx.branchID, "delivery", dn.ID)
	if err != nil {
		t.Fatalf("first post: %v", err)
	}
	var debit, credit int64
	for _, l := range first.Lines {
		debit += l.Debit
		credit += l.Credit
	}
	if debit == 0 || debit != credit {
		t.Fatalf("posted entry not balanced: debit=%d credit=%d", debit, credit)
	}
	second, err := m.finance.Service().Post(ctx(), 0, fx.branchID, "delivery", dn.ID)
	if err != nil {
		t.Fatalf("second post: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("re-post created entry %d, want dedupe to %d", second.ID, first.ID)
	}
}

// TestDeliveryConfirmMovesStock walks SO → SJ → confirm and asserts stock
// leaves exactly once and the order completes.
func TestDeliveryConfirmMovesStock(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	before, err := m.stock.Stock().GetBalance(ctx(), fx.branchID, fx.productID, fx.variantID, fx.locationID)
	if err != nil {
		t.Fatalf("balance before: %v", err)
	}
	if before.Available() != 50 {
		t.Fatalf("opening available = %v, want 50", before.Available())
	}
	soID := sellFixture(t, ctx(), fx, 2)
	dn, err := m.delivery.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", deliveryapp.DocInput{},
		[]deliveryapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: 2,
		}})
	if err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	mid, err := m.stock.Stock().GetBalance(ctx(), fx.branchID, fx.productID, fx.variantID, fx.locationID)
	if err != nil {
		t.Fatalf("balance draft: %v", err)
	}
	if mid.Available() != 50 {
		t.Fatalf("draft SJ moved stock: available = %v, want 50", mid.Available())
	}
	if _, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{}); err != nil {
		t.Fatalf("confirm delivery: %v", err)
	}
	after, err := m.stock.Stock().GetBalance(ctx(), fx.branchID, fx.productID, fx.variantID, fx.locationID)
	if err != nil {
		t.Fatalf("balance after: %v", err)
	}
	if after.Available() != 48 {
		t.Fatalf("confirmed available = %v, want 48", after.Available())
	}
	so, err := m.sales.Service().GetByID(ctx(), soID)
	if err != nil {
		t.Fatalf("get SO: %v", err)
	}
	if so.Status != "completed" {
		t.Fatalf("SO status = %q, want completed", so.Status)
	}
}

// TestPaymentCreateAllocation pays a confirmed order and asserts the live
// outstanding math closes to zero.
func TestPaymentCreateAllocation(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	soID := sellFixture(t, ctx(), fx, 2)
	so, err := m.sales.Service().GetByID(ctx(), soID)
	if err != nil {
		t.Fatalf("get SO: %v", err)
	}
	total := so.GrandTotal
	if total <= 0 {
		t.Fatalf("SO total = %d, want positive", total)
	}
	pay, err := m.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID,
		"cash", "", "", total,
		[]paymentapp.AllocationInput{{OrderType: paymentcontracts.OrderSales, OrderID: soID, Amount: total}})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if pay.Amount != total {
		t.Fatalf("payment amount = %d, want %d", pay.Amount, total)
	}
	bal, err := m.payment.Service().GetPartyBalance(ctx(), fx.branchID, fx.customerID)
	if err != nil {
		t.Fatalf("party balance: %v", err)
	}
	if bal.Outstanding != 0 {
		t.Fatalf("outstanding = %d, want 0 (billed=%d paid=%d)", bal.Outstanding, bal.TotalBilled, bal.TotalPaid)
	}
}
