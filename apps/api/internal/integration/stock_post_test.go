package integration

import (
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	productapp "mini-erp/internal/modules/product/application"
	productcontracts "mini-erp/internal/modules/product/contracts"
	stockapp "mini-erp/internal/modules/stock/application"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/apperror"
)

func movementCount(t *testing.T, refType string, refID int64) int {
	t.Helper()
	var n int
	if err := testMods.db.QueryRow(
		`SELECT COUNT(*) FROM stock_movements WHERE ref_type = ? AND ref_id = ?`, refType, refID).Scan(&n); err != nil {
		t.Fatalf("count movements: %v", err)
	}
	return n
}

func onHand(t *testing.T, fx shopFixture, productID, variantID, locationID int64) float64 {
	t.Helper()
	b, err := testMods.stock.Service().GetBalance(ctx(), fx.branchID, productID, variantID, locationID)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	return b.OnHand
}

func isConflict(err error) bool {
	var ae *apperror.AppError
	return errors.As(err, &ae) && ae.Code == apperror.CodeConflict
}

func outReq(fx shopFixture, refID int64, qty float64) stockcontracts.PostRequest {
	return stockcontracts.PostRequest{
		BranchID: fx.branchID, Direction: "out", RefType: "delivery", RefID: refID,
		Lines: []stockcontracts.PostLine{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, Qty: qty}},
	}
}

// F2a-0 #1: two goroutines posting the same document → one movement, one
// balance change, exactly one Replayed.
func TestStockPostConcurrentSameDocument(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.stock.Service()

	var wg sync.WaitGroup
	results := make([]*stockcontracts.PostResult, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = svc.Post(ctx(), outReq(fx, 9001, 5))
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("post %d: %v", i, err)
		}
	}
	if got := results[0].Posted + results[1].Posted; got != 1 {
		t.Fatalf("posted = %d, want 1", got)
	}
	if got := results[0].Replayed + results[1].Replayed; got != 1 {
		t.Fatalf("replayed = %d, want 1", got)
	}
	if n := movementCount(t, "delivery", 9001); n != 1 {
		t.Fatalf("movements = %d, want 1", n)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 45 {
		t.Fatalf("onhand = %v, want 45", got)
	}
}

// F2a-0 #2: sequential replay is a successful no-op.
func TestStockPostReplayNoop(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.stock.Service()
	first, err := svc.Post(ctx(), outReq(fx, 9002, 4))
	if err != nil || first.Posted != 1 {
		t.Fatalf("first = %+v, %v", first, err)
	}
	again, err := svc.Post(ctx(), outReq(fx, 9002, 4))
	if err != nil || again.Posted != 0 || again.Replayed != 1 {
		t.Fatalf("again = %+v, %v; want replay", again, err)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 46 {
		t.Fatalf("onhand = %v, want 46", got)
	}
}

// F2a-0 #3: a short position rolls the whole document back.
func TestStockPostBatchAtomic(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.stock.Service()
	p2 := newProduct(t, "TST-002")
	loc2 := newLocation(t, fx, "L2")
	p3 := newProduct(t, "TST-003")
	// Seed p2 and p3 with 5 each; the third line asks for 99.
	for _, p := range []*productcontracts.Product{p2, p3} {
		seedIn(t, fx, p.ID, p.Variants[0].ID, loc2, 5)
	}
	_, err := svc.Post(ctx(), stockcontracts.PostRequest{
		BranchID: fx.branchID, Direction: "out", RefType: "delivery", RefID: 9003,
		Lines: []stockcontracts.PostLine{
			{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, Qty: 1},
			{ProductID: p2.ID, VariantID: p2.Variants[0].ID, LocationID: loc2, Qty: 1},
			{ProductID: p3.ID, VariantID: p3.Variants[0].ID, LocationID: loc2, Qty: 99},
		},
	})
	if !isConflict(err) {
		t.Fatalf("err = %v, want conflict", err)
	}
	if n := movementCount(t, "delivery", 9003); n != 0 {
		t.Fatalf("movements = %d, want 0 (rolled back)", n)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 50 {
		t.Fatalf("onhand = %v, want 50 (unchanged)", got)
	}
}

// F2a-0 #4: two lines on the same position become one summed movement.
func TestStockPostSumsSamePosition(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	line := stockcontracts.PostLine{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, Qty: 3}
	res, err := testMods.stock.Service().Post(ctx(), stockcontracts.PostRequest{
		BranchID: fx.branchID, Direction: "out", RefType: "delivery", RefID: 9004,
		Lines: []stockcontracts.PostLine{line, line},
	})
	if err != nil || res.Posted != 1 {
		t.Fatalf("res = %+v, %v", res, err)
	}
	var qty float64
	if err := testMods.db.QueryRow(`SELECT qty_base FROM stock_movements WHERE ref_type='delivery' AND ref_id=9004`).Scan(&qty); err != nil || qty != 6 {
		t.Fatalf("qty = %v, %v; want 6", qty, err)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 44 {
		t.Fatalf("onhand = %v, want 44", got)
	}
}

// F2a-0 #5: documents with overlapping positions in opposite caller order
// never deadlock (positions are locked in sorted order).
func TestStockPostNoDeadlock(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	p2 := newProduct(t, "TST-002")
	seedIn(t, fx, p2.ID, p2.Variants[0].ID, fx.locationID, 1000)
	a := stockcontracts.PostLine{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, Qty: 1}
	b := stockcontracts.PostLine{ProductID: p2.ID, VariantID: p2.Variants[0].ID, LocationID: fx.locationID, Qty: 1}
	seedIn(t, fx, fx.productID, fx.variantID, fx.locationID, 1000)
	var wg sync.WaitGroup
	errs := make(chan error, 100)
	for i := 0; i < 50; i++ {
		for j, lines := range [][]stockcontracts.PostLine{{a, b}, {b, a}} {
			wg.Add(1)
			go func(ref int64, lines []stockcontracts.PostLine) {
				defer wg.Done()
				_, err := testMods.stock.Service().Post(ctx(), stockcontracts.PostRequest{
					BranchID: fx.branchID, Direction: "out", RefType: "delivery", RefID: ref, Lines: lines,
				})
				errs <- err
			}(int64(10000+i*2+j), lines)
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent post: %v", err)
		}
	}
}

// F2a-0 #6: Reverse writes one set of opposite movements; reversing an "in"
// whose stock was consumed is refused without any change.
func TestStockReverse(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.stock.Service()
	in := stockcontracts.PostRequest{
		BranchID: fx.branchID, Direction: "in", RefType: "goods_receipt", RefID: 9006,
		Lines: []stockcontracts.PostLine{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, Qty: 10}},
	}
	if _, err := svc.Post(ctx(), in); err != nil {
		t.Fatalf("post in: %v", err)
	}
	first, err := svc.Reverse(ctx(), fx.branchID, "goods_receipt", 9006, 0)
	if err != nil || first.Posted != 1 {
		t.Fatalf("reverse = %+v, %v", first, err)
	}
	second, err := svc.Reverse(ctx(), fx.branchID, "goods_receipt", 9006, 0)
	if err != nil || second.Posted != 0 || second.Replayed != 1 {
		t.Fatalf("reverse again = %+v, %v", second, err)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 50 {
		t.Fatalf("onhand = %v, want 50", got)
	}
	if n := movementCount(t, "goods_receipt_cancel", 9006); n != 1 {
		t.Fatalf("cancel movements = %d, want 1", n)
	}

	// Stock consumed after receipt → cannot reverse.
	in.RefID = 9007
	if _, err := svc.Post(ctx(), in); err != nil {
		t.Fatalf("post in 2: %v", err)
	}
	if _, err := svc.Post(ctx(), outReq(fx, 9008, 55)); err != nil {
		t.Fatalf("consume: %v", err)
	}
	if _, err := svc.Reverse(ctx(), fx.branchID, "goods_receipt", 9007, 0); !isConflict(err) {
		t.Fatalf("reverse consumed = %v, want conflict", err)
	}
	if n := movementCount(t, "goods_receipt_cancel", 9007); n != 0 {
		t.Fatalf("cancel movements = %d, want 0", n)
	}
}

// F2a-0 #7: a hold consumed by Post leaves reserved at 0; an expired hold
// falls back to plain availability.
func TestStockPostConsumesReservation(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.stock.Service()
	if _, err := svc.Hold(ctx(), 0, fx.branchID, fx.productID, fx.variantID, fx.locationID, 10, "cart-1", 60); err != nil {
		t.Fatalf("hold: %v", err)
	}
	req := outReq(fx, 9009, 10)
	req.Lines[0].ReservationKey = "cart-1"
	if _, err := svc.Post(ctx(), req); err != nil {
		t.Fatalf("post with hold: %v", err)
	}
	b, _ := svc.GetBalance(ctx(), fx.branchID, fx.productID, fx.variantID, fx.locationID)
	if b.OnHand != 40 || b.Reserved != 0 {
		t.Fatalf("balance = %+v, want onhand 40 reserved 0", b)
	}

	// Expired hold: Post must not fail merely because the TTL passed.
	if _, err := svc.Hold(ctx(), 0, fx.branchID, fx.productID, fx.variantID, fx.locationID, 5, "cart-2", 60); err != nil {
		t.Fatalf("hold 2: %v", err)
	}
	if _, err := testMods.db.Exec(`UPDATE stock_reservations SET expires_at = ? WHERE reservation_key = 'cart-2'`,
		time.Now().UTC().Add(-time.Hour).Format("2006-01-02 15:04:05")); err != nil {
		t.Fatalf("expire: %v", err)
	}
	req2 := outReq(fx, 9010, 5)
	req2.Lines[0].ReservationKey = "cart-2"
	if _, err := svc.Post(ctx(), req2); err != nil {
		t.Fatalf("post with expired hold: %v", err)
	}
	b, _ = svc.GetBalance(ctx(), fx.branchID, fx.productID, fx.variantID, fx.locationID)
	if b.OnHand != 35 || b.Reserved != 0 {
		t.Fatalf("balance = %+v, want onhand 35 reserved 0", b)
	}
}

// Movements carry exact before/after balances.
func TestStockPostRecordsBalances(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	if _, err := testMods.stock.Service().Post(ctx(), outReq(fx, 9011, 8)); err != nil {
		t.Fatalf("post: %v", err)
	}
	var before, after sql.NullFloat64
	if err := testMods.db.QueryRow(
		`SELECT balance_before, balance_after FROM stock_movements WHERE ref_type='delivery' AND ref_id=9011`).Scan(&before, &after); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if before.Float64 != 50 || after.Float64 != 42 {
		t.Fatalf("before/after = %v/%v, want 50/42", before.Float64, after.Float64)
	}
}

var seedRef int64 = 500000

// seedIn puts stock on a position through the same Post path production uses.
func seedIn(t *testing.T, fx shopFixture, productID, variantID, locationID int64, qty float64) {
	t.Helper()
	seedRef++
	if _, err := testMods.stock.Service().Post(ctx(), stockcontracts.PostRequest{
		BranchID: fx.branchID, Direction: "in", RefType: "opening", RefID: seedRef,
		Lines: []stockcontracts.PostLine{{ProductID: productID, VariantID: variantID, LocationID: locationID, Qty: qty}},
	}); err != nil {
		t.Fatalf("seed stock: %v", err)
	}
}

func newProduct(t *testing.T, code string) *productcontracts.Product {
	t.Helper()
	p, err := testMods.product.Service().CreateProduct(ctx(), 0, productapp.ProductInput{
		Code: code, Name: "Barang " + code, Type: "barang", Tracked: true,
		BaseUOM: "pcs", PurchaseUOM: "pcs", SalesUOM: "pcs",
		PurchaseFactor: 1, SalesFactor: 1, PurchasePrice: 10000, SellingPrice: 15000,
	})
	if err != nil {
		t.Fatalf("create product %s: %v", code, err)
	}
	return p
}

func newLocation(t *testing.T, fx shopFixture, code string) int64 {
	t.Helper()
	l, err := testMods.stock.Service().CreateLocation(ctx(), 0, fx.branchID, stockapp.LocationInput{Code: code, Name: "Lokasi " + code})
	if err != nil {
		t.Fatalf("create location: %v", err)
	}
	return l.ID
}

// seedMovement writes a document movement directly — the footprint an earlier,
// crashed attempt left behind (stock moved, document status not flipped yet).
func seedMovement(t *testing.T, fx shopFixture, direction, refType string, refID int64, qty float64) {
	t.Helper()
	if _, err := testMods.stock.Service().Post(ctx(), stockcontracts.PostRequest{
		BranchID: fx.branchID, Direction: direction, RefType: refType, RefID: refID,
		Lines: []stockcontracts.PostLine{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, Qty: qty}},
	}); err != nil {
		t.Fatalf("seed movement: %v", err)
	}
}
