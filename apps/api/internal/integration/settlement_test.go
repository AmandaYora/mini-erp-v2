package integration

import (
	"strings"
	"testing"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	salesapp "mini-erp/internal/modules/sales/application"
)

func orderWithTerms(t *testing.T, fx shopFixture, partyID int64, terms, due string, qty float64) int64 {
	t.Helper()
	so, err := testMods.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: partyID, Channel: "regular", PaymentTerms: terms, DueDate: due, TaxType: "none",
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: qty}},
	})
	if err != nil {
		t.Fatalf("create %s order: %v", terms, err)
	}
	if _, err := testMods.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	return so.ID
}

// F2a-10b: the shipment that completes an unpaid COD order must settle it; a
// refused settlement leaves the note a draft and the stock untouched.
func TestCODFinalShipmentRequiresSettlement(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 4)
	note, err := draftNote(t, fx, soID, 4)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	svc := testMods.delivery.Service()
	_, err = svc.Confirm(ctx(), 0, fx.branchID, note.ID, deliveryapp.ConfirmInput{})
	ae := mustAppErr(t, err)
	if len(ae.Fields) == 0 || ae.Fields[0].Field != "settlement" {
		t.Fatalf("no settlement = %+v, want a settlement validation error", ae)
	}
	if n := movementCount(t, "delivery", note.ID); n != 0 {
		t.Fatalf("stock moved (%d movements) although settlement was refused", n)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 50 {
		t.Fatalf("onhand = %v, want 50", got)
	}

	pay := &deliveryapp.Settlement{Mode: deliveryapp.SettlePayNow, Method: "cash", AmountTendered: 999999}
	if _, err := svc.Confirm(ctx(), 0, fx.branchID, note.ID, deliveryapp.ConfirmInput{Settlement: pay}); err != nil {
		t.Fatalf("confirm with pay_now: %v", err)
	}
	// Repeating the request replays: still exactly one payment, paid in full.
	if _, err := svc.Confirm(ctx(), 0, fx.branchID, note.ID, deliveryapp.ConfirmInput{Settlement: pay}); err != nil {
		t.Fatalf("repeat confirm: %v", err)
	}
	var n int
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM payments WHERE status='active'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("payments = %d, %v; want 1", n, err)
	}
	paid, _ := testMods.payment.Service().PaidForOrders(ctx(), paymentcontracts.OrderSales, []int64{soID})
	if paid[soID] != orderTotal(t, soID) {
		t.Fatalf("paid = %d, want the order total %d", paid[soID], orderTotal(t, soID))
	}
	if got := orderStatus(t, soID); got != "completed" {
		t.Fatalf("order = %s, want completed", got)
	}
}

// F2a-10b: switch_to_net converts the order to credit terms with the due date.
func TestCODSwitchToNetAtShipment(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 2)
	note, _ := draftNote(t, fx, soID, 2)
	if _, err := testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, note.ID, deliveryapp.ConfirmInput{
		Settlement: &deliveryapp.Settlement{Mode: deliveryapp.SettleSwitchToNet, DueDate: "2026-11-30"},
	}); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	so, _ := testMods.sales.Service().GetByID(ctx(), soID)
	if so.PaymentTerms != "net" || !strings.HasPrefix(so.DueDate, "2026-11-30") {
		t.Fatalf("terms = %s due = %s, want net 2026-11-30", so.PaymentTerms, so.DueDate)
	}
}

// F2a-10b: a partial COD shipment needs no settlement yet.
func TestCODPartialShipmentNeedsNoSettlement(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 10)
	note, _ := draftNote(t, fx, soID, 4)
	if _, err := testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, note.ID, deliveryapp.ConfirmInput{}); err != nil {
		t.Fatalf("partial confirm: %v", err)
	}
}

// F2a-10b: prepaid orders ship only once fully paid; a settlement is refused.
func TestPrepaidShipsOnlyWhenPaid(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := orderWithTerms(t, fx, fx.customerID, "prepaid", "", 2)
	if _, err := draftNote(t, fx, soID, 2); !isConflict(err) {
		t.Fatalf("shipment on an unpaid prepaid order = %v, want conflict", err)
	}
	total := orderTotal(t, soID)
	if _, err := testMods.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", total, payAll(soID, total)); err != nil {
		t.Fatalf("prepay: %v", err)
	}
	note, err := draftNote(t, fx, soID, 2)
	if err != nil {
		t.Fatalf("shipment after payment: %v", err)
	}
	if _, err := testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, note.ID, deliveryapp.ConfirmInput{
		Settlement: &deliveryapp.Settlement{Mode: deliveryapp.SettlePayNow},
	}); err == nil {
		t.Fatal("settlement accepted on a prepaid order")
	}
	if _, err := testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, note.ID, deliveryapp.ConfirmInput{}); err != nil {
		t.Fatalf("confirm paid prepaid: %v", err)
	}
}

// F2a-10b: credit terms need a known customer with a phone number.
func TestNetTermsNeedReachableCustomer(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	mk := func(party int64) error {
		_, err := testMods.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
			PartyID: party, Channel: "regular", PaymentTerms: "net", DueDate: "2026-12-31", TaxType: "none",
			Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1}},
		})
		return err
	}
	if err := mk(0); err == nil {
		t.Fatal("net order for a walk-in accepted")
	}
	if err := mk(fx.customerID); err != nil {
		t.Fatalf("net order for a customer with a phone: %v", err)
	}
}
