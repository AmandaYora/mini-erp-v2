package integration

import (
	"context"
	"fmt"
	"testing"

	"mini-erp/internal/modules/purchasing/contracts"
	purinfra "mini-erp/internal/modules/purchasing/infrastructure"
)

// Characterization tests for the purchasing Repository (F8-18): they pin the
// behaviour of every public method so the sqlc rewrite is provably neutral.
// Repositories take primitive ids, so no cross-module fixture is needed.

func sqlcPurRepo() *purinfra.Repository { return purinfra.NewRepository(testMods.db) }

type purSpec struct {
	branch, party int64
	number, date  string
	due           string
	terms         string
	taxType       string
	total         int64
	invNo, invDt  string
	notes         string
	termDays      int
	items         []*contracts.Item
}

func purMk(t *testing.T, r *purinfra.Repository, s purSpec) int64 {
	t.Helper()
	if s.terms == "" {
		s.terms = "net"
	}
	if s.taxType == "" {
		s.taxType = "none"
	}
	if s.date == "" {
		s.date = "2026-03-10 09:30:00"
	}
	id, err := r.CreateOrder(context.Background(), &contracts.PurchaseOrder{
		Number: s.number, BranchID: s.branch, PartyID: s.party, OrderDate: s.date, DueDate: s.due,
		PaymentTerms: s.terms, TaxType: s.taxType, TaxRate: 11, Subtotal: s.total, GrandTotal: s.total,
		Notes: s.notes, SupplierInvoiceNumber: s.invNo, SupplierInvoiceDate: s.invDt,
		PaymentTermDays: s.termDays, Items: s.items,
	}, 5)
	if err != nil {
		t.Fatalf("create %s: %v", s.number, err)
	}
	return id
}

func purIDs(os []*contracts.PurchaseOrder) []int64 {
	out := make([]int64, 0, len(os))
	for _, o := range os {
		out = append(out, o.ID)
	}
	return out
}

func sumIDs(os []*contracts.OrderSummary) []int64 {
	out := make([]int64, 0, len(os))
	for _, o := range os {
		out = append(out, o.ID)
	}
	return out
}

func sameInts(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSqlcPurchasingCRUDAndNulls(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPurRepo()

	full := purMk(t, r, purSpec{branch: 1, party: 10, number: "PO-FULL", due: "2026-04-01 00:00:00", terms: "net",
		taxType: "ppn", total: 123456, invNo: "INV-9", invDt: "2026-03-11 00:00:00", notes: "catatan", termDays: 30,
		items: []*contracts.Item{{
			ProductID: 1, VariantID: 2, ProductCode: "P1", ProductName: "Barang", UOM: "box", UOMFactor: 0.333333333333,
			Qty: 2.5, QtyBase: 0.833333333333, UnitPrice: 1000, DiscountPct: 12.5, DiscountNominal: 125, LineTotal: 2375,
			StockTracked: true, ItemType: "barang", VariantName: "Merah", VariantCode: "M", BaseUOM: "pcs",
			DiscountReason: "promo", Note: "baris", Taxable: true, TaxBase: 2375, TaxAmount: 261,
		}, {
			ProductID: 3, VariantID: 4, ProductCode: "J1", ProductName: "Jasa", UOM: "jam", Qty: 1, QtyBase: 1,
			UnitPrice: 500, LineTotal: 500,
		}}})
	bare := purMk(t, r, purSpec{branch: 1, party: 11, number: "PO-BARE", total: 7})

	o, err := r.GetOrder(ctx, full)
	if err != nil || o == nil {
		t.Fatalf("get full: %v %v", o, err)
	}
	if o.Number != "PO-FULL" || o.BranchID != 1 || o.PartyID != 10 || o.OrderDate != "2026-03-10 09:30:00" ||
		o.DueDate != "2026-04-01 00:00:00" || o.PaymentTerms != "net" || o.TaxType != "ppn" || o.TaxRate != 11 ||
		o.Subtotal != 123456 || o.GrandTotal != 123456 || o.Status != "draft" || o.Notes != "catatan" ||
		o.SupplierInvoiceNumber != "INV-9" || o.SupplierInvoiceDate != "2026-03-11 00:00:00" || o.PaymentTermDays != 30 {
		t.Fatalf("full header mismatch: %+v", o)
	}
	o, err = r.GetOrder(ctx, bare)
	if err != nil || o == nil {
		t.Fatalf("get bare: %v %v", o, err)
	}
	if o.DueDate != "" || o.Notes != "" || o.SupplierInvoiceNumber != "" || o.SupplierInvoiceDate != "" || o.PaymentTermDays != 0 {
		t.Fatalf("NULL columns must read as zero values: %+v", o)
	}
	if o, err := r.GetOrder(ctx, 999999); err != nil || o != nil {
		t.Fatalf("missing must be nil,nil: %v %v", o, err)
	}

	items, err := r.ItemsByOrder(ctx, full)
	if err != nil || len(items) != 2 {
		t.Fatalf("items: %v %v", items, err)
	}
	a, b := items[0], items[1]
	if a.ID >= b.ID {
		t.Fatalf("lines must be in id order: %d %d", a.ID, b.ID)
	}
	if a.ProductID != 1 || a.VariantID != 2 || a.ProductCode != "P1" || a.ProductName != "Barang" || a.UOM != "box" ||
		a.UOMFactor != 0.333333333333 || a.Qty != 2.5 || a.QtyBase != 0.833333333333 || a.UnitPrice != 1000 ||
		a.DiscountPct != 12.5 || a.DiscountNominal != 125 || a.LineTotal != 2375 || !a.StockTracked ||
		a.ItemType != "barang" || a.VariantName != "Merah" || a.VariantCode != "M" || a.BaseUOM != "pcs" ||
		a.DiscountReason != "promo" || a.Note != "baris" || !a.Taxable || a.TaxBase != 2375 || a.TaxAmount != 261 {
		t.Fatalf("line 1 mismatch: %+v", a)
	}
	// Defaults: empty item type -> barang; false flags -> 0; NULL strings -> "".
	if b.ItemType != "barang" || b.StockTracked || b.Taxable || b.VariantName != "" || b.VariantCode != "" ||
		b.BaseUOM != "" || b.DiscountReason != "" || b.Note != "" || b.UOMFactor != 0 {
		t.Fatalf("line 2 defaults mismatch: %+v", b)
	}
	if got, err := r.ItemsByOrder(ctx, bare); err != nil || len(got) != 0 {
		t.Fatalf("bare has no lines: %v %v", got, err)
	}

	// GetOrderByNumber is branch scoped.
	if o, err := r.GetOrderByNumber(ctx, 1, "PO-FULL"); err != nil || o == nil || o.ID != full {
		t.Fatalf("by number: %v %v", o, err)
	}
	if o, err := r.GetOrderByNumber(ctx, 2, "PO-FULL"); err != nil || o != nil {
		t.Fatalf("other branch must read missing: %v %v", o, err)
	}
	if o, err := r.GetOrderByNumber(ctx, 1, "NOPE"); err != nil || o != nil {
		t.Fatalf("unknown number: %v %v", o, err)
	}

	// UpdateOrder rewrites header + lines (bare gains lines and loses nothing it should keep).
	upd := &contracts.PurchaseOrder{ID: bare, PartyID: 12, OrderDate: "2026-05-01 00:00:00", PaymentTerms: "cod", TaxType: "ppn",
		TaxRate: 11, Subtotal: 100, DiscountTotal: 10, TaxTotal: 9, GrandTotal: 99, Notes: "baru",
		SupplierInvoiceNumber: "S-1", SupplierInvoiceDate: "2026-05-02 00:00:00", PaymentTermDays: 14,
		Items: []*contracts.Item{{ProductID: 9, VariantID: 9, ProductCode: "Z", ProductName: "Z", UOM: "pcs", Qty: 1, QtyBase: 1, UnitPrice: 100, LineTotal: 100}}}
	if err := r.UpdateOrder(ctx, upd, 6); err != nil {
		t.Fatalf("update: %v", err)
	}
	o, _ = r.GetOrder(ctx, bare)
	if o.PartyID != 12 || o.OrderDate != "2026-05-01 00:00:00" || o.PaymentTerms != "cod" || o.Notes != "baru" ||
		o.DiscountTotal != 10 || o.TaxTotal != 9 || o.GrandTotal != 99 || o.SupplierInvoiceNumber != "S-1" ||
		o.PaymentTermDays != 14 || o.Number != "PO-BARE" || o.BranchID != 1 {
		t.Fatalf("update header: %+v", o)
	}
	if items, _ := r.ItemsByOrder(ctx, bare); len(items) != 1 || items[0].ProductCode != "Z" {
		t.Fatalf("update lines: %+v", items)
	}
	// Updating with no items clears lines and NULLs empty strings.
	upd.Items, upd.Notes, upd.SupplierInvoiceNumber, upd.SupplierInvoiceDate, upd.PaymentTermDays, upd.DueDate = nil, "", "", "", 0, ""
	if err := r.UpdateOrder(ctx, upd, 6); err != nil {
		t.Fatal(err)
	}
	o, _ = r.GetOrder(ctx, bare)
	if o.Notes != "" || o.SupplierInvoiceNumber != "" || o.SupplierInvoiceDate != "" || o.PaymentTermDays != 0 || o.DueDate != "" {
		t.Fatalf("emptied fields: %+v", o)
	}
	if items, _ := r.ItemsByOrder(ctx, bare); len(items) != 0 {
		t.Fatalf("lines must be cleared: %+v", items)
	}
	// the other order's lines are untouched
	if items, _ := r.ItemsByOrder(ctx, full); len(items) != 2 {
		t.Fatalf("foreign lines touched: %d", len(items))
	}

	// UpdateHeaderLight leaves party/total/lines alone.
	light := &contracts.PurchaseOrder{ID: full, DueDate: "2026-06-01 00:00:00", Notes: "ringan", SupplierInvoiceNumber: "INV-10",
		SupplierInvoiceDate: "2026-06-02 00:00:00", PaymentTermDays: 45}
	if err := r.UpdateHeaderLight(ctx, light, 7); err != nil {
		t.Fatal(err)
	}
	o, _ = r.GetOrder(ctx, full)
	if o.DueDate != "2026-06-01 00:00:00" || o.Notes != "ringan" || o.SupplierInvoiceNumber != "INV-10" ||
		o.SupplierInvoiceDate != "2026-06-02 00:00:00" || o.PaymentTermDays != 45 || o.PartyID != 10 || o.GrandTotal != 123456 {
		t.Fatalf("light update: %+v", o)
	}
	if items, _ := r.ItemsByOrder(ctx, full); len(items) != 2 {
		t.Fatalf("light update must keep lines: %d", len(items))
	}
	light.Notes, light.SupplierInvoiceNumber, light.SupplierInvoiceDate, light.PaymentTermDays, light.DueDate = "", "", "", 0, ""
	if err := r.UpdateHeaderLight(ctx, light, 7); err != nil {
		t.Fatal(err)
	}
	o, _ = r.GetOrder(ctx, full)
	if o.DueDate != "" || o.Notes != "" || o.SupplierInvoiceNumber != "" || o.PaymentTermDays != 0 {
		t.Fatalf("light update NULLs: %+v", o)
	}

	// Duplicate number is rejected atomically (no orphan lines).
	if _, err := r.CreateOrder(ctx, &contracts.PurchaseOrder{Number: "PO-FULL", BranchID: 1, PartyID: 1, OrderDate: "2026-01-01",
		PaymentTerms: "net", TaxType: "none", Items: []*contracts.Item{{ProductCode: "X", ProductName: "X", UOM: "pcs"}}}, 5); err == nil {
		t.Fatal("duplicate number must fail")
	}
	var n int
	if err := testMods.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM purchase_order_items WHERE product_code = 'X'").Scan(&n); err != nil || n != 0 {
		t.Fatalf("orphan lines after failed create: %d %v", n, err)
	}
}

func TestSqlcPurchasingTransitions(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPurRepo()
	id := purMk(t, r, purSpec{branch: 1, party: 1, number: "PO-T1", terms: "cod", total: 10})

	if ok, err := r.Transition(ctx, id, nil, "confirmed", 1); err != nil || ok {
		t.Fatalf("empty origin list: %v %v", ok, err)
	}
	if ok, err := r.Transition(ctx, id, []string{"confirmed", "completed"}, "cancelled", 1); err != nil || ok {
		t.Fatalf("wrong origin must lose the CAS: %v %v", ok, err)
	}
	if ok, err := r.Transition(ctx, 424242, []string{"draft"}, "confirmed", 1); err != nil || ok {
		t.Fatalf("unknown id: %v %v", ok, err)
	}
	if ok, err := r.Transition(ctx, id, []string{"draft"}, "confirmed", 2); err != nil || !ok {
		t.Fatalf("draft->confirmed: %v %v", ok, err)
	}
	if ok, err := r.Transition(ctx, id, []string{"draft"}, "confirmed", 2); err != nil || ok {
		t.Fatalf("replay must lose: %v %v", ok, err)
	}
	var by int64
	if err := testMods.db.QueryRow("SELECT updated_by FROM purchase_orders WHERE id = ?", id).Scan(&by); err != nil || by != 2 {
		t.Fatalf("updated_by: %v %v", by, err)
	}

	// SwitchToNet: only confirmed cod/prepaid.
	if ok, err := r.SwitchToNet(ctx, id, "2026-09-01 00:00:00", 3); err != nil || !ok {
		t.Fatalf("switch: %v %v", ok, err)
	}
	o, _ := r.GetOrder(ctx, id)
	if o.PaymentTerms != "net" || o.DueDate != "2026-09-01 00:00:00" {
		t.Fatalf("after switch: %+v", o)
	}
	if ok, err := r.SwitchToNet(ctx, id, "2026-10-01 00:00:00", 3); err != nil || ok {
		t.Fatalf("already net: %v %v", ok, err)
	}
	draft := purMk(t, r, purSpec{branch: 1, party: 1, number: "PO-T2", terms: "prepaid", total: 10})
	if ok, err := r.SwitchToNet(ctx, draft, "2026-09-01 00:00:00", 3); err != nil || ok {
		t.Fatalf("draft must not switch: %v %v", ok, err)
	}
	if err := r.SetStatus(ctx, draft, "confirmed", 4); err != nil {
		t.Fatal(err)
	}
	if ok, err := r.SwitchToNet(ctx, draft, "2026-09-01 00:00:00", 3); err != nil || !ok {
		t.Fatalf("confirmed prepaid switches: %v %v", ok, err)
	}
	if err := r.SetStatus(ctx, draft, "completed", 4); err != nil {
		t.Fatal(err)
	}
	o, _ = r.GetOrder(ctx, draft)
	if o.Status != "completed" {
		t.Fatalf("set status: %+v", o)
	}
	// updated_at moves forward on every write.
	var moved int
	if err := testMods.db.QueryRow("SELECT updated_at > created_at FROM purchase_orders WHERE id = ?", draft).Scan(&moved); err != nil || moved != 1 {
		t.Fatalf("updated_at must advance: %v %v", moved, err)
	}
}

func TestSqlcPurchasingListFilters(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPurRepo()
	// branch 1: a..f ; branch 2: other
	a := purMk(t, r, purSpec{branch: 1, party: 10, number: "PO-2026-001", date: "2026-01-05 10:00:00", terms: "net", total: 300})
	b := purMk(t, r, purSpec{branch: 1, party: 10, number: "PO-2026-002", date: "2026-01-31 23:59:59", terms: "cod", total: 100})
	c := purMk(t, r, purSpec{branch: 1, party: 11, number: "PO-2026-003", date: "2026-02-01 00:00:00", terms: "net", total: 200})
	d := purMk(t, r, purSpec{branch: 1, party: 12, number: "INV_%x", date: "2026-02-10 08:00:00", terms: "prepaid", total: 100})
	e := purMk(t, r, purSpec{branch: 1, party: 11, number: "PO-ALPHA BETA", date: "2026-03-01 08:00:00", terms: "net", total: 50})
	f := purMk(t, r, purSpec{branch: 1, party: 10, number: "PO-2026-006", date: "2026-03-02 08:00:00", terms: "net", total: 400})
	other := purMk(t, r, purSpec{branch: 2, party: 10, number: "PO-2026-009", date: "2026-02-02 08:00:00", terms: "net", total: 999})
	for id, st := range map[int64]string{b: "confirmed", c: "confirmed", d: "completed", e: "cancelled"} {
		if err := r.SetStatus(ctx, id, st, 1); err != nil {
			t.Fatal(err)
		}
	}

	type tc struct {
		name string
		f    purinfra.ListFilter
		want []int64 // default order: id DESC
	}
	desc := func(ids ...int64) []int64 {
		out := append([]int64(nil), ids...)
		for i := 0; i < len(out); i++ {
			for j := i + 1; j < len(out); j++ {
				if out[j] > out[i] {
					out[i], out[j] = out[j], out[i]
				}
			}
		}
		return out
	}
	cases := []tc{
		{"no filter (branch scoped)", purinfra.ListFilter{}, desc(a, b, c, d, e, f)},
		{"status single", purinfra.ListFilter{Statuses: []string{"confirmed"}}, desc(b, c)},
		{"status multi", purinfra.ListFilter{Statuses: []string{"confirmed", "completed"}}, desc(b, c, d)},
		{"status none matching", purinfra.ListFilter{Statuses: []string{"nope"}}, nil},
		{"terms", purinfra.ListFilter{PaymentTerms: "net"}, desc(a, c, e, f)},
		{"party", purinfra.ListFilter{PartyID: 11}, desc(c, e)},
		{"date from", purinfra.ListFilter{From: "2026-02-01"}, desc(c, d, e, f)},
		{"date to is inclusive of last day", purinfra.ListFilter{To: "2026-01-31"}, desc(a, b)},
		{"date range", purinfra.ListFilter{From: "2026-01-31", To: "2026-02-01"}, desc(b, c)},
		{"malformed dates ignored", purinfra.ListFilter{From: "31-01-2026", To: "bogus"}, desc(a, b, c, d, e, f)},
		{"search number word", purinfra.ListFilter{Search: "2026-00"}, desc(a, b, c, f)},
		{"search two words AND", purinfra.ListFilter{Search: "alpha beta"}, desc(e)},
		{"search words AND no match", purinfra.ListFilter{Search: "alpha 2026"}, nil},
		{"search wildcard escaped", purinfra.ListFilter{Search: "_%"}, desc(d)},
		{"search party ids OR number", purinfra.ListFilter{Search: "alpha", SearchPartyIDs: []int64{12}}, desc(d, e)},
		{"search party ids with no number hit", purinfra.ListFilter{Search: "zzz", SearchPartyIDs: []int64{10}}, desc(a, b, f)},
		{"empty search party ids = plain like", purinfra.ListFilter{Search: "zzz", SearchPartyIDs: []int64{}}, nil},
		{"combined", purinfra.ListFilter{Statuses: []string{"confirmed"}, PaymentTerms: "net", PartyID: 11, From: "2026-02-01", To: "2026-02-01"}, desc(c)},
	}
	for _, c := range cases {
		got, err := r.List(ctx, 1, c.f, 100, 0)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !sameInts(purIDs(got), c.want) {
			t.Fatalf("%s: got %v want %v", c.name, purIDs(got), c.want)
		}
		n, err := r.Count(ctx, 1, c.f)
		if err != nil || n != int64(len(c.want)) {
			t.Fatalf("%s: count %d (%v) want %d", c.name, n, err, len(c.want))
		}
		// keyset export = same set, id DESC
		ka, err := r.ListAfter(ctx, 1, c.f, 0, 100)
		if err != nil || !sameInts(purIDs(ka), c.want) {
			t.Fatalf("%s: listAfter %v (%v) want %v", c.name, purIDs(ka), err, c.want)
		}
	}
	// other branch isolated
	if got, _ := r.List(ctx, 2, purinfra.ListFilter{}, 10, 0); !sameInts(purIDs(got), []int64{other}) {
		t.Fatalf("branch 2: %v", purIDs(got))
	}
	if n, _ := r.Count(ctx, 3, purinfra.ListFilter{}); n != 0 {
		t.Fatalf("empty branch count: %d", n)
	}

	// Sorting: whitelisted expression + id DESC tiebreak (d and b both total 100).
	got, err := r.List(ctx, 1, purinfra.ListFilter{OrderBy: "grand_total ASC"}, 100, 0)
	if err != nil || !sameInts(purIDs(got), []int64{e, d, b, c, a, f}) {
		t.Fatalf("total asc: %v %v", purIDs(got), err)
	}
	got, _ = r.List(ctx, 1, purinfra.ListFilter{OrderBy: "grand_total DESC"}, 100, 0)
	if !sameInts(purIDs(got), []int64{f, a, c, d, b, e}) {
		t.Fatalf("total desc: %v", purIDs(got))
	}
	got, _ = r.List(ctx, 1, purinfra.ListFilter{OrderBy: "order_date ASC"}, 100, 0)
	if !sameInts(purIDs(got), []int64{a, b, c, d, e, f}) {
		t.Fatalf("date asc: %v", purIDs(got))
	}
	got, _ = r.List(ctx, 1, purinfra.ListFilter{OrderBy: "number ASC"}, 100, 0)
	if !sameInts(purIDs(got), []int64{d, a, b, c, f, e}) {
		t.Fatalf("number asc: %v", purIDs(got))
	}
	got, _ = r.List(ctx, 1, purinfra.ListFilter{OrderBy: "status DESC"}, 100, 0)
	if !sameInts(purIDs(got), []int64{f, a, c, b, d, e}) {
		t.Fatalf("status desc: %v", purIDs(got))
	}

	// Pagination edges.
	all := desc(a, b, c, d, e, f)
	p1, _ := r.List(ctx, 1, purinfra.ListFilter{}, 4, 0)
	p2, _ := r.List(ctx, 1, purinfra.ListFilter{}, 4, 4)
	p3, _ := r.List(ctx, 1, purinfra.ListFilter{}, 4, 8)
	if !sameInts(purIDs(p1), all[:4]) || !sameInts(purIDs(p2), all[4:]) || len(p3) != 0 {
		t.Fatalf("paging: %v %v %v", purIDs(p1), purIDs(p2), purIDs(p3))
	}
	if z, err := r.List(ctx, 1, purinfra.ListFilter{}, 0, 0); err != nil || len(z) != 0 {
		t.Fatalf("limit 0: %v %v", z, err)
	}

	// Keyset: strictly below the cursor; pages walk the whole set exactly once.
	var walked []int64
	cursor := int64(0)
	for i := 0; i < 10; i++ {
		page, err := r.ListAfter(ctx, 1, purinfra.ListFilter{}, cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		walked = append(walked, purIDs(page)...)
		cursor = page[len(page)-1].ID
	}
	if !sameInts(walked, all) {
		t.Fatalf("keyset walk: %v want %v", walked, all)
	}

	// Row content of a list row equals GetOrder.
	lr, _ := r.List(ctx, 1, purinfra.ListFilter{PartyID: 12}, 10, 0)
	one, _ := r.GetOrder(ctx, d)
	if len(lr) != 1 || fmt.Sprintf("%+v", *lr[0]) != fmt.Sprintf("%+v", *one) {
		t.Fatalf("list row != get row:\n%+v\n%+v", lr, one)
	}
}

func TestSqlcPurchasingSummariesAndAggregates(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPurRepo()
	a := purMk(t, r, purSpec{branch: 1, party: 10, number: "S-1", date: "2026-03-01 10:00:00", due: "2026-03-31 00:00:00", terms: "net", total: 100, taxType: "ppn", invNo: "INV-A", invDt: "2026-03-02 00:00:00"})
	b := purMk(t, r, purSpec{branch: 1, party: 10, number: "S-2", date: "2026-03-05 10:00:00", terms: "cod", total: 200, taxType: "ppn"})
	c := purMk(t, r, purSpec{branch: 1, party: 11, number: "S-3", date: "2026-03-10 23:59:59", terms: "net", total: 300, taxType: "ppn", invNo: "INV-C"})
	d := purMk(t, r, purSpec{branch: 1, party: 12, number: "S-4", date: "2026-03-20 00:00:00", terms: "net", total: 400, taxType: "none"})
	e := purMk(t, r, purSpec{branch: 1, party: 10, number: "S-5", date: "2026-04-01 00:00:00", terms: "net", total: 500, taxType: "ppn"})
	x := purMk(t, r, purSpec{branch: 2, party: 10, number: "S-6", date: "2026-03-05 00:00:00", terms: "net", total: 600, taxType: "ppn"})
	for id, st := range map[int64]string{b: "confirmed", c: "confirmed", d: "completed", e: "cancelled", x: "confirmed"} {
		if err := r.SetStatus(ctx, id, st, 1); err != nil {
			t.Fatal(err)
		}
	}

	// StatusCounts
	sc, err := r.StatusCounts(ctx, 1)
	if err != nil || len(sc) != 4 || sc["draft"] != 1 || sc["confirmed"] != 2 || sc["completed"] != 1 || sc["cancelled"] != 1 {
		t.Fatalf("status counts: %v %v", sc, err)
	}
	if sc, err := r.StatusCounts(ctx, 99); err != nil || sc == nil || len(sc) != 0 {
		t.Fatalf("empty branch counts must be a non-nil empty map: %v %v", sc, err)
	}

	// OrderParties: empty, duplicates, unknown ids.
	if m, err := r.OrderParties(ctx, nil); err != nil || m == nil || len(m) != 0 {
		t.Fatalf("nil ids: %v %v", m, err)
	}
	m, err := r.OrderParties(ctx, []int64{a, a, c, 777777})
	if err != nil || len(m) != 2 || m[a] != 10 || m[c] != 11 {
		t.Fatalf("parties: %v %v", m, err)
	}
	big := make([]int64, 0, 3000)
	for i := int64(5000); i < 8000; i++ {
		big = append(big, i)
	}
	big = append(big, d)
	if m, err := r.OrderParties(ctx, big); err != nil || len(m) != 1 || m[d] != 12 {
		t.Fatalf("large list: %v %v", m, err)
	}

	// Supplier invoice gate: taxed, confirmed/completed, blank number.
	// b (confirmed, ppn, no inv) counts; c has INV-C; d is untaxed; a draft; e cancelled; x other branch.
	if n, err := r.CountMissingSupplierInvoice(ctx, 1); err != nil || n != 1 {
		t.Fatalf("missing invoice: %d %v", n, err)
	}
	if n, err := r.CountMissingSupplierInvoice(ctx, 2); err != nil || n != 1 {
		t.Fatalf("missing invoice branch 2: %d %v", n, err)
	}
	if n, err := r.CountMissingSupplierInvoiceBetween(ctx, 1, "2026-03-01 00:00:00", "2026-03-04 23:59:59"); err != nil || n != 0 {
		t.Fatalf("between excludes b: %d %v", n, err)
	}
	if n, err := r.CountMissingSupplierInvoiceBetween(ctx, 1, "2026-03-05 10:00:00", "2026-03-05 10:00:00"); err != nil || n != 1 {
		t.Fatalf("between inclusive bounds: %d %v", n, err)
	}
	// blank (empty-string) invoice number also counts as missing
	if _, err := testMods.db.Exec("UPDATE purchase_orders SET supplier_invoice_number = '' WHERE id = ?", c); err != nil {
		t.Fatal(err)
	}
	if n, _ := r.CountMissingSupplierInvoice(ctx, 1); n != 2 {
		t.Fatalf("empty string invoice counts as missing: %d", n)
	}

	// CountOrders: non-cancelled within a date range, trimmed bounds, whole last day.
	if n, err := r.CountOrders(ctx, 1, " 2026-03-01 ", "2026-03-10"); err != nil || n != 3 {
		t.Fatalf("count orders: %d %v", n, err)
	}
	if n, _ := r.CountOrders(ctx, 1, "2026-04-01", "2026-04-30"); n != 0 {
		t.Fatalf("cancelled excluded: %d", n)
	}
	if n, _ := r.CountOrders(ctx, 1, "2026-03-10", "2026-03-10"); n != 1 {
		t.Fatalf("single day (23:59:59 row): %d", n)
	}

	// OpenOrderTotals: confirmed/completed, id ascending, cursor, limit, date formats.
	ot, err := r.OpenOrderTotals(ctx, 1, 0, 2)
	if err != nil || len(ot) != 2 || ot[0].ID != b || ot[1].ID != c {
		t.Fatalf("open totals p1: %+v %v", ot, err)
	}
	if ot[0].Number != "S-2" || ot[0].PartyID != 10 || ot[0].GrandTotal != 200 || ot[0].OrderDate != "2026-03-05" || ot[0].DueDate != "" {
		t.Fatalf("open totals row: %+v", ot[0])
	}
	ot, _ = r.OpenOrderTotals(ctx, 1, c, 10)
	if len(ot) != 1 || ot[0].ID != d {
		t.Fatalf("open totals p2: %+v", ot)
	}
	if _, err := testMods.db.Exec("UPDATE purchase_orders SET due_date = '2026-07-08 12:00:00' WHERE id = ?", d); err != nil {
		t.Fatal(err)
	}
	ot, _ = r.OpenOrderTotals(ctx, 1, c, 10)
	if ot[0].DueDate != "2026-07-08" {
		t.Fatalf("due date format: %+v", ot[0])
	}
	if ot, _ := r.OpenOrderTotals(ctx, 1, d, 10); len(ot) != 0 {
		t.Fatalf("past the end: %+v", ot)
	}

	// ReportOrders
	rows, total, sum, err := r.ReportOrders(ctx, 1, "", "", "", 10, 0)
	if err != nil || total != 4 || sum != 100+200+300+400 || !sameInts(sumIDs(rows), []int64{d, c, b, a}) {
		t.Fatalf("report all non-cancelled: %v %d %d %v", sumIDs(rows), total, sum, err)
	}
	rows, total, sum, _ = r.ReportOrders(ctx, 1, "cancelled", "", "", 10, 0)
	if total != 1 || sum != 500 || !sameInts(sumIDs(rows), []int64{e}) {
		t.Fatalf("report cancelled: %v %d %d", sumIDs(rows), total, sum)
	}
	rows, total, sum, _ = r.ReportOrders(ctx, 1, "confirmed", "2026-03-06", "2026-03-10", 10, 0)
	if total != 1 || sum != 300 || !sameInts(sumIDs(rows), []int64{c}) {
		t.Fatalf("report confirmed in range: %v %d %d", sumIDs(rows), total, sum)
	}
	rows, total, sum, _ = r.ReportOrders(ctx, 1, "", "2026-03-01", "", 2, 1)
	if total != 4 || sum != 1000 || !sameInts(sumIDs(rows), []int64{c, b}) {
		t.Fatalf("report paging (total/sum cover the filter, not the page): %v %d %d", sumIDs(rows), total, sum)
	}
	rows, total, sum, _ = r.ReportOrders(ctx, 1, "", "", "", 5, 100)
	if total != 4 || sum != 1000 || len(rows) != 0 {
		t.Fatalf("report offset past the end: %v %d %d", sumIDs(rows), total, sum)
	}
	rows, total, sum, _ = r.ReportOrders(ctx, 1, "", "2030-01-01", "", 5, 0)
	if total != 0 || sum != 0 || len(rows) != 0 {
		t.Fatalf("report empty: %v %d %d", rows, total, sum)
	}
	if rows, _, _, _ := r.ReportOrders(ctx, 1, "", "", "", 5, 0); rows[0].PartyID != 12 || rows[0].Number != "S-4" || rows[0].GrandTotal != 400 || rows[0].Status != "completed" || rows[0].OrderDate != "2026-03-20 00:00:00" {
		t.Fatalf("report row content: %+v", rows[0])
	}

	// ListByParty: newest first, branch scoped, empty = nil slice.
	bp, err := r.ListByParty(ctx, 1, 10)
	if err != nil || !sameInts(sumIDs(bp), []int64{e, b, a}) {
		t.Fatalf("by party: %v %v", sumIDs(bp), err)
	}
	if bp[2].DueDate != "2026-03-31 00:00:00" || bp[2].OrderDate != "2026-03-01 10:00:00" || bp[2].Status != "draft" || bp[2].GrandTotal != 100 || bp[2].Number != "S-1" || bp[2].PartyID != 10 {
		t.Fatalf("by party row: %+v", bp[2])
	}
	if bp, err := r.ListByParty(ctx, 1, 999); err != nil || len(bp) != 0 {
		t.Fatalf("by party none: %v %v", bp, err)
	}

	// ListSummaries
	ls, err := r.ListSummaries(ctx, 1, nil, "", "", 0)
	if err != nil || ls == nil || !sameInts(sumIDs(ls), []int64{d, c, b, a}) {
		t.Fatalf("summaries default (non-cancelled, limit 20): %v %v", sumIDs(ls), err)
	}
	ls, _ = r.ListSummaries(ctx, 1, []string{"cancelled", "draft"}, "", "", 10)
	if !sameInts(sumIDs(ls), []int64{e, a}) {
		t.Fatalf("summaries statuses: %v", sumIDs(ls))
	}
	ls, _ = r.ListSummaries(ctx, 1, nil, "2026-03-05", "2026-03-10", 10)
	if !sameInts(sumIDs(ls), []int64{c, b}) {
		t.Fatalf("summaries range: %v", sumIDs(ls))
	}
	ls, _ = r.ListSummaries(ctx, 1, nil, " ", " 2026-03-05 ", 10)
	if !sameInts(sumIDs(ls), []int64{b, a}) {
		t.Fatalf("summaries only-to (trimmed): %v", sumIDs(ls))
	}
	ls, _ = r.ListSummaries(ctx, 1, nil, "", "", 2)
	if !sameInts(sumIDs(ls), []int64{d, c}) {
		t.Fatalf("summaries limit: %v", sumIDs(ls))
	}
	ls, _ = r.ListSummaries(ctx, 1, nil, "", "", 51)
	if len(ls) != 4 {
		t.Fatalf("summaries limit >50 falls back to 20: %v", sumIDs(ls))
	}
	ls, _ = r.ListSummaries(ctx, 1, nil, "2031-01-01", "", 10)
	if ls == nil || len(ls) != 0 {
		t.Fatalf("summaries empty must be non-nil empty slice: %#v", ls)
	}

	// SummariesByIDs: zero ids, duplicates, unknown, chunking over >1000 ids.
	if m, err := r.SummariesByIDs(ctx, nil); err != nil || m == nil || len(m) != 0 {
		t.Fatalf("nil: %v %v", m, err)
	}
	if m, err := r.SummariesByIDs(ctx, []int64{0, 0}); err != nil || len(m) != 0 {
		t.Fatalf("zeros: %v %v", m, err)
	}
	sm, err := r.SummariesByIDs(ctx, []int64{a, a, c, 888888, 0})
	if err != nil || len(sm) != 2 {
		t.Fatalf("summaries by ids: %v %v", sm, err)
	}
	if s := sm[a]; s.Number != "S-1" || s.BranchID != 1 || s.SupplierInvoiceNumber != "INV-A" || s.SupplierInvoiceDate != "2026-03-02 00:00:00" ||
		s.DueDate != "2026-03-31 00:00:00" || s.GrandTotal != 100 || s.PartyID != 10 || s.Status != "draft" {
		t.Fatalf("summary a: %+v", s)
	}
	if s := sm[c]; s.SupplierInvoiceNumber != "" || s.SupplierInvoiceDate != "" || s.DueDate != "" {
		t.Fatalf("summary c: %+v", s)
	}
	many := make([]int64, 0, 2500)
	for i := int64(10000); i < 12400; i++ {
		many = append(many, i)
	}
	many = append(many, d, x) // one real id lands in the last chunk
	many = append([]int64{a}, many...)
	sm, err = r.SummariesByIDs(ctx, many)
	if err != nil || len(sm) != 3 || sm[a] == nil || sm[d] == nil || sm[x] == nil || sm[x].BranchID != 2 {
		t.Fatalf("chunked summaries: %d %v", len(sm), err)
	}
}
