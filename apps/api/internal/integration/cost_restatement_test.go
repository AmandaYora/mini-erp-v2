package integration

import (
	"strings"
	"testing"

	financeapp "mini-erp/internal/modules/finance/application"
)

type restateSnap struct {
	costSum   string
	costRows  int
	entries   int
	lineSum   string
	auditRows int
}

func snapRestate(t *testing.T) restateSnap {
	t.Helper()
	var s restateSnap
	if err := testMods.db.QueryRow(`SELECT COALESCE(SUM(total_cost),0), COUNT(*) FROM finance_inventory_cost_movements`).Scan(&s.costSum, &s.costRows); err != nil {
		t.Fatalf("snap cost: %v", err)
	}
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM finance_journal_entries`).Scan(&s.entries); err != nil {
		t.Fatalf("snap entries: %v", err)
	}
	if err := testMods.db.QueryRow(`SELECT CONCAT(COALESCE(SUM(debit),0),'/',COALESCE(SUM(credit),0)) FROM finance_journal_lines`).Scan(&s.lineSum); err != nil {
		t.Fatalf("snap lines: %v", err)
	}
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action_key = ?`, "finance.cost_restatement").Scan(&s.auditRows); err != nil {
		t.Fatalf("snap audit: %v", err)
	}
	return s
}

// OP-04: cost restatement. A shipment was costed (and journalled) at a stale
// average; the dry run reports the difference and changes nothing; a commit is
// refused while the trial balance does not foot; a commit on a sound ledger
// rewrites the row and books ONE balanced correction journal, after which the
// book-vs-ledger reconciliation is still exact and a second run finds nothing.
func TestCostRestatementDryRunGateAndCommit(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	clearFixtureStock(t)
	fin := testMods.finance.Service()

	p := productWith(t, "RESTATE", 0, 1)
	seedInCost(t, fx, p.ID, p.Variants[0].ID, fx.locationID, 10, 7000)
	noteID := sellAndDeliver(t, fx, p.ID, p.Variants[0].ID, 3)
	if _, err := fin.Sync(ctx(), fx.branchID); err != nil {
		t.Fatalf("sync: %v", err)
	}
	legs := postedLegs(t, "delivery", noteID)
	if legs["5000"][0] != 21000 || legs["1200"][1] != 21000 {
		t.Fatalf("legs = %v, want 21000 COGS/inventory", legs)
	}

	// Corrupt history consistently: the ledger row and its journal both carry a
	// stale 5.000 average, so the books and the ledger still agree with each other.
	var outID, entryID int64
	if err := testMods.db.QueryRow(`SELECT id FROM finance_inventory_cost_movements WHERE direction = 'out'`).Scan(&outID); err != nil {
		t.Fatalf("out row: %v", err)
	}
	if err := testMods.db.QueryRow(`SELECT id FROM finance_journal_entries WHERE source_doc_type = 'delivery'`).Scan(&entryID); err != nil {
		t.Fatalf("journal: %v", err)
	}
	for _, st := range []struct {
		q  string
		id int64
	}{
		{`UPDATE finance_inventory_cost_movements SET unit_cost = 5000, total_cost = -15000 WHERE id = ?`, outID},
		{`UPDATE finance_journal_lines SET debit = 15000 WHERE entry_id = ? AND debit > 0`, entryID},
		{`UPDATE finance_journal_lines SET credit = 15000 WHERE entry_id = ? AND credit > 0`, entryID},
	} {
		if _, err := testMods.db.Exec(st.q, st.id); err != nil {
			t.Fatalf("tamper: %v", err)
		}
	}
	if err := fin.RebuildRollups(ctx(), fx.branchID); err != nil {
		t.Fatalf("rebuild rollups: %v", err)
	}
	rec, err := fin.ReconcileInventory(ctx(), fx.branchID)
	if err != nil || !rec.Consistent || rec.Pending != 0 {
		t.Fatalf("precondition recon = %+v err=%v, want consistent without pending", rec, err)
	}

	// (1) Dry run: reports the per-product difference, writes nothing.
	snap0 := snapRestate(t)
	dry, err := fin.RestateCost(ctx(), financeapp.RestateCostOptions{BranchID: fx.branchID})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if dry.Commit || dry.Applied || dry.RowsChanged != 1 || len(dry.Products) != 1 || dry.Delta != -6000 {
		t.Fatalf("dry run = %+v, want one row, delta -6000, nothing applied", dry)
	}
	if got := dry.Products[0]; got.ProductID != p.ID || got.ProductCode != "RESTATE" || got.Delta != -6000 {
		t.Fatalf("dry product = %+v", got)
	}
	for _, g := range dry.Gates {
		if !g.OK {
			t.Fatalf("gate %s failed on a sound ledger: %s", g.Key, g.Detail)
		}
	}
	if snap1 := snapRestate(t); snap1 != snap0 {
		t.Fatalf("dry run changed data: %+v -> %+v", snap0, snap1)
	}

	// (2) The trial-balance gate refuses a commit and leaves everything alone.
	var lineID int64
	if err := testMods.db.QueryRow(`SELECT id FROM finance_journal_lines WHERE entry_id = ? AND debit > 0`, entryID).Scan(&lineID); err != nil {
		t.Fatalf("line: %v", err)
	}
	if _, err := testMods.db.Exec(`UPDATE finance_journal_lines SET debit = 15500 WHERE id = ?`, lineID); err != nil {
		t.Fatalf("unbalance: %v", err)
	}
	snapBad := snapRestate(t)
	_, err = fin.RestateCost(ctx(), financeapp.RestateCostOptions{BranchID: fx.branchID, Commit: true})
	if err == nil || !strings.Contains(err.Error(), "trial_balance") {
		t.Fatalf("commit on an unbalanced ledger = %v, want a trial_balance refusal", err)
	}
	if snap := snapRestate(t); snap != snapBad {
		t.Fatalf("refused commit changed data: %+v -> %+v", snapBad, snap)
	}
	if _, err := testMods.db.Exec(`UPDATE finance_journal_lines SET debit = 15000 WHERE id = ?`, lineID); err != nil {
		t.Fatalf("rebalance: %v", err)
	}

	// (3) Commit: row corrected, one balanced journal, audit, reconciliation exact.
	res, err := fin.RestateCost(ctx(), financeapp.RestateCostOptions{BranchID: fx.branchID, Commit: true})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if !res.Applied || res.EntryID == 0 || res.Delta != -6000 {
		t.Fatalf("commit = %+v, want applied with a journal and delta -6000", res)
	}
	var unit, total string
	if err := testMods.db.QueryRow(`SELECT unit_cost, total_cost FROM finance_inventory_cost_movements WHERE id = ?`, outID).Scan(&unit, &total); err != nil {
		t.Fatalf("row after: %v", err)
	}
	if unit != "7000.0000" || total != "-21000.00" {
		t.Fatalf("row after = %s / %s, want 7000.0000 / -21000.00", unit, total)
	}
	var debit, credit, inv, cogs int64
	if err := testMods.db.QueryRow(`SELECT COALESCE(SUM(debit),0), COALESCE(SUM(credit),0) FROM finance_journal_lines WHERE entry_id = ?`, res.EntryID).Scan(&debit, &credit); err != nil {
		t.Fatalf("correction lines: %v", err)
	}
	if debit != 6000 || credit != 6000 {
		t.Fatalf("correction journal debit %d credit %d, want balanced 6000", debit, credit)
	}
	_ = testMods.db.QueryRow(`SELECT COALESCE(SUM(l.credit),0) FROM finance_journal_lines l JOIN finance_accounts a ON a.id = l.account_id WHERE l.entry_id = ? AND a.code = '1200'`, res.EntryID).Scan(&inv)
	_ = testMods.db.QueryRow(`SELECT COALESCE(SUM(l.debit),0) FROM finance_journal_lines l JOIN finance_accounts a ON a.id = l.account_id WHERE l.entry_id = ? AND a.code = '5000'`, res.EntryID).Scan(&cogs)
	if inv != 6000 || cogs != 6000 {
		t.Fatalf("Cr Persediaan %d / Dr HPP %d, want 6000 / 6000", inv, cogs)
	}
	if res.ReconAfter == nil || !res.ReconAfter.Consistent || res.ReconAfter.Diff != 0 {
		t.Fatalf("recon after = %+v, want exact", res.ReconAfter)
	}
	if snap := snapRestate(t); snap.auditRows != snap0.auditRows+1 {
		t.Fatalf("audit rows = %d, want %d", snap.auditRows, snap0.auditRows+1)
	}
	rep, err := fin.VerifyRollups(ctx(), fx.branchID)
	if err != nil || !rep.Consistent || rep.Unbalanced != 0 || len(rep.RollupDiffs) != 0 {
		t.Fatalf("integrity after commit = %+v err=%v", rep, err)
	}

	// (4) Idempotent: nothing left to restate.
	again, err := fin.RestateCost(ctx(), financeapp.RestateCostOptions{BranchID: fx.branchID, Commit: true})
	if err != nil || again.RowsChanged != 0 || again.Applied {
		t.Fatalf("second run = %+v err=%v, want nothing to do", again, err)
	}
}

// OP-04: a transfer row is never restated (the other branch's ledger carries
// the same cost and no journal exists for it).
func TestCostRestatementLeavesTransfersAlone(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	clearFixtureStock(t)
	p := productWith(t, "XFER", 0, 1)
	seedInCost(t, fx, p.ID, p.Variants[0].ID, fx.locationID, 10, 7000)
	fin := testMods.finance.Service()
	if _, err := fin.Sync(ctx(), fx.branchID); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if _, err := testMods.db.Exec(`INSERT INTO finance_inventory_cost_movements
		(branch_id, product_id, variant_id, stock_movement_id, ref_type, ref_id, direction, qty_base, unit_cost, total_cost, is_estimated)
		VALUES (?, ?, ?, 900001, 'transfer', 1, 'out', 4, 1000, -4000, 0)`, fx.branchID, p.ID, p.Variants[0].ID); err != nil {
		t.Fatalf("insert transfer row: %v", err)
	}
	rep, err := fin.RestateCost(ctx(), financeapp.RestateCostOptions{BranchID: fx.branchID})
	if err != nil || rep.RowsChanged != 0 {
		t.Fatalf("restate = %+v err=%v, want the transfer row left alone", rep, err)
	}
}
