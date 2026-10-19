package integration

import (
	"sync"
	"testing"

	stockcontracts "mini-erp/internal/modules/stock/contracts"
)

func rollupConsistent(t *testing.T, branchID int64, when string) {
	t.Helper()
	rep, err := testMods.finance.Service().VerifyRollups(ctx(), branchID)
	if err != nil {
		t.Fatalf("%s: verify: %v", when, err)
	}
	if !rep.Consistent {
		t.Fatalf("%s: rollup inconsistent: %+v", when, rep)
	}
}

// F2b-14a: the daily rollup stays equal to the journal lines through posting and
// reversal, and a rebuild reproduces it after tampering.
func TestRollupMatchesLines(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	_, noteID := confirmedNote(t, fx, 3)
	for _, id := range queuedStockMovements(t, fx.branchID) {
		postedLegs(t, "stock_movement", id)
	}
	if _, err := fin.Post(ctx(), 0, fx.branchID, "delivery", noteID); err != nil {
		t.Fatalf("post: %v", err)
	}
	rollupConsistent(t, fx.branchID, "after post")
	if _, err := fin.Reverse(ctx(), 0, fx.branchID, mustEntryID(t, fx.branchID, "delivery", noteID), "uji"); err != nil {
		t.Fatalf("reverse: %v", err)
	}
	rollupConsistent(t, fx.branchID, "after reverse")
	// Tamper, detect, rebuild.
	if _, err := testMods.db.Exec(`UPDATE finance_account_daily SET debit = debit + 7 WHERE branch_id = ? LIMIT 1`, fx.branchID); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	if rep, _ := fin.VerifyRollups(ctx(), fx.branchID); rep.Consistent {
		t.Fatal("tampered rollup reported consistent")
	}
	if err := fin.RebuildRollups(ctx(), fx.branchID); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	rollupConsistent(t, fx.branchID, "after rebuild")
}

func mustEntryID(t *testing.T, branchID int64, docType string, docID int64) int64 {
	t.Helper()
	var id int64
	if err := testMods.db.QueryRow(`SELECT id FROM finance_journal_entries WHERE branch_id = ? AND source_doc_type = ? AND source_doc_id = ? AND status = 'posted'`,
		branchID, docType, docID).Scan(&id); err != nil {
		t.Fatalf("entry id: %v", err)
	}
	return id
}

// F2b-14a: concurrent Sync runs neither fail nor duplicate ledger rows, and a
// movement that committed below an already-ingested id is picked up later.
func TestCostSyncConcurrentAndGapTolerant(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	var wg sync.WaitGroup
	errs := make([]error, 6)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = fin.Sync(ctx(), fx.branchID)
		}(i)
	}
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			t.Fatalf("concurrent sync: %v", e)
		}
	}
	var dup int
	_ = testMods.db.QueryRow(`SELECT COUNT(*) FROM (SELECT stock_movement_id FROM finance_inventory_cost_movements GROUP BY stock_movement_id HAVING COUNT(*) > 1) d`).Scan(&dup)
	if dup != 0 {
		t.Fatalf("duplicate ledger rows: %d", dup)
	}
	// Simulate a late committer: a low-id movement appears after a higher id was ingested.
	if _, err := testMods.stock.Service().Post(ctx(), stockcontracts.PostRequest{
		BranchID: fx.branchID, Direction: "in", RefType: "opening", RefID: 9001,
		Lines: []stockcontracts.PostLine{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, Qty: 1, UnitCost: 5000}},
	}); err != nil {
		t.Fatalf("post: %v", err)
	}
	if _, err := fin.Sync(ctx(), fx.branchID); err != nil {
		t.Fatalf("sync: %v", err)
	}
	var lowID int64
	_ = testMods.db.QueryRow(`SELECT MIN(stock_movement_id) FROM finance_inventory_cost_movements WHERE branch_id = ?`, fx.branchID).Scan(&lowID)
	if _, err := testMods.db.Exec(`DELETE FROM finance_inventory_cost_movements WHERE branch_id = ? AND stock_movement_id = ?`, fx.branchID, lowID); err != nil {
		t.Fatalf("delete low row: %v", err)
	}
	if _, err := fin.Sync(ctx(), fx.branchID); err != nil {
		t.Fatalf("sync after gap: %v", err)
	}
	var back int
	_ = testMods.db.QueryRow(`SELECT COUNT(*) FROM finance_inventory_cost_movements WHERE branch_id = ? AND stock_movement_id = ?`, fx.branchID, lowID).Scan(&back)
	if back != 1 {
		t.Fatalf("late/missed movement %d not re-ingested (rows=%d)", lowID, back)
	}
}
