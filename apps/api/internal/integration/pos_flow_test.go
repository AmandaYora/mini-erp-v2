package integration

import (
	"strings"
	"sync"
	"testing"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	salesapp "mini-erp/internal/modules/sales/application"
	stockapp "mini-erp/internal/modules/stock/application"
)

func posOrderInput(fx shopFixture, ref string, qty float64) salesapp.OrderInput {
	return salesapp.OrderInput{
		Channel: "pos", PaymentTerms: "cod", TaxType: "none", ClientRef: ref,
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: qty}},
	}
}

func balanceAt(t *testing.T, fx shopFixture, loc int64) (onHand, reserved float64) {
	t.Helper()
	b, err := testMods.stock.Service().GetBalance(ctx(), fx.branchID, fx.productID, fx.variantID, loc)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	return b.OnHand, b.Reserved
}

// F2a-12 #1: the same cart reference never creates a second order, even when
// the requests race; the same reference with other content is refused.
func TestPosOrderClientRefIdempotent(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.sales.Service()
	var wg sync.WaitGroup
	ids := make([]int64, 8)
	errs := make([]error, 8)
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			o, _, err := svc.CreateOrderIdempotent(ctx(), 0, fx.branchID, posOrderInput(fx, "cart-1", 2))
			errs[i] = err
			if o != nil {
				ids[i] = o.ID
			}
		}(i)
	}
	wg.Wait()
	for i := range ids {
		if errs[i] != nil || ids[i] == 0 || ids[i] != ids[0] {
			t.Fatalf("call %d: id=%d err=%v; want one shared order", i, ids[i], errs[i])
		}
	}
	var n int
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM sales_orders WHERE client_ref = 'cart-1'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("orders with ref = %d, %v; want 1", n, err)
	}
	if _, _, err := svc.CreateOrderIdempotent(ctx(), 0, fx.branchID, posOrderInput(fx, "cart-1", 3)); err == nil {
		t.Fatal("same reference with different content was accepted")
	}
}

// F2a-12 #2/#3: the checkout runs step by step with the server telling where to
// resume; the hold is consumed by the shipment (reserved back to 0, on-hand
// down once), the leftover of a partial line is released, and the single
// confirm call settles the payment.
func TestPosFlowResumeAndHoldConsumption(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	sales, deliv, stock := testMods.sales.Service(), testMods.delivery.Service(), testMods.stock.Service()
	step := func() string {
		t.Helper()
		st, err := sales.StateByClientRef(ctx(), fx.branchID, "cart-2")
		if err != nil {
			t.Fatalf("state: %v", err)
		}
		return st.NextStep
	}

	o, _, err := sales.CreateOrderIdempotent(ctx(), 0, fx.branchID, posOrderInput(fx, "cart-2", 4))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got := step(); got != salesapp.StepConfirm {
		t.Fatalf("after create next = %q, want confirm", got)
	}
	if _, err := stock.Hold(ctx(), 0, fx.branchID, fx.productID, fx.variantID, fx.locationID, 5, "pos:cart-2:1", 20); err != nil {
		t.Fatalf("hold: %v", err)
	}
	if _, err := sales.ConfirmOrder(ctx(), 0, fx.branchID, o.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if got := step(); got != salesapp.StepCreateDelivery {
		t.Fatalf("after confirm next = %q, want create_delivery", got)
	}
	lines := []deliveryapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID,
		LocationID: fx.locationID, UOM: "pcs", Qty: 4, ReservationKey: "pos:cart-2:1"}}
	dn, replayed, err := deliv.CreateIdempotent(ctx(), 0, fx.branchID, o.ID, "", "", deliveryapp.DocInput{ClientRef: "cart-2"}, lines)
	if err != nil || replayed {
		t.Fatalf("create delivery: replayed=%v err=%v", replayed, err)
	}
	again, replayed, err := deliv.CreateIdempotent(ctx(), 0, fx.branchID, o.ID, "", "", deliveryapp.DocInput{ClientRef: "cart-2"}, lines)
	if err != nil || !replayed || again.ID != dn.ID {
		t.Fatalf("replayed create: id=%v replayed=%v err=%v; want the first note", again, replayed, err)
	}
	if got := step(); got != salesapp.StepConfirmDelivery {
		t.Fatalf("after draft next = %q, want confirm_delivery", got)
	}
	_, reservedBefore := balanceAt(t, fx, fx.locationID)
	if reservedBefore != 5 {
		t.Fatalf("reserved before confirm = %v, want 5", reservedBefore)
	}
	total := orderTotal(t, o.ID)
	if _, err := deliv.Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{
		Settlement: &deliveryapp.Settlement{Mode: deliveryapp.SettlePayNow, Method: "cash", AmountTendered: total + 5000},
	}); err != nil {
		t.Fatalf("confirm delivery: %v", err)
	}
	onHand, reserved := balanceAt(t, fx, fx.locationID)
	if onHand != 46 || reserved != 0 {
		t.Fatalf("after confirm on_hand=%v reserved=%v; want 46 and 0 (hold consumed, leftover released)", onHand, reserved)
	}
	if got := step(); got != salesapp.StepDone {
		t.Fatalf("after pay next = %q, want done", got)
	}
	// Confirm again: replay, stock moves once.
	if _, err := deliv.Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{}); err != nil {
		t.Fatalf("replayed confirm: %v", err)
	}
	if onHand, _ := balanceAt(t, fx, fx.locationID); onHand != 46 {
		t.Fatalf("replayed confirm moved stock again: on_hand=%v", onHand)
	}
}

// F2a-12 #4: the last units go to the first cashier; the second hold is refused.
func TestPosTwoCashiersRaceForLastStock(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	stock := testMods.stock.Service()
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = stock.Hold(ctx(), 0, fx.branchID, fx.productID, fx.variantID, fx.locationID, 40, []string{"pos:a", "pos:b"}[i], 20)
		}(i)
	}
	wg.Wait()
	wins := 0
	for _, e := range errs {
		if e == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("winning holds = %d (%v); want exactly 1 of 2", wins, errs)
	}
	if _, reserved := balanceAt(t, fx, fx.locationID); reserved != 40 {
		t.Fatalf("reserved = %v, want 40", reserved)
	}
}

// F2a-12 #5: a stock line without a location is allocated across locations.
func TestDeliveryAutoAllocatesAcrossLocations(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	second, err := testMods.stock.Service().CreateLocation(ctx(), 0, fx.branchID, stockapp.LocationInput{Code: "TST2", Name: "Gudang Uji 2"})
	if err != nil {
		t.Fatalf("location: %v", err)
	}
	seedIn(t, fx, fx.productID, fx.variantID, second.ID, 30)
	soID := sellFixture(t, ctx(), fx, 70) // 50 at the first location + 30 at the second
	dn, err := testMods.delivery.Service().Create(ctx(), 0, fx.branchID, soID, "", "", deliveryapp.DocInput{},
		[]deliveryapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 70}})
	if err != nil {
		t.Fatalf("create (auto allocation): %v", err)
	}
	if len(dn.Items) != 2 {
		t.Fatalf("items = %d, want 2 (one per location)", len(dn.Items))
	}
	var sum float64
	for _, it := range dn.Items {
		sum += it.QtyBase
	}
	if sum != 70 {
		t.Fatalf("allocated %v, want 70", sum)
	}
	// Not enough stock overall: refused.
	so2 := sellFixture(t, ctx(), fx, 200)
	if _, err := testMods.delivery.Service().Create(ctx(), 0, fx.branchID, so2, "", "", deliveryapp.DocInput{},
		[]deliveryapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 200}}); err == nil {
		t.Fatal("auto allocation beyond stock was accepted")
	}
}

// F2a-12 #6: a POS order that stays confirmed for over 10 minutes shows up in the
// delivery queue flagged as orphaned; a fresh one does not.
func TestAbandonedPosOrderSurfaces(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	sales := testMods.sales.Service()
	o, _, err := sales.CreateOrderIdempotent(ctx(), 0, fx.branchID, posOrderInput(fx, "cart-orphan", 1))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := sales.ConfirmOrder(ctx(), 0, fx.branchID, o.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	open, err := sales.OpenShipments(ctx(), fx.branchID, 50)
	if err != nil || len(open) != 0 {
		t.Fatalf("fresh POS order in queue: %d, %v; want none", len(open), err)
	}
	if _, err := testMods.db.Exec(`UPDATE sales_orders SET updated_at = UTC_TIMESTAMP(6) - INTERVAL 11 MINUTE WHERE id = ?`, o.ID); err != nil {
		t.Fatalf("age order: %v", err)
	}
	open, err = sales.OpenShipments(ctx(), fx.branchID, 50)
	if err != nil || len(open) != 1 || !open[0].OrphanPos {
		t.Fatalf("queue = %+v, %v; want one orphanPos order", open, err)
	}
}

// F2a-12 #8/#9: a hold follows its basket (upsert), a foreign key conflicts, an
// expired hold is returned by the sweeper, and a revived hold needs stock again.
func TestHoldUpsertAndSweeper(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	stock := testMods.stock.Service()
	hold := func(qty float64, key string) error {
		_, err := stock.Hold(ctx(), 0, fx.branchID, fx.productID, fx.variantID, fx.locationID, qty, key, 20)
		return err
	}
	if err := hold(10, "pos:u:1"); err != nil {
		t.Fatalf("hold: %v", err)
	}
	if err := hold(25, "pos:u:1"); err != nil {
		t.Fatalf("raise hold: %v", err)
	}
	if _, r := balanceAt(t, fx, fx.locationID); r != 25 {
		t.Fatalf("reserved after raise = %v, want 25", r)
	}
	if err := hold(5, "pos:u:1"); err != nil {
		t.Fatalf("lower hold: %v", err)
	}
	if _, r := balanceAt(t, fx, fx.locationID); r != 5 {
		t.Fatalf("reserved after lower = %v, want 5", r)
	}
	if err := hold(500, "pos:u:1"); err == nil {
		t.Fatal("raising a hold beyond availability was accepted")
	}
	// Branch-scoped release ignores a stranger.
	if err := stock.Release(ctx(), 0, fx.branchID+999, "pos:u:1"); err != nil {
		t.Fatalf("foreign release: %v", err)
	}
	if _, r := balanceAt(t, fx, fx.locationID); r != 5 {
		t.Fatalf("a foreign branch released the hold: reserved=%v", r)
	}
	if err := stock.Release(ctx(), 0, fx.branchID, "pos:u:1"); err != nil {
		t.Fatalf("release: %v", err)
	}
	// A released key is revived by the same cart line (POS: line removed, then
	// added back): reserved counts it once, the hold is active again.
	if _, r := balanceAt(t, fx, fx.locationID); r != 0 {
		t.Fatalf("reserved after release = %v, want 0", r)
	}
	if err := hold(3, "pos:u:1"); err != nil {
		t.Fatalf("revive released hold: %v", err)
	}
	if _, r := balanceAt(t, fx, fx.locationID); r != 3 {
		t.Fatalf("reserved after reviving a released hold = %v, want 3", r)
	}
	var status string
	if err := testMods.db.QueryRow(`SELECT status FROM stock_reservations WHERE reservation_key = 'pos:u:1'`).Scan(&status); err != nil || status != "active" {
		t.Fatalf("revived hold status = %q, %v; want active", status, err)
	}
	if err := stock.Release(ctx(), 0, fx.branchID, "pos:u:1"); err != nil {
		t.Fatalf("release again: %v", err)
	}
	// Reviving still checks availability: 50 on hand, nothing reserved.
	if err := hold(51, "pos:u:1"); err == nil || !strings.Contains(err.Error(), "Stok tersedia tidak mencukupi") {
		t.Fatalf("reviving beyond availability = %v, want insufficient stock", err)
	}
	if _, r := balanceAt(t, fx, fx.locationID); r != 0 {
		t.Fatalf("reserved after refused revive = %v, want 0", r)
	}
	// A consumed hold (its stock already left) stays final.
	if _, err := testMods.db.Exec(`UPDATE stock_reservations SET status = 'consumed', qty_remaining = 0 WHERE reservation_key = 'pos:u:1'`); err != nil {
		t.Fatalf("consume: %v", err)
	}
	if err := hold(1, "pos:u:1"); err == nil || !strings.Contains(err.Error(), "Reservasi sudah consumed") {
		t.Fatalf("hold on a consumed key = %v, want conflict", err)
	}
	// Sweeper.
	if err := hold(7, "pos:u:2"); err != nil {
		t.Fatalf("hold 2: %v", err)
	}
	if _, err := testMods.db.Exec(`UPDATE stock_reservations SET expires_at = UTC_TIMESTAMP(6) - INTERVAL 1 MINUTE WHERE reservation_key = 'pos:u:2'`); err != nil {
		t.Fatalf("age hold: %v", err)
	}
	n, err := stock.SweepExpired(ctx(), 500)
	if err != nil || n != 1 {
		t.Fatalf("sweep = %d, %v; want 1", n, err)
	}
	if _, r := balanceAt(t, fx, fx.locationID); r != 0 {
		t.Fatalf("reserved after sweep = %v, want 0", r)
	}
	if n, _ := stock.SweepExpired(ctx(), 500); n != 0 {
		t.Fatalf("second sweep = %d, want 0", n)
	}
	// An expired hold of the same position is revived by the same cart.
	if err := hold(9, "pos:u:2"); err != nil {
		t.Fatalf("revive: %v", err)
	}
	if _, r := balanceAt(t, fx, fx.locationID); r != 9 {
		t.Fatalf("reserved after revive = %v, want 9", r)
	}
}
