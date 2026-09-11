package integration

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
	stockapp "mini-erp/internal/modules/stock/application"
)

func openingBook(t *testing.T, rows ...[]string) *bytes.Buffer {
	t.Helper()
	f := excelize.NewFile()
	sheet := "opening"
	f.SetSheetName("Sheet1", sheet)
	headers := []string{"product_code", "variant_code", "location_code", "qty"}
	for c, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(c+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}
	for r, row := range rows {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			_ = f.SetCellValue(sheet, cell, v)
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write book: %v", err)
	}
	return &buf
}

// TestOpeningCommitBalances (A2): opname/saldo awal lands as in-movements
// with exact balances; unknown codes report row errors instead of failing.
func TestOpeningCommitBalances(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	p, err := m.product.Service().GetByID(ctx(), fx.productID)
	if err != nil {
		t.Fatalf("get product: %v", err)
	}
	// Fresh position: the fixture's own position already has history and
	// would be skipped as resumable.
	loc2, err := m.stock.Service().CreateLocation(ctx(), 0, fx.branchID, stockapp.LocationInput{
		Code: "OPN", Name: "Gudang Opname",
	})
	if err != nil {
		t.Fatalf("create location: %v", err)
	}
	book := openingBook(t,
		[]string{p.Code, p.Variants[0].Code, "OPN", "7"},
		[]string{"TIDAK-ADA", "X", "OPN", "3"},
	)
	prev, err := m.stock.Service().PreviewOpening(ctx(), fx.branchID, bytes.NewReader(book.Bytes()))
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if prev.ValidRows != 1 || len(prev.Errors) != 1 {
		t.Fatalf("preview = %+v, want 1 valid + 1 error", prev)
	}
	res, err := m.stock.Service().CommitOpening(ctx(), 0, fx.branchID, bytes.NewReader(book.Bytes()))
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if res.Posted != 1 || len(res.Failures) != 1 {
		t.Fatalf("committed = %+v, want 1 posted + 1 failure", res)
	}
	bal, err := m.stock.Service().GetBalance(ctx(), fx.branchID, fx.productID, fx.variantID, loc2.ID)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	if bal.OnHand != 7 {
		t.Fatalf("onhand = %v, want 7", bal.OnHand)
	}
}

// TestReservationHoldRelease (A2): holds reduce availability, over-holds
// are rejected, release restores — no stock created or destroyed.
func TestReservationHoldRelease(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	avail := func() float64 {
		t.Helper()
		b, err := m.stock.Service().GetBalance(ctx(), fx.branchID, fx.productID, fx.variantID, fx.locationID)
		if err != nil {
			t.Fatalf("balance: %v", err)
		}
		return b.Available()
	}
	if avail() != 50 {
		t.Fatalf("available = %v, want 50", avail())
	}
	if _, err := m.stock.Service().Hold(ctx(), 0, fx.branchID, fx.productID, fx.variantID,
		fx.locationID, 10, "uji-kunci-1", 60); err != nil {
		t.Fatalf("hold: %v", err)
	}
	if avail() != 40 {
		t.Fatalf("available after hold = %v, want 40", avail())
	}
	if _, err := m.stock.Service().Hold(ctx(), 0, fx.branchID, fx.productID, fx.variantID,
		fx.locationID, 500, "uji-kunci-2", 60); err == nil {
		t.Fatal("expected over-hold rejection, got nil")
	}
	if err := m.stock.Service().Release(ctx(), 0, "uji-kunci-1"); err != nil {
		t.Fatalf("release: %v", err)
	}
	if avail() != 50 {
		t.Fatalf("available after release = %v, want 50", avail())
	}
	// Unknown keys succeed idempotently.
	if err := m.stock.Service().Release(ctx(), 0, "tidak-ada"); err != nil {
		t.Fatalf("release unknown: %v", err)
	}
}

// TestAdjustSetMode (A2): set-mode computes the delta against live on-hand;
// a no-op set is rejected instead of writing an empty movement.
func TestAdjustSetMode(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	if err := m.stock.Service().Adjust(ctx(), 0, fx.branchID, stockapp.AdjustInput{
		ProductID: fx.productID, VariantID: fx.variantID, Location: fx.locationID,
		Mode: "set", QtyAfter: 60, Reason: "opname uji",
	}); err != nil {
		t.Fatalf("adjust set: %v", err)
	}
	bal, err := m.stock.Service().GetBalance(ctx(), fx.branchID, fx.productID, fx.variantID, fx.locationID)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	if bal.OnHand != 60 {
		t.Fatalf("onhand = %v, want 60", bal.OnHand)
	}
	if err := m.stock.Service().Adjust(ctx(), 0, fx.branchID, stockapp.AdjustInput{
		ProductID: fx.productID, VariantID: fx.variantID, Location: fx.locationID,
		Mode: "set", QtyAfter: 60, Reason: "opname uji",
	}); err == nil {
		t.Fatal("expected no-op rejection, got nil")
	}
}
