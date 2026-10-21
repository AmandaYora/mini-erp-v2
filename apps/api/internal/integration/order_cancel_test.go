package integration

import (
	"sync"
	"testing"

	salesapp "mini-erp/internal/modules/sales/application"
)

// F2a-6b #1: an order cannot be cancelled while a shipment, payment or
// receipt hangs off it — and can once those are cleared.
func TestCancelSalesOrderBlockedByDependents(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 10)

	note, err := draftNote(t, fx, soID, 4)
	if err != nil {
		t.Fatalf("draft shipment: %v", err)
	}
	if _, err := testMods.sales.Service().CancelOrder(ctx(), 0, fx.branchID, soID); !isConflict(err) {
		t.Fatalf("cancel with a draft shipment = %v, want conflict", err)
	}
	if _, err := testMods.delivery.Service().Cancel(ctx(), 0, fx.branchID, note.ID, ""); err != nil {
		t.Fatalf("cancel shipment: %v", err)
	}

	total := orderTotal(t, soID)
	pay, err := testMods.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", total/2, payAll(soID, total/2))
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	if _, err := testMods.sales.Service().CancelOrder(ctx(), 0, fx.branchID, soID); !isConflict(err) {
		t.Fatalf("cancel with an active payment = %v, want conflict", err)
	}
	if _, err := testMods.payment.Service().Cancel(ctx(), 0, fx.branchID, pay.ID); err != nil {
		t.Fatalf("cancel payment: %v", err)
	}

	cancelled, err := testMods.sales.Service().CancelOrder(ctx(), 0, fx.branchID, soID)
	if err != nil || cancelled.Status != "cancelled" {
		t.Fatalf("cancel after clearing = %+v, %v", cancelled, err)
	}
	// A repeat is a no-op success.
	if again, err := testMods.sales.Service().CancelOrder(ctx(), 0, fx.branchID, soID); err != nil || again.Status != "cancelled" {
		t.Fatalf("repeat cancel = %+v, %v", again, err)
	}
}

// F2a-6b: a purchase order with a receipt cannot be cancelled; a discarded
// (void) receipt no longer blocks.
func TestCancelPurchaseOrderBlockedByReceipt(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	poID := confirmedPO(t, fx, 10)
	rec := insertPendingReceipt(t, fx, poID, 10, "GR-CANCEL-1")
	if _, err := testMods.purchasing.Service().CancelOrder(ctx(), 0, fx.branchID, poID, "salah input"); !isConflict(err) {
		t.Fatalf("cancel PO with a pending receipt = %v, want conflict", err)
	}
	if _, err := testMods.goodsreceipt.Service().Discard(ctx(), 0, fx.branchID, rec, ""); err != nil {
		t.Fatalf("discard: %v", err)
	}
	po, err := testMods.purchasing.Service().CancelOrder(ctx(), 0, fx.branchID, poID, "salah input")
	if err != nil || po.Status != "cancelled" {
		t.Fatalf("cancel after discard = %+v, %v", po, err)
	}
}

// F2a-6b #2: cancelling an order races a new shipment draft — the order lock
// makes the outcome consistent: never a live shipment on a cancelled order.
func TestCancelSalesOrderVersusCreateShipment(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 10)
	var wg sync.WaitGroup
	var cancelErr, createErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, cancelErr = testMods.sales.Service().CancelOrder(ctx(), 0, fx.branchID, soID)
	}()
	go func() { defer wg.Done(); _, createErr = draftNote(t, fx, soID, 5) }()
	wg.Wait()
	st := orderStatus(t, soID)
	var live int
	if err := testMods.db.QueryRow(
		`SELECT COUNT(*) FROM delivery_notes WHERE sales_order_id = ? AND status IN ('draft','confirmed')`, soID).Scan(&live); err != nil {
		t.Fatalf("count: %v", err)
	}
	if st == "cancelled" && live != 0 {
		t.Fatalf("order cancelled but %d live shipment(s) remain", live)
	}
	if st != "cancelled" && cancelErr == nil {
		t.Fatalf("cancel reported success but order is %s", st)
	}
	if st == "cancelled" && createErr == nil {
		t.Fatal("shipment was created on a cancelled order")
	}
}

// Confirming twice replays (the POS retries a checkout) and never errors.
func TestConfirmOrderReplay(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	so, err := testMods.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none",
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	for i := 0; i < 2; i++ {
		got, err := testMods.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, so.ID)
		if err != nil || got.Status != "confirmed" {
			t.Fatalf("confirm #%d = %+v, %v", i+1, got, err)
		}
	}
}

// F2a-6b: SwitchToNet is a CAS on the terms — cod -> net once, an identical
// repeat replays, another due date on a net order conflicts.
func TestSwitchToNetCAS(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 1) // confirmed, cod
	svc := testMods.sales.Service()
	if err := svc.SwitchToNet(ctx(), fx.branchID, soID, "2026-12-31", 0); err != nil {
		t.Fatalf("switch: %v", err)
	}
	if err := svc.SwitchToNet(ctx(), fx.branchID, soID, "2026-12-31", 0); err != nil {
		t.Fatalf("identical repeat: %v", err)
	}
	if err := svc.SwitchToNet(ctx(), fx.branchID, soID, "2027-01-15", 0); !isConflict(err) {
		t.Fatalf("different due date on a net order = %v, want conflict", err)
	}
	so, _ := svc.GetByID(ctx(), soID)
	if so.PaymentTerms != "net" {
		t.Fatalf("terms = %s, want net", so.PaymentTerms)
	}
}
