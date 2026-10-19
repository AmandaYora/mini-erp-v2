package integration

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"

	"mini-erp/internal/modules/purchasereturn/contracts"
	prinfra "mini-erp/internal/modules/purchasereturn/infrastructure"
)

// Characterization tests for the purchasereturn Repository (F8-18). Twin of
// sqlc_salesreturn_test.go: keep both in step.

func sqlcPRRepo() *prinfra.Repository { return prinfra.NewRepository(testMods.db) }

func sqlcMkPRet(t *testing.T, r *prinfra.Repository, number string, branch, po int64, date, status string, mod func(*contracts.PurchaseReturn)) int64 {
	t.Helper()
	ret := &contracts.PurchaseReturn{
		Number: number, BranchID: branch, PurchaseOrderID: po, ReturnDate: date, Subtotal: 900, DiscountTotal: 100,
		TaxTotal: 99, TaxType: "exclusive", TaxRate: 11, Total: 999, Notes: "n-" + number, Reason: "rusak",
	}
	if mod != nil {
		mod(ret)
	}
	id, err := r.CreateReturn(context.Background(), ret, 5)
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

func TestSqlcPurchaseReturnCreateGetItems(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPRRepo()
	ret := &contracts.PurchaseReturn{
		Number: "PR-1", BranchID: 1, PurchaseOrderID: 10, ReturnDate: "2026-10-05", Subtotal: 1000, DiscountTotal: 50, TaxTotal: 104,
		TaxType: "exclusive", TaxRate: 11, Total: 1054, Notes: "catatan", Reason: "salah ukuran",
		Items: []*contracts.Item{
			{ProductID: 1, VariantID: 2, LocationID: 3, UOM: "pcs", UOMFactor: 1, Qty: 2.5, QtyBase: 2.5, UnitPrice: 400, DiscountPct: 5, DiscountNominal: 50, TaxBase: 950, TaxAmount: 104, LineTotal: 1054},
			{ProductID: 4, VariantID: 5, LocationID: 3, UOM: "box", UOMFactor: 12, Qty: 1, QtyBase: 12, UnitPrice: 1},
		},
	}
	id, err := r.CreateReturn(ctx, ret, 6)
	if err != nil || id == 0 {
		t.Fatalf("create: %d %v", id, err)
	}
	got, err := r.GetReturn(ctx, id)
	if err != nil || got == nil {
		t.Fatalf("get: %v %v", got, err)
	}
	want := &contracts.PurchaseReturn{
		ID: id, Number: "PR-1", BranchID: 1, PurchaseOrderID: 10, ReturnDate: "2026-10-05 00:00:00", Subtotal: 1000, DiscountTotal: 50,
		TaxTotal: 104, TaxType: "exclusive", TaxRate: 11, Total: 1054, Status: "draft", Notes: "catatan", Reason: "salah ukuran",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("return mismatch:\n got %+v\nwant %+v", got, want)
	}
	var cb, ub int64
	_ = testMods.db.QueryRow("SELECT created_by, updated_by FROM purchase_returns WHERE id=?", id).Scan(&cb, &ub)
	if cb != 6 || ub != 6 {
		t.Fatalf("audit %d %d", cb, ub)
	}

	items, err := r.ItemsByReturn(ctx, id)
	if err != nil || len(items) != 2 {
		t.Fatalf("items: %v %v", items, err)
	}
	wi := &contracts.Item{ID: items[0].ID, ProductID: 1, VariantID: 2, LocationID: 3, UOM: "pcs", UOMFactor: 1, Qty: 2.5, QtyBase: 2.5, UnitPrice: 400, DiscountPct: 5, DiscountNominal: 50, TaxBase: 950, TaxAmount: 104, LineTotal: 1054}
	if !reflect.DeepEqual(items[0], wi) {
		t.Fatalf("item0:\n got %+v\nwant %+v", items[0], wi)
	}
	if items[1].UOMFactor != 12 || items[1].QtyBase != 12 || items[1].DiscountPct != 0 || items[1].ID <= items[0].ID {
		t.Fatalf("item1: %+v", items[1])
	}
	if items, err := r.ItemsByReturn(ctx, 4242); err != nil || len(items) != 0 {
		t.Fatalf("no items: %v %v", items, err)
	}

	// defaults: empty notes/reason read back ""; datetime passes through.
	id2 := sqlcMkPRet(t, r, "PR-2", 1, 10, "2026-10-06 13:45:10", "draft", func(p *contracts.PurchaseReturn) {
		p.Notes = ""
		p.Reason = ""
	})
	g2, _ := r.GetReturn(ctx, id2)
	if g2.Notes != "" || g2.Reason != "" || g2.ReturnDate != "2026-10-06 13:45:10" || g2.PayableReduction != 0 {
		t.Fatalf("defaults: %+v", g2)
	}
	if g, err := r.GetReturn(ctx, 99999); err != nil || g != nil {
		t.Fatalf("missing: %v %v", g, err)
	}

	// failed create (duplicate number) leaves no orphan lines.
	if _, err := r.CreateReturn(ctx, &contracts.PurchaseReturn{Number: "PR-1", BranchID: 1, PurchaseOrderID: 1, ReturnDate: "2026-10-05", TaxType: "none",
		Items: []*contracts.Item{{ProductID: 1, VariantID: 1, LocationID: 1, UOM: "pcs", UOMFactor: 1, Qty: 1, QtyBase: 1}}}, 1); err == nil {
		t.Fatal("duplicate number must fail")
	}
	var n int
	_ = testMods.db.QueryRow("SELECT COUNT(*) FROM purchase_return_items").Scan(&n)
	if n != 2 {
		t.Fatalf("orphan items: %d", n)
	}
	// oversize uom rolls the header back too.
	var before int
	_ = testMods.db.QueryRow("SELECT COUNT(*) FROM purchase_returns").Scan(&before)
	if _, err := r.CreateReturn(ctx, &contracts.PurchaseReturn{Number: "PR-ROLL", BranchID: 1, PurchaseOrderID: 1, ReturnDate: "2026-10-05",
		Items: []*contracts.Item{{ProductID: 1, VariantID: 1, LocationID: 1, UOM: "this-uom-is-way-longer-than-thirty-chars-for-sure", UOMFactor: 1, Qty: 1, QtyBase: 1}}}, 1); err == nil {
		t.Fatal("oversize uom must fail")
	}
	var after int
	_ = testMods.db.QueryRow("SELECT COUNT(*) FROM purchase_returns").Scan(&after)
	if after != before {
		t.Fatalf("header must roll back: %d -> %d", before, after)
	}
}

func TestSqlcPurchaseReturnQuantities(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPRRepo()
	line := func(p, v int64, qtyBase float64) *contracts.Item {
		return &contracts.Item{ProductID: p, VariantID: v, LocationID: 1, UOM: "pcs", UOMFactor: 1, Qty: qtyBase, QtyBase: qtyBase}
	}
	mk := func(num string, po int64, status string, items ...*contracts.Item) int64 {
		id := sqlcMkPRet(t, r, num, 1, po, "2026-10-05", "draft", func(p *contracts.PurchaseReturn) { p.Items = items })
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
	m, err := r.CommittedQtyByPO(ctx, 10)
	if err != nil || !reflect.DeepEqual(m, map[string]float64{"1/1": 6.625, "2/1": 1}) {
		t.Fatalf("committed: %v %v", m, err)
	}
	if m, err := r.CommittedQtyByPO(ctx, 999); err != nil || m == nil || len(m) != 0 {
		t.Fatalf("committed empty: %v %v", m, err)
	}
	if n, err := r.ActiveReturnCount(ctx, 10); err != nil || n != 3 {
		t.Fatalf("active count: %d %v", n, err)
	}
	if n, err := r.ActiveReturnCount(ctx, 999); err != nil || n != 0 {
		t.Fatalf("active count none: %d %v", n, err)
	}
}

func TestSqlcPurchaseReturnTransitions(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPRRepo()
	id := sqlcMkPRet(t, r, "T1", 1, 1, "2026-10-01", "draft", nil)

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
	_ = testMods.db.QueryRow("SELECT status, updated_by FROM purchase_returns WHERE id=?", id).Scan(&st, &ub)
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
	_ = testMods.db.QueryRow("SELECT status FROM purchase_returns WHERE id=?", id).Scan(&st)
	if st != "confirmed" {
		t.Fatalf("hook failure must roll back: %s", st)
	}
	if ok, err := r.Transition(ctx, id, []string{"confirmed"}, "cancelled", 8); err != nil || !ok {
		t.Fatalf("plain: %v %v", ok, err)
	}
	if ok, err := r.Transition(ctx, 9999, []string{"draft"}, "confirmed", 1); err != nil || ok {
		t.Fatalf("missing: %v %v", ok, err)
	}

	// reduction
	if err := r.SetReduction(ctx, id, 777); err != nil {
		t.Fatal(err)
	}
	if g, _ := r.GetReturn(ctx, id); g.PayableReduction != 777 {
		t.Fatalf("reduction %d", g.PayableReduction)
	}
}

func TestSqlcPurchaseReturnSettlements(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPRRepo()
	id := sqlcMkPRet(t, r, "S1", 1, 1, "2026-10-01", "draft", nil)

	full := &contracts.Settlement{Type: "refund", Date: "2026-10-07", Amount: 500, PaymentMethod: "cash", ReferenceNumber: "R-1", Notes: "ok"}
	var hookID int64
	sid, err := r.InsertSettlementAudited(ctx, id, full, 4, func(ctx context.Context, tx *sql.Tx, id int64) error { hookID = id; return nil })
	if err != nil || sid == 0 || hookID != sid {
		t.Fatalf("insert audited: %d %v %d", sid, err, hookID)
	}
	bare, err := r.InsertSettlement(ctx, id, &contracts.Settlement{Type: "refund", Date: "2026-10-08 09:30:00", Amount: 10}, 4)
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
	if !reflect.DeepEqual(got[0], &contracts.Settlement{ID: sid, Type: "refund", Date: "2026-10-07 00:00:00", Amount: 500, PaymentMethod: "cash", ReferenceNumber: "R-1", Notes: "ok"}) {
		t.Fatalf("s0: %+v", got[0])
	}
	if !reflect.DeepEqual(got[1], &contracts.Settlement{ID: bare, Type: "refund", Date: "2026-10-08 09:30:00", Amount: 10}) {
		t.Fatalf("s1: %+v", got[1])
	}
	var cb int64
	_ = testMods.db.QueryRow("SELECT created_by FROM purchase_return_settlements WHERE id=?", sid).Scan(&cb)
	if cb != 4 {
		t.Fatalf("created_by %d", cb)
	}
	if got, err := r.Settlements(ctx, 4242); err != nil || len(got) != 0 {
		t.Fatalf("none: %v %v", got, err)
	}
}

func TestSqlcPurchaseReturnLists(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPRRepo()
	a := sqlcMkPRet(t, r, "PR-A-001", 1, 10, "2026-10-01", "confirmed", func(p *contracts.PurchaseReturn) {
		p.Notes = "barang rusak"
		p.Total = 100
		p.Items = []*contracts.Item{{ProductID: 1, VariantID: 1, LocationID: 1, UOM: "pcs", UOMFactor: 1, Qty: 2, QtyBase: 2}}
	})
	b := sqlcMkPRet(t, r, "PR-B-002", 1, 10, "2026-10-02 18:00:00", "draft", func(p *contracts.PurchaseReturn) { p.Notes = "salah kirim 100%" })
	c := sqlcMkPRet(t, r, "PR-C-003", 1, 11, "2026-10-03", "cancelled", func(p *contracts.PurchaseReturn) { p.Notes = "" })
	d := sqlcMkPRet(t, r, "PR-D-004", 2, 10, "2026-10-03", "confirmed", nil) // other branch
	e := sqlcMkPRet(t, r, "PR-E-005", 1, 11, "2026-10-03 23:59:59", "confirmed", func(p *contracts.PurchaseReturn) { p.Total = 300 })
	_ = d
	ids := func(ps []*contracts.PurchaseReturn) []int64 {
		var o []int64
		for _, p := range ps {
			o = append(o, p.ID)
		}
		return o
	}
	cases := []struct {
		name string
		f    prinfra.ListFilter
		want []int64
	}{
		{"none", prinfra.ListFilter{}, []int64{e, c, b, a}},
		{"status one", prinfra.ListFilter{Statuses: []string{"confirmed"}}, []int64{e, a}},
		{"status two", prinfra.ListFilter{Statuses: []string{"draft", "cancelled"}}, []int64{c, b}},
		{"status dup", prinfra.ListFilter{Statuses: []string{"draft", "draft"}}, []int64{b}},
		{"po", prinfra.ListFilter{PurchaseOrderID: 11}, []int64{e, c}},
		{"from", prinfra.ListFilter{From: "2026-10-03"}, []int64{e, c}},
		{"to", prinfra.ListFilter{To: "2026-10-02"}, []int64{b, a}},
		{"range", prinfra.ListFilter{From: "2026-10-02", To: "2026-10-02"}, []int64{b}},
		{"to includes whole last day", prinfra.ListFilter{To: "2026-10-03"}, []int64{e, c, b, a}},
		{"search number", prinfra.ListFilter{Search: "c-003"}, []int64{c}},
		{"search notes", prinfra.ListFilter{Search: "barang rusak"}, []int64{a}},
		// RT-01: the reason column is searched too; the fixture gives every row reason "rusak".
		{"search reason", prinfra.ListFilter{Search: "rusak"}, []int64{e, c, b, a}},
		{"search two words", prinfra.ListFilter{Search: "salah kirim"}, []int64{b}},
		{"search literal %", prinfra.ListFilter{Search: "100%"}, []int64{b}},
		{"search none", prinfra.ListFilter{Search: "zzz"}, nil},
		{"combined", prinfra.ListFilter{Statuses: []string{"confirmed"}, PurchaseOrderID: 10, From: "2026-10-01", To: "2026-10-01", Search: "rusak"}, []int64{a}},
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
	pg, err := r.ListFiltered(ctx, 1, prinfra.ListFilter{}, 1, 1)
	if err != nil || !reflect.DeepEqual(ids(pg), []int64{c}) || pg[0].Number != "PR-C-003" || pg[0].TaxRate != 11 || pg[0].Notes != "" || pg[0].ReturnDate != "2026-10-03 00:00:00" {
		t.Fatalf("page: %+v %v", pg, err)
	}
	if pg, err := r.ListFiltered(ctx, 1, prinfra.ListFilter{}, 5, 50); err != nil || len(pg) != 0 {
		t.Fatalf("beyond: %v %v", pg, err)
	}

	// CancelledAmong
	ca, err := r.CancelledAmong(ctx, 1, []int64{a, c, d, 999})
	if err != nil || !reflect.DeepEqual(ca, []int64{c}) {
		t.Fatalf("cancelled: %v %v", ca, err)
	}
	if ca, err := r.CancelledAmong(ctx, 1, nil); err != nil || len(ca) != 0 {
		t.Fatalf("cancelled empty: %v %v", ca, err)
	}

	// ConfirmedAfter
	ca2, err := r.ConfirmedAfter(ctx, 1, 0, 10)
	if err != nil || len(ca2) != 2 || ca2[0].ID != a || ca2[1].ID != e || ca2[0].Number != "PR-A-001" || ca2[0].Total != 100 || ca2[0].Status != "confirmed" || ca2[0].ReturnDate != "2026-10-01 00:00:00" {
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
}
