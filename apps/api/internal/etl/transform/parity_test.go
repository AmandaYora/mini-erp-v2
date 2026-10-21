package transform_test

import (
	"strings"
	"testing"

	"mini-erp/internal/etl/etltest"
	"mini-erp/internal/etl/extract"
	"mini-erp/internal/etl/report"
	"mini-erp/internal/etl/row"
	"mini-erp/internal/etl/transform"
)

// DL-01: the legacy media purpose picks the revamp owner type of a delivery proof.
func TestTransformDeliveryProofPurposeOwnerType(t *testing.T) {
	env := etltest.Setup(t)
	work := t.TempDir()
	extractFixture(t, env, work)
	out := runTransform(t, work)
	got := map[int64]string{}
	for _, r := range out.Tables["media_candidates"] {
		got[r.Int("legacy_id")] = r.Str("owner_type")
	}
	if got[3] != "delivery_document" {
		t.Errorf("signed document (legacy 3) owner_type = %q, want delivery_document", got[3])
	}
	if got[4] != "delivery_proof" {
		t.Errorf("location photo (legacy 4) owner_type = %q, want delivery_proof", got[4])
	}
	if got[1] != "product" {
		t.Errorf("product image owner_type = %q, want product", got[1])
	}
}

// ST-01/ST-02: location attributes and the transfer's driver/vehicle/date travel.
func TestTransformLocationAttributesAndTransferFace(t *testing.T) {
	env := etltest.Setup(t)
	work := t.TempDir()
	extractFixture(t, env, work)
	out := runTransform(t, work)
	byCode := map[string]int64{}
	var rakSort, gdgPrimary, damagedFlags int64
	for _, r := range out.Tables["stock_locations"] {
		byCode[r.Str("code")] = r.Int("id")
		switch r.Str("code") {
		case "RAK-A":
			rakSort = r.Int("sort_order")
		case "GDG":
			gdgPrimary = r.Int("is_primary")
		case "RUSAK":
			damagedFlags = r.Int("is_primary") + r.Int("is_picking_area") + r.Int("sort_order")
		}
	}
	if len(byCode) == 0 || rakSort != 2 || gdgPrimary != 1 || damagedFlags != 0 {
		t.Errorf("location attrs: rak sort=%d gdg primary=%d damaged flags=%d (codes %v)", rakSort, gdgPrimary, damagedFlags, byCode)
	}
	rows := out.Tables["stock_transfers"]
	if len(rows) == 0 {
		t.Fatal("no transfers")
	}
	r := rows[0]
	if r.Str("driver_name") != "Pak Sopir" || r.Str("vehicle_plate") != "B 1" || len(r.Str("transfer_date")) < 10 {
		t.Errorf("transfer face = driver %q plate %q date %q", r.Str("driver_name"), r.Str("vehicle_plate"), r.Str("transfer_date"))
	}
}

// Q-HPP-2: legacy books the finance opening as cost rows WITHOUT a movement,
// one per (branch, product), while the opening quantity already sits in the
// opening_stock movements. The value must land on those movements (pro rata
// qty, real cost) and no synthetic stock movement may appear.
func TestTransformOpeningCostSpread(t *testing.T) {
	env := etltest.Setup(t)
	work := t.TempDir()
	extractFixture(t, env, work)
	out := runTransform(t, work)

	newMov := map[int64]int64{}
	for _, e := range out.IDMap {
		if e.Entity == "stock_movements" && e.Sub == 0 {
			newMov[e.LegacyID] = e.NewID
		}
	}
	cost := map[int64]struct{ total, est string }{}
	for _, r := range out.Tables["finance_inventory_cost_movements"] {
		cost[r.Int("stock_movement_id")] = struct{ total, est string }{r.Str("total_cost"), r.Str("is_estimated")}
	}
	// Legacy opening movements 1 (20) and 2 (5) share product 2's pooled
	// Rp300.000,50 -> Rp300.001; the odd rupiah goes to the larger remainder.
	// Movement 3 takes product 3's row whole; movement 4 keeps its own linked row.
	want := map[int64]string{1: "240001.00", 2: "60000.00", 3: "80000.00", 4: "100000.00"}
	for lid, total := range want {
		got := cost[newMov[lid]]
		if got.total != total || got.est != "0" {
			t.Errorf("opening movement %d: cost %q estimated %q, want %s real", lid, got.total, got.est, total)
		}
	}
	opening := 0
	for _, r := range out.Tables["stock_movements"] {
		if r.Str("ref_type") == "opening" {
			opening++
		}
	}
	if opening != 4 {
		t.Errorf("opening stock movements = %d, want the 4 legacy ones (no synthetic movement)", opening)
	}
	codes := map[string]int{}
	for _, f := range out.Log.Findings {
		codes[f.Code]++
	}
	if codes["COST_NO_MOVEMENT"] != 0 || codes["COST_OPENING_UNMATCHED"] != 0 || codes["COST_NOOP"] != 1 || codes["COST_OPENING_SPLIT"] != 2 {
		t.Errorf("findings = %v, want COST_NOOP 1, COST_OPENING_SPLIT 2, no COST_NO_MOVEMENT/COST_OPENING_UNMATCHED", codes)
	}
	if out.Explain["opening_cost_split_extra"] != 1 || out.Explain["cost_rows_unmigrated"] != 1 {
		t.Errorf("explain split_extra=%d unmigrated=%d, want 1 and 1", out.Explain["opening_cost_split_extra"], out.Explain["cost_rows_unmigrated"])
	}
}

// Q-HPP-2: when the opening quantity in the cost ledger disagrees with the
// opening_stock movements, the value cannot be placed without changing stock;
// the transform refuses (blocker) instead of guessing.
func TestTransformOpeningCostUnmatchedBlocks(t *testing.T) {
	env := etltest.Setup(t)
	work := t.TempDir()
	extractFixture(t, env, work)
	raw, err := extract.Load(work)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range raw["finance_inventory_cost_movements"] {
		if r.Int("id_finance_inventory_cost_movement") == 3 {
			r["quantity_in_base_uom"] = "12.000000000000"
		}
	}
	out, err := transform.Run(transform.Input{Cfg: testCfg(work), Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	var blocked bool
	for _, f := range out.Log.Findings {
		if f.Code == "COST_OPENING_UNMATCHED" && f.LegacyID == 3 && f.Severity == report.Blocker {
			blocked = true
		}
	}
	if !blocked {
		t.Error("qty mismatch must raise blocker COST_OPENING_UNMATCHED for legacy cost row 3")
	}
}

// Legacy adjustments overwrite the balance (stock opname): quantity is the new
// count, the movement is the difference. Taking quantity as the change made
// hundreds of phantom STOCK_DRIFT corrections on production data.
func TestTransformAdjustmentIsDifference(t *testing.T) {
	env := etltest.Setup(t)
	work := t.TempDir()
	extractFixture(t, env, work)
	out := runTransform(t, work)
	var id int64
	for _, e := range out.IDMap {
		if e.Entity == "stock_movements" && e.LegacyID == 16 && e.Sub == 0 {
			id = e.NewID
		}
	}
	for _, r := range out.Tables["stock_movements"] {
		if r.Int("id") == id && (r.Str("direction") != "out" || r.Rat("qty_base").Cmp(row.ParseRat("1")) != 0) {
			t.Errorf("adjustment 87 -> 86 = %s %s, want out 1", r.Str("direction"), r.Str("qty_base"))
		}
	}
	for _, f := range out.Log.Findings {
		if f.Code == "STOCK_DRIFT" && f.LegacyID == 1 {
			t.Errorf("phantom drift on the adjusted position: %s", f.Detail)
		}
	}
}

// The revamp ledger sums total_cost as stored (an out is negative) and holds
// legacy's cost STATE: value legacy moved without cost rows arrives as
// cost-only rows (qty 0, ids from 1<<62, ref opening_cost).
func TestTransformCostSignAndBaseline(t *testing.T) {
	env := etltest.Setup(t)
	work := t.TempDir()
	extractFixture(t, env, work)
	out := runTransform(t, work)
	value := map[int64]int64{}
	var baseline []row.Row
	for _, r := range out.Tables["finance_inventory_cost_movements"] {
		total := transform.RoundHalfAway(r.Rat("total_cost"))
		if (r.Str("direction") == "out") != (total < 0) && total != 0 {
			t.Errorf("cost row %d: %s with total %d", r.Int("stock_movement_id"), r.Str("direction"), total)
		}
		if r.Str("is_estimated") == "0" {
			value[r.Int("product_id")] += total
		}
		if r.Str("ref_type") == "opening_cost" {
			baseline = append(baseline, r)
		}
	}
	if len(baseline) != 1 || baseline[0].Int("stock_movement_id") < 1<<62 || baseline[0].Str("total_cost") != "6000.00" || baseline[0].Rat("qty_base").Sign() != 0 {
		t.Fatalf("baseline rows = %v, want one cost-only row of 6000 above 1<<62", baseline)
	}
	// Per product the non-estimated ledger equals the legacy state (rounded).
	for code, want := range map[string]int64{"BRG-001": 1000000, "BRG-002": 264001, "BRG-003": 530000} {
		var pid int64
		for _, p := range out.Tables["products"] {
			if p.Str("code") == code {
				pid = p.Int("id")
			}
		}
		if pid == 0 {
			t.Fatalf("product %s missing", code)
		}
		if value[pid] != want {
			t.Errorf("product %s ledger value = %d, want legacy state %d", code, value[pid], want)
		}
	}
}

// A document queued for posting in the revamp must not carry a live legacy
// journal, or it would be booked twice.
func TestTransformPendingJournaledBlocks(t *testing.T) {
	env := etltest.Setup(t)
	work := t.TempDir()
	extractFixture(t, env, work)
	out := runTransform(t, work)
	pending := 0
	for _, f := range out.Log.Findings {
		switch f.Code {
		case "POST_PENDING":
			pending++
		case "POST_PENDING_JOURNALED":
			t.Errorf("fixture: unexpected %s", f.Detail)
		}
	}
	if pending == 0 {
		t.Fatal("fixture has pending documents (order 6): POST_PENDING expected")
	}
	raw, err := extract.Load(work)
	if err != nil {
		t.Fatal(err)
	}
	raw["finance_journal_entries"] = append(raw["finance_journal_entries"], row.Row{
		"id_finance_journal_entry": int64(9999), "id_branch": int64(2), "journal_number": "JE-X", "journal_date": "2026-02-10T03:00:00Z",
		"posting_type": "auto", "source_type": "order", "id_source": int64(6), "event_key": "sales_completed", "description": "x", "status": "posted",
	})
	out, err = transform.Run(transform.Input{Cfg: testCfg(work), Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range out.Log.Findings {
		if f.Code == "POST_PENDING_JOURNALED" && f.Severity == report.Blocker {
			return
		}
	}
	t.Error("a pending document with a live legacy journal must raise POST_PENDING_JOURNALED")
}

// A cancelled SJ brings its goods back with no legacy cost row; the estimate
// reverses exactly what that SJ relieved (not the catalog price), and the
// value legacy never put back on the books is reported.
func TestTransformDeliveryCancelCost(t *testing.T) {
	env := etltest.Setup(t)
	work := t.TempDir()
	extractFixture(t, env, work)
	raw, err := extract.Load(work)
	if err != nil {
		t.Fatal(err)
	}
	// SJ 1 took product 1 out at 10000 (movements 6 and 7); 2 come back.
	raw["inventory_movements"] = append(raw["inventory_movements"], row.Row{
		"id_inventory_movement": int64(900), "id_branch": int64(1), "id_product": int64(1), "id_product_variant": int64(1), "id_stock_location": int64(1),
		"movement_type": "in", "quantity": "2.000000000000", "balance_before": "94.000000000000", "balance_after": "96.000000000000",
		"reference_type": "delivery_note", "id_reference": int64(1), "reason_text": "Pembatalan SJ", "id_moved_by": int64(2), "moved_at": "2025-12-12T04:00:00Z",
	})
	out, err := transform.Run(transform.Input{Cfg: testCfg(work), Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	for _, e := range out.IDMap {
		if e.Entity == "stock_movements" && e.LegacyID == 900 {
			id = e.NewID
		}
	}
	for _, r := range out.Tables["finance_inventory_cost_movements"] {
		if r.Int("stock_movement_id") == id {
			if r.Str("total_cost") != "20000.00" || r.Str("is_estimated") != "1" {
				t.Errorf("cancellation cost = %s estimated %s, want 20000.00 estimated", r.Str("total_cost"), r.Str("is_estimated"))
			}
			if out.Explain["dn_cancel_value"] != 20000 {
				t.Errorf("dn_cancel_value = %d, want 20000", out.Explain["dn_cancel_value"])
			}
			assertCorrection(t, out, "dn_cancel_cogs", 20000, "1200", transform.OpeningCorrectionCode)
			return
		}
	}
	t.Fatalf("no cost row for the cancellation movement (new id %d)", id)
}

// assertCorrection checks one migration correction: its report row and its
// posted, balanced journal (ADR-0018 D11).
func assertCorrection(t *testing.T, out *transform.Output, kind string, amount int64, debit, credit string) {
	t.Helper()
	var entry int64
	for _, r := range out.Tables["migration_corrections"] {
		if r.Str("kind") == kind {
			if r.Int("amount") != amount || r.Str("debit_account") != debit || r.Str("credit_account") != credit {
				t.Errorf("%s correction = %v, want Rp%d Dr %s / Cr %s", kind, r, amount, debit, credit)
			}
			entry = r.Int("entry_id")
		}
	}
	if entry == 0 {
		t.Fatalf("no %s correction in %v", kind, out.Tables["migration_corrections"])
	}
	for _, e := range out.Tables["finance_journal_entries"] {
		if e.Int("id") == entry && (e.Str("source_doc_type") != "migration_correction" || e.Str("status") != "posted" || !strings.HasPrefix(e.Str("number"), "JV-")) {
			t.Errorf("correction entry = %v", e)
		}
	}
	var dr, cr int64
	for _, l := range out.Tables["finance_journal_lines"] {
		if l.Int("entry_id") == entry {
			dr += l.Int("debit")
			cr += l.Int("credit")
		}
	}
	if dr != amount || cr != amount {
		t.Errorf("correction journal Dr %d Cr %d, want %d each", dr, cr, amount)
	}
}

// A legacy cost state that values a product with no unit left is written
// down: no ledger value, a correction against the stock-difference account.
func TestTransformStateNoQtyWriteDown(t *testing.T) {
	env := etltest.Setup(t)
	work := t.TempDir()
	extractFixture(t, env, work)
	raw, err := extract.Load(work)
	if err != nil {
		t.Fatal(err)
	}
	raw["finance_inventory_cost_states"] = append(raw["finance_inventory_cost_states"], row.Row{
		"id_finance_inventory_cost_state": int64(9), "id_branch": int64(1), "id_product": int64(5),
		"inventory_value": "5000.00", "updated_at": "2026-02-13T04:00:00Z",
	})
	out, err := transform.Run(transform.Input{Cfg: testCfg(work), Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	assertCorrection(t, out, "state_no_qty", 5000, transform.OpeningCorrectionCode, "1200")
	pid, _ := func() (int64, bool) {
		for _, p := range out.Tables["products"] {
			if p.Str("code") == "BDL-001" {
				return p.Int("id"), true
			}
		}
		return 0, false
	}()
	for _, r := range out.Tables["finance_inventory_cost_movements"] {
		if r.Int("product_id") == pid && r.Str("total_cost") != "0.00" {
			t.Errorf("written-down product keeps ledger value: %v", r)
		}
	}
}

// Every estimated cost row that no coming journal covers is corrected, and
// none that the posting queue will journal is: books follow the ledger.
func TestTransformUnjournalledCorrected(t *testing.T) {
	env := etltest.Setup(t)
	work := t.TempDir()
	extractFixture(t, env, work)
	out := runTransform(t, work)
	pending := map[string]bool{}
	for _, m := range out.Marks {
		if m.State == "pending" {
			pending[m.DocType+":"+row.Row{"v": m.DocID}.Str("v")] = true
		}
	}
	listed := map[string]bool{}
	for _, r := range out.Tables["migration_corrections"] {
		listed[r.Str("stock_movement_id")] = true
	}
	n := 0
	for _, r := range out.Tables["finance_inventory_cost_movements"] {
		if r.Str("is_estimated") != "1" || r.Str("ref_type") == "transfer" || r.Str("total_cost") == "0.00" {
			continue
		}
		queued := pending[r.Str("ref_type")+":"+r.Str("ref_id")]
		if got := listed[r.Str("stock_movement_id")]; got == queued {
			t.Errorf("estimated row %s (%s %s): corrected=%v but queued for posting=%v", r.Str("stock_movement_id"), r.Str("ref_type"), r.Str("ref_id"), got, queued)
		}
		n++
	}
	if n == 0 {
		t.Fatal("fixture has no estimated rows to check")
	}
}

// A legacy cost state that holds units at no value while its own rows price
// them is written up to the rows' value (books and ledger together).
func TestTransformStateNoValueWriteUp(t *testing.T) {
	env := etltest.Setup(t)
	work := t.TempDir()
	extractFixture(t, env, work)
	raw, err := extract.Load(work)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range raw["finance_inventory_cost_states"] {
		if r.Int("id_product") == 3 {
			r["inventory_value"] = "0.00"
		}
	}
	out, err := transform.Run(transform.Input{Cfg: testCfg(work), Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	// Product 3's rows: opening 80000 + receipt 444000.
	assertCorrection(t, out, "state_no_value", 524000, "1200", transform.OpeningCorrectionCode)
}

// ADR-0024: a legacy stock adjustment journal books its stock-difference leg
// on Koreksi Saldo Awal (the opening stock was being counted and fixed), the
// added account and its mapping exist, and every moved line is logged.
func TestTransformStockAdjustmentToOpeningCorrection(t *testing.T) {
	env := etltest.Setup(t)
	work := t.TempDir()
	extractFixture(t, env, work)
	raw, err := extract.Load(work)
	if err != nil {
		t.Fatal(err)
	}
	// Journal 4 (current year) becomes a stock adjustment whose first line
	// sits on the legacy stock-difference account (13 = 5100).
	for _, r := range raw["finance_journal_entries"] {
		if r.Int("id_finance_journal_entry") == 4 {
			r["event_key"] = "stock_adjustment"
		}
	}
	var moved int64
	for _, l := range raw["finance_journal_lines"] {
		if l.Int("id_finance_journal_entry") == 4 && moved == 0 {
			l["id_finance_account"] = "13"
			moved = l.Int("id_finance_journal_line")
		}
	}
	if moved == 0 {
		t.Fatal("fixture journal 4 has no lines")
	}
	out, err := transform.Run(transform.Input{Cfg: testCfg(work), Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	acct := map[int64]string{}
	for _, a := range out.Tables["finance_accounts"] {
		acct[a.Int("id")] = a.Str("code")
		if a.Str("code") == transform.OpeningCorrectionCode && a.Str("type") != "modal" {
			t.Errorf("Koreksi Saldo Awal type = %q, want modal", a.Str("type"))
		}
	}
	mapped := false
	for _, m := range out.Tables["finance_account_mappings"] {
		if m.Str("mapping_key") == "opening_correction" && acct[m.Int("account_id")] == transform.OpeningCorrectionCode {
			mapped = true
		}
	}
	if !mapped {
		t.Error("opening_correction mapping to 3100 missing")
	}
	var entry int64
	for _, e := range out.Tables["finance_journal_entries"] {
		if e.Str("source_doc_type") == "legacy_je" && e.Int("source_doc_id") == 4 {
			entry = e.Int("id")
		}
	}
	var onEquity, onProfit int
	for _, l := range out.Tables["finance_journal_lines"] {
		if l.Int("entry_id") != entry {
			continue
		}
		switch acct[l.Int("account_id")] {
		case transform.OpeningCorrectionCode:
			onEquity++
		case "5100":
			onProfit++
		}
	}
	if onEquity != 1 || onProfit != 0 {
		t.Errorf("stock adjustment lines: %d on 3100, %d on 5100; want 1 and 0", onEquity, onProfit)
	}
	logged := false
	for _, a := range out.Log.Adjustments {
		if a.Rule == "opening_stock_correction" && a.LegacyID == 4 && a.Before == "5100" && a.After == transform.OpeningCorrectionCode {
			logged = true
		}
	}
	if !logged {
		t.Error("reclassified line not in adjustments.csv")
	}
}
