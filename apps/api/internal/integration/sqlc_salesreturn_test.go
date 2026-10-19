package integration

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"

	"mini-erp/internal/modules/salesreturn/contracts"
	srinfra "mini-erp/internal/modules/salesreturn/infrastructure"
)

// Characterization tests for the salesreturn Repository (F8-18).

func sqlcSRRepo() *srinfra.Repository { return srinfra.NewRepository(testMods.db) }

func sqlcMkRet(t *testing.T, r *srinfra.Repository, number string, branch, so int64, date, status string, mod func(*contracts.SalesReturn)) int64 {
	t.Helper()
	ret := &contracts.SalesReturn{
		Number: number, BranchID: branch, SalesOrderID: so, ReturnDate: date, Subtotal: 900, DiscountTotal: 100,
		TaxTotal: 99, TaxType: "exclusive", TaxRate: 11, Total: 999, Notes: "n-" + number, Reason: "rusak",
	}
	if mod != nil {
		mod(ret)
	}
	id, err := r.CreateReturn(context.Background(), ret, nil, 5)
	if err != nil {
		t.Fatalf("create %s: %v", number, err)
	}
	if status != "draft" {
		if err := r.SetStatus(context.Background(), id, status, 5); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func TestSqlcSalesReturnCreateGetItems(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcSRRepo()
	ret := &contracts.SalesReturn{
		Number: "SR-1", BranchID: 1, SalesOrderID: 10, ReturnDate: "2026-10-05", Subtotal: 1000, DiscountTotal: 50, TaxTotal: 104,
		TaxType: "exclusive", TaxRate: 11, Total: 1054, Notes: "catatan", ReturnMode: "exchange", Reason: "salah ukuran",
		ReplacementSubtotal: 2000, ReplacementTaxTotal: 220, ReplacementTotal: 2220, DifferenceAmount: 1166,
		Items: []*contracts.Item{
			{ProductID: 1, VariantID: 2, LocationID: 3, UOM: "pcs", UOMFactor: 1, Qty: 2.5, QtyBase: 2.5, UnitPrice: 400, DiscountPct: 5, DiscountNominal: 50, TaxBase: 950, TaxAmount: 104, LineTotal: 1054, Condition: "damaged"},
			{ProductID: 4, VariantID: 5, LocationID: 3, UOM: "box", UOMFactor: 12, Qty: 1, QtyBase: 12, UnitPrice: 1, Condition: "bogus"},
		},
	}
	repl := []*contracts.ReplacementItem{{ProductID: 7, VariantID: 8, LocationID: 3, UOM: "pcs", UOMFactor: 1, Qty: 3, QtyBase: 3, UnitPrice: 700, TaxBase: 2000, TaxAmount: 220, LineTotal: 2220}}
	id, err := r.CreateReturn(ctx, ret, repl, 6)
	if err != nil || id == 0 {
		t.Fatalf("create: %d %v", id, err)
	}
	got, err := r.GetReturn(ctx, id)
	if err != nil || got == nil {
		t.Fatalf("get: %v %v", got, err)
	}
	want := &contracts.SalesReturn{
		ID: id, Number: "SR-1", BranchID: 1, SalesOrderID: 10, ReturnDate: "2026-10-05 00:00:00", Subtotal: 1000, DiscountTotal: 50,
		TaxTotal: 104, TaxType: "exclusive", TaxRate: 11, Total: 1054, Status: "draft", Notes: "catatan", ReturnMode: "exchange",
		ReplacementDeliveryStatus: "not_required", Reason: "salah ukuran", ReplacementSubtotal: 2000, ReplacementTaxTotal: 220,
		ReplacementTotal: 2220, DifferenceAmount: 1166,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("return mismatch:\n got %+v\nwant %+v", got, want)
	}
	var cb, ub int64
	_ = testMods.db.QueryRow("SELECT created_by, updated_by FROM sales_returns WHERE id=?", id).Scan(&cb, &ub)
	if cb != 6 || ub != 6 {
		t.Fatalf("audit %d %d", cb, ub)
	}

	items, err := r.ItemsByReturn(ctx, id)
	if err != nil || len(items) != 2 {
		t.Fatalf("items: %v %v", items, err)
	}
	wi := &contracts.Item{ID: items[0].ID, ProductID: 1, VariantID: 2, LocationID: 3, UOM: "pcs", UOMFactor: 1, Qty: 2.5, QtyBase: 2.5, UnitPrice: 400, DiscountPct: 5, DiscountNominal: 50, TaxBase: 950, TaxAmount: 104, LineTotal: 1054, Condition: "damaged"}
	if !reflect.DeepEqual(items[0], wi) {
		t.Fatalf("item0:\n got %+v\nwant %+v", items[0], wi)
	}
	if items[1].Condition != "normal" || items[1].UOMFactor != 12 || items[1].QtyBase != 12 || items[1].ID <= items[0].ID {
		t.Fatalf("item1: %+v", items[1])
	}
	if items, err := r.ItemsByReturn(ctx, 4242); err != nil || len(items) != 0 {
		t.Fatalf("no items: %v %v", items, err)
	}
	ri, err := r.ReplacementItems(ctx, id)
	if err != nil || len(ri) != 1 {
		t.Fatalf("repl: %v %v", ri, err)
	}
	wr := &contracts.ReplacementItem{ID: ri[0].ID, ProductID: 7, VariantID: 8, LocationID: 3, UOM: "pcs", UOMFactor: 1, Qty: 3, QtyBase: 3, UnitPrice: 700, TaxBase: 2000, TaxAmount: 220, LineTotal: 2220}
	if !reflect.DeepEqual(ri[0], wr) {
		t.Fatalf("repl mismatch: %+v", ri[0])
	}
	if err := r.InsertReplacementItems(ctx, id, []*contracts.ReplacementItem{{ProductID: 9, VariantID: 1, LocationID: 1, UOM: "pcs", UOMFactor: 1, Qty: 1, QtyBase: 1}}); err != nil {
		t.Fatal(err)
	}
	if ri, _ := r.ReplacementItems(ctx, id); len(ri) != 2 || ri[1].ProductID != 9 {
		t.Fatalf("repl after insert: %v", ri)
	}
	if err := r.InsertReplacementItems(ctx, id, nil); err != nil {
		t.Fatal(err)
	}

	// defaults: unknown mode / replacement status / empty notes
	id2 := sqlcMkRet(t, r, "SR-2", 1, 10, "2026-10-06 13:45:10", "draft", func(s *contracts.SalesReturn) {
		s.Notes = ""
		s.ReturnMode = "weird"
		s.ReplacementDeliveryStatus = "weird"
	})
	g2, _ := r.GetReturn(ctx, id2)
	if g2.ReturnMode != "return_only" || g2.ReplacementDeliveryStatus != "not_required" || g2.Notes != "" || g2.ReturnDate != "2026-10-06 13:45:10" {
		t.Fatalf("defaults: %+v", g2)
	}
	id3 := sqlcMkRet(t, r, "SR-3", 1, 10, "2026-10-06", "draft", func(s *contracts.SalesReturn) { s.ReplacementDeliveryStatus = "pending" })
	if g3, _ := r.GetReturn(ctx, id3); g3.ReplacementDeliveryStatus != "pending" {
		t.Fatalf("kept status: %+v", g3)
	}
	if g, err := r.GetReturn(ctx, 99999); err != nil || g != nil {
		t.Fatalf("missing: %v %v", g, err)
	}

	// failed create (duplicate number) leaves no orphan lines
	if _, err := r.CreateReturn(ctx, &contracts.SalesReturn{Number: "SR-1", BranchID: 1, SalesOrderID: 1, ReturnDate: "2026-10-05", TaxType: "none",
		Items: []*contracts.Item{{ProductID: 1, VariantID: 1, LocationID: 1, UOM: "pcs", UOMFactor: 1, Qty: 1, QtyBase: 1}}}, nil, 1); err == nil {
		t.Fatal("duplicate number must fail")
	}
	var n int
	_ = testMods.db.QueryRow("SELECT COUNT(*) FROM sales_return_items").Scan(&n)
	if n != 2 {
		t.Fatalf("orphan items: %d", n)
	}
}

func TestSqlcSalesReturnQuantities(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcSRRepo()
	line := func(p, v int64, qtyBase float64) *contracts.Item {
		return &contracts.Item{ProductID: p, VariantID: v, LocationID: 1, UOM: "pcs", UOMFactor: 1, Qty: qtyBase, QtyBase: qtyBase}
	}
	mk := func(num string, so int64, status string, items ...*contracts.Item) int64 {
		id := sqlcMkRet(t, r, num, 1, so, "2026-10-05", "draft", func(s *contracts.SalesReturn) { s.Items = items })
		if status != "draft" {
			_ = r.SetStatus(ctx, id, status, 1)
		}
		return id
	}
	mk("A", 10, "confirmed", line(1, 1, 2.5), line(2, 1, 1))
	mk("B", 10, "draft", line(1, 1, 4))
	mk("C", 10, "cancelled", line(1, 1, 100))
	mk("D", 10, "confirmed", line(1, 1, 0.125))
	mk("E", 11, "confirmed", line(1, 1, 50))

	if q, err := r.ReturnedQty(ctx, 10, 1, 1); err != nil || q != 2.625 {
		t.Fatalf("returned qty: %v %v", q, err)
	}
	if q, err := r.ReturnedQty(ctx, 10, 9, 9); err != nil || q != 0 {
		t.Fatalf("returned none: %v %v", q, err)
	}
	m, err := r.CommittedQtyBySO(ctx, 10)
	if err != nil || !reflect.DeepEqual(m, map[string]float64{"1/1": 6.625, "2/1": 1}) {
		t.Fatalf("committed: %v %v", m, err)
	}
	if m, err := r.CommittedQtyBySO(ctx, 999); err != nil || m == nil || len(m) != 0 {
		t.Fatalf("committed empty: %v %v", m, err)
	}
	if n, err := r.ActiveReturnCount(ctx, 10); err != nil || n != 3 {
		t.Fatalf("active count: %d %v", n, err)
	}
	if n, err := r.ActiveReturnCount(ctx, 999); err != nil || n != 0 {
		t.Fatalf("active count none: %d %v", n, err)
	}
}

func TestSqlcSalesReturnTransitions(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcSRRepo()
	id := sqlcMkRet(t, r, "T1", 1, 1, "2026-10-01", "draft", nil)

	if ok, err := r.Transition(ctx, id, nil, "confirmed", 1); err != nil || ok {
		t.Fatalf("empty from: %v %v", ok, err)
	}
	if ok, err := r.Transition(ctx, id, []string{"confirmed"}, "cancelled", 1); err != nil || ok {
		t.Fatalf("wrong origin: %v %v", ok, err)
	}
	hooks := 0
	ok, err := r.TransitionAudited(ctx, id, []string{"draft", "x"}, "confirmed", 8, func(context.Context, *sql.Tx) error { hooks++; return nil })
	if err != nil || !ok || hooks != 1 {
		t.Fatalf("win: %v %v %d", ok, err, hooks)
	}
	var st string
	var ub int64
	_ = testMods.db.QueryRow("SELECT status, updated_by FROM sales_returns WHERE id=?", id).Scan(&st, &ub)
	if st != "confirmed" || ub != 8 {
		t.Fatalf("state %s %d", st, ub)
	}
	if ok, err := r.TransitionAudited(ctx, id, []string{"draft"}, "confirmed", 8, func(context.Context, *sql.Tx) error { hooks++; return nil }); err != nil || ok || hooks != 1 {
		t.Fatalf("lost CAS must not run hook: %v %v %d", ok, err, hooks)
	}
	boom := errors.New("boom")
	if ok, err := r.TransitionAudited(ctx, id, []string{"confirmed"}, "cancelled", 8, func(context.Context, *sql.Tx) error { return boom }); !errors.Is(err, boom) || ok {
		t.Fatalf("hook error: %v %v", ok, err)
	}
	_ = testMods.db.QueryRow("SELECT status FROM sales_returns WHERE id=?", id).Scan(&st)
	if st != "confirmed" {
		t.Fatalf("hook failure must roll back: %s", st)
	}
	if ok, err := r.Transition(ctx, id, []string{"confirmed"}, "cancelled", 8); err != nil || !ok {
		t.Fatalf("plain: %v %v", ok, err)
	}
	if ok, err := r.Transition(ctx, 9999, []string{"draft"}, "confirmed", 1); err != nil || ok {
		t.Fatalf("missing: %v %v", ok, err)
	}

	// replacement CAS
	rid := sqlcMkRet(t, r, "T2", 1, 1, "2026-10-01", "draft", func(s *contracts.SalesReturn) { s.ReplacementDeliveryStatus = "pending" })
	if ok, err := r.SetReplacementStatus(ctx, rid, "dispatched", "confirmed", 2); err != nil || ok {
		t.Fatalf("wrong from: %v %v", ok, err)
	}
	if ok, err := r.SetReplacementStatus(ctx, rid, "pending", "dispatched", 2); err != nil || !ok {
		t.Fatalf("cas: %v %v", ok, err)
	}
	if g, _ := r.GetReturn(ctx, rid); g.ReplacementDeliveryStatus != "dispatched" {
		t.Fatalf("status %s", g.ReplacementDeliveryStatus)
	}
	if ok, err := r.SetReplacementStatus(ctx, rid, "pending", "dispatched", 2); err != nil || ok {
		t.Fatalf("replay: %v %v", ok, err)
	}

	// reduction
	if err := r.SetReduction(ctx, rid, 777); err != nil {
		t.Fatal(err)
	}
	if g, _ := r.GetReturn(ctx, rid); g.ReceivableReduction != 777 {
		t.Fatalf("reduction %d", g.ReceivableReduction)
	}
}

func TestSqlcSalesReturnSettlements(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcSRRepo()
	id := sqlcMkRet(t, r, "S1", 1, 1, "2026-10-01", "draft", nil)

	full := &contracts.Settlement{Type: "refund", Date: "2026-10-07", Amount: 500, PaymentMethod: "cash", ReferenceNumber: "R-1", Notes: "ok", Auto: true}
	var hookID int64
	sid, err := r.InsertSettlementAudited(ctx, id, full, 4, func(ctx context.Context, tx *sql.Tx, id int64) error { hookID = id; return nil })
	if err != nil || sid == 0 || hookID != sid {
		t.Fatalf("insert audited: %d %v %d", sid, err, hookID)
	}
	bare, err := r.InsertSettlement(ctx, id, &contracts.Settlement{Type: "customer_credit", Date: "2026-10-08 09:30:00", Amount: 10}, 4)
	if err != nil || bare == 0 {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	if _, err := r.InsertSettlementAudited(ctx, id, full, 4, func(context.Context, *sql.Tx, int64) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("hook error: %v", err)
	}
	got, err := r.Settlements(ctx, id)
	if err != nil || len(got) != 2 {
		t.Fatalf("settlements: %v %v", got, err)
	}
	if !reflect.DeepEqual(got[0], &contracts.Settlement{ID: sid, Type: "refund", Date: "2026-10-07 00:00:00", Amount: 500, PaymentMethod: "cash", ReferenceNumber: "R-1", Notes: "ok", Auto: true}) {
		t.Fatalf("s0: %+v", got[0])
	}
	if !reflect.DeepEqual(got[1], &contracts.Settlement{ID: bare, Type: "customer_credit", Date: "2026-10-08 09:30:00", Amount: 10}) {
		t.Fatalf("s1: %+v", got[1])
	}
	var cb int64
	_ = testMods.db.QueryRow("SELECT created_by FROM sales_return_settlements WHERE id=?", sid).Scan(&cb)
	if cb != 4 {
		t.Fatalf("created_by %d", cb)
	}
	if got, err := r.Settlements(ctx, 4242); err != nil || len(got) != 0 {
		t.Fatalf("none: %v %v", got, err)
	}
}

func TestSqlcSalesReturnListsAndReports(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcSRRepo()
	a := sqlcMkRet(t, r, "SR-A-001", 1, 10, "2026-10-01", "confirmed", func(s *contracts.SalesReturn) {
		s.Notes = "barang rusak"
		s.Total = 100
		s.Items = []*contracts.Item{{ProductID: 1, VariantID: 1, LocationID: 1, UOM: "pcs", UOMFactor: 1, Qty: 2, QtyBase: 2}}
	})
	b := sqlcMkRet(t, r, "SR-B-002", 1, 10, "2026-10-02 18:00:00", "draft", func(s *contracts.SalesReturn) { s.Notes = "salah kirim 100%" })
	c := sqlcMkRet(t, r, "SR-C-003", 1, 11, "2026-10-03", "cancelled", func(s *contracts.SalesReturn) { s.Notes = "" })
	d := sqlcMkRet(t, r, "SR-D-004", 2, 10, "2026-10-03", "confirmed", nil) // other branch
	e := sqlcMkRet(t, r, "SR-E-005", 1, 11, "2026-10-03 23:59:59", "confirmed", func(s *contracts.SalesReturn) {
		s.ReturnMode = "exchange"
		s.Total = 300
		s.ReplacementTotal = 1000
		s.Items = []*contracts.Item{{ProductID: 1, VariantID: 1, LocationID: 1, UOM: "pcs", UOMFactor: 1, Qty: 3, QtyBase: 3}, {ProductID: 2, VariantID: 1, LocationID: 1, UOM: "pcs", UOMFactor: 1, Qty: 1, QtyBase: 1}}
	})
	_ = d
	ids := func(ps []*contracts.SalesReturn) []int64 {
		var o []int64
		for _, p := range ps {
			o = append(o, p.ID)
		}
		return o
	}
	cases := []struct {
		name string
		f    srinfra.ListFilter
		want []int64
	}{
		{"none", srinfra.ListFilter{}, []int64{e, c, b, a}},
		{"status one", srinfra.ListFilter{Statuses: []string{"confirmed"}}, []int64{e, a}},
		{"status two", srinfra.ListFilter{Statuses: []string{"draft", "cancelled"}}, []int64{c, b}},
		{"status dup", srinfra.ListFilter{Statuses: []string{"draft", "draft"}}, []int64{b}},
		{"so", srinfra.ListFilter{SalesOrderID: 11}, []int64{e, c}},
		{"from", srinfra.ListFilter{From: "2026-10-03"}, []int64{e, c}},
		{"to", srinfra.ListFilter{To: "2026-10-02"}, []int64{b, a}},
		{"range", srinfra.ListFilter{From: "2026-10-02", To: "2026-10-02"}, []int64{b}},
		{"to includes whole last day", srinfra.ListFilter{To: "2026-10-03"}, []int64{e, c, b, a}},
		{"search number", srinfra.ListFilter{Search: "c-003"}, []int64{c}},
		{"search notes", srinfra.ListFilter{Search: "barang rusak"}, []int64{a}},
		// RT-01: the reason column is searched too; the fixture gives every row reason "rusak".
		{"search reason", srinfra.ListFilter{Search: "rusak"}, []int64{e, c, b, a}},
		{"search two words", srinfra.ListFilter{Search: "salah kirim"}, []int64{b}},
		{"search literal %", srinfra.ListFilter{Search: "100%"}, []int64{b}},
		{"search none", srinfra.ListFilter{Search: "zzz"}, nil},
		{"combined", srinfra.ListFilter{Statuses: []string{"confirmed"}, SalesOrderID: 10, From: "2026-10-01", To: "2026-10-01", Search: "rusak"}, []int64{a}},
	}
	for _, tc := range cases {
		got, err := r.ListFiltered(ctx, 1, tc.f, 50, 0)
		if err != nil || !reflect.DeepEqual(ids(got), tc.want) {
			t.Fatalf("%s: %v err %v want %v", tc.name, ids(got), err, tc.want)
		}
		if n, err := r.CountFiltered(ctx, 1, tc.f); err != nil || int(n) != len(tc.want) {
			t.Fatalf("%s: count %d err %v want %d", tc.name, n, err, len(tc.want))
		}
	}
	// narrow List/Count (status CSV)
	l, err := r.List(ctx, 1, 11, "confirmed,cancelled", 50, 0)
	if err != nil || !reflect.DeepEqual(ids(l), []int64{e, c}) {
		t.Fatalf("list csv: %v %v", ids(l), err)
	}
	if n, err := r.Count(ctx, 1, 0, ""); err != nil || n != 4 {
		t.Fatalf("count: %d %v", n, err)
	}
	// paging + mapped columns of the list projection
	pg, err := r.ListFiltered(ctx, 1, srinfra.ListFilter{}, 1, 1)
	if err != nil || !reflect.DeepEqual(ids(pg), []int64{c}) || pg[0].Number != "SR-C-003" || pg[0].TaxRate != 11 || pg[0].Notes != "" || pg[0].ReturnMode != "return_only" || pg[0].ReturnDate != "2026-10-03 00:00:00" {
		t.Fatalf("page: %+v %v", pg, err)
	}
	if pg, err := r.ListFiltered(ctx, 1, srinfra.ListFilter{}, 5, 50); err != nil || len(pg) != 0 {
		t.Fatalf("beyond: %v %v", pg, err)
	}

	// CancelledAmong / OrderIDsByReturns
	ca, err := r.CancelledAmong(ctx, 1, []int64{a, c, d, 999})
	if err != nil || !reflect.DeepEqual(ca, []int64{c}) {
		t.Fatalf("cancelled: %v %v", ca, err)
	}
	if ca, err := r.CancelledAmong(ctx, 1, nil); err != nil || len(ca) != 0 {
		t.Fatalf("cancelled empty: %v %v", ca, err)
	}
	om, err := r.OrderIDsByReturns(ctx, []int64{a, c, a, 0, 999})
	if err != nil || !reflect.DeepEqual(om, map[int64]int64{a: 10, c: 11}) {
		t.Fatalf("order ids: %v %v", om, err)
	}
	if om, err := r.OrderIDsByReturns(ctx, nil); err != nil || om == nil || len(om) != 0 {
		t.Fatalf("order ids empty: %v %v", om, err)
	}
	big := make([]int64, 0, 2500)
	for i := int64(1); i <= 2500; i++ {
		big = append(big, i)
	}
	if om, err := r.OrderIDsByReturns(ctx, big); err != nil || len(om) != 5 {
		t.Fatalf("order ids big: %d %v", len(om), err)
	}

	// ConfirmedAfter
	ca2, err := r.ConfirmedAfter(ctx, 1, 0, 10)
	if err != nil || len(ca2) != 2 || ca2[0].ID != a || ca2[1].ID != e || ca2[0].Number != "SR-A-001" || ca2[0].Total != 100 || ca2[0].Status != "confirmed" || ca2[0].ReturnDate != "2026-10-01 00:00:00" {
		t.Fatalf("confirmed after: %+v %v", ca2, err)
	}
	if ca2, _ := r.ConfirmedAfter(ctx, 1, a, 1); len(ca2) != 1 || ca2[0].ID != e {
		t.Fatalf("confirmed cursor: %+v", ca2)
	}
	if ca2, _ := r.ConfirmedAfter(ctx, 1, e, 5); len(ca2) != 0 {
		t.Fatalf("confirmed end: %+v", ca2)
	}

	// ConfirmedByOrders (also > one 500 chunk)
	cb, err := r.ConfirmedByOrders(ctx, 1, []int64{10, 11, 12})
	if err != nil || len(cb) != 2 || cb[0].ID != a || cb[0].OrderID != 10 || cb[0].Branch != 1 || cb[0].Total != 100 || cb[0].Status != "confirmed" || cb[1].ID != e {
		t.Fatalf("by orders: %+v %v", cb, err)
	}
	if cb, err := r.ConfirmedByOrders(ctx, 1, nil); err != nil || len(cb) != 0 {
		t.Fatalf("by orders empty: %v %v", cb, err)
	}
	orders := make([]int64, 0, 1200)
	for i := int64(1); i <= 1200; i++ {
		orders = append(orders, i)
	}
	if cb, err := r.ConfirmedByOrders(ctx, 1, orders); err != nil || len(cb) != 2 {
		t.Fatalf("by orders chunked: %v %v", len(cb), err)
	}

	// reports over confirmed returns (inclusive date range)
	rp, err := r.ReturnedByProduct(ctx, 1, "2026-10-01", "2026-10-03")
	if err != nil || !reflect.DeepEqual(rp, map[int64]float64{1: 5, 2: 1}) {
		t.Fatalf("returned by product: %v %v", rp, err)
	}
	rp, _ = r.ReturnedByProduct(ctx, 1, "2026-10-01", "2026-10-02")
	if !reflect.DeepEqual(rp, map[int64]float64{1: 2}) {
		t.Fatalf("range: %v", rp)
	}
	if rp, err := r.ReturnedByProduct(ctx, 1, "2027-01-01", "2027-01-02"); err != nil || rp == nil || len(rp) != 0 {
		t.Fatalf("empty: %v %v", rp, err)
	}
	// a: return_only total 100 -> -100; e: exchange 1000-300 -> +700
	if adj, err := r.NetAdjustment(ctx, 1, "2026-10-01", "2026-10-03"); err != nil || adj != 600 {
		t.Fatalf("net adj: %d %v", adj, err)
	}
	if adj, err := r.NetAdjustment(ctx, 1, "2026-10-01", "2026-10-02"); err != nil || adj != -100 {
		t.Fatalf("net adj partial: %d %v", adj, err)
	}
	if adj, err := r.NetAdjustment(ctx, 1, "2027-01-01", "2027-01-02"); err != nil || adj != 0 {
		t.Fatalf("net adj none: %d %v", adj, err)
	}
}
