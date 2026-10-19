package integration

import (
	"sync"
	"testing"

	"errors"
	goodsreceiptapp "mini-erp/internal/modules/goodsreceipt/application"
	goodsreceiptcontracts "mini-erp/internal/modules/goodsreceipt/contracts"
	partyapp "mini-erp/internal/modules/party/application"
	purchasereturnapp "mini-erp/internal/modules/purchasereturn/application"
	purchasingapp "mini-erp/internal/modules/purchasing/application"
	"mini-erp/internal/shared/apperror"
)

// confirmedPO creates and confirms a PO for qty of the fixture product, with
// no receipt yet.
func confirmedPO(t *testing.T, fx shopFixture, qty float64) int64 {
	t.Helper()
	m := testMods
	sup, err := m.party.Service().CreateSupplier(ctx(), 0, partyapp.PartyInput{Name: "Supplier Uji"})
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}
	po, err := m.purchasing.Service().CreateOrder(ctx(), 0, fx.branchID, purchasingapp.OrderInput{
		PartyID: sup.ID, PaymentTerms: "net", DueDate: "2026-10-10", TaxType: "none",
		Items: []purchasingapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: qty, UnitPrice: 10000,
		}},
	})
	if err != nil {
		t.Fatalf("create PO: %v", err)
	}
	if _, err := m.purchasing.Service().ConfirmOrder(ctx(), 0, fx.branchID, po.ID); err != nil {
		t.Fatalf("confirm PO: %v", err)
	}
	return po.ID
}

func receiptLines(fx shopFixture, qtys ...float64) []goodsreceiptapp.LineInput {
	out := make([]goodsreceiptapp.LineInput, 0, len(qtys))
	for _, q := range qtys {
		out = append(out, goodsreceiptapp.LineInput{
			ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: q,
		})
	}
	return out
}

func poStatus(t *testing.T, poID int64) string {
	t.Helper()
	var st string
	if err := testMods.db.QueryRow(`SELECT status FROM purchase_orders WHERE id = ?`, poID).Scan(&st); err != nil {
		t.Fatalf("po status: %v", err)
	}
	return st
}

// insertPendingReceipt reproduces the state a failed stock posting leaves:
// the receipt exists as pending and no stock moved.
func insertPendingReceipt(t *testing.T, fx shopFixture, poID int64, qty float64, number string) int64 {
	t.Helper()
	res, err := testMods.db.Exec(
		`INSERT INTO goods_receipts (number, branch_id, purchase_order_id, received_at, status)
		 VALUES (?, ?, ?, UTC_TIMESTAMP(6), 'pending')`, number, fx.branchID, poID)
	if err != nil {
		t.Fatalf("insert pending: %v", err)
	}
	id, _ := res.LastInsertId()
	if _, err := testMods.db.Exec(
		`INSERT INTO goods_receipt_items (receipt_id, product_id, variant_id, location_id, uom, uom_factor, qty, qty_base)
		 VALUES (?, ?, ?, ?, 'pcs', 1, ?, ?)`, id, fx.productID, fx.variantID, fx.locationID, qty, qty); err != nil {
		t.Fatalf("insert pending item: %v", err)
	}
	return id
}

// F2a-3 #1: the same product twice in one receipt is rejected.
func TestGoodsReceiptRejectsDuplicateLines(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	poID := confirmedPO(t, fx, 10)
	_, err := testMods.goodsreceipt.Service().Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "", receiptLines(fx, 10, 10))
	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.Code != apperror.CodeValidation {
		t.Fatalf("duplicate lines = %v, want validation error", err)
	}
}

// F2a-3: a receipt gets a number and supplier delivery number, and the PO
// completes when fully posted.
func TestGoodsReceiptNumberAndCompletion(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	poID := confirmedPO(t, fx, 10)
	rec, err := testMods.goodsreceipt.Service().Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "",
		receiptLines(fx, 10), goodsreceiptapp.WithSupplierDeliveryNumber("SJ-SUP-77"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if rec.Number == "" || rec.Status != goodsreceiptcontracts.StatusPosted || rec.SupplierDeliveryNumber != "SJ-SUP-77" {
		t.Fatalf("receipt = %+v", rec)
	}
	if got := poStatus(t, poID); got != "completed" {
		t.Fatalf("po = %s, want completed", got)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 60 {
		t.Fatalf("onhand = %v, want 60", got)
	}
}

// F2a-3 #2: a pending receipt is not "received" yet, holds its quota, is not
// journalled; Complete finishes it and stock enters once.
func TestGoodsReceiptPendingThenComplete(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	poID := confirmedPO(t, fx, 10)
	id := insertPendingReceipt(t, fx, poID, 10, "GR-PENDING-1")
	svc := testMods.goodsreceipt.Service()

	if got, _ := svc.ReceivedQty(ctx(), poID, fx.productID, fx.variantID); got != 0 {
		t.Fatalf("received while pending = %v, want 0", got)
	}
	// The pending receipt holds the whole quota.
	if _, err := svc.Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "", receiptLines(fx, 1)); !isConflict(err) {
		t.Fatalf("second receipt over a pending one = %v, want conflict", err)
	}
	recent, err := svc.ListRecent(ctx(), fx.branchID, 50)
	if err != nil || len(recent) != 0 {
		t.Fatalf("ListRecent = %d rows, %v; want none (pending is not journalled)", len(recent), err)
	}

	done, err := svc.Complete(ctx(), 0, fx.branchID, id, nil)
	if err != nil || done.Status != goodsreceiptcontracts.StatusPosted {
		t.Fatalf("complete = %+v, %v", done, err)
	}
	if again, err := svc.Complete(ctx(), 0, fx.branchID, id, nil); err != nil || again.Status != goodsreceiptcontracts.StatusPosted {
		t.Fatalf("complete again = %+v, %v", again, err)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 60 {
		t.Fatalf("onhand = %v, want 60 (once)", got)
	}
	if got := poStatus(t, poID); got != "completed" {
		t.Fatalf("po = %s, want completed", got)
	}
	recent, _ = svc.ListRecent(ctx(), fx.branchID, 50)
	if len(recent) != 1 {
		t.Fatalf("ListRecent after complete = %d rows, want 1", len(recent))
	}
}

// F2a-3 #3: the same client reference never records a second receipt.
func TestGoodsReceiptClientRefReplay(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	poID := confirmedPO(t, fx, 10)
	svc := testMods.goodsreceipt.Service()
	first, err := svc.Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "", receiptLines(fx, 4), goodsreceiptapp.WithClientRef("ref-1"))
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := svc.Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "", receiptLines(fx, 4), goodsreceiptapp.WithClientRef("ref-1"))
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("replay created receipt %d, want %d", second.ID, first.ID)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 54 {
		t.Fatalf("onhand = %v, want 54 (once)", got)
	}
}

// F2a-3 #4: two receipts racing for the single remainder — one wins.
func TestGoodsReceiptConcurrentOneWins(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	poID := confirmedPO(t, fx, 10)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = testMods.goodsreceipt.Service().Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "", receiptLines(fx, 10))
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
		t.Fatalf("successful receipts = %d, want 1", ok)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 60 {
		t.Fatalf("onhand = %v, want 60", got)
	}
}

// F2a-3 #5: Discard undoes a pending receipt's stock; a posted one is final.
func TestGoodsReceiptDiscard(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	poID := confirmedPO(t, fx, 10)
	svc := testMods.goodsreceipt.Service()
	id := insertPendingReceipt(t, fx, poID, 10, "GR-PENDING-2")
	seedMovement(t, fx, "in", "goods_receipt", id, 10) // Post succeeded, the flip never happened
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 60 {
		t.Fatalf("precondition onhand = %v, want 60", got)
	}
	voided, err := svc.Discard(ctx(), 0, fx.branchID, id)
	if err != nil || voided.Status != goodsreceiptcontracts.StatusVoid {
		t.Fatalf("discard = %+v, %v", voided, err)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 50 {
		t.Fatalf("onhand after discard = %v, want 50", got)
	}
	// Quota is released: a fresh receipt can take the whole order.
	posted, err := svc.Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "", receiptLines(fx, 10))
	if err != nil {
		t.Fatalf("receipt after discard: %v", err)
	}
	if _, err := svc.Discard(ctx(), 0, fx.branchID, posted.ID); !isConflict(err) {
		t.Fatalf("discard posted = %v, want conflict", err)
	}
}

// F2a-3 #6: a pending receipt is not returnable stock.
func TestPurchaseReturnCapIgnoresPendingReceipt(t *testing.T) {
	freshDB(t)
	fx := purchaseProduct(t, newShop(t, ctx()))
	poID := confirmedPO(t, fx, 10)
	insertPendingReceipt(t, fx, poID, 10, "GR-PENDING-3")
	_, err := testMods.purchasereturn.Service().Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "",
		[]purchasereturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 1}})
	if !isConflict(err) {
		t.Fatalf("return against pending receipt = %v, want conflict", err)
	}
}

// F2a-3: receipt date in the future is rejected.
func TestGoodsReceiptFutureDateRejected(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	poID := confirmedPO(t, fx, 10)
	_, err := testMods.goodsreceipt.Service().Create(ctx(), 0, fx.branchID, poID, "2999-01-01", "", receiptLines(fx, 1))
	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.Code != apperror.CodeValidation {
		t.Fatalf("future date = %v, want validation error", err)
	}
}
