package integration

import (
	paymentapp "mini-erp/internal/modules/payment/application"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	"sync"
	"testing"

	purchasereturnapp "mini-erp/internal/modules/purchasereturn/application"
	salesreturnapp "mini-erp/internal/modules/salesreturn/application"
	"mini-erp/internal/shared/doclock"
)

func salesReturnLines(fx shopFixture, qtys ...float64) []salesreturnapp.LineInput {
	out := make([]salesreturnapp.LineInput, 0, len(qtys))
	for _, q := range qtys {
		out = append(out, salesreturnapp.LineInput{
			ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: q,
		})
	}
	return out
}

func createSalesReturnLines(fx shopFixture, soID int64, qtys ...float64) (int64, error) {
	ret, err := testMods.salesreturn.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", "return_only",
		salesReturnLines(fx, qtys...), nil)
	if err != nil {
		return 0, err
	}
	return ret.ID, nil
}

// F2a-1 #1: two lines of one request on the same product add up against the
// delivered quantity instead of each seeing the whole rest.
func TestSalesReturnLinesAccumulate(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 10)
	deliverAll(t, ctx(), fx, soID, 10)
	if _, err := createSalesReturnLines(fx, soID, 6, 6); !isConflict(err) {
		t.Fatalf("6+6 of 10 = %v, want conflict", err)
	}
	if _, err := createSalesReturnLines(fx, soID, 6, 4); err != nil {
		t.Fatalf("6+4 of 10: %v", err)
	}
}

// F2a-1 #2: a draft holds its share; cancelling it releases the share.
func TestSalesReturnDraftHoldsQuota(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 10)
	deliverAll(t, ctx(), fx, soID, 10)
	first, err := createSalesReturnLines(fx, soID, 10)
	if err != nil {
		t.Fatalf("first full draft: %v", err)
	}
	if _, err := createSalesReturnLines(fx, soID, 1); !isConflict(err) {
		t.Fatalf("second draft while first holds all = %v, want conflict", err)
	}
	if _, err := testMods.salesreturn.Service().Cancel(ctx(), 0, fx.branchID, first, ""); err != nil {
		t.Fatalf("cancel first: %v", err)
	}
	if _, err := createSalesReturnLines(fx, soID, 10); err != nil {
		t.Fatalf("draft after release: %v", err)
	}
}

// F2a-1 #3/#4: simultaneous and repeated confirms of one return restock once.
func TestSalesReturnConfirmOnce(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 10)
	deliverAll(t, ctx(), fx, soID, 10) // stock 50 -> 40
	payInFull(t, fx, soID)             // nothing left to offset
	id, err := createSalesReturnLines(fx, soID, 10)
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
			_, errs[i] = testMods.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, id)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("confirm %d: %v", i, err)
		}
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 50 {
		t.Fatalf("onhand = %v, want 50 (restocked once)", got)
	}
	// A plain repeat is a clean replay.
	if _, err := testMods.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, id); err != nil {
		t.Fatalf("repeat confirm: %v", err)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 50 {
		t.Fatalf("onhand after repeat = %v, want 50", got)
	}
}

// F2a-1 #5: two cancels of a confirmed return take the stock back out once.
func TestSalesReturnCancelOnce(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 10)
	deliverAll(t, ctx(), fx, soID, 10)
	payInFull(t, fx, soID)
	id, err := createSalesReturnLines(fx, soID, 10)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if _, err := testMods.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, id); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	doclock.SetBypass(true)
	defer doclock.SetBypass(false)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = testMods.salesreturn.Service().Cancel(ctx(), 0, fx.branchID, id, "")
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("cancel %d: %v", i, err)
		}
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 40 {
		t.Fatalf("onhand = %v, want 40 (back out once)", got)
	}
}

// F2a-1 #6: stock already posted by a confirm that died before the status
// flip; repeating Confirm finishes it without moving stock again.
func TestSalesReturnConfirmRecoversAfterPost(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 10)
	deliverAll(t, ctx(), fx, soID, 10)
	id, err := createSalesReturnLines(fx, soID, 10)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	seedMovement(t, fx, "in", "sales_return", id, 10) // footprint of the crashed attempt
	if _, err := testMods.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, id); err != nil {
		t.Fatalf("recovery confirm: %v", err)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 50 {
		t.Fatalf("onhand = %v, want 50 (once)", got)
	}
}

// F2a-1 #7 (purchase side): accumulate, draft holds quota, confirm/cancel once.
func TestPurchaseReturnQuotaAndOnce(t *testing.T) {
	freshDB(t)
	fx := purchaseProduct(t, newShop(t, ctx()))
	poID := receivePO(t, fx, 10000, 10) // 10 received into fx.locationID
	if _, err := testMods.payment.Service().Create(ctx(), 0, fx.branchID, poSupplier(t, poID), "cash", "", "", 100000,
		[]paymentapp.AllocationInput{{OrderType: paymentcontracts.OrderPurchase, OrderID: poID, Amount: 100000}}); err != nil {
		t.Fatalf("pay PO: %v", err) // nothing left to offset
	}
	lines := func(qtys ...float64) []purchasereturnapp.LineInput {
		out := []purchasereturnapp.LineInput{}
		for _, q := range qtys {
			out = append(out, purchasereturnapp.LineInput{
				ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: q,
			})
		}
		return out
	}
	svc := testMods.purchasereturn.Service()
	if _, err := svc.Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "", lines(6, 6)); !isConflict(err) {
		t.Fatalf("6+6 of 10 = %v, want conflict", err)
	}
	first, err := svc.Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "", lines(10))
	if err != nil {
		t.Fatalf("full draft: %v", err)
	}
	if _, err := svc.Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "", lines(1)); !isConflict(err) {
		t.Fatalf("second draft = %v, want conflict", err)
	}
	doclock.SetBypass(true)
	defer doclock.SetBypass(false)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.Confirm(ctx(), 0, fx.branchID, first.ID)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("confirm %d: %v", i, err)
		}
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 0 {
		t.Fatalf("onhand = %v, want 0 (shipped back once)", got)
	}
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.Cancel(ctx(), 0, fx.branchID, first.ID, "")
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("cancel %d: %v", i, err)
		}
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 10 {
		t.Fatalf("onhand = %v, want 10 (restored once)", got)
	}
}

// payInFull settles the whole sales order so a later return has nothing open
// to be offset against.
func payInFull(t *testing.T, fx shopFixture, soID int64) {
	t.Helper()
	total := orderTotal(t, soID)
	if _, err := testMods.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", total, payAll(soID, total)); err != nil {
		t.Fatalf("pay order in full: %v", err)
	}
}

func poSupplier(t *testing.T, poID int64) int64 {
	t.Helper()
	po, err := testMods.purchasing.Service().GetByID(ctx(), poID)
	if err != nil || po == nil {
		t.Fatalf("get PO: %v", err)
	}
	return po.PartyID
}
