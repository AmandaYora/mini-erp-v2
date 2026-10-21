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
			if o.Str("status") != "draft" {
				t.Errorf("draft order 5 became %s", o.Str("status"))
			}
		}
	}
	if f := finding(out, "ORDER_COMPLETED_DELIVERED", 6); f == nil || f.Severity != report.Info {
		t.Errorf("ORDER_COMPLETED_DELIVERED = %+v", f)
	}
	if rc := out.Tables["order_reclass"]; len(rc) != 1 || rc[0].Str("from") != "confirmed" || rc[0].Str("to") != "completed" || rc[0].Int("grand_total") != 60000 {
		t.Errorf("order_reclass = %v", rc)
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
