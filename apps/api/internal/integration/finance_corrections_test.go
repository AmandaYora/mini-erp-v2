package integration

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"mini-erp/internal/bootstrap"
	financeapp "mini-erp/internal/modules/finance/application"
	"mini-erp/internal/shared/apperror"
)

// correctionTokens seeds the permission catalog and returns two sessions on
// the branch: one that may preview (finance.manage) and one that may also
// commit (finance.close.force).
func correctionTokens(t *testing.T, branchID int64) (manage, force string) {
	t.Helper()
	if err := bootstrap.EnsureAuthz(ctx(), testMods.user.Service()); err != nil {
		t.Fatalf("seed authz: %v", err)
	}
	manage = guardUser(t, branchID, "fin-manage", []string{"finance.view", "finance.manage"})
	force = guardUser(t, branchID, "fin-force", []string{"finance.view", "finance.manage", "finance.close.force"})
	return manage, force
}

func auditMetadata(t *testing.T, action string) (int, string) {
	t.Helper()
	var n int
	var meta string
	if err := testMods.db.QueryRow(`SELECT COUNT(*), COALESCE(MAX(CAST(d.metadata_json AS CHAR)), '')
		FROM audit_logs a LEFT JOIN audit_log_details d ON d.log_id = a.id WHERE a.action_key = ?`, action).Scan(&n, &meta); err != nil {
		t.Fatalf("audit %s: %v", action, err)
	}
	return n, meta
}

// Web restatement: the dry run over HTTP writes nothing; a commit needs
// finance.close.force and a reason, is refused by a failing gate without
// touching anything, and records the reason in its audit entry.
func TestCostRestatementOverHTTP(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	clearFixtureStock(t)
	fin := testMods.finance.Service()

	p := productWith(t, "RESTWEB", 0, 1)
	seedInCost(t, fx, p.ID, p.Variants[0].ID, fx.locationID, 10, 7000)
	noteID := sellAndDeliver(t, fx, p.ID, p.Variants[0].ID, 3)
	if _, err := fin.Sync(ctx(), fx.branchID); err != nil {
		t.Fatalf("sync: %v", err)
	}
	postedLegs(t, "delivery", noteID)
	// The same stale history as TestCostRestatementDryRunGateAndCommit.
	var outID, entryID int64
	if err := testMods.db.QueryRow(`SELECT id FROM finance_inventory_cost_movements WHERE direction = 'out'`).Scan(&outID); err != nil {
		t.Fatalf("out row: %v", err)
	}
	if err := testMods.db.QueryRow(`SELECT id FROM finance_journal_entries WHERE source_doc_type = 'delivery'`).Scan(&entryID); err != nil {
		t.Fatalf("journal: %v", err)
	}
	for _, q := range []string{
		`UPDATE finance_inventory_cost_movements SET unit_cost = 5000, total_cost = -15000 WHERE id = ?`,
		`UPDATE finance_journal_lines SET debit = 15000 WHERE entry_id = ? AND debit > 0`,
		`UPDATE finance_journal_lines SET credit = 15000 WHERE entry_id = ? AND credit > 0`,
	} {
		id := entryID
		if strings.Contains(q, "cost_movements") {
			id = outID
		}
		if _, err := testMods.db.Exec(q, id); err != nil {
			t.Fatalf("tamper: %v", err)
		}
	}
	if err := fin.RebuildRollups(ctx(), fx.branchID); err != nil {
		t.Fatalf("rebuild rollups: %v", err)
	}

	manage, force := correctionTokens(t, fx.branchID)
	h := routeServer(t).Handler()

	// (1) Dry run over HTTP: the report, nothing written.
	snap0 := snapRestate(t)
	rec := doReq(h, http.MethodPost, "/api/v1/finance/cost-restatement/preview", manage, `{}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview = %d %s", rec.Code, rec.Body.String())
	}
	var dry financeapp.RestateReport
	decodeData(t, rec, &dry)
	if dry.Commit || dry.Applied || dry.RowsChanged != 1 || dry.Delta != -6000 || len(dry.Products) != 1 ||
		dry.Products[0].ProductCode != "RESTWEB" || len(dry.Gates) != 4 || dry.ReconBefore == nil {
		t.Fatalf("preview = %+v", dry)
	}
	if snap := snapRestate(t); snap != snap0 {
		t.Fatalf("preview changed data: %+v -> %+v", snap0, snap)
	}

	// (2) Commit needs finance.close.force and a reason.
	if rec := doReq(h, http.MethodPost, "/api/v1/finance/cost-restatement/commit", manage, `{"reason":"x"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("commit without finance.close.force = %d, want 403", rec.Code)
	}
	if rec := doReq(h, http.MethodPost, "/api/v1/finance/cost-restatement/commit", force, `{"reason":"  "}`); rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("commit without reason = %d %s, want validation", rec.Code, rec.Body.String())
	} else if !strings.Contains(rec.Body.String(), "alasan wajib diisi") {
		t.Fatalf("commit without reason body = %s", rec.Body.String())
	}
	if snap := snapRestate(t); snap != snap0 {
		t.Fatalf("refused commit changed data: %+v -> %+v", snap0, snap)
	}

	// (3) A failing gate refuses the commit and leaves everything alone.
	var lineID int64
	if err := testMods.db.QueryRow(`SELECT id FROM finance_journal_lines WHERE entry_id = ? AND debit > 0`, entryID).Scan(&lineID); err != nil {
		t.Fatalf("line: %v", err)
	}
	if _, err := testMods.db.Exec(`UPDATE finance_journal_lines SET debit = 15500 WHERE id = ?`, lineID); err != nil {
		t.Fatalf("unbalance: %v", err)
	}
	snapBad := snapRestate(t)
	rec = doReq(h, http.MethodPost, "/api/v1/finance/cost-restatement/commit", force, `{"reason":"uji gerbang"}`)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "trial_balance") {
		t.Fatalf("gated commit = %d %s, want 409 trial_balance", rec.Code, rec.Body.String())
	}
	if snap := snapRestate(t); snap != snapBad {
		t.Fatalf("gated commit changed data: %+v -> %+v", snapBad, snap)
	}
	if _, err := testMods.db.Exec(`UPDATE finance_journal_lines SET debit = 15000 WHERE id = ?`, lineID); err != nil {
		t.Fatalf("rebalance: %v", err)
	}

	// (4) Commit: journal number, recon before/after, audit with the reason.
	rec = doReq(h, http.MethodPost, "/api/v1/finance/cost-restatement/commit", force, `{"reason":"HPP SJ lama memakai rata-rata basi"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("commit = %d %s", rec.Code, rec.Body.String())
	}
	var done financeapp.RestateReport
	decodeData(t, rec, &done)
	if !done.Applied || done.EntryNumber == "" || done.Delta != -6000 || done.ReconBefore == nil ||
		done.ReconAfter == nil || !done.ReconAfter.Consistent || done.ReconAfter.Diff != 0 {
		t.Fatalf("commit = %+v", done)
	}
	n, meta := auditMetadata(t, "finance.cost_restatement")
	if n != 1 || !strings.Contains(meta, "HPP SJ lama memakai rata-rata basi") {
		t.Fatalf("audit = %d rows, metadata %s; want one with the reason", n, meta)
	}
	var actor int64
	if err := testMods.db.QueryRow(`SELECT COALESCE(actor_id, 0) FROM audit_logs WHERE action_key = 'finance.cost_restatement'`).Scan(&actor); err != nil || actor == 0 {
		t.Fatalf("audit actor = %d err=%v, want the session user", actor, err)
	}

	// (5) Nothing left: a second commit reports so and writes nothing.
	snap1 := snapRestate(t)
	rec = doReq(h, http.MethodPost, "/api/v1/finance/cost-restatement/commit", force, `{"reason":"ulang"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Tidak ada biaya keluar") {
		t.Fatalf("second commit = %d %s", rec.Code, rec.Body.String())
	}
	if snap := snapRestate(t); snap != snap1 {
		t.Fatalf("empty commit changed data: %+v -> %+v", snap1, snap)
	}
}

type revalSnap struct {
	costRows, entries, revaluations, audit int
	ledger                                 string
}

func snapReval(t *testing.T) revalSnap {
	t.Helper()
	var s revalSnap
	for q, dst := range map[string]*int{
		`SELECT COUNT(*) FROM finance_inventory_cost_movements`:                              &s.costRows,
		`SELECT COUNT(*) FROM finance_journal_entries`:                                       &s.entries,
		`SELECT COUNT(*) FROM finance_inventory_revaluations`:                                &s.revaluations,
		`SELECT COUNT(*) FROM audit_logs WHERE action_key = 'finance.inventory_revaluation'`: &s.audit,
	} {
		if err := testMods.db.QueryRow(q).Scan(dst); err != nil {
			t.Fatalf("snap %s: %v", q, err)
		}
	}
	if err := testMods.db.QueryRow(`SELECT COALESCE(SUM(total_cost), 0) FROM finance_inventory_cost_movements`).Scan(&s.ledger); err != nil {
		t.Fatalf("snap ledger: %v", err)
	}
	return s
}

func revalBody(productID, variantID, cost int64, extra string) string {
	return fmt.Sprintf(`{"productId":%d,"variantId":%d,"newUnitCost":%d%s}`, productID, variantID, cost, extra)
}

// Revaluation ("Revaluasi harga modal"): a wrong non-zero cost basis is set to
// a new unit cost by ONE ledger value row and ONE journal (Persediaan vs
// Selisih Stok); books equal the ledger afterwards, the next shipment is
// costed at the new average, the restatement leaves that cost alone, a replay
// changes nothing, and the refusal cases write nothing.
func TestInventoryRevaluation(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	clearFixtureStock(t)
	fin := testMods.finance.Service()

	p := productWith(t, "REVAL", 0, 1)
	vid := p.Variants[0].ID
	empty := productWith(t, "REVALNOL", 0, 1)
	seedInCost(t, fx, p.ID, vid, fx.locationID, 10, 7000) // opening cost typed wrong: should be 9.000
	one := []financeapp.RevaluationItem{{ProductID: p.ID, VariantID: vid, NewUnitCost: 9000}}

	// Before the finance opening is posted the cutover journal prices the
	// ledger itself: a revaluation would be booked twice.
	_, err := fin.CommitRevaluation(ctx(), financeapp.RevaluationInput{BranchID: fx.branchID, Reason: "x", Items: one})
	if ae := mustAppErr(t, err); ae.Code != apperror.CodeConflict {
		t.Fatalf("revaluation before opening = %v, want conflict", err)
	}
	user := []financeapp.ManualLine{{AccountCode: "1000", Debit: 50000}, {AccountCode: "3000", Credit: 50000}}
	if _, err := fin.PostOpening(ctx(), 0, fx.branchID, "2026-10-01", "Saldo awal", user); err != nil {
		t.Fatalf("post opening: %v", err)
	}
	recBefore, err := fin.ReconcileInventory(ctx(), fx.branchID)
	if err != nil || !recBefore.Consistent {
		t.Fatalf("precondition recon = %+v err=%v", recBefore, err)
	}

	// Refusals: nothing is written.
	snap0 := snapReval(t)
	for name, in := range map[string]financeapp.RevaluationInput{
		"negative cost": {Reason: "x", Items: []financeapp.RevaluationItem{{ProductID: p.ID, VariantID: vid, NewUnitCost: -1}}},
		"no stock":      {Reason: "x", Items: []financeapp.RevaluationItem{{ProductID: empty.ID, VariantID: empty.Variants[0].ID, NewUnitCost: 5000}}},
		"future date":   {Reason: "x", Date: "2099-01-01", Items: one},
		"no reason":     {Items: one},
		"no change":     {Reason: "x", Items: []financeapp.RevaluationItem{{ProductID: p.ID, VariantID: vid, NewUnitCost: 7000}}},
		"duplicate":     {Reason: "x", Items: append(append([]financeapp.RevaluationItem{}, one...), one...)},
	} {
		in.BranchID = fx.branchID
		_, err := fin.CommitRevaluation(ctx(), in)
		if ae := mustAppErr(t, err); ae.Code != apperror.CodeValidation {
			t.Fatalf("%s: code %s (%v), want validation", name, ae.Code, err)
		}
	}
	if snap := snapReval(t); snap != snap0 {
		t.Fatalf("refusals changed data: %+v -> %+v", snap0, snap)
	}

	manage, force := correctionTokens(t, fx.branchID)
	h := routeServer(t).Handler()

	// The page's position read: qty, average, reference prices.
	rec := doReq(h, http.MethodGet, fmt.Sprintf("/api/v1/finance/inventory-revaluations/positions?productId=%d", p.ID), manage, "")
	var positions []financeapp.RevaluationPosition
	decodeData(t, rec, &positions)
	if rec.Code != http.StatusOK || len(positions) != 1 || positions[0].Qty != 10 || positions[0].Value != 70000 ||
		positions[0].AverageCost != 7000 || positions[0].ProductCode != "REVAL" {
		t.Fatalf("positions = %d %+v", rec.Code, positions)
	}

	// Preview over HTTP: the delta, nothing written.
	rec = doReq(h, http.MethodPost, "/api/v1/finance/inventory-revaluations/preview", manage, revalBody(p.ID, vid, 9000, ``))
	var prev financeapp.RevaluationResult
	decodeData(t, rec, &prev)
	if rec.Code != http.StatusOK || prev.Applied || prev.TotalDelta != 20000 || len(prev.Lines) != 1 ||
		prev.Lines[0].ValueBefore != 70000 || prev.Lines[0].ValueAfter != 90000 {
		t.Fatalf("preview = %d %+v", rec.Code, prev)
	}
	if snap := snapReval(t); snap != snap0 {
		t.Fatalf("preview changed data: %+v -> %+v", snap0, snap)
	}
	// Commit needs finance.close.force.
	if rec := doReq(h, http.MethodPost, "/api/v1/finance/inventory-revaluations/commit", manage,
		revalBody(p.ID, vid, 9000, `,"reason":"x"`)); rec.Code != http.StatusForbidden {
		t.Fatalf("commit without finance.close.force = %d, want 403", rec.Code)
	}

	// (1) Commit: one ledger value row, one journal, the record, the audit.
	body := revalBody(p.ID, vid, 9000, `,"reason":"Harga saldo awal salah ketik","clientRef":"rv-1"`)
	rec = doReq(h, http.MethodPost, "/api/v1/finance/inventory-revaluations/commit", force, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("commit = %d %s", rec.Code, rec.Body.String())
	}
	var done financeapp.RevaluationResult
	decodeData(t, rec, &done)
	if !done.Applied || done.ID == 0 || done.EntryNumber == "" || done.TotalDelta != 20000 ||
		done.ReconBefore == nil || done.ReconAfter == nil || !done.ReconAfter.Consistent ||
		done.ReconAfter.Unexplained != done.ReconBefore.Unexplained {
		t.Fatalf("commit = %+v (before %+v after %+v)", done, done.ReconBefore, done.ReconAfter)
	}
	var qty, total, refType string
	var refID int64
	if err := testMods.db.QueryRow(`SELECT qty_base, total_cost, ref_type, ref_id FROM finance_inventory_cost_movements
		WHERE ref_type = 'inventory_revaluation'`).Scan(&qty, &total, &refType, &refID); err != nil {
		t.Fatalf("ledger row: %v", err)
	}
	if dec(qty) != 0 || total != "20000.00" || refID != done.ID {
		t.Fatalf("ledger row = qty %s total %s ref %d, want 0 / 20000.00 / %d", qty, total, refID, done.ID)
	}
	legs := entryLegs(t, done.EntryID)
	if legs["1200"] != [2]int64{20000, 0} || legs["5100"] != [2]int64{0, 20000} {
		t.Fatalf("journal legs = %v, want Dr 1200 / Cr 5100 20000", legs)
	}
	var journalOnRecord int64
	if err := testMods.db.QueryRow(`SELECT journal_entry_id FROM finance_inventory_revaluations WHERE id = ?`, done.ID).Scan(&journalOnRecord); err != nil || journalOnRecord != done.EntryID {
		t.Fatalf("record journal = %d err=%v, want %d", journalOnRecord, err, done.EntryID)
	}
	if n, meta := auditMetadata(t, "finance.inventory_revaluation"); n != 1 || !strings.Contains(meta, "Harga saldo awal salah ketik") {
		t.Fatalf("audit = %d %s, want one with the reason", n, meta)
	}
	if d, c, err := fin.AccountBalance(ctx(), fx.branchID, "1200"); err != nil || d-c != 90000 {
		t.Fatalf("book Persediaan = %d err=%v, want 90000 (= ledger)", d-c, err)
	}

	// (2) Replay with the same clientRef: the first commit, nothing new.
	snap1 := snapReval(t)
	rec = doReq(h, http.MethodPost, "/api/v1/finance/inventory-revaluations/commit", force, body)
	var again financeapp.RevaluationResult
	decodeData(t, rec, &again)
	if rec.Code != http.StatusOK || !again.Replayed || again.ID != done.ID || again.EntryNumber != done.EntryNumber {
		t.Fatalf("replay = %d %+v", rec.Code, again)
	}
	if snap := snapReval(t); snap != snap1 {
		t.Fatalf("replay changed data: %+v -> %+v", snap1, snap)
	}
	// Without the key the same request changes nothing either: the position
	// already carries that cost.
	if rec := doReq(h, http.MethodPost, "/api/v1/finance/inventory-revaluations/commit", force,
		revalBody(p.ID, vid, 9000, `,"reason":"lagi"`)); rec.Code == http.StatusCreated || !strings.Contains(rec.Body.String(), "sudah sesuai") {
		t.Fatalf("repeat without key = %d %s, want refused", rec.Code, rec.Body.String())
	}

	// (3) The next shipment is costed at the new average (3 × 9.000).
	noteID := sellAndDeliver(t, fx, p.ID, vid, 3)
	if _, err := fin.Sync(ctx(), fx.branchID); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if legs := postedLegs(t, "delivery", noteID); legs["5000"][0] != 27000 || legs["1200"][1] != 27000 {
		t.Fatalf("delivery legs = %v, want HPP 27000 at the revalued cost", legs)
	}
	// The restatement replays the revaluation where it took effect, so it
	// finds nothing to "correct" in that shipment.
	if rep, err := fin.RestateCost(ctx(), financeapp.RestateCostOptions{BranchID: fx.branchID}); err != nil || rep.RowsChanged != 0 {
		t.Fatalf("restatement after revaluation = %+v err=%v, want nothing to restate", rep, err)
	}

	// (4) Down: 7 left at 9.000 (63.000) set to 5.000 (35.000): Cr Persediaan.
	down, err := fin.CommitRevaluation(ctx(), financeapp.RevaluationInput{BranchID: fx.branchID, ActorID: 1,
		Reason: "harga turun", Items: []financeapp.RevaluationItem{{ProductID: p.ID, VariantID: vid, NewUnitCost: 5000}}})
	if err != nil {
		t.Fatalf("revalue down: %v", err)
	}
	if down.TotalDelta != -28000 || !down.ReconAfter.Consistent || down.ReconAfter.Unexplained != down.ReconBefore.Unexplained {
		t.Fatalf("revalue down = %+v", down)
	}
	if legs := entryLegs(t, down.EntryID); legs["1200"] != [2]int64{0, 28000} || legs["5100"] != [2]int64{28000, 0} {
		t.Fatalf("down legs = %v, want Cr 1200 / Dr 5100 28000", legs)
	}
	// ADR-0023: "Balik Jurnal" would move the books without the ledger.
	if _, err := fin.Reverse(ctx(), 1, fx.branchID, down.EntryID, "uji balik"); err == nil {
		t.Fatal("reversing a revaluation journal was accepted")
	}
	_, invTotal, err := fin.InventoryValue(ctx(), fx.branchID)
	if err != nil || invTotal != 35000 {
		t.Fatalf("inventory value = %d err=%v, want 35000", invTotal, err)
	}
	if d, c, _ := fin.AccountBalance(ctx(), fx.branchID, "1200"); d-c != invTotal {
		t.Fatalf("book Persediaan %d != ledger %d", d-c, invTotal)
	}
	rep, err := fin.VerifyRollups(ctx(), fx.branchID)
	if err != nil || !rep.Consistent || rep.Inventory == nil || rep.Inventory.Diff != 0 {
		t.Fatalf("integrity = %+v (%+v) err=%v", rep, rep.Inventory, err)
	}

	// (5) History lists both, newest first.
	rec = doReq(h, http.MethodGet, "/api/v1/finance/inventory-revaluations", manage, "")
	var list []financeapp.RevaluationResult
	decodeData(t, rec, &list)
	if rec.Code != http.StatusOK || len(list) != 2 || list[0].ID != down.ID || list[1].Reason != "Harga saldo awal salah ketik" ||
		len(list[1].Lines) != 1 || list[1].Lines[0].ProductCode != "REVAL" || list[1].EntryNumber != done.EntryNumber {
		t.Fatalf("list = %d %+v", rec.Code, list)
	}

	// (6) A closed period refuses the date.
	if err := fin.ClosePeriod(ctx(), 0, fx.branchID, 2026, 9); err != nil {
		t.Fatalf("close september: %v", err)
	}
	_, err = fin.CommitRevaluation(ctx(), financeapp.RevaluationInput{BranchID: fx.branchID, Reason: "x", Date: "2026-09-30",
		Items: []financeapp.RevaluationItem{{ProductID: p.ID, VariantID: vid, NewUnitCost: 6000}}})
	if ae := mustAppErr(t, err); ae.Code != apperror.CodeConflict || !strings.Contains(ae.Message, "ditutup") {
		t.Fatalf("closed period = %v, want conflict", err)
	}
}

func entryLegs(t *testing.T, entryID int64) map[string][2]int64 {
	t.Helper()
	rows, err := testMods.db.Query(`SELECT a.code, l.debit, l.credit FROM finance_journal_lines l
		JOIN finance_accounts a ON a.id = l.account_id WHERE l.entry_id = ?`, entryID)
	if err != nil {
		t.Fatalf("legs: %v", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string][2]int64{}
	for rows.Next() {
		var code string
		var d, c int64
		if err := rows.Scan(&code, &d, &c); err != nil {
			t.Fatalf("scan leg: %v", err)
		}
		cur := out[code]
		out[code] = [2]int64{cur[0] + d, cur[1] + c}
	}
	return out
}

func dec(s string) float64 {
	var f float64
	_, _ = fmt.Sscanf(s, "%g", &f)
	return f
}
