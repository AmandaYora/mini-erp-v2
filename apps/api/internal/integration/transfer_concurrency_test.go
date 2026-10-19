package integration

import (
	"sync"
	"testing"

	branchapp "mini-erp/internal/modules/branch/application"
	stockapp "mini-erp/internal/modules/stock/application"
)

func transferTo(t *testing.T, fx shopFixture, qty float64) (toBranch int64, toLoc int64, docID int64) {
	t.Helper()
	m := testMods
	br, err := m.branch.Service().CreateBranch(ctx(), 0, branchapp.CreateBranchInput{Code: "TS2", Name: "Cabang Dua"})
	if err != nil {
		t.Fatalf("create branch: %v", err)
	}
	loc, err := m.stock.Service().CreateLocation(ctx(), 0, br.ID, stockapp.LocationInput{Code: "TS2", Name: "Gudang Dua"})
	if err != nil {
		t.Fatalf("create location: %v", err)
	}
	doc, err := m.stock.Service().CreateTransfer(ctx(), 0, fx.branchID, br.ID, fx.locationID, loc.ID, "uji",
		[]stockapp.TransferItemInput{{ProductID: fx.productID, VariantID: fx.variantID, Qty: qty}})
	if err != nil {
		t.Fatalf("create transfer: %v", err)
	}
	return br.ID, loc.ID, doc.ID
}

// F2a-5 #1: two simultaneous dispatches of one document take stock out once.
func TestTransferConcurrentDispatchOnce(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	_, _, id := transferTo(t, fx, 10)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = testMods.stock.Service().DispatchTransfer(ctx(), 0, fx.branchID, id)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("dispatch %d: %v", i, err)
		}
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 40 {
		t.Fatalf("onhand = %v, want 40 (out once)", got)
	}
}

// F2a-5 #2: dispatch racing a cancel of the draft — the stock ends consistent
// with whichever document state won.
func TestTransferDispatchVersusCancel(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	_, _, id := transferTo(t, fx, 10)
	svc := testMods.stock.Service()
	var wg sync.WaitGroup
	var dErr, cErr error
	wg.Add(2)
	go func() { defer wg.Done(); _, dErr = svc.DispatchTransfer(ctx(), 0, fx.branchID, id) }()
	go func() { defer wg.Done(); _, cErr = svc.CancelTransfer(ctx(), 0, fx.branchID, id) }()
	wg.Wait()
	got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID)
	doc, err := svc.GetTransfer(ctx(), fx.branchID, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	switch doc.Status {
	case "dispatched":
		if dErr != nil || got != 40 {
			t.Fatalf("dispatched: dispatchErr=%v onhand=%v want nil/40", dErr, got)
		}
	case "cancelled":
		// Cancel either beat the dispatch (dispatch conflicts, stock 50) or
		// followed it (dispatch succeeded, then the cancel reversed it: 50).
		if cErr != nil || got != 50 {
			t.Fatalf("cancelled: cancelErr=%v onhand=%v want nil/50", cErr, got)
		}
	default:
		t.Fatalf("unexpected status %s", doc.Status)
	}
}

// F2a-5 #3: receiving twice books the destination once; a received transfer
// can no longer be cancelled.
func TestTransferReceiveOnceAndFinal(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	toBranch, toLoc, id := transferTo(t, fx, 10)
	svc := testMods.stock.Service()
	if _, err := svc.DispatchTransfer(ctx(), 0, fx.branchID, id); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.ReceiveTransfer(ctx(), 0, toBranch, id)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("receive %d: %v", i, err)
		}
	}
	bal, err := svc.GetBalance(ctx(), toBranch, fx.productID, fx.variantID, toLoc)
	if err != nil || bal.OnHand != 10 {
		t.Fatalf("destination = %+v, %v; want 10 (once)", bal, err)
	}
	if _, err := svc.CancelTransfer(ctx(), 0, fx.branchID, id); !isConflict(err) {
		t.Fatalf("cancel after receive = %v, want conflict", err)
	}
}

// F2a-5: cancelling a dispatched transfer puts the stock back at the source.
func TestTransferCancelDispatchedRestores(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	_, _, id := transferTo(t, fx, 10)
	svc := testMods.stock.Service()
	if _, err := svc.DispatchTransfer(ctx(), 0, fx.branchID, id); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if _, err := svc.CancelTransfer(ctx(), 0, fx.branchID, id); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 50 {
		t.Fatalf("onhand = %v, want 50", got)
	}
	// A repeat is a clean replay.
	if _, err := svc.CancelTransfer(ctx(), 0, fx.branchID, id); err != nil {
		t.Fatalf("cancel again: %v", err)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 50 {
		t.Fatalf("onhand after repeat = %v, want 50", got)
	}
}

// F2a-5: two lines of the same product in one document are summed instead of
// colliding on the movement key.
func TestTransferDuplicateProductLines(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	to := newLocation(t, fx, "DUP")
	doc, err := m.stock.Service().MoveLocation(ctx(), 0, fx.branchID, fx.locationID, to, "dua baris",
		[]stockapp.TransferItemInput{
			{ProductID: fx.productID, VariantID: fx.variantID, Qty: 3},
			{ProductID: fx.productID, VariantID: fx.variantID, Qty: 4},
		})
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if doc.Status != "received" {
		t.Fatalf("status = %s", doc.Status)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, to); got != 7 {
		t.Fatalf("destination = %v, want 7", got)
	}
}

// F2a-5 #6: opposing moves between two locations never deadlock.
func TestMoveLocationNoDeadlock(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	other := newLocation(t, fx, "OPP")
	seedIn(t, fx, fx.productID, fx.variantID, other, 1000)
	seedIn(t, fx, fx.productID, fx.variantID, fx.locationID, 1000)
	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for i := 0; i < 20; i++ {
		for _, dir := range [][2]int64{{fx.locationID, other}, {other, fx.locationID}} {
			wg.Add(1)
			go func(from, to int64) {
				defer wg.Done()
				_, err := testMods.stock.Service().MoveLocation(ctx(), 0, fx.branchID, from, to, "bolak-balik",
					[]stockapp.TransferItemInput{{ProductID: fx.productID, VariantID: fx.variantID, Qty: 1}})
				errs <- err
			}(dir[0], dir[1])
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("move: %v", err)
		}
	}
	a := onHand(t, fx, fx.productID, fx.variantID, fx.locationID)
	b := onHand(t, fx, fx.productID, fx.variantID, other)
	if a+b != 2050 {
		t.Fatalf("total = %v, want 2050 (conserved)", a+b)
	}
}
