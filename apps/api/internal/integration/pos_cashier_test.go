package integration

import (
	"testing"

	deliveryapp "mini-erp/internal/modules/delivery/application"
)

// A cashier without payment.create may take the cash at the counter of a POS
// order, but never settle a regular order that way.
func TestPosOnlySettlementScope(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	deliv := testMods.delivery.Service()
	pay := &deliveryapp.Settlement{PosOnly: true, Mode: deliveryapp.SettlePayNow, Method: "cash"}

	reg := sellFixture(t, ctx(), fx, 1)
	n, err := draftNote(t, fx, reg, 1)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if _, err := deliv.Confirm(ctx(), 0, fx.branchID, n.ID, deliveryapp.ConfirmInput{Settlement: pay}); err == nil {
		t.Fatal("PosOnly settlement accepted on a regular order")
	}

	posSO, _, err := testMods.sales.Service().CreateOrderIdempotent(ctx(), 0, fx.branchID, posOrderInput(fx, "cashier-1", 1))
	if err != nil {
		t.Fatalf("pos order: %v", err)
	}
	if _, err := testMods.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, posSO.ID); err != nil {
		t.Fatalf("confirm pos: %v", err)
	}
	pn, err := draftNote(t, fx, posSO.ID, 1)
	if err != nil {
		t.Fatalf("pos draft: %v", err)
	}
	if _, err := deliv.Confirm(ctx(), 0, fx.branchID, pn.ID, deliveryapp.ConfirmInput{Settlement: pay}); err != nil {
		t.Fatalf("cashier pay_now on a POS order: %v", err)
	}
}

// An archived customer can still pay what it owes (legacy BR-05).
func TestArchivedCustomerCanPayReceivable(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 2)
	deliverAll(t, ctx(), fx, soID, 2)
	total := orderTotal(t, soID)
	if _, err := testMods.db.Exec(`UPDATE parties SET status = 'archived' WHERE id = ?`, fx.customerID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if _, err := testMods.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", total, payAll(soID, total)); err != nil {
		t.Fatalf("archived customer could not pay: %v", err)
	}
}
