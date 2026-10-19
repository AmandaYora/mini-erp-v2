package integration

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	purcontracts "mini-erp/internal/modules/purchasing/contracts"
	purinfra "mini-erp/internal/modules/purchasing/infrastructure"
	"mini-erp/internal/modules/sales/contracts"
	salesinfra "mini-erp/internal/modules/sales/infrastructure"
)

// Characterization tests for the sales Repository (F8-18). The purchasing twin
// has the same shape; TestSqlcSalesPurchasingParity runs one scenario over both
// and demands identical observable behaviour.

func sqlcSalesRepo() *salesinfra.Repository { return salesinfra.NewRepository(testMods.db) }

type salSpec struct {
	branch, party int64
	number, date  string
	due, terms    string
	channel       string
	taxType       string
	total         int64
	taxInvNo      string
	taxInvDt      string
	clientRef     string
	member        string
	notes         string
	items         []*contracts.Item
}

func salMk(t *testing.T, r *salesinfra.Repository, s salSpec) int64 {
	t.Helper()
	if s.terms == "" {
		s.terms = "cod"
	}
	if s.taxType == "" {
		s.taxType = "none"
	}
	if s.channel == "" {
		s.channel = "regular"
	}
	if s.date == "" {
		s.date = "2026-03-10 09:30:00"
	}
	id, err := r.CreateOrder(context.Background(), &contracts.SalesOrder{
		Number: s.number, BranchID: s.branch, PartyID: s.party, Channel: s.channel, MemberCode: s.member,
		OrderDate: s.date, DueDate: s.due, PaymentTerms: s.terms, TaxType: s.taxType, TaxRate: 11,
		Subtotal: s.total, GrandTotal: s.total, Notes: s.notes,
		TaxInvoiceNumber: s.taxInvNo, TaxInvoiceDate: s.taxInvDt, ClientRef: s.clientRef, Items: s.items,
	}, 5)
	if err != nil {
		t.Fatalf("create %s: %v", s.number, err)
	}
	return id
}

func salIDs(os []*contracts.SalesOrder) []int64 {
	out := make([]int64, 0, len(os))
	for _, o := range os {
		out = append(out, o.ID)
	}
	return out
}

func salSumIDs(os []*contracts.OrderSummary) []int64 {
	out := make([]int64, 0, len(os))
	for _, o := range os {
		out = append(out, o.ID)
	}
	return out
}

func descIDs(ids ...int64) []int64 {
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

func TestSqlcSalesCRUDAndNulls(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcSalesRepo()

	full := salMk(t, r, salSpec{branch: 1, party: 10, number: "SO-FULL", due: "2026-04-01 00:00:00", terms: "net", channel: "pos",
		member: "MBR-1", taxType: "ppn", total: 123456, notes: "catatan", taxInvNo: "FP-1", taxInvDt: "2026-03-11 00:00:00", clientRef: "ref-1",
		items: []*contracts.Item{{
			ProductID: 1, VariantID: 2, ProductCode: "P1", ProductName: "Barang", UOM: "box", UOMFactor: 0.333333333333,
			Qty: 2.5, QtyBase: 0.833333333333, UnitPrice: 1000, DiscountPct: 12.5, DiscountNominal: 125, LineTotal: 2375,
			StockTracked: true, ItemType: "barang", VariantName: "Merah", VariantCode: "M", BaseUOM: "pcs",
			PricingSource: "manual", BasePrice: 1500, DiscountReason: "promo", Note: "baris", Taxable: true, TaxBase: 2375, TaxAmount: 261,
		}, {
			ProductID: 3, VariantID: 4, ProductCode: "J1", ProductName: "Jasa", UOM: "jam", Qty: 1, QtyBase: 1, UnitPrice: 500, LineTotal: 500,
		}}})
	bare := salMk(t, r, salSpec{branch: 1, party: 11, number: "SO-BARE", total: 7})

	o, err := r.GetOrder(ctx, full)
	if err != nil || o == nil {
		t.Fatalf("get full: %v %v", o, err)
	}
	if o.Number != "SO-FULL" || o.BranchID != 1 || o.PartyID != 10 || o.Channel != "pos" || o.MemberCode != "MBR-1" ||
		o.OrderDate != "2026-03-10 09:30:00" || o.DueDate != "2026-04-01 00:00:00" || o.PaymentTerms != "net" ||
		o.TaxType != "ppn" || o.TaxRate != 11 || o.Subtotal != 123456 || o.GrandTotal != 123456 || o.Status != "draft" ||
		o.Notes != "catatan" || o.TaxInvoiceNumber != "FP-1" || o.TaxInvoiceDate != "2026-03-11 00:00:00" || o.ClientRef != "ref-1" ||
		o.ShipToAddressID != 0 || o.ShipToLabel != "" {
		t.Fatalf("full header mismatch: %+v", o)
	}
	o, _ = r.GetOrder(ctx, bare)
	if o.DueDate != "" || o.Notes != "" || o.MemberCode != "" || o.TaxInvoiceNumber != "" || o.TaxInvoiceDate != "" || o.ClientRef != "" ||
		o.ShipToAddressID != 0 || o.ShipToRecipient != "" || o.ShipToPhone != "" || o.ShipToAddress != "" || o.Channel != "regular" {
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
		t.Fatalf("lines must be in id order")
	}
	if a.ProductID != 1 || a.VariantID != 2 || a.ProductCode != "P1" || a.UOM != "box" || a.UOMFactor != 0.333333333333 ||
		a.Qty != 2.5 || a.QtyBase != 0.833333333333 || a.UnitPrice != 1000 || a.DiscountPct != 12.5 || a.DiscountNominal != 125 ||
		a.LineTotal != 2375 || !a.StockTracked || a.ItemType != "barang" || a.VariantName != "Merah" || a.VariantCode != "M" ||
		a.BaseUOM != "pcs" || a.PricingSource != "manual" || a.BasePrice != 1500 || a.DiscountReason != "promo" || a.Note != "baris" ||
		!a.Taxable || a.TaxBase != 2375 || a.TaxAmount != 261 {
		t.Fatalf("line 1 mismatch: %+v", a)
	}
	if b.ItemType != "barang" || b.PricingSource != "catalog" || b.BasePrice != 0 || b.StockTracked || b.Taxable ||
		b.VariantName != "" || b.BaseUOM != "" || b.Note != "" || b.DiscountReason != "" {
		t.Fatalf("line 2 defaults mismatch: %+v", b)
	}
	if got, err := r.ItemsByOrder(ctx, bare); err != nil || len(got) != 0 {
		t.Fatalf("no lines: %v %v", got, err)
	}

	// Ship-to snapshot round trip through UpdateOrder.
	upd := &contracts.SalesOrder{ID: bare, PartyID: 12, Channel: "regular", MemberCode: "M2", OrderDate: "2026-05-01 00:00:00",
		DueDate: "2026-05-30 00:00:00", PaymentTerms: "net", TaxType: "ppn", TaxRate: 11, Subtotal: 100, DiscountTotal: 10, TaxTotal: 9,
		GrandTotal: 99, Notes: "baru", ShipToAddressID: 77, ShipToLabel: "Rumah", ShipToRecipient: "Budi", ShipToPhone: "0812",
		ShipToAddress: "Jl. A", TaxInvoiceNumber: "FP-2", TaxInvoiceDate: "2026-05-02 00:00:00",
		Items: []*contracts.Item{{ProductID: 9, VariantID: 9, ProductCode: "Z", ProductName: "Z", UOM: "pcs", Qty: 1, QtyBase: 1, UnitPrice: 100, LineTotal: 100, BasePrice: 120}}}
	if err := r.UpdateOrder(ctx, upd, 6); err != nil {
		t.Fatal(err)
	}
	o, _ = r.GetOrder(ctx, bare)
	if o.PartyID != 12 || o.MemberCode != "M2" || o.DueDate != "2026-05-30 00:00:00" || o.PaymentTerms != "net" || o.Notes != "baru" ||
		o.ShipToAddressID != 77 || o.ShipToLabel != "Rumah" || o.ShipToRecipient != "Budi" || o.ShipToPhone != "0812" || o.ShipToAddress != "Jl. A" ||
		o.TaxInvoiceNumber != "FP-2" || o.TaxInvoiceDate != "2026-05-02 00:00:00" || o.Number != "SO-BARE" || o.ClientRef != "" {
		t.Fatalf("update header: %+v", o)
	}
	if items, _ := r.ItemsByOrder(ctx, bare); len(items) != 1 || items[0].BasePrice != 120 {
		t.Fatalf("update lines: %+v", items)
	}
	upd.Items, upd.MemberCode, upd.DueDate, upd.Notes, upd.ShipToAddressID, upd.ShipToLabel, upd.ShipToRecipient, upd.ShipToPhone,
		upd.ShipToAddress, upd.TaxInvoiceNumber, upd.TaxInvoiceDate = nil, "", "", "", 0, "", "", "", "", "", ""
	if err := r.UpdateOrder(ctx, upd, 6); err != nil {
		t.Fatal(err)
	}
	o, _ = r.GetOrder(ctx, bare)
	if o.MemberCode != "" || o.DueDate != "" || o.Notes != "" || o.ShipToAddressID != 0 || o.ShipToLabel != "" || o.TaxInvoiceNumber != "" || o.TaxInvoiceDate != "" {
		t.Fatalf("emptied fields: %+v", o)
	}
	if items, _ := r.ItemsByOrder(ctx, bare); len(items) != 0 {
		t.Fatalf("lines cleared: %+v", items)
	}
	if items, _ := r.ItemsByOrder(ctx, full); len(items) != 2 {
		t.Fatalf("foreign lines touched")
	}

	// UpdateHeaderLight: header snapshot only, never party/totals/lines/channel/terms.
	light := &contracts.SalesOrder{ID: full, DueDate: "2026-06-01 00:00:00", Notes: "ringan", ShipToAddressID: 5, ShipToLabel: "L",
		ShipToRecipient: "R", ShipToPhone: "P", ShipToAddress: "A", TaxInvoiceNumber: "FP-9", TaxInvoiceDate: "2026-06-02 00:00:00"}
	if err := r.UpdateHeaderLight(ctx, light, 7); err != nil {
		t.Fatal(err)
	}
	o, _ = r.GetOrder(ctx, full)
	if o.DueDate != "2026-06-01 00:00:00" || o.Notes != "ringan" || o.ShipToAddressID != 5 || o.ShipToLabel != "L" ||
		o.TaxInvoiceNumber != "FP-9" || o.PartyID != 10 || o.GrandTotal != 123456 || o.Channel != "pos" || o.PaymentTerms != "net" || o.MemberCode != "MBR-1" {
		t.Fatalf("light update: %+v", o)
	}
	if items, _ := r.ItemsByOrder(ctx, full); len(items) != 2 {
		t.Fatal("light update must keep lines")
	}
	if err := r.UpdateHeaderLight(ctx, &contracts.SalesOrder{ID: full}, 7); err != nil {
		t.Fatal(err)
	}
	o, _ = r.GetOrder(ctx, full)
	if o.DueDate != "" || o.Notes != "" || o.ShipToAddressID != 0 || o.TaxInvoiceNumber != "" || o.TaxInvoiceDate != "" {
		t.Fatalf("light update NULLs: %+v", o)
	}

	// Branch scoped lookups.
	if o, err := r.GetOrderByNumber(ctx, 1, "SO-FULL"); err != nil || o == nil || o.ID != full {
		t.Fatalf("by number: %v %v", o, err)
	}
	if o, err := r.GetOrderByNumber(ctx, 2, "SO-FULL"); err != nil || o != nil {
		t.Fatalf("other branch: %v %v", o, err)
	}
	if o, err := r.OrderByClientRef(ctx, 1, "ref-1"); err != nil || o == nil || o.ID != full {
		t.Fatalf("by client ref: %v %v", o, err)
	}
	if o, err := r.OrderByClientRef(ctx, 2, "ref-1"); err != nil || o != nil {
		t.Fatalf("client ref other branch: %v %v", o, err)
	}
	if o, err := r.OrderByClientRef(ctx, 1, "nope"); err != nil || o != nil {
		t.Fatalf("client ref unknown: %v %v", o, err)
	}
	// client_ref is unique per branch; same ref on another branch is fine, duplicate on the same one is not.
	salMk(t, r, salSpec{branch: 2, party: 1, number: "SO-B2", clientRef: "ref-1", total: 1})
	if _, err := r.CreateOrder(ctx, &contracts.SalesOrder{Number: "SO-DUP", BranchID: 1, PartyID: 1, Channel: "regular", OrderDate: "2026-01-01",
		PaymentTerms: "cod", TaxType: "none", ClientRef: "ref-1", Items: []*contracts.Item{{ProductCode: "X", ProductName: "X", UOM: "pcs"}}}, 5); err == nil {
		t.Fatal("duplicate client ref must fail")
	}
	var n int
	if err := testMods.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sales_order_items WHERE product_code = 'X'").Scan(&n); err != nil || n != 0 {
		t.Fatalf("orphan lines after failed create: %d %v", n, err)
	}
	// blank client ref is NULL: many allowed.
	salMk(t, r, salSpec{branch: 1, party: 1, number: "SO-N1", total: 1})
	salMk(t, r, salSpec{branch: 1, party: 1, number: "SO-N2", total: 1})
}

func TestSqlcSalesTransitions(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcSalesRepo()
	id := salMk(t, r, salSpec{branch: 1, party: 1, number: "SO-T1", terms: "cod", total: 10})
	completedAt := func(id int64) (sqlNull bool) {
		var v *string
		if err := testMods.db.QueryRow("SELECT CAST(completed_at AS CHAR) FROM sales_orders WHERE id = ?", id).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v == nil
	}

	if ok, err := r.Transition(ctx, id, nil, "confirmed", 1); err != nil || ok {
		t.Fatalf("empty origin: %v %v", ok, err)
	}
	if ok, err := r.Transition(ctx, id, []string{"confirmed"}, "completed", 1); err != nil || ok {
		t.Fatalf("wrong origin: %v %v", ok, err)
	}
	if !completedAt(id) {
		t.Fatal("a lost CAS must not stamp completed_at")
	}
	if ok, err := r.Transition(ctx, id, []string{"draft", "confirmed"}, "confirmed", 2); err != nil || !ok {
		t.Fatalf("draft->confirmed: %v %v", ok, err)
	}
	if !completedAt(id) {
		t.Fatal("confirmed must not stamp completed_at")
	}
	if ok, err := r.Transition(ctx, id, []string{"draft"}, "confirmed", 2); err != nil || ok {
		t.Fatalf("replay: %v %v", ok, err)
	}
	if ok, err := r.Transition(ctx, id, []string{"confirmed"}, "completed", 3); err != nil || !ok {
		t.Fatalf("confirmed->completed: %v %v", ok, err)
	}
	if completedAt(id) {
		t.Fatal("completed must stamp completed_at")
	}
	var by int64
	_ = testMods.db.QueryRow("SELECT updated_by FROM sales_orders WHERE id = ?", id).Scan(&by)
	if by != 3 {
		t.Fatalf("updated_by %d", by)
	}
	// completed_at is kept when the status moves on.
	var before string
	_ = testMods.db.QueryRow("SELECT CAST(completed_at AS CHAR(26)) FROM sales_orders WHERE id = ?", id).Scan(&before)
	if err := r.SetStatus(ctx, id, "cancelled", 4); err != nil {
		t.Fatal(err)
	}
	var after string
	_ = testMods.db.QueryRow("SELECT CAST(completed_at AS CHAR(26)) FROM sales_orders WHERE id = ?", id).Scan(&after)
	if before != after || before == "" {
		t.Fatalf("completed_at must survive a later status: %q %q", before, after)
	}
	// SetStatus(completed) stamps too.
	id2 := salMk(t, r, salSpec{branch: 1, party: 1, number: "SO-T2", total: 10})
	if err := r.SetStatus(ctx, id2, "completed", 4); err != nil {
		t.Fatal(err)
	}
	if completedAt(id2) {
		t.Fatal("SetStatus(completed) must stamp completed_at")
	}
	id3 := salMk(t, r, salSpec{branch: 1, party: 1, number: "SO-T3", total: 10})
	if err := r.SetStatus(ctx, id3, "confirmed", 4); err != nil {
		t.Fatal(err)
	}
	if !completedAt(id3) {
		t.Fatal("SetStatus(confirmed) must not stamp completed_at")
	}

	// SwitchToNet / SetTerms.
	if ok, err := r.SwitchToNet(ctx, id3, "2026-09-01 00:00:00", 3); err != nil || !ok {
		t.Fatalf("switch: %v %v", ok, err)
	}
	o, _ := r.GetOrder(ctx, id3)
	if o.PaymentTerms != "net" || o.DueDate != "2026-09-01 00:00:00" {
		t.Fatalf("after switch: %+v", o)
	}
	if ok, err := r.SwitchToNet(ctx, id3, "2026-10-01 00:00:00", 3); err != nil || ok {
		t.Fatalf("already net: %v %v", ok, err)
	}
	draft := salMk(t, r, salSpec{branch: 1, party: 1, number: "SO-T4", terms: "prepaid", total: 1})
	if ok, err := r.SwitchToNet(ctx, draft, "2026-09-01 00:00:00", 3); err != nil || ok {
		t.Fatalf("draft must not switch: %v %v", ok, err)
	}
	if err := r.SetTerms(ctx, draft, "prepaid", "2026-12-01 00:00:00", 8); err != nil {
		t.Fatal(err)
	}
	o, _ = r.GetOrder(ctx, draft)
	if o.PaymentTerms != "prepaid" || o.DueDate != "2026-12-01 00:00:00" {
		t.Fatalf("set terms: %+v", o)
	}
	var moved int
	if err := testMods.db.QueryRow("SELECT updated_at > created_at FROM sales_orders WHERE id = ?", draft).Scan(&moved); err != nil || moved != 1 {
		t.Fatalf("updated_at must advance: %v %v", moved, err)
	}
}

func TestSqlcSalesListFilters(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcSalesRepo()
	a := salMk(t, r, salSpec{branch: 1, party: 10, number: "SO-2026-001", date: "2026-01-05 10:00:00", terms: "net", channel: "regular", total: 300})
	b := salMk(t, r, salSpec{branch: 1, party: 10, number: "SO-2026-002", date: "2026-01-31 23:59:59", terms: "cod", channel: "pos", total: 100})
	c := salMk(t, r, salSpec{branch: 1, party: 11, number: "SO-2026-003", date: "2026-02-01 00:00:00", terms: "net", channel: "regular", total: 200})
	d := salMk(t, r, salSpec{branch: 1, party: 12, number: "INV_%x", date: "2026-02-10 08:00:00", terms: "prepaid", channel: "pos", total: 100})
	e := salMk(t, r, salSpec{branch: 1, party: 11, number: "SO-ALPHA BETA", date: "2026-03-01 08:00:00", terms: "net", channel: "regular", total: 50})
	f := salMk(t, r, salSpec{branch: 1, party: 10, number: "SO-2026-006", date: "2026-03-02 08:00:00", terms: "net", channel: "regular", total: 400})
	other := salMk(t, r, salSpec{branch: 2, party: 10, number: "SO-2026-009", date: "2026-02-02 08:00:00", terms: "net", total: 999})
	for id, st := range map[int64]string{b: "confirmed", c: "confirmed", d: "completed", e: "cancelled"} {
		if err := r.SetStatus(ctx, id, st, 1); err != nil {
			t.Fatal(err)
		}
	}
	type tc struct {
		name string
		f    salesinfra.ListFilter
		want []int64
	}
	cases := []tc{
		{"no filter (branch scoped)", salesinfra.ListFilter{}, descIDs(a, b, c, d, e, f)},
		{"status single", salesinfra.ListFilter{Statuses: []string{"confirmed"}}, descIDs(b, c)},
		{"status multi", salesinfra.ListFilter{Statuses: []string{"confirmed", "completed"}}, descIDs(b, c, d)},
		{"status none", salesinfra.ListFilter{Statuses: []string{"nope"}}, nil},
		{"channel", salesinfra.ListFilter{Channel: "pos"}, descIDs(b, d)},
		{"terms", salesinfra.ListFilter{PaymentTerms: "net"}, descIDs(a, c, e, f)},
		{"party", salesinfra.ListFilter{PartyID: 11}, descIDs(c, e)},
		{"date from", salesinfra.ListFilter{From: "2026-02-01"}, descIDs(c, d, e, f)},
		{"date to inclusive of last day", salesinfra.ListFilter{To: "2026-01-31"}, descIDs(a, b)},
		{"date range", salesinfra.ListFilter{From: "2026-01-31", To: "2026-02-01"}, descIDs(b, c)},
		{"malformed dates ignored", salesinfra.ListFilter{From: "31-01-2026", To: "bogus"}, descIDs(a, b, c, d, e, f)},
		{"search number word", salesinfra.ListFilter{Search: "2026-00"}, descIDs(a, b, c, f)},
		{"search two words AND", salesinfra.ListFilter{Search: "alpha beta"}, descIDs(e)},
		{"search words AND no match", salesinfra.ListFilter{Search: "alpha 2026"}, nil},
		{"search wildcard escaped", salesinfra.ListFilter{Search: "_%"}, descIDs(d)},
		{"search party ids OR number", salesinfra.ListFilter{Search: "alpha", SearchPartyIDs: []int64{12}}, descIDs(d, e)},
		{"search party ids no number hit", salesinfra.ListFilter{Search: "zzz", SearchPartyIDs: []int64{10}}, descIDs(a, b, f)},
		{"empty search party ids = plain like", salesinfra.ListFilter{Search: "zzz", SearchPartyIDs: []int64{}}, nil},
		{"combined", salesinfra.ListFilter{Statuses: []string{"confirmed"}, PaymentTerms: "net", PartyID: 11, Channel: "regular", From: "2026-02-01", To: "2026-02-01"}, descIDs(c)},
	}
	for _, c := range cases {
		got, err := r.List(ctx, 1, c.f, 100, 0)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !sameInts(salIDs(got), c.want) {
			t.Fatalf("%s: got %v want %v", c.name, salIDs(got), c.want)
		}
		n, err := r.Count(ctx, 1, c.f)
		if err != nil || n != int64(len(c.want)) {
			t.Fatalf("%s: count %d (%v) want %d", c.name, n, err, len(c.want))
		}
		ka, err := r.ListAfter(ctx, 1, c.f, 0, 100)
		if err != nil || !sameInts(salIDs(ka), c.want) {
			t.Fatalf("%s: listAfter %v (%v) want %v", c.name, salIDs(ka), err, c.want)
		}
	}
	if got, _ := r.List(ctx, 2, salesinfra.ListFilter{}, 10, 0); !sameInts(salIDs(got), []int64{other}) {
		t.Fatalf("branch 2: %v", salIDs(got))
	}
	if n, _ := r.Count(ctx, 3, salesinfra.ListFilter{}); n != 0 {
		t.Fatalf("empty branch count: %d", n)
	}

	sorts := []struct {
		order string
		want  []int64
	}{
		{"grand_total ASC", []int64{e, d, b, c, a, f}},
		{"grand_total DESC", []int64{f, a, c, d, b, e}},
		{"order_date ASC", []int64{a, b, c, d, e, f}},
		{"number ASC", []int64{d, a, b, c, f, e}},
		{"status DESC", []int64{f, a, c, b, d, e}},
	}
	for _, s := range sorts {
		got, err := r.List(ctx, 1, salesinfra.ListFilter{OrderBy: s.order}, 100, 0)
		if err != nil || !sameInts(salIDs(got), s.want) {
			t.Fatalf("sort %q: %v (%v) want %v", s.order, salIDs(got), err, s.want)
		}
	}

	all := descIDs(a, b, c, d, e, f)
	p1, _ := r.List(ctx, 1, salesinfra.ListFilter{}, 4, 0)
	p2, _ := r.List(ctx, 1, salesinfra.ListFilter{}, 4, 4)
	p3, _ := r.List(ctx, 1, salesinfra.ListFilter{}, 4, 8)
	if !sameInts(salIDs(p1), all[:4]) || !sameInts(salIDs(p2), all[4:]) || len(p3) != 0 {
		t.Fatalf("paging: %v %v %v", salIDs(p1), salIDs(p2), salIDs(p3))
	}
	if z, err := r.List(ctx, 1, salesinfra.ListFilter{}, 0, 0); err != nil || len(z) != 0 {
		t.Fatalf("limit 0: %v %v", z, err)
	}
	var walked []int64
	cursor := int64(0)
	for i := 0; i < 10; i++ {
		page, err := r.ListAfter(ctx, 1, salesinfra.ListFilter{}, cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		walked = append(walked, salIDs(page)...)
		cursor = page[len(page)-1].ID
	}
	if !sameInts(walked, all) {
		t.Fatalf("keyset walk: %v want %v", walked, all)
	}
	lr, _ := r.List(ctx, 1, salesinfra.ListFilter{PartyID: 12}, 10, 0)
	one, _ := r.GetOrder(ctx, d)
	if len(lr) != 1 || fmt.Sprintf("%+v", *lr[0]) != fmt.Sprintf("%+v", *one) {
		t.Fatalf("list row != get row:\n%+v\n%+v", lr, one)
	}
}

func TestSqlcSalesSummariesAndAggregates(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcSalesRepo()
	a := salMk(t, r, salSpec{branch: 1, party: 10, number: "S-1", date: "2026-03-01 10:00:00", due: "2026-03-31 00:00:00", terms: "net", total: 100, taxType: "ppn", taxInvNo: "FP-A", taxInvDt: "2026-03-02 00:00:00"})
	b := salMk(t, r, salSpec{branch: 1, party: 10, number: "S-2", date: "2026-03-05 10:00:00", terms: "cod", total: 200, taxType: "ppn"})
	c := salMk(t, r, salSpec{branch: 1, party: 11, number: "S-3", date: "2026-03-10 23:59:59", terms: "net", total: 300, taxType: "ppn", taxInvNo: "FP-C"})
	d := salMk(t, r, salSpec{branch: 1, party: 12, number: "S-4", date: "2026-03-20 00:00:00", terms: "net", total: 400, taxType: "none"})
	e := salMk(t, r, salSpec{branch: 1, party: 10, number: "S-5", date: "2026-04-01 00:00:00", terms: "net", total: 500, taxType: "ppn"})
	x := salMk(t, r, salSpec{branch: 2, party: 10, number: "S-6", date: "2026-03-05 00:00:00", terms: "net", total: 600, taxType: "ppn"})
	for id, st := range map[int64]string{b: "confirmed", c: "confirmed", d: "completed", e: "cancelled", x: "confirmed"} {
		if err := r.SetStatus(ctx, id, st, 1); err != nil {
			t.Fatal(err)
		}
	}

	sc, err := r.StatusCounts(ctx, 1)
	if err != nil || len(sc) != 4 || sc["draft"] != 1 || sc["confirmed"] != 2 || sc["completed"] != 1 || sc["cancelled"] != 1 {
		t.Fatalf("status counts: %v %v", sc, err)
	}
	if sc, err := r.StatusCounts(ctx, 99); err != nil || sc == nil || len(sc) != 0 {
		t.Fatalf("empty: %v %v", sc, err)
	}
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

	if n, err := r.CountMissingTaxInvoice(ctx, 1); err != nil || n != 1 {
		t.Fatalf("missing tax invoice: %d %v", n, err)
	}
	if n, _ := r.CountMissingTaxInvoice(ctx, 2); n != 1 {
		t.Fatalf("missing tax invoice b2: %d", n)
	}
	if n, err := r.CountMissingTaxInvoiceBetween(ctx, 1, "2026-03-01 00:00:00", "2026-03-04 23:59:59"); err != nil || n != 0 {
		t.Fatalf("between excludes b: %d %v", n, err)
	}
	if n, _ := r.CountMissingTaxInvoiceBetween(ctx, 1, "2026-03-05 10:00:00", "2026-03-05 10:00:00"); n != 1 {
		t.Fatalf("between inclusive bounds: %d", n)
	}
	if _, err := testMods.db.Exec("UPDATE sales_orders SET tax_invoice_number = '' WHERE id = ?", c); err != nil {
		t.Fatal(err)
	}
	if n, _ := r.CountMissingTaxInvoice(ctx, 1); n != 2 {
		t.Fatalf("empty string counts as missing: %d", n)
	}

	if n, err := r.CountOrders(ctx, 1, " 2026-03-01 ", "2026-03-10"); err != nil || n != 3 {
		t.Fatalf("count orders: %d %v", n, err)
	}
	if n, _ := r.CountOrders(ctx, 1, "2026-04-01", "2026-04-30"); n != 0 {
		t.Fatalf("cancelled excluded: %d", n)
	}
	if n, _ := r.CountOrders(ctx, 1, "2026-03-10", "2026-03-10"); n != 1 {
		t.Fatalf("single day: %d", n)
	}

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
	_, _ = testMods.db.Exec("UPDATE sales_orders SET due_date = '2026-07-08 12:00:00' WHERE id = ?", d)
	ot, _ = r.OpenOrderTotals(ctx, 1, c, 10)
	if ot[0].DueDate != "2026-07-08" {
		t.Fatalf("due format: %+v", ot[0])
	}
	if ot, _ := r.OpenOrderTotals(ctx, 1, d, 10); len(ot) != 0 {
		t.Fatalf("past the end: %+v", ot)
	}

	rows, total, sum, err := r.ReportOrders(ctx, 1, "", "", "", 10, 0)
	if err != nil || total != 4 || sum != 1000 || !sameInts(salSumIDs(rows), []int64{d, c, b, a}) {
		t.Fatalf("report all: %v %d %d %v", salSumIDs(rows), total, sum, err)
	}
	rows, total, sum, _ = r.ReportOrders(ctx, 1, "cancelled", "", "", 10, 0)
	if total != 1 || sum != 500 || !sameInts(salSumIDs(rows), []int64{e}) {
		t.Fatalf("report cancelled: %v %d %d", salSumIDs(rows), total, sum)
	}
	rows, total, sum, _ = r.ReportOrders(ctx, 1, "confirmed", "2026-03-06", "2026-03-10", 10, 0)
	if total != 1 || sum != 300 || !sameInts(salSumIDs(rows), []int64{c}) {
		t.Fatalf("report confirmed range: %v %d %d", salSumIDs(rows), total, sum)
	}
	rows, total, sum, _ = r.ReportOrders(ctx, 1, "", "2026-03-01", "", 2, 1)
	if total != 4 || sum != 1000 || !sameInts(salSumIDs(rows), []int64{c, b}) {
		t.Fatalf("report paging: %v %d %d", salSumIDs(rows), total, sum)
	}
	rows, total, sum, _ = r.ReportOrders(ctx, 1, "", "", "", 5, 100)
	if total != 4 || sum != 1000 || len(rows) != 0 {
		t.Fatalf("report offset past end: %v %d %d", salSumIDs(rows), total, sum)
	}
	rows, total, sum, _ = r.ReportOrders(ctx, 1, "", "2030-01-01", "", 5, 0)
	if total != 0 || sum != 0 || len(rows) != 0 {
		t.Fatalf("report empty: %v %d %d", rows, total, sum)
	}
	if rows, _, _, _ := r.ReportOrders(ctx, 1, "", "", "", 5, 0); rows[0].PartyID != 12 || rows[0].Number != "S-4" || rows[0].GrandTotal != 400 || rows[0].Status != "completed" || rows[0].OrderDate != "2026-03-20 00:00:00" {
		t.Fatalf("report row: %+v", rows[0])
	}

	bp, err := r.ListByParty(ctx, 1, 10)
	if err != nil || !sameInts(salSumIDs(bp), []int64{e, b, a}) {
		t.Fatalf("by party: %v %v", salSumIDs(bp), err)
	}
	if bp[2].DueDate != "2026-03-31 00:00:00" || bp[2].OrderDate != "2026-03-01 10:00:00" || bp[2].Status != "draft" || bp[2].GrandTotal != 100 {
		t.Fatalf("by party row: %+v", bp[2])
	}
	if bp, err := r.ListByParty(ctx, 1, 999); err != nil || len(bp) != 0 {
		t.Fatalf("by party none: %v %v", bp, err)
	}

	ls, err := r.ListSummaries(ctx, 1, nil, "", "", 0)
	if err != nil || ls == nil || !sameInts(salSumIDs(ls), []int64{d, c, b, a}) {
		t.Fatalf("summaries default: %v %v", salSumIDs(ls), err)
	}
	ls, _ = r.ListSummaries(ctx, 1, []string{"cancelled", "draft"}, "", "", 10)
	if !sameInts(salSumIDs(ls), []int64{e, a}) {
		t.Fatalf("summaries statuses: %v", salSumIDs(ls))
	}
	ls, _ = r.ListSummaries(ctx, 1, nil, "2026-03-05", "2026-03-10", 10)
	if !sameInts(salSumIDs(ls), []int64{c, b}) {
		t.Fatalf("summaries range: %v", salSumIDs(ls))
	}
	ls, _ = r.ListSummaries(ctx, 1, nil, " ", " 2026-03-05 ", 10)
	if !sameInts(salSumIDs(ls), []int64{b, a}) {
		t.Fatalf("summaries only-to: %v", salSumIDs(ls))
	}
	ls, _ = r.ListSummaries(ctx, 1, nil, "", "", 2)
	if !sameInts(salSumIDs(ls), []int64{d, c}) {
		t.Fatalf("summaries limit: %v", salSumIDs(ls))
	}
	ls, _ = r.ListSummaries(ctx, 1, nil, "", "", 51)
	if len(ls) != 4 {
		t.Fatalf("summaries limit>50: %v", salSumIDs(ls))
	}
	ls, _ = r.ListSummaries(ctx, 1, nil, "2031-01-01", "", 10)
	if ls == nil || len(ls) != 0 {
		t.Fatalf("summaries empty must be non-nil: %#v", ls)
	}

	if m, err := r.SummariesByIDs(ctx, nil); err != nil || m == nil || len(m) != 0 {
		t.Fatalf("nil: %v %v", m, err)
	}
	if m, err := r.SummariesByIDs(ctx, []int64{0, 0}); err != nil || len(m) != 0 {
		t.Fatalf("zeros: %v %v", m, err)
	}
	sm, err := r.SummariesByIDs(ctx, []int64{a, a, c, 888888, 0})
	if err != nil || len(sm) != 2 {
		t.Fatalf("by ids: %v %v", sm, err)
	}
	if s := sm[a]; s.Number != "S-1" || s.BranchID != 1 || s.TaxInvoiceNumber != "FP-A" || s.TaxInvoiceDate != "2026-03-02 00:00:00" ||
		s.DueDate != "2026-03-31 00:00:00" || s.GrandTotal != 100 || s.PartyID != 10 || s.Status != "draft" {
		t.Fatalf("summary a: %+v", s)
	}
	if s := sm[c]; s.TaxInvoiceDate != "" || s.DueDate != "" {
		t.Fatalf("summary c: %+v", s)
	}
	many := []int64{a}
	for i := int64(10000); i < 12400; i++ {
		many = append(many, i)
	}
	many = append(many, d, x)
	sm, err = r.SummariesByIDs(ctx, many)
	if err != nil || len(sm) != 3 || sm[a] == nil || sm[d] == nil || sm[x] == nil || sm[x].BranchID != 2 {
		t.Fatalf("large summaries: %d %v", len(sm), err)
	}
}

func TestSqlcSalesOperationalReads(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcSalesRepo()
	item := func(pid, vid int64, code, name string, qty float64, total int64) *contracts.Item {
		return &contracts.Item{ProductID: pid, VariantID: vid, ProductCode: code, ProductName: name, UOM: "pcs", UOMFactor: 1,
			Qty: qty, QtyBase: qty, UnitPrice: total, LineTotal: total, DiscountPct: 1.5, DiscountNominal: 7}
	}
	o1 := salMk(t, r, salSpec{branch: 1, party: 1, number: "O-1", total: 1, items: []*contracts.Item{
		item(1, 1, "A", "Apel", 2.5, 25000), item(2, 1, "B", "Bola", 1, 1000)}})
	o2 := salMk(t, r, salSpec{branch: 1, party: 1, number: "O-2", total: 1, items: []*contracts.Item{
		item(1, 1, "A", "Apel", 4, 40000), item(1, 2, "A2", "Apel Besar", 100, 50)}})
	o3 := salMk(t, r, salSpec{branch: 1, party: 1, number: "O-3", total: 1, items: []*contracts.Item{item(1, 1, "A", "Apel", 10, 1)}}) // not completed
	o4 := salMk(t, r, salSpec{branch: 2, party: 1, number: "O-4", total: 1, items: []*contracts.Item{item(1, 1, "A", "Apel", 50, 9)}})
	o5 := salMk(t, r, salSpec{branch: 1, party: 1, number: "O-5", total: 1, items: []*contracts.Item{item(2, 1, "B", "Bola", 3, 3000)}})
	for _, id := range []int64{o1, o2, o4, o5} {
		if err := r.SetStatus(ctx, id, "completed", 1); err != nil {
			t.Fatal(err)
		}
	}
	_ = r.SetStatus(ctx, o3, "confirmed", 1)
	setDone := func(id int64, at string) {
		if _, err := testMods.db.Exec("UPDATE sales_orders SET completed_at = ? WHERE id = ?", at, id); err != nil {
			t.Fatal(err)
		}
	}
	setDone(o1, "2026-03-10 00:00:00")
	setDone(o2, "2026-03-31 23:59:59")
	setDone(o4, "2026-03-10 00:00:00")
	setDone(o5, "2026-04-01 00:00:00") // outside the range below

	// ItemsByOrders: grouped, id order, only the slim columns.
	im, err := r.ItemsByOrders(ctx, []int64{o1, o2, o1, 424242})
	if err != nil || len(im) != 2 || len(im[o1]) != 2 || len(im[o2]) != 2 {
		t.Fatalf("items by orders: %v %v", im, err)
	}
	if im[o1][0].ProductCode != "A" || im[o1][0].Qty != 2.5 || im[o1][0].DiscountPct != 1.5 || im[o1][0].DiscountNominal != 7 || im[o1][0].LineTotal != 25000 ||
		im[o1][1].ProductCode != "B" || im[o1][0].ID >= im[o1][1].ID {
		t.Fatalf("items by orders rows: %+v %+v", im[o1][0], im[o1][1])
	}
	if m, err := r.ItemsByOrders(ctx, nil); err != nil || m == nil || len(m) != 0 {
		t.Fatalf("items none: %v %v", m, err)
	}
	many := []int64{o2}
	for i := int64(9000); i < 12000; i++ {
		many = append(many, i)
	}
	if m, err := r.ItemsByOrders(ctx, many); err != nil || len(m) != 1 || len(m[o2]) != 2 {
		t.Fatalf("items large list: %v %v", len(m), err)
	}

	// TopProducts: completed only, completed_at inside [from 00:00:00, to 23:59:59], branch scoped,
	// grouped per snapshot, biggest qty first, limit.
	tp, err := r.TopProducts(ctx, 1, " 2026-03-01 ", "2026-03-31", 10)
	if err != nil || len(tp) != 3 {
		t.Fatalf("top products: %+v %v", tp, err)
	}
	if tp[0].ProductCode != "A2" || tp[0].QtyBase != 100 || tp[0].Revenue != 50 || tp[0].Orders != 1 || tp[0].ProductID != 1 || tp[0].VariantID != 2 ||
		tp[1].ProductCode != "A" || tp[1].QtyBase != 6.5 || tp[1].Revenue != 65000 || tp[1].Orders != 2 || tp[1].ProductName != "Apel" ||
		tp[2].ProductCode != "B" || tp[2].QtyBase != 1 || tp[2].Revenue != 1000 || tp[2].Orders != 1 {
		t.Fatalf("top products rows: %+v %+v %+v", tp[0], tp[1], tp[2])
	}
	if tp, _ := r.TopProducts(ctx, 1, "2026-03-01", "2026-03-31", 1); len(tp) != 1 || tp[0].ProductCode != "A2" {
		t.Fatalf("top limit: %+v", tp)
	}
	if tp, _ := r.TopProducts(ctx, 1, "2026-03-11", "2026-03-30", 10); len(tp) != 0 {
		t.Fatalf("top empty: %+v", tp)
	}
	if tp, _ := r.TopProducts(ctx, 1, "2026-04-01", "2026-04-01", 10); len(tp) != 1 || tp[0].ProductCode != "B" || tp[0].Revenue != 3000 {
		t.Fatalf("top single day: %+v", tp)
	}

	// CompletedSales: Jakarta-day range converted to UTC bounds on completed_at.
	setGT := func(id, gt int64, at string) {
		if _, err := testMods.db.Exec("UPDATE sales_orders SET grand_total = ?, completed_at = ? WHERE id = ?", gt, at, id); err != nil {
			t.Fatal(err)
		}
	}
	setGT(o1, 100, "2026-03-09 17:00:00") // Jakarta 2026-03-10 00:00:00: first instant of the day
	setGT(o2, 200, "2026-03-10 16:59:59") // last second of the Jakarta day
	setGT(o5, 400, "2026-03-10 17:00:00") // next Jakarta day
	setGT(o4, 800, "2026-03-10 05:00:00") // other branch
	if tot, n, err := r.CompletedSales(ctx, 1, "2026-03-10", "2026-03-10"); err != nil || tot != 300 || n != 2 {
		t.Fatalf("completed sales day: %d %d %v", tot, n, err)
	}
	if tot, n, _ := r.CompletedSales(ctx, 1, "2026-03-10", "2026-03-11"); tot != 700 || n != 3 {
		t.Fatalf("completed sales two days: %d %d", tot, n)
	}
	if tot, n, _ := r.CompletedSales(ctx, 1, "bad", "bad"); tot != 700 || n != 3 {
		t.Fatalf("malformed range = everything: %d %d", tot, n)
	}
	if tot, n, err := r.CompletedSales(ctx, 1, "2031-01-01", "2031-01-02"); err != nil || tot != 0 || n != 0 {
		t.Fatalf("empty: %d %d %v", tot, n, err)
	}

	// OpenHeaders: confirmed only; POS only once it sat for 10 minutes.
	freshDB(t)
	reg := salMk(t, r, salSpec{branch: 1, party: 4, number: "H-REG", date: "2026-03-01 10:00:00", due: "2026-03-31 00:00:00", terms: "net", total: 900})
	posNew := salMk(t, r, salSpec{branch: 1, party: 5, number: "H-POSNEW", channel: "pos", total: 1})
	posOld := salMk(t, r, salSpec{branch: 1, party: 6, number: "H-POSOLD", channel: "pos", total: 2})
	draft := salMk(t, r, salSpec{branch: 1, party: 7, number: "H-DRAFT", total: 3})
	otherBr := salMk(t, r, salSpec{branch: 2, party: 8, number: "H-OTHER", total: 4})
	for _, id := range []int64{reg, posNew, posOld, otherBr} {
		_ = r.SetStatus(ctx, id, "confirmed", 1)
	}
	_ = draft
	if _, err := testMods.db.Exec("UPDATE sales_orders SET updated_at = UTC_TIMESTAMP(6) - INTERVAL 11 MINUTE WHERE id = ?", posOld); err != nil {
		t.Fatal(err)
	}
	oh, err := r.OpenHeaders(ctx, 1, 0)
	if err != nil || len(oh) != 2 || oh[0].OrderID != reg || oh[1].OrderID != posOld {
		t.Fatalf("open headers: %+v %v", oh, err)
	}
	if oh[0].Number != "H-REG" || oh[0].PartyID != 4 || oh[0].OrderDate != "2026-03-01 10:00:00" || oh[0].DueDate != "2026-03-31 00:00:00" ||
		oh[0].PaymentTerms != "net" || oh[0].GrandTotal != 900 || oh[0].OrphanPos || !oh[1].OrphanPos || oh[1].DueDate != "" {
		t.Fatalf("open headers rows: %+v %+v", oh[0], oh[1])
	}
	if oh, _ := r.OpenHeaders(ctx, 1, 1); len(oh) != 1 || oh[0].OrderID != reg {
		t.Fatalf("open headers limit: %+v", oh)
	}
	if oh, _ := r.OpenHeaders(ctx, 1, 500); len(oh) != 2 {
		t.Fatalf("open headers limit>200 falls back to 100: %+v", oh)
	}
	if oh, _ := r.OpenHeaders(ctx, 1, -3); len(oh) != 2 {
		t.Fatalf("open headers negative limit: %+v", oh)
	}
	// boundary: a POS order 9 minutes old is not an orphan yet
	if _, err := testMods.db.Exec("UPDATE sales_orders SET updated_at = UTC_TIMESTAMP(6) - INTERVAL 9 MINUTE WHERE id = ?", posOld); err != nil {
		t.Fatal(err)
	}
	if oh, _ := r.OpenHeaders(ctx, 1, 10); len(oh) != 1 {
		t.Fatalf("9 min old pos is not orphan: %+v", oh)
	}

	// PriorityOrders: scored in SQL with the caller's "today".
	freshDB(t)
	mk := func(num, date, due, status string) int64 {
		id := salMk(t, r, salSpec{branch: 1, party: 1, number: num, date: date, due: due, total: 10})
		_ = r.SetStatus(ctx, id, status, 1)
		return id
	}
	p1 := mk("P-1", "2026-06-01 10:00:00", "2026-06-10 00:00:00", "confirmed") // overdue 100 + 14
	p2 := mk("P-2", "2026-05-01 10:00:00", "2026-06-17 23:00:00", "confirmed") // due in 2 days 80 + 30(cap)
	p3 := mk("P-3", "2026-06-14 10:00:00", "", "draft")                        // 60 + 1
	p4 := mk("P-4", "2026-06-20 10:00:00", "2026-07-30 00:00:00", "confirmed") // 40 + 0 (future order date)
	p6 := mk("P-6", "2026-06-14 11:00:00", "", "draft")                        // tie with p3: id ASC
	_ = mk("P-5", "2026-06-01 10:00:00", "2026-06-10 00:00:00", "completed")   // not live
	p7 := mk("P-7", "2026-06-15 10:00:00", "2026-06-15 23:00:00", "confirmed") // due today: 80 + 0
	pr, err := r.PriorityOrders(ctx, 1, "2026-06-15", 10)
	if err != nil {
		t.Fatal(err)
	}
	var got []int64
	var scores []int
	for _, p := range pr {
		got = append(got, p.ID)
		scores = append(scores, p.Score)
	}
	if !sameInts(got, []int64{p1, p2, p7, p3, p6, p4}) || !reflect.DeepEqual(scores, []int{114, 110, 80, 61, 61, 40}) {
		t.Fatalf("priority: %v %v", got, scores)
	}
	if pr[0].Number != "P-1" || pr[0].PartyID != 1 || pr[0].GrandTotal != 10 || pr[0].Status != "confirmed" || pr[0].OrderDate != "2026-06-01 10:00:00" || pr[0].DueDate != "2026-06-10 00:00:00" || pr[3].DueDate != "" {
		t.Fatalf("priority row: %+v", pr[0])
	}
	if pr, _ := r.PriorityOrders(ctx, 1, "2026-06-15", 2); len(pr) != 2 || pr[1].ID != p2 {
		t.Fatalf("priority limit: %+v", pr)
	}
	// moving "today" changes the score: a month later everything past due and aged.
	pr, _ = r.PriorityOrders(ctx, 1, "2026-08-15", 10)
	if pr[0].Score != 130 {
		t.Fatalf("priority later today: %+v", pr[0])
	}
}

// TestSqlcSalesPurchasingParity runs one scenario through both twin
// repositories and demands the same observable results (ADR-0012: the sales and
// purchasing document stores must stay behaviourally identical).
func TestSqlcSalesPurchasingParity(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	sr, pr := sqlcSalesRepo(), sqlcPurRepo()

	type twin struct{ s, p int64 }
	mk := func(num, date, due, terms, status string, party int64, total int64, branch int64) twin {
		s := salMk(t, sr, salSpec{branch: branch, party: party, number: "S" + num, date: date, due: due, terms: terms, total: total, taxType: "ppn"})
		p := purMk(t, pr, purSpec{branch: branch, party: party, number: "P" + num, date: date, due: due, terms: terms, total: total, taxType: "ppn"})
		if status != "draft" {
			_ = sr.SetStatus(ctx, s, status, 1)
			_ = pr.SetStatus(ctx, p, status, 1)
		}
		return twin{s, p}
	}
	t1 := mk("1", "2026-03-01 10:00:00", "2026-03-31 00:00:00", "net", "confirmed", 10, 100, 1)
	t2 := mk("2", "2026-03-02 10:00:00", "", "cod", "draft", 10, 200, 1)
	t3 := mk("3", "2026-03-03 10:00:00", "", "prepaid", "confirmed", 11, 300, 1)
	t4 := mk("4", "2026-03-04 10:00:00", "", "net", "completed", 12, 400, 1)
	_ = mk("5", "2026-03-05 10:00:00", "", "net", "cancelled", 10, 500, 1)
	t6 := mk("6", "2026-03-06 10:00:00", "", "net", "confirmed", 10, 600, 2)

	// ids line up because both tables are filled in lock-step
	if t1.s != t1.p || t6.s != t6.p {
		t.Fatalf("ids diverge: %v %v", t1, t6)
	}
	norm := func(v any) string { return fmt.Sprintf("%+v", v) }
	same := func(name string, a, b any) {
		t.Helper()
		if norm(a) != norm(b) {
			t.Fatalf("%s diverges:\n sales:      %s\n purchasing: %s", name, norm(a), norm(b))
		}
	}

	sc1, _ := sr.StatusCounts(ctx, 1)
	pc1, _ := pr.StatusCounts(ctx, 1)
	same("StatusCounts", sc1, pc1)
	sp, _ := sr.OrderParties(ctx, []int64{t1.s, t3.s, t6.s, 999})
	pp, _ := pr.OrderParties(ctx, []int64{t1.p, t3.p, t6.p, 999})
	same("OrderParties", sp, pp)

	sumNorm := func(s []*contracts.OrderSummary) []string {
		var out []string
		for _, x := range s {
			out = append(out, fmt.Sprintf("%d|%s|%d|%s|%d|%s|%s", x.ID, x.Number[1:], x.GrandTotal, x.Status, x.PartyID, x.OrderDate, x.DueDate))
		}
		return out
	}
	purSumNorm := func(s []*purcontracts.OrderSummary) []string {
		var out []string
		for _, x := range s {
			out = append(out, fmt.Sprintf("%d|%s|%d|%s|%d|%s|%s", x.ID, x.Number[1:], x.GrandTotal, x.Status, x.PartyID, x.OrderDate, x.DueDate))
		}
		return out
	}
	a, _ := sr.ListByParty(ctx, 1, 10)
	b, _ := pr.ListByParty(ctx, 1, 10)
	same("ListByParty", sumNorm(a), purSumNorm(b))
	a, _ = sr.ListSummaries(ctx, 1, []string{"confirmed", "completed"}, "2026-03-02", "2026-03-04", 10)
	b, _ = pr.ListSummaries(ctx, 1, []string{"confirmed", "completed"}, "2026-03-02", "2026-03-04", 10)
	same("ListSummaries", sumNorm(a), purSumNorm(b))
	a, _ = sr.ListSummaries(ctx, 1, nil, "", "", 3)
	b, _ = pr.ListSummaries(ctx, 1, nil, "", "", 3)
	same("ListSummaries default", sumNorm(a), purSumNorm(b))

	ra, rt, rs, _ := sr.ReportOrders(ctx, 1, "", "2026-03-02", "2026-03-31", 2, 1)
	rb, rt2, rs2, _ := pr.ReportOrders(ctx, 1, "", "2026-03-02", "2026-03-31", 2, 1)
	same("ReportOrders", []any{sumNorm(ra), rt, rs}, []any{purSumNorm(rb), rt2, rs2})
	ra, rt, rs, _ = sr.ReportOrders(ctx, 1, "cancelled", "", "", 10, 0)
	rb, rt2, rs2, _ = pr.ReportOrders(ctx, 1, "cancelled", "", "", 10, 0)
	same("ReportOrders cancelled", []any{sumNorm(ra), rt, rs}, []any{purSumNorm(rb), rt2, rs2})

	sa, _ := sr.OpenOrderTotals(ctx, 1, 0, 10)
	pa, _ := pr.OpenOrderTotals(ctx, 1, 0, 10)
	totNorm := func(n int, num string, v any) string { return fmt.Sprintf("%v|%s", v, num[1:]) }
	var sn, pn []string
	for _, x := range sa {
		sn = append(sn, totNorm(0, x.Number, fmt.Sprint(x.ID, x.PartyID, x.GrandTotal, x.OrderDate, x.DueDate)))
	}
	for _, x := range pa {
		pn = append(pn, totNorm(0, x.Number, fmt.Sprint(x.ID, x.PartyID, x.GrandTotal, x.OrderDate, x.DueDate)))
	}
	same("OpenOrderTotals", sn, pn)
	na, _ := sr.CountOrders(ctx, 1, "2026-03-01", "2026-03-31")
	nb, _ := pr.CountOrders(ctx, 1, "2026-03-01", "2026-03-31")
	same("CountOrders", na, nb)
	ma, _ := sr.CountMissingTaxInvoice(ctx, 1)
	mb, _ := pr.CountMissingSupplierInvoice(ctx, 1)
	same("CountMissing", ma, mb)
	ma, _ = sr.CountMissingTaxInvoiceBetween(ctx, 1, "2026-03-01 00:00:00", "2026-03-03 23:59:59")
	mb, _ = pr.CountMissingSupplierInvoiceBetween(ctx, 1, "2026-03-01 00:00:00", "2026-03-03 23:59:59")
	same("CountMissingBetween", ma, mb)
	sm1, _ := sr.SummariesByIDs(ctx, []int64{t1.s, t4.s, 12345})
	sm2, _ := pr.SummariesByIDs(ctx, []int64{t1.p, t4.p, 12345})
	if len(sm1) != len(sm2) || len(sm1) != 2 {
		t.Fatalf("SummariesByIDs size: %d %d", len(sm1), len(sm2))
	}
	for id, s := range sm1 {
		p := sm2[id]
		if p == nil || s.GrandTotal != p.GrandTotal || s.Status != p.Status || s.BranchID != p.BranchID || s.OrderDate != p.OrderDate {
			t.Fatalf("SummariesByIDs row %d diverges: %+v %+v", id, s, p)
		}
	}

	// Same list filter semantics (search/status/terms/party/dates/paging/sort).
	filters := []struct {
		name string
		s    salesinfra.ListFilter
		p    purinfra.ListFilter
	}{
		{"all", salesinfra.ListFilter{}, purinfra.ListFilter{}},
		{"status", salesinfra.ListFilter{Statuses: []string{"confirmed", "draft"}}, purinfra.ListFilter{Statuses: []string{"confirmed", "draft"}}},
		{"terms", salesinfra.ListFilter{PaymentTerms: "net"}, purinfra.ListFilter{PaymentTerms: "net"}},
		{"party", salesinfra.ListFilter{PartyID: 10}, purinfra.ListFilter{PartyID: 10}},
		{"dates", salesinfra.ListFilter{From: "2026-03-02", To: "2026-03-04"}, purinfra.ListFilter{From: "2026-03-02", To: "2026-03-04"}},
		{"search number+party", salesinfra.ListFilter{Search: "3", SearchPartyIDs: []int64{12}}, purinfra.ListFilter{Search: "3", SearchPartyIDs: []int64{12}}},
		{"sort", salesinfra.ListFilter{OrderBy: "grand_total ASC"}, purinfra.ListFilter{OrderBy: "grand_total ASC"}},
	}
	for _, f := range filters {
		ls, _ := sr.List(ctx, 1, f.s, 3, 1)
		lp, _ := pr.List(ctx, 1, f.p, 3, 1)
		if !sameInts(salIDs(ls), purIDs(lp)) {
			t.Fatalf("List %s diverges: %v %v", f.name, salIDs(ls), purIDs(lp))
		}
		cs, _ := sr.Count(ctx, 1, f.s)
		cp, _ := pr.Count(ctx, 1, f.p)
		if cs != cp {
			t.Fatalf("Count %s diverges: %d %d", f.name, cs, cp)
		}
		ks, _ := sr.ListAfter(ctx, 1, f.s, t4.s, 5)
		kp, _ := pr.ListAfter(ctx, 1, f.p, t4.p, 5)
		if !sameInts(salIDs(ks), purIDs(kp)) {
			t.Fatalf("ListAfter %s diverges: %v %v", f.name, salIDs(ks), purIDs(kp))
		}
	}

	// Same CAS/transition semantics.
	for _, step := range []struct {
		from []string
		to   string
	}{{nil, "confirmed"}, {[]string{"completed"}, "cancelled"}, {[]string{"draft", "confirmed"}, "confirmed"}, {[]string{"draft"}, "confirmed"}} {
		oks, _ := sr.Transition(ctx, t2.s, step.from, step.to, 3)
		okp, _ := pr.Transition(ctx, t2.p, step.from, step.to, 3)
		if oks != okp {
			t.Fatalf("Transition %v->%s diverges: %v %v", step.from, step.to, oks, okp)
		}
	}
	oks, _ := sr.SwitchToNet(ctx, t3.s, "2026-09-01 00:00:00", 3)
	okp, _ := pr.SwitchToNet(ctx, t3.p, "2026-09-01 00:00:00", 3)
	oks2, _ := sr.SwitchToNet(ctx, t3.s, "2026-09-01 00:00:00", 3)
	okp2, _ := pr.SwitchToNet(ctx, t3.p, "2026-09-01 00:00:00", 3)
	if !oks || !okp || oks2 || okp2 {
		t.Fatalf("SwitchToNet diverges: %v %v %v %v", oks, okp, oks2, okp2)
	}
	gs, _ := sr.GetOrder(ctx, t3.s)
	gp, _ := pr.GetOrder(ctx, t3.p)
	if gs.PaymentTerms != gp.PaymentTerms || gs.DueDate != gp.DueDate || gs.Status != gp.Status {
		t.Fatalf("after SwitchToNet: %+v %+v", gs, gp)
	}
}
