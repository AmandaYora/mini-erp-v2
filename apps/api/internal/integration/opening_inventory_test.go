package integration

import (
	branchapp "mini-erp/internal/modules/branch/application"
	stockapp "mini-erp/internal/modules/stock/application"
	"testing"

	financeapp "mini-erp/internal/modules/finance/application"
	"mini-erp/internal/shared/apperror"
)

// clearFixtureStock removes the fixture's own stock so a test controls every
// unit the cost ledger sees (the fixture adjustment would otherwise sit in the
// posting queue and blur the book-vs-ledger comparison).
func clearFixtureStock(t *testing.T) {
	t.Helper()
	if _, err := testMods.db.Exec(`DELETE FROM stock_movements`); err != nil {
		t.Fatalf("clear movements: %v", err)
	}
}

func checkOf(t *testing.T, r *financeapp.Readiness, key string) financeapp.ReadinessCheck {
	t.Helper()
	for _, c := range r.Checks {
		if c.Key == key {
			return c
		}
	}
	t.Fatalf("readiness has no check %q: %+v", key, r.Checks)
	return financeapp.ReadinessCheck{}
}

// K-05 (F-05): the cutover journal carries a server-computed Persediaan leg equal to
// the inventory-value report; the leg cannot be typed; a drift between books and
// the cost ledger shows up in integrity and readiness; products held without a
// cost are listed and can be completed after the opening is locked.
func TestOpeningInventoryJournalReconcilesWithLedger(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	clearFixtureStock(t)
	fin := testMods.finance.Service()

	p1 := productWith(t, "COSTED", 0, 1)      // explicit cost 7000
	p2 := productWith(t, "NOMODAL", 0, 1)     // no catalog price, no hint
	p3 := productWith(t, "DUS12", 120000, 12) // catalog 10.000 per pcs (estimated)
	seedInCost(t, fx, p1.ID, p1.Variants[0].ID, fx.locationID, 10, 7000)
	seedIn(t, fx, p2.ID, p2.Variants[0].ID, fx.locationID, 5)
	seedIn(t, fx, p3.ID, p3.Variants[0].ID, fx.locationID, 12)

	// (1) the server computes the figure and lists the product without a cost.
	inv, err := fin.OpeningInventory(ctx(), fx.branchID)
	if err != nil {
		t.Fatalf("opening inventory: %v", err)
	}
	if inv.Total != 190000 || inv.Positions != 2 {
		t.Fatalf("inventory = total %d positions %d, want 190000 over 2", inv.Total, inv.Positions)
	}
	if len(inv.MissingCost) != 1 || inv.MissingCost[0].ProductID != p2.ID || inv.MissingCost[0].Qty != 5 {
		t.Fatalf("missing cost = %+v, want only NOMODAL x5", inv.MissingCost)
	}
	if inv.OpeningPosted {
		t.Fatal("opening reported posted before it was")
	}

	// (2) the Persediaan account cannot be typed.
	user := []financeapp.ManualLine{{AccountCode: "1000", Debit: 50000}, {AccountCode: "3000", Credit: 50000}}
	if _, err := fin.PreviewOpening(ctx(), fx.branchID, "2026-10-01", "", append([]financeapp.ManualLine{{AccountCode: "1200", Debit: 1}}, user...)); err == nil {
		t.Fatal("typed Persediaan leg accepted, want validation")
	} else if ae := mustAppErr(t, err); ae.Code != apperror.CodeValidation {
		t.Fatalf("typed Persediaan code = %s, want validation", ae.Code)
	}

	// (3) preview shows ONE inventory line + equity; totals balance.
	prev, err := fin.PreviewOpening(ctx(), fx.branchID, "2026-10-01", "", user)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	var pd, pc, invDebit int64
	for _, l := range prev.Lines {
		pd += l.Debit
		pc += l.Credit
		if l.AccountCode == "1200" {
			invDebit += l.Debit
		}
	}
	if invDebit != 190000 || pd != pc || pd != 240000 {
		t.Fatalf("preview debit %d credit %d inventory %d, want 240000 balanced with 190000 inventory", pd, pc, invDebit)
	}

	// (4) post: per-product inventory legs, equity plugs, report agrees.
	entry, err := fin.PostOpening(ctx(), 0, fx.branchID, "2026-10-01", "Saldo awal", user)
	if err != nil {
		t.Fatalf("post opening: %v", err)
	}
	var invLines int
	var invSum, eqCredit int64
	for _, l := range entry.Lines {
		switch l.AccountCode {
		case "1200":
			invLines++
			invSum += l.Debit
			if l.ProductID == 0 {
				t.Fatal("inventory leg lost its product dimension")
			}
		case "3000":
			eqCredit += l.Credit
		}
	}
	if invLines != 2 || invSum != 190000 || eqCredit != 240000 {
		t.Fatalf("posted legs: %d inventory lines sum %d, equity %d; want 2 / 190000 / 240000", invLines, invSum, eqCredit)
	}
	_, reportTotal, err := fin.InventoryValue(ctx(), fx.branchID)
	if err != nil {
		t.Fatalf("inventory value: %v", err)
	}
	if reportTotal != invSum {
		t.Fatalf("inventory report %d != opening inventory legs %d", reportTotal, invSum)
	}
	if d, c, err := fin.AccountBalance(ctx(), fx.branchID, "1200"); err != nil || d-c != reportTotal {
		t.Fatalf("book 1200 = %d-%d err=%v, want %d", d, c, err, reportTotal)
	}
	again, err := fin.PostOpening(ctx(), 0, fx.branchID, "2026-10-01", "ulang", user)
	if err != nil || again.ID != entry.ID {
		t.Fatalf("re-post = %v / %v, want the same entry %d", again, err, entry.ID)
	}

	// (5) books == ledger: integrity consistent, readiness flags only the missing cost.
	rep, err := fin.VerifyRollups(ctx(), fx.branchID)
	if err != nil {
		t.Fatalf("integrity: %v", err)
	}
	if rep.Inventory == nil || rep.Inventory.Diff != 0 || !rep.Consistent {
		t.Fatalf("integrity after opening = %+v (%+v), want zero diff", rep, rep.Inventory)
	}
	ready, err := fin.Readiness(ctx(), fx.branchID, 2026, 10)
	if err != nil {
		t.Fatalf("readiness: %v", err)
	}
	if c := checkOf(t, ready, "inventory_recon"); c.Status != financeapp.CheckOK {
		t.Fatalf("inventory_recon = %+v, want ok", c)
	}
	if c := checkOf(t, ready, "missing_cost"); c.Status != financeapp.CheckWarning || c.Count != 1 {
		t.Fatalf("missing_cost = %+v, want one warning", c)
	}

	// (6) complete the uncosted product after the lock: one supplement journal
	// and one ledger row, books and ledger move together.
	sup, err := fin.CompleteOpening(ctx(), 0, fx.branchID, []financeapp.CompleteItem{{ProductID: p2.ID, VariantID: p2.Variants[0].ID, UnitCost: 3000}})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if sup.Total != 15000 {
		t.Fatalf("supplement total = %d, want 15000", sup.Total)
	}
	if _, total, _ := fin.InventoryValue(ctx(), fx.branchID); total != 205000 {
		t.Fatalf("inventory after complete = %d, want 205000", total)
	}
	if rep, _ := fin.VerifyRollups(ctx(), fx.branchID); rep.Inventory.Diff != 0 || !rep.Consistent {
		t.Fatalf("integrity after complete = %+v", rep.Inventory)
	}
	if inv, _ := fin.OpeningInventory(ctx(), fx.branchID); len(inv.MissingCost) != 0 || !inv.OpeningPosted {
		t.Fatalf("after complete missing=%d posted=%v, want none / true", len(inv.MissingCost), inv.OpeningPosted)
	}
	if _, err := fin.CompleteOpening(ctx(), 0, fx.branchID, []financeapp.CompleteItem{{ProductID: p2.ID, VariantID: p2.Variants[0].ID, UnitCost: 3000}}); err == nil {
		t.Fatal("completing an already costed product accepted, want validation")
	}
	// The synthetic ledger row must not hide later real movements from costing.
	seedInCost(t, fx, p1.ID, p1.Variants[0].ID, fx.locationID, 1, 8000)
	if res, err := fin.Sync(ctx(), fx.branchID); err != nil || res.Ingested != 1 {
		t.Fatalf("sync after complete = %+v err=%v, want 1 ingested", res, err)
	}

	// (7) the new stock reached the ledger without a journal: drift is detected
	// as a blocker (no pending documents explain it).
	rep, err = fin.VerifyRollups(ctx(), fx.branchID)
	if err != nil {
		t.Fatalf("integrity drift: %v", err)
	}
	if rep.Inventory.Diff != -8000 || rep.Consistent {
		t.Fatalf("drift = %+v consistent=%v, want diff -8000 and inconsistent", rep.Inventory, rep.Consistent)
	}
	ready, err = fin.Readiness(ctx(), fx.branchID, 2026, 10)
	if err != nil {
		t.Fatalf("readiness drift: %v", err)
	}
	if c := checkOf(t, ready, "inventory_recon"); c.Status != financeapp.CheckFailed || ready.Ready {
		t.Fatalf("inventory_recon = %+v ready=%v, want failed blocker", c, ready.Ready)
	}
}

// K-05: completing before the opening is posted is refused (the plain opening
// already prices the stock).
func TestCompleteOpeningRequiresPostedOpening(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	_, err := testMods.finance.Service().CompleteOpening(ctx(), 0, fx.branchID, []financeapp.CompleteItem{{ProductID: fx.productID, VariantID: fx.variantID, UnitCost: 1000}})
	if ae := mustAppErr(t, err); ae.Code != apperror.CodeConflict {
		t.Fatalf("complete before opening code = %s, want conflict", ae.Code)
	}
}

// K-05 regression (found by e2e E09): value that moves through the cost ledger
// without a journal BY DESIGN — cross-branch transfers and opening stock before
// the finance opening is posted — must not make the book-vs-ledger check block a
// period close. It is excluded from the comparison and reported instead.
func TestInventoryReconExcludesUnjournaledByDesign(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	// Journal the fixture's stock adjustment so the books hold the 50 units.
	for _, id := range queuedStockMovements(t, fx.branchID) {
		postedLegs(t, "stock_movement", id)
	}
	// Opening stock with no finance opening yet: awaiting cutover, not drift.
	p := productWith(t, "PREOPEN", 0, 1)
	seedInCost(t, fx, p.ID, p.Variants[0].ID, fx.locationID, 4, 5000)

	// Cross-branch transfer: 5 units HQ -> CB2 at the HQ average.
	br2, err := testMods.branch.Service().CreateBranch(ctx(), 0, branchapp.CreateBranchInput{Code: "CB2", Name: "Cabang 2"})
	if err != nil {
		t.Fatalf("branch: %v", err)
	}
	loc2, err := testMods.stock.Service().CreateLocation(ctx(), 0, br2.ID, stockapp.LocationInput{Code: "CB2", Name: "Gudang CB2"})
	if err != nil {
		t.Fatalf("location: %v", err)
	}
	tr, err := testMods.stock.Service().CreateTransfer(ctx(), 0, fx.branchID, br2.ID, fx.locationID, loc2.ID, "antar cabang",
		[]stockapp.TransferItemInput{{ProductID: fx.productID, VariantID: fx.variantID, Qty: 5}})
	if err != nil {
		t.Fatalf("create transfer: %v", err)
	}
	if _, err := testMods.stock.Service().DispatchTransfer(ctx(), 0, fx.branchID, tr.ID); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if _, err := testMods.stock.Service().ReceiveTransfer(ctx(), 0, br2.ID, tr.ID); err != nil {
		t.Fatalf("receive: %v", err)
	}

	for _, b := range []int64{fx.branchID, br2.ID} {
		rec, err := fin.ReconcileInventory(ctx(), b)
		if err != nil {
			t.Fatalf("recon %d: %v", b, err)
		}
		if rec.Diff != 0 || !rec.Consistent {
			t.Fatalf("branch %d recon = %+v, want zero diff", b, rec)
		}
	}
	hq, _ := fin.ReconcileInventory(ctx(), fx.branchID)
	if hq.TransferNet != -50000 || hq.OpeningUnposted != 20000 {
		t.Fatalf("hq transferNet=%d openingUnposted=%d, want -50000 / 20000", hq.TransferNet, hq.OpeningUnposted)
	}
	if cb, _ := fin.ReconcileInventory(ctx(), br2.ID); cb.TransferNet != 50000 {
		t.Fatalf("cb2 transferNet=%d, want 50000", cb.TransferNet)
	}
	ready, err := fin.Readiness(ctx(), fx.branchID, 2026, 10)
	if err != nil {
		t.Fatalf("readiness: %v", err)
	}
	if c := checkOf(t, ready, "inventory_recon"); c.Status != financeapp.CheckOK {
		t.Fatalf("inventory_recon = %+v, want ok", c)
	}
	if checkOf(t, ready, "transfer_value").Status != financeapp.CheckWarning || checkOf(t, ready, "opening_unposted").Status != financeapp.CheckWarning {
		t.Fatalf("expected informational warnings, got %+v", ready.Checks)
	}
}
