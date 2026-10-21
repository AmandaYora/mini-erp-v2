package transform_test

import (
	"math/big"
	"strings"
	"testing"

	"mini-erp/internal/etl/etltest"
	"mini-erp/internal/etl/extract"
	"mini-erp/internal/etl/report"
	"mini-erp/internal/etl/row"
	"mini-erp/internal/etl/transform"
)

func loadRaw(t *testing.T) map[string][]row.Row {
	t.Helper()
	env := etltest.Setup(t)
	work := t.TempDir()
	extractFixture(t, env, work)
	raw, err := extract.Load(work)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func run(t *testing.T, raw map[string][]row.Row) *transform.Output {
	t.Helper()
	out, err := transform.Run(transform.Input{Cfg: testCfg(t.TempDir()), Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func newID(out *transform.Output, entity string, legacy int64) int64 {
	for _, e := range out.IDMap {
		if e.Entity == entity && e.LegacyID == legacy && e.Sub == 0 {
			return e.NewID
		}
	}
	return 0
}

func finding(out *transform.Output, code string, legacy int64) *report.Finding {
	for i, f := range out.Log.Findings {
		if f.Code == code && f.LegacyID == legacy {
			return &out.Log.Findings[i]
		}
	}
	return nil
}

func cogsPosted(id, branch, movement int64) row.Row {
	return row.Row{"id_finance_posting_source": id, "id_branch": branch, "source_type": "inventory_movement", "id_source": movement,
		"event_key": "stock_issue_cogs", "posting_status": "posted", "id_journal_entry": nil}
}

// Legacy journals revenue when an ORDER completes and COGS when its SJ
// issues the goods. An SJ whose COGS legacy posted while the order never
// completed sits below the delivery cutover (posted in part: ignored), so
// neither system would ever book its revenue: the migration books it openly
// (ADR-0018 D11), Dr Piutang (customer) / Cr Penjualan (+ PPN Keluaran).
func TestTransformRevenueUnbooked(t *testing.T) {
	raw := loadRaw(t)
	// SJ 2 (order 2, tax-inclusive) and SJ 3 (order 6): COGS posted, sale not.
	raw["finance_posting_sources"] = append(raw["finance_posting_sources"],
		cogsPosted(901, 1, 13), cogsPosted(902, 2, 17), cogsPosted(903, 2, 18))
	out := run(t, raw)

	acct := map[int64]string{}
	for _, a := range out.Tables["finance_accounts"] {
		acct[a.Int("id")] = a.Str("code")
	}
	type sums struct{ revenue, tax, receivable, entry int64 }
	got := map[string]*sums{}
	for _, r := range out.Tables["migration_corrections"] {
		if r.Str("kind") != "revenue_unbooked" {
			continue
		}
		g := got[r.Str("ref")]
		if g == nil {
			g = &sums{entry: r.Int("entry_id")}
			got[r.Str("ref")] = g
		}
		if r.Str("debit_account") != "1100" {
			t.Errorf("revenue correction debits %s, want 1100 (Piutang): %v", r.Str("debit_account"), r)
		}
		switch r.Str("credit_account") {
		case "4000":
			g.revenue += r.Int("amount")
		case "2100":
			g.tax += r.Int("amount")
		default:
			t.Errorf("revenue correction credits %s: %v", r.Str("credit_account"), r)
		}
	}
	// Order 6: 4 x 15.000, untaxed. Order 2: 5 x 11.100 including 11% tax.
	want := map[string]sums{"ORD-CBG/PJ/2026/02/00001": {revenue: 60000}, "ORD-PST/PJ/2026/01/00001": {revenue: 50000, tax: 5500}}
	for ref, w := range want {
		g := got[ref]
		if g == nil || g.revenue != w.revenue || g.tax != w.tax {
			t.Errorf("%s: correction %+v, want revenue %d tax %d", ref, g, w.revenue, w.tax)
			continue
		}
		// The journal: receivable on the customer, revenue on the product.
		var dr, cr int64
		for _, l := range out.Tables["finance_journal_lines"] {
			if l.Int("entry_id") != g.entry {
				continue
			}
			dr += l.Int("debit")
			cr += l.Int("credit")
			switch acct[l.Int("account_id")] {
			case "1100":
				if !l.Has("party_id") || l.Has("product_id") {
					t.Errorf("%s: Piutang line without customer (or with product): %v", ref, l)
				}
			case "4000":
				if !l.Has("product_id") || l.Has("party_id") {
					t.Errorf("%s: Penjualan line without product: %v", ref, l)
				}
			}
		}
		if dr != w.revenue+w.tax || cr != dr {
			t.Errorf("%s: journal Dr %d Cr %d, want %d each", ref, dr, cr, w.revenue+w.tax)
		}
		for _, e := range out.Tables["finance_journal_entries"] {
			if e.Int("id") == g.entry && (e.Str("source_doc_type") != "migration_correction" || !strings.Contains(e.Str("memo"), ref)) {
				t.Errorf("%s: correction entry %v", ref, e)
			}
		}
	}
	f := finding(out, "REVENUE_UNBOOKED", 6)
	if f == nil || f.Severity != report.Warn || !strings.Contains(f.Detail, "Rp60000") {
		t.Errorf("REVENUE_UNBOOKED finding for order 6 = %+v", f)
	}
	if out.Explain["revenue_unbooked_value"] != 115500 {
		t.Errorf("revenue_unbooked_value = %d, want 115500", out.Explain["revenue_unbooked_value"])
	}
	// Never twice: the SJ stays below the delivery boundary (no queue item).
	dn := newID(out, "delivery_notes", 3)
	for _, m := range out.Marks {
		if m.DocType == "delivery" && m.DocID == dn && (m.State != "ignored" || m.Reason != "legacy_partial") {
			t.Errorf("SJ 3 mark %+v, want ignored/legacy_partial", m)
		}
	}
	for _, c := range out.Cutovers {
		if c.DocType == "delivery" && dn > c.Through {
			t.Errorf("SJ 3 (id %d) above delivery cutover %d of branch %d: the queue would book it again", dn, c.Through, c.BranchID)
		}
	}
}

// A legacy sale journal for the order means the revenue IS in the books:
// no correction, whatever the posting-source rows say.
func TestTransformRevenueBookedByLegacyNotCorrected(t *testing.T) {
	raw := loadRaw(t)
	raw["finance_posting_sources"] = append(raw["finance_posting_sources"], cogsPosted(902, 2, 17), cogsPosted(903, 2, 18))
	raw["finance_journal_entries"] = append(raw["finance_journal_entries"], row.Row{
		"id_finance_journal_entry": int64(9999), "id_branch": int64(2), "journal_number": "JE-X", "journal_date": "2026-02-10T03:00:00Z",
		"posting_type": "auto", "source_type": "order", "id_source": int64(6), "event_key": "sales_completed", "description": "x", "status": "posted",
	})
	out := run(t, raw)
	for _, r := range out.Tables["migration_corrections"] {
		if r.Str("kind") == "revenue_unbooked" {
			t.Errorf("revenue booked twice: %v", r)
		}
	}
}

// An SJ dispatched but never confirmed in legacy migrates confirmed (the
// revamp has no in-transit state) with confirmed_at = dispatch, and is listed.
func TestTransformDeliveryInTransit(t *testing.T) {
	raw := loadRaw(t)
	for _, d := range raw["delivery_notes"] {
		if d.Int("id_delivery_note") == 3 {
			d["confirmed_at"], d["confirmed_by"] = nil, nil
		}
	}
	out := run(t, raw)
	f := finding(out, "DN_IN_TRANSIT", 3)
	if f == nil || f.Severity != report.Warn {
		t.Fatalf("DN_IN_TRANSIT finding = %+v", f)
	}
	id := newID(out, "delivery_notes", 3)
	for _, d := range out.Tables["delivery_notes"] {
		if d.Int("id") == id {
			if d.Str("status") != "confirmed" || d.Str("confirmed_at") != d.Str("dispatched_at") || d.Str("confirmed_at") == "" || !d.Has("confirmed_by") {
				t.Errorf("in-transit SJ = status %s confirmed_at %q dispatched_at %q by %v", d.Str("status"), d.Str("confirmed_at"), d.Str("dispatched_at"), d["confirmed_by"])
			}
		}
	}
	if out.Explain["dn_in_transit"] != 1 {
		t.Errorf("dn_in_transit = %d", out.Explain["dn_in_transit"])
	}
}

// A confirmed order whose lines are all delivered migrates completed (as the
// revamp's completeOrderIfDone would have done); a draft carrying an active
// payment is listed for operations.
func TestTransformOrderCleanups(t *testing.T) {
	raw := loadRaw(t)
	for _, o := range raw["orders"] {
		if o.Int("id_order") == 6 {
			o["id_current_status"] = int64(3) // in_progress
		}
	}
	raw["payments"] = append(raw["payments"], row.Row{
		"id_payment": int64(9), "id_order": int64(5), "id_branch": int64(1), "id_business_party": nil, "financial_side": "receivable",
		"payment_number": "PAY-PST/2026/00009", "payment_date": "2026-03-02T03:00:00Z", "amount": "20000.00", "amount_tendered": nil,
		"payment_method": "cash", "reference_number": nil, "notes": nil, "proof_file_path": nil, "id_created_by": int64(2),
		"created_at": "2026-03-02 10:00:00.000000", "archived_at": nil,
	})
	out := run(t, raw)
	so := newID(out, "sales_orders", 6)
	for _, o := range out.Tables["sales_orders"] {
		switch o.Int("id") {
		case so:
			if o.Str("status") != "completed" || o.Str("completed_at") != "2026-02-11 05:00:00.000000" {
				t.Errorf("delivered order 6 = %s completed_at %q, want completed at the SJ confirmation", o.Str("status"), o.Str("completed_at"))
			}
		case newID(out, "sales_orders", 5):
			if o.Str("status") != "confirmed" {
				t.Errorf("paid draft order 5 = %s, want confirmed (ADR-0018 D11f)", o.Str("status"))
			}
		}
	}
	if f := finding(out, "ORDER_COMPLETED_DELIVERED", 6); f == nil || f.Severity != report.Info {
		t.Errorf("ORDER_COMPLETED_DELIVERED = %+v", f)
	}
	moves := map[string]int64{}
	for _, rc := range out.Tables["order_reclass"] {
		moves[rc.Str("from")+">"+rc.Str("to")] += rc.Int("grand_total")
	}
	if len(moves) != 2 || moves["confirmed>completed"] != 60000 || moves["draft>confirmed"] == 0 {
		t.Errorf("order_reclass = %v", out.Tables["order_reclass"])
	}
	if f := finding(out, "DRAFT_ORDER_PAID", 5); f == nil || f.Severity != report.Warn || !strings.Contains(f.Detail, "Rp20000") {
		t.Errorf("DRAFT_ORDER_PAID = %+v", f)
	}
}

// Movements of the same instant: stock that arrives (opening, adjustment,
// in) is carded before stock that leaves, whatever the legacy ids.
func TestTransformStockSameInstantOrder(t *testing.T) {
	raw := loadRaw(t)
	var at any
	for _, m := range raw["inventory_movements"] {
		if m.Int("id_inventory_movement") == 15 {
			at = m["moved_at"]
		}
	}
	for _, m := range raw["inventory_movements"] {
		if m.Int("id_inventory_movement") == 16 {
			m["moved_at"] = at
		}
	}
	out := run(t, raw)
	if a, o := newID(out, "stock_movements", 16), newID(out, "stock_movements", 15); a == 0 || o == 0 || a > o {
		t.Errorf("adjustment (new id %d) must precede the same-instant out (new id %d)", a, o)
	}
	if finding(out, "STOCK_CARD_RECOMPUTED", 0) == nil {
		t.Error("STOCK_CARD_RECOMPUTED summary missing")
	}
}

// ADR-0018 D11f: an SJ of an order legacy cancelled (and re-entered as a new
// order) keeps its quantities, books no revenue or receivable, and the COGS
// legacy journalled for it moves to the stock-difference account.
func TestTransformCancelledOrderShippedCOGS(t *testing.T) {
	raw := loadRaw(t)
	var cancelled, cogs, inventory int64
	for _, d := range raw["order_status_definitions"] {
		if d.Str("status_group") == "cancelled" {
			cancelled = d.Int("id_order_status_definition")
		}
	}
	for _, a := range raw["finance_accounts"] {
		switch a.Str("system_key") {
		case "cogs":
			cogs = a.Int("id_finance_account")
		case "inventory":
			inventory = a.Int("id_finance_account")
		}
	}
	if cancelled == 0 || cogs == 0 || inventory == 0 {
		t.Fatalf("fixture lacks a cancelled status (%d) or COGS/inventory accounts (%d/%d)", cancelled, cogs, inventory)
	}
	for _, o := range raw["orders"] {
		if o.Int("id_order") == 6 {
			o["id_current_status"] = cancelled
		}
	}
	entry := row.Row{"id_finance_journal_entry": int64(990), "id_company": int64(1), "id_branch": int64(2), "journal_number": "JE-CBG/2026/00990",
		"journal_date": "2026-02-11T04:00:00.000000Z", "description": "HPP SJ order batal", "event_key": "stock_issue_cogs",
		"source_type": "inventory_movement", "id_source": int64(17), "status": "posted", "posting_type": "auto",
		"posted_at": "2026-02-11T04:00:00.000000Z", "posted_by": int64(2), "created_at": "2026-02-11T04:00:00.000000Z",
		"updated_at": "2026-02-11T04:00:00.000000Z", "archived_at": nil, "id_reversal_of": nil, "metadata_json": nil, "reversed_at": nil, "reversed_by": nil}
	line := func(id, acct int64, dr, cr string) row.Row {
		return row.Row{"id_finance_journal_line": id, "id_company": int64(1), "id_branch": int64(2), "id_finance_journal_entry": int64(990),
			"id_finance_account": acct, "debit_amount": dr, "credit_amount": cr, "line_no": id - 9900, "id_product": int64(1),
			"id_product_variant": int64(1), "id_business_party": nil, "id_inventory_movement": nil, "id_order": nil, "id_payment": nil,
			"description": nil, "metadata_json": nil, "created_at": "2026-02-11T04:00:00.000000Z"}
	}
	raw["finance_journal_entries"] = append(raw["finance_journal_entries"], entry)
	raw["finance_journal_lines"] = append(raw["finance_journal_lines"], line(9901, cogs, "33000.00", "0.00"), line(9902, inventory, "0.00", "33000.00"))
	src := cogsPosted(902, 2, 17)
	src["id_journal_entry"] = int64(990)
	raw["finance_posting_sources"] = append(raw["finance_posting_sources"], src, cogsPosted(903, 2, 18))
	out := run(t, raw)

	var reclass int64
	for _, r := range out.Tables["migration_corrections"] {
		switch r.Str("kind") {
		case "revenue_unbooked":
			if r.Str("ref") == "ORD-CBG/PJ/2026/02/00001" {
				t.Errorf("revenue invented for a cancelled order: %v", r)
			}
		case "cancelled_order_cogs":
			if r.Str("credit_account") == r.Str("debit_account") {
				t.Errorf("reclass on one account: %v", r)
			}
			reclass += r.Int("amount")
		}
	}
	if reclass != 33000 {
		t.Errorf("COGS reclassified = %d, want the 33000 legacy journalled", reclass)
	}
	if f := finding(out, "REVENUE_UNBOOKED_CANCELLED", 6); f == nil || !strings.Contains(f.Detail, "Rp33000") {
		t.Errorf("REVENUE_UNBOOKED_CANCELLED = %+v", f)
	}
}

// ADR-0018 D11g: a legacy book value that drifted from what the goods cost
// (here twice the average, which the purchase price supports) is revalued to
// average x units; the correction moves the books by the same amount.
func TestTransformRevaluesDriftedBookCost(t *testing.T) {
	raw := loadRaw(t)
	for _, c := range raw["finance_inventory_cost_states"] {
		if c.Int("id_finance_inventory_cost_state") == 1 {
			c["inventory_value"] = "2000000.00" // book doubled; unit_cost (average) stays 10.000
		}
	}
	out := run(t, raw)
	rv := out.Tables["cost_revaluation"]
	if len(rv) != 1 || rv[0].Str("product_code") != "BRG-001" {
		t.Fatalf("cost_revaluation = %v, want BRG-001 only", rv)
	}
	qty := rv[0].Rat("qty")
	want := new(big.Rat).Mul(qty, big.NewRat(10000, 1))
	if got := rv[0].Rat("revalued_value"); got.Cmp(want) != 0 {
		t.Errorf("revalued value = %s, want 10.000 x %s", got.FloatString(2), qty.FloatString(4))
	}
	var corr int64
	for _, r := range out.Tables["migration_corrections"] {
		if r.Str("kind") == "cost_revalue" {
			corr += r.Int("amount")
		}
	}
	if corr >= 0 || corr != rv[0].Int("delta") {
		t.Errorf("revaluation correction = %d, want the (negative) delta %d", corr, rv[0].Int("delta"))
	}
	if f := finding(out, "COST_REVALUED", 1); f == nil {
		t.Error("COST_REVALUED finding missing")
	}
}

// ADR-0018 D11h: a payment cancelled in legacy whose journal stayed live is
// mirrored, on the customer's receivable, so the books match the documents.
func TestTransformMirrorsCancelledPaymentJournal(t *testing.T) {
	raw := loadRaw(t)
	raw["payments"] = append(raw["payments"], row.Row{
		"id_payment": int64(77), "id_order": int64(1), "id_branch": int64(1), "id_business_party": int64(3), "financial_side": "receivable",
		"payment_number": "PAY-PST/2026/00077", "payment_date": "2026-03-02T03:00:00Z", "amount": "50000.00", "amount_tendered": nil,
		"payment_method": "transfer", "reference_number": nil, "notes": nil, "proof_file_path": nil, "id_created_by": int64(2),
		"created_at": "2026-03-02 10:00:00.000000", "archived_at": "2026-03-03 10:00:00.000000",
	})
	raw["finance_posting_sources"] = append(raw["finance_posting_sources"], row.Row{"id_finance_posting_source": int64(977), "id_branch": int64(1),
		"source_type": "payment", "id_source": int64(77), "event_key": "customer_payment", "posting_status": "posted", "id_journal_entry": int64(991)})
	raw["finance_journal_entries"] = append(raw["finance_journal_entries"], row.Row{"id_finance_journal_entry": int64(991), "id_company": int64(1),
		"id_branch": int64(1), "journal_number": "JE-PST/2026/00991", "journal_date": "2026-03-02T03:00:00.000000Z", "description": "Pembayaran",
		"event_key": "customer_payment", "source_type": "payment", "id_source": int64(77), "status": "posted", "posting_type": "auto",
		"posted_at": "2026-03-02T03:00:00.000000Z", "posted_by": int64(2), "created_at": "2026-03-02T03:00:00.000000Z",
		"updated_at": "2026-03-02T03:00:00.000000Z", "archived_at": nil, "id_reversal_of": nil, "metadata_json": nil, "reversed_at": nil, "reversed_by": nil})
	line := func(id, acct int64, dr, cr string) row.Row {
		return row.Row{"id_finance_journal_line": id, "id_company": int64(1), "id_branch": int64(1), "id_finance_journal_entry": int64(991),
			"id_finance_account": acct, "debit_amount": dr, "credit_amount": cr, "line_no": id - 9910, "id_product": nil, "id_product_variant": nil,
			"id_business_party": int64(3), "id_inventory_movement": nil, "id_order": nil, "id_payment": int64(77), "description": nil,
			"metadata_json": nil, "created_at": "2026-03-02T03:00:00.000000Z"}
	}
	raw["finance_journal_lines"] = append(raw["finance_journal_lines"], line(9911, 2, "50000.00", "0.00"), line(9912, 3, "0.00", "50000.00"))
	out := run(t, raw)

	var entry int64
	for _, r := range out.Tables["migration_corrections"] {
		if r.Str("kind") == "payment_cancelled_journal" {
			if r.Int("amount") != 50000 || r.Str("debit_account") != "1100" || r.Str("credit_account") != "1010" {
				t.Errorf("mirror = %v, want Dr 1100 / Cr 1010 Rp50000", r)
			}
			entry = r.Int("entry_id")
		}
	}
	if entry == 0 {
		t.Fatal("cancelled payment with a live journal was not mirrored")
	}
	party, _ := newID(out, "parties", 3), 0
	for _, l := range out.Tables["finance_journal_lines"] {
		if l.Int("entry_id") == entry && l.Int("debit") > 0 && l.Int("party_id") != party {
			t.Errorf("receivable leg without the customer: %v (want party %d)", l, party)
		}
	}
}
