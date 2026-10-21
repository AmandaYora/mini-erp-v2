package integration

import (
	"testing"

	stockapp "mini-erp/internal/modules/stock/application"
)

func queuedStockMovements(t *testing.T, branchID int64) []int64 {
	t.Helper()
	q, err := testMods.finance.Service().UnpostedSources(ctx(), branchID)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	var ids []int64
	for _, it := range q {
		if it.DocType == "stock_movement" {
			ids = append(ids, it.DocID)
		}
	}
	return ids
}

// F2b-3: stock-count differences and write-offs are journalled at the cost
// ledger's exact value; damaged moves are not (they are value-neutral).
func TestStockDifferencesAreJournalled(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	p := productWith(t, "OPNAME", 0, 1)
	seedInCost(t, fx, p.ID, p.Variants[0].ID, fx.locationID, 50, 10000)
	svc := testMods.stock.Service()
	// The fixture's own opening adjustment is a queued stock movement too:
	// clear it so the assertions below see only what this test does.
	for _, id := range queuedStockMovements(t, fx.branchID) {
		postedLegs(t, "stock_movement", id)
	}

	// Opname finds 48 instead of 50: shortage of 2 @10.000.
	if err := svc.Adjust(ctx(), 0, fx.branchID, stockapp.AdjustInput{
		ProductID: p.ID, VariantID: p.Variants[0].ID, Location: fx.locationID,
		Mode: "out", QtyDelta: 2, Reason: "opname",
	}); err != nil {
		t.Fatalf("adjust out: %v", err)
	}
	ids := queuedStockMovements(t, fx.branchID)
	if len(ids) != 1 {
		t.Fatalf("queued stock movements = %v, want 1", ids)
	}
	legs := postedLegs(t, "stock_movement", ids[0])
	if legs["5100"][0] != 20000 || legs["1200"][1] != 20000 {
		t.Fatalf("shortage legs = %v, want Dr 5100 / Cr 1200 20000", legs)
	}
	// Re-posting is a no-op returning the same entry.
	again, err := testMods.finance.Service().Post(ctx(), 0, fx.branchID, "stock_movement", ids[0])
	if err != nil || again == nil {
		t.Fatalf("repost: %v", err)
	}
	if left := queuedStockMovements(t, fx.branchID); len(left) != 0 {
		t.Fatalf("queue after posting = %v, want empty", left)
	}

	// Surplus of 3.
	if err := svc.Adjust(ctx(), 0, fx.branchID, stockapp.AdjustInput{
		ProductID: p.ID, VariantID: p.Variants[0].ID, Location: fx.locationID,
		Mode: "in", QtyDelta: 3, Reason: "opname lebih",
	}); err != nil {
		t.Fatalf("adjust in: %v", err)
	}
	ids = queuedStockMovements(t, fx.branchID)
	if len(ids) != 1 {
		t.Fatalf("queued after surplus = %v", ids)
	}
	legs = postedLegs(t, "stock_movement", ids[0])
	if legs["1200"][0] != 30000 || legs["5100"][1] != 30000 {
		t.Fatalf("surplus legs = %v, want Dr 1200 / Cr 5100 30000", legs)
	}

	// Damaged move (neutral) then write-off 5.
	if err := svc.MoveToDamaged(ctx(), 0, fx.branchID, stockapp.DamagedInput{
		ProductID: p.ID, VariantID: p.Variants[0].ID, LocationID: fx.locationID, Qty: 5, Reason: "pecah",
	}); err != nil {
		t.Fatalf("damaged: %v", err)
	}
	if left := queuedStockMovements(t, fx.branchID); len(left) != 0 {
		t.Fatalf("a damaged move was queued: %v", left)
	}
	if err := svc.WriteOff(ctx(), 0, fx.branchID, stockapp.DamagedInput{
		ProductID: p.ID, VariantID: p.Variants[0].ID, Qty: 5, Reason: "dibuang",
	}); err != nil {
		t.Fatalf("write-off: %v", err)
	}
	ids = queuedStockMovements(t, fx.branchID)
	if len(ids) != 1 {
		t.Fatalf("queued after write-off = %v", ids)
	}
	legs = postedLegs(t, "stock_movement", ids[0])
	if legs["5100"][0] != 50000 || legs["1200"][1] != 50000 {
		t.Fatalf("write-off legs = %v, want Dr 5100 / Cr 1200 50000", legs)
	}
}

// ADR-0024: an opening-stock correction is journalled against equity
// (3100 Koreksi Saldo Awal), never against profit, needs its own permission,
// and applies only to a position that received an opening stock.
func TestOpeningCorrectionMovesEquity(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	p := productWith(t, "SALDOAWAL", 0, 1)
	seedInCost(t, fx, p.ID, p.Variants[0].ID, fx.locationID, 100, 10000)
	svc := testMods.stock.Service()
	for _, id := range queuedStockMovements(t, fx.branchID) {
		postedLegs(t, "stock_movement", id)
	}
	fix := stockapp.AdjustInput{
		ProductID: p.ID, VariantID: p.Variants[0].ID, Location: fx.locationID,
		Mode: "set", QtyAfter: 2, Reason: "stok awal impor salah", Purpose: stockapp.AdjustPurposeOpening,
	}
	if err := svc.Adjust(ctx(), 0, fx.branchID, fix); err == nil {
		t.Fatal("opening correction without stock.adjust_opening was accepted")
	}
	fix.OpeningAllowed = true
	if err := svc.Adjust(ctx(), 0, fx.branchID, fix); err != nil {
		t.Fatalf("opening correction: %v", err)
	}
	ids := queuedStockMovements(t, fx.branchID)
	if len(ids) != 1 {
		t.Fatalf("queued = %v, want 1", ids)
	}
	legs := postedLegs(t, "stock_movement", ids[0])
	if _, onProfit := legs["5100"]; onProfit || legs["3100"][0] != 980000 || legs["1200"][1] != 980000 {
		t.Fatalf("opening correction legs = %v, want Dr 3100 / Cr 1200 980000 and nothing on 5100", legs)
	}

	// A product that never had an opening stock cannot take the equity route.
	q := productWith(t, "TANPAAWAL", 5000, 1)
	if err := svc.Adjust(ctx(), 0, fx.branchID, stockapp.AdjustInput{
		ProductID: q.ID, VariantID: q.Variants[0].ID, Location: fx.locationID,
		Mode: "in", QtyDelta: 4, Reason: "lebih", Purpose: stockapp.AdjustPurposeOpening, OpeningAllowed: true,
	}); err == nil {
		t.Fatal("opening correction accepted for a product without opening stock")
	}
}
