package integration

import (
	"sync"
	"testing"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	salesapp "mini-erp/internal/modules/sales/application"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/doclock"
)

func draftNote(t *testing.T, fx shopFixture, soID int64, qtys ...float64) (*deliverycontracts.DeliveryNote, error) {
	t.Helper()
	lines := make([]deliveryapp.LineInput, 0, len(qtys))
	for _, q := range qtys {
		lines = append(lines, deliveryapp.LineInput{
			ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: q,
		})
	}
	return testMods.delivery.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", deliveryapp.DocInput{}, lines)
}

func orderStatus(t *testing.T, soID int64) string {
	t.Helper()
	var st string
	if err := testMods.db.QueryRow(`SELECT status FROM sales_orders WHERE id = ?`, soID).Scan(&st); err != nil {
		t.Fatalf("order status: %v", err)
	}
	return st
}

// F2a-2 #1: lines of one request accumulate against the order's remainder.
func TestDeliveryQuotaAccumulatesWithinRequest(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 10)
	if _, err := draftNote(t, fx, soID, 6, 6); !isConflict(err) {
		t.Fatalf("6+6 of 10 = %v, want conflict", err)
	}
	if _, err := draftNote(t, fx, soID, 6, 4); err != nil {
		t.Fatalf("6+4 of 10: %v", err)
	}
}

// F2a-2 #3: drafts hold quota; confirming both completes the order and moves
// exactly the ordered stock.
func TestDeliveryDraftsHoldQuota(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 10)
	a, err := draftNote(t, fx, soID, 6)
	if err != nil {
		t.Fatalf("draft 6: %v", err)
	}
	b, err := draftNote(t, fx, soID, 4)
	if err != nil {
		t.Fatalf("draft 4: %v", err)
	}
	if _, err := draftNote(t, fx, soID, 1); !isConflict(err) {
		t.Fatalf("third draft = %v, want conflict", err)
	}
	svc := testMods.delivery.Service()
	if _, err := svc.Confirm(ctx(), 0, fx.branchID, a.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); err != nil {
		t.Fatalf("confirm a: %v", err)
	}
	if got := orderStatus(t, soID); got != "confirmed" {
		t.Fatalf("order after first note = %s, want confirmed", got)
	}
	if _, err := svc.Confirm(ctx(), 0, fx.branchID, b.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); err != nil {
		t.Fatalf("confirm b: %v", err)
	}
	if got := orderStatus(t, soID); got != "completed" {
		t.Fatalf("order after both = %s, want completed", got)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 40 {
		t.Fatalf("onhand = %v, want 40", got)
	}
}

// F2a-2: two simultaneous drafts that each want the whole remainder → one wins.
func TestDeliveryConcurrentCreateOneWins(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 10)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = draftNote(t, fx, soID, 10)
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		} else if !isConflict(err) {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("successful creates = %d, want 1", ok)
	}
}

// F2a-2 #4: two goroutines confirming the same note (process lock off, so the
// CAS and the idempotent Post are what protect us) → stock leaves once.
func TestDeliveryConcurrentConfirmOnce(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 10)
	n, err := draftNote(t, fx, soID, 10)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	doclock.SetBypass(true)
	defer doclock.SetBypass(false)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, n.ID, deliveryapp.ConfirmInput{Settlement: codSettle})
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("confirm %d: %v", i, err)
		}
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 40 {
		t.Fatalf("onhand = %v, want 40 (once)", got)
	}
	if c := movementCount(t, "delivery", n.ID); c != 1 {
		t.Fatalf("movements = %d, want 1", c)
	}
	if got := orderStatus(t, soID); got != "completed" {
		t.Fatalf("order = %s, want completed", got)
	}
}

// F2a-2 #5: a short line fails the whole note with no movement, and the same
// request succeeds once stock arrives.
func TestDeliveryShortStockThenRetry(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	p2 := newProduct(t, "TST-002")
	soID := sellTwoProducts(t, fx, p2, 10, 10)
	n, err := testMods.delivery.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", deliveryapp.DocInput{},
		[]deliveryapp.LineInput{
			{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 10},
			{ProductID: p2.ID, VariantID: p2.Variants[0].ID, LocationID: fx.locationID, UOM: "pcs", Qty: 10},
		})
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if _, err := testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, n.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); !isConflict(err) {
		t.Fatalf("confirm with no stock for p2 = %v, want conflict", err)
	}
	if c := movementCount(t, "delivery", n.ID); c != 0 {
		t.Fatalf("movements after failed confirm = %d, want 0", c)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 50 {
		t.Fatalf("onhand p1 = %v, want 50 (untouched)", got)
	}
	seedIn(t, fx, p2.ID, p2.Variants[0].ID, fx.locationID, 10)
	if _, err := testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, n.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); err != nil {
		t.Fatalf("retry confirm: %v", err)
	}
	if got := orderStatus(t, soID); got != "completed" {
		t.Fatalf("order = %s, want completed", got)
	}
}

// F2a-2 #6: the stock already left (an earlier Confirm died after Post, before
// the status flip). Repeating the request finishes the job; stock left once.
func TestDeliveryConfirmRecoversAfterPostBeforeFlip(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 10)
	n, err := draftNote(t, fx, soID, 10)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	// The crashed attempt's footprint: movements written, note still draft.
	if _, err := testMods.stock.Service().Post(ctx(), stockcontracts.PostRequest{
		BranchID: fx.branchID, Direction: "out", RefType: stockcontracts.RefDelivery, RefID: n.ID,
		Lines: []stockcontracts.PostLine{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, Qty: 10}},
	}); err != nil {
		t.Fatalf("simulated crash post: %v", err)
	}
	if _, err := testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, n.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); err != nil {
		t.Fatalf("recovery confirm: %v", err)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 40 {
		t.Fatalf("onhand = %v, want 40 (once)", got)
	}
	if got := orderStatus(t, soID); got != "completed" {
		t.Fatalf("order = %s, want completed", got)
	}
	// And a plain repeat is a clean replay.
	if _, err := testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, n.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); err != nil {
		t.Fatalf("replay confirm: %v", err)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 40 {
		t.Fatalf("onhand after replay = %v, want 40", got)
	}
}

// F2a-2 #8: cancelling a draft whose stock already left reverses the balance.
func TestDeliveryCancelDraftReversesStock(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 10)
	n, err := draftNote(t, fx, soID, 10)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if _, err := testMods.stock.Service().Post(ctx(), stockcontracts.PostRequest{
		BranchID: fx.branchID, Direction: "out", RefType: stockcontracts.RefDelivery, RefID: n.ID,
		Lines: []stockcontracts.PostLine{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, Qty: 10}},
	}); err != nil {
		t.Fatalf("post: %v", err)
	}
	if _, err := testMods.delivery.Service().Cancel(ctx(), 0, fx.branchID, n.ID, ""); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 50 {
		t.Fatalf("onhand = %v, want 50 (restored)", got)
	}
	// Cancelled drafts release their quota.
	if _, err := draftNote(t, fx, soID, 10); err != nil {
		t.Fatalf("new draft after cancel: %v", err)
	}
}

// sellTwoProducts creates + confirms an SO with the fixture product and p2.
func sellTwoProducts(t *testing.T, fx shopFixture, p2 *productcontracts.Product, q1, q2 float64) int64 {
	t.Helper()
	so, err := testMods.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none",
		Items: []salesapp.LineInput{
			{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: q1},
			{ProductID: p2.ID, VariantID: p2.Variants[0].ID, UOM: "pcs", Qty: q2},
		},
	})
	if err != nil {
		t.Fatalf("create SO: %v", err)
	}
	if _, err := testMods.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
		t.Fatalf("confirm SO: %v", err)
	}
	return so.ID
}

// F2a-2: a regular shipment needs driver, staff, a named recipient and a photo
// before it can be confirmed (unless the company opted out); a POS handover
// needs none.
func TestDeliveryEvidenceRequired(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	if _, err := testMods.db.Exec(`DELETE FROM company_settings WHERE setting_key = 'delivery_require_evidence'`); err != nil {
		t.Fatalf("setting: %v", err)
	}
	soID := sellFixture(t, ctx(), fx, 2)
	dn, err := draftNote(t, fx, soID, 2)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if _, err := testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); err == nil {
		t.Fatal("regular shipment confirmed without any evidence")
	}
	if on, _ := balanceAt(t, fx, fx.locationID); on != 50 {
		t.Fatalf("stock moved by a refused confirm: on_hand=%v", on)
	}
	// POS orders are exempt.
	posSO, _, err := testMods.sales.Service().CreateOrderIdempotent(ctx(), 0, fx.branchID, posOrderInput(fx, "ev-pos", 1))
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
	if _, err := testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, pn.ID, deliveryapp.ConfirmInput{Settlement: &deliveryapp.Settlement{Mode: deliveryapp.SettlePayNow, Method: "cash"}}); err != nil {
		t.Fatalf("pos handover needs no evidence: %v", err)
	}
}
