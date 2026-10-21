package integration

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"

	"mini-erp/internal/modules/delivery/contracts"
	dlinfra "mini-erp/internal/modules/delivery/infrastructure"
	"mini-erp/internal/shared/timeutil"
)

// Characterization tests for the delivery Repository (F8-18).

func sqlcDLRepo() *dlinfra.Repository { return dlinfra.NewRepository(testMods.db) }

func dlLine(prod, variant, loc int64, uom string, factor, qty, base float64) *contracts.Item {
	return &contracts.Item{ProductID: prod, VariantID: variant, LocationID: loc, UOM: uom, UOMFactor: factor, Qty: qty, QtyBase: base}
}

func sqlcMkNote(t *testing.T, r *dlinfra.Repository, number string, branch, so int64, date, status string, mod func(*contracts.DeliveryNote)) int64 {
	t.Helper()
	n := &contracts.DeliveryNote{
		Number: number, BranchID: branch, SalesOrderID: so, DeliveryDate: date,
		DriverName: "d-" + number, VehiclePlate: "B 1", WarehouseStaffName: "gudang",
		DropLocationNote: "titip", Notes: "n-" + number,
		Items: []*contracts.Item{dlLine(1, 0, 5, "pcs", 1, 1, 1)},
	}
	if mod != nil {
		mod(n)
	}
	id, err := r.CreateNote(context.Background(), n, 5)
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

func TestSqlcDeliveryCreateGetItems(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcDLRepo()
	n := &contracts.DeliveryNote{
		Number: "SJ-1", BranchID: 1, SalesOrderID: 10, DeliveryDate: "2026-10-05",
		DriverName: "Budi", VehiclePlate: "B 1234", WarehouseStaffName: "Asep",
		DropLocationNote: "belakang",
		Notes:            "catatan", ClientRef: "c-1",
		Items: []*contracts.Item{
			{ProductID: 1, VariantID: 2, LocationID: 3, UOM: "pcs", UOMFactor: 1, Qty: 2.5, QtyBase: 2.5, ReservationKey: "hold-9"},
			{ProductID: 4, VariantID: 0, LocationID: 3, UOM: "box", UOMFactor: 12, Qty: 1, QtyBase: 12},
		},
	}
	id, err := r.CreateNote(ctx, n, 6)
	if err != nil || id == 0 {
		t.Fatalf("create: %d %v", id, err)
	}
	got, err := r.GetNote(ctx, id)
	if err != nil || got == nil {
		t.Fatalf("get: %v %v", got, err)
	}
	// Recipient evidence is NOT stored at creation (only at confirm/dispatch).
	want := &contracts.DeliveryNote{
		ID: id, Number: "SJ-1", BranchID: 1, SalesOrderID: 10, DeliveryDate: "2026-10-05 00:00:00",
		DriverName: "Budi", VehiclePlate: "B 1234", WarehouseStaffName: "Asep",
		DropLocationNote: "belakang",
		DocumentKind:     "order", Origin: "order", Status: "draft", Notes: "catatan",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("note mismatch:\n got %+v\nwant %+v", got, want)
	}
	var cb, ub int64
	_ = testMods.db.QueryRow("SELECT created_by, updated_by FROM delivery_notes WHERE id=?", id).Scan(&cb, &ub)
	if cb != 6 || ub != 6 {
		t.Fatalf("audit %d %d", cb, ub)
	}

	items, err := r.ItemsByDelivery(ctx, id)
	if err != nil || len(items) != 2 {
		t.Fatalf("items: %v %v", items, err)
	}
	if items[0].ReservationKey != "hold-9" || items[0].Qty != 2.5 || items[0].UOMFactor != 1 || items[1].ReservationKey != "" || items[1].QtyBase != 12 || items[1].ID <= items[0].ID {
		t.Fatalf("items content: %+v %+v", items[0], items[1])
	}
	if items, err := r.ItemsByDelivery(ctx, 4242); err != nil || len(items) != 0 {
		t.Fatalf("no items: %v %v", items, err)
	}

	// defaults: empty kind -> order, nulls read "".
	id2 := sqlcMkNote(t, r, "SJ-2", 1, 10, "2026-10-06 13:45:10", "draft", func(n *contracts.DeliveryNote) {
		n.DriverName, n.VehiclePlate, n.WarehouseStaffName = "", "", ""
		n.DropLocationNote, n.Notes = "", ""
	})
	g2, _ := r.GetNote(ctx, id2)
	if g2.DocumentKind != "order" || g2.DriverName != "" || g2.SalesReturnID != 0 {
		t.Fatalf("defaults: %+v", g2)
	}
	g2, _ = r.GetNote(ctx, id2)
	if g2.DeliveryDate != "2026-10-06 13:45:10" {
		t.Fatalf("datetime: %+v", g2)
	}
	// replacement kind keeps its return link.
	id3 := sqlcMkNote(t, r, "SJ-3", 1, 10, "2026-10-06", "draft", func(n *contracts.DeliveryNote) {
		n.DocumentKind = "replacement"
		n.SalesReturnID = 77
	})
	if g3, _ := r.GetNote(ctx, id3); g3.DocumentKind != "replacement" || g3.SalesReturnID != 77 {
		t.Fatalf("replacement: %+v", g3)
	}
	if g, err := r.GetNote(ctx, 99999); err != nil || g != nil {
		t.Fatalf("missing: %v %v", g, err)
	}

	// duplicate number fails atomically (no orphan lines).
	if _, err := r.CreateNote(ctx, &contracts.DeliveryNote{Number: "SJ-1", BranchID: 1, SalesOrderID: 1, DeliveryDate: "2026-10-05",
		Items: []*contracts.Item{dlLine(1, 0, 1, "pcs", 1, 1, 1)}}, 1); err == nil {
		t.Fatal("duplicate number must fail")
	}
	var ni int
	_ = testMods.db.QueryRow("SELECT COUNT(*) FROM delivery_note_items").Scan(&ni)
	if ni != 4 {
		t.Fatalf("orphan items: %d", ni)
	}
	// oversize uom rolls the header back too.
	var before int
	_ = testMods.db.QueryRow("SELECT COUNT(*) FROM delivery_notes").Scan(&before)
	if _, err := r.CreateNote(ctx, &contracts.DeliveryNote{Number: "SJ-ROLL", BranchID: 1, SalesOrderID: 1, DeliveryDate: "2026-10-05",
		Items: []*contracts.Item{dlLine(1, 0, 1, "this-uom-is-way-longer-than-thirty-chars-for-sure", 1, 1, 1)}}, 1); err == nil {
		t.Fatal("oversize uom must fail")
	}
	var after int
	_ = testMods.db.QueryRow("SELECT COUNT(*) FROM delivery_notes").Scan(&after)
	if after != before {
		t.Fatalf("header must roll back: %d -> %d", before, after)
	}
}

func TestSqlcDeliveryQuantities(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcDLRepo()
	mk := func(num string, so int64, kind string, items ...*contracts.Item) int64 {
		return sqlcMkNote(t, r, num, 1, so, "2026-10-05", "draft", func(n *contracts.DeliveryNote) {
			n.DocumentKind = kind
			n.Items = items
		})
	}
	setSt := func(id int64, st string) {
		t.Helper()
		if err := r.SetStatus(ctx, id, st, 1); err != nil {
			t.Fatal(err)
		}
	}
	a := mk("A", 10, "order", dlLine(1, 1, 1, "pcs", 1, 2.5, 2.5), dlLine(2, 1, 1, "pcs", 1, 1, 1))
	setSt(a, "confirmed")
	mk("B", 10, "order", dlLine(1, 1, 1, "pcs", 1, 4, 4)) // draft
	c := mk("C", 10, "order", dlLine(1, 1, 1, "pcs", 1, 100, 100))
	setSt(c, "cancelled")
	d := mk("D", 10, "order", dlLine(1, 1, 1, "pcs", 1, 0.125, 0.125))
	setSt(d, "confirmed")
	e := mk("E", 10, "replacement", dlLine(1, 1, 1, "pcs", 1, 50, 50))
	setSt(e, "confirmed")
	mk("F", 11, "order", dlLine(1, 1, 1, "pcs", 1, 7, 7)) // draft on another order
	setSt(mk("G", 11, "order", dlLine(9, 9, 9, "pcs", 1, 7, 7)), "confirmed")

	if q, err := r.DeliveredQty(ctx, 10, 1, 1); err != nil || q != 2.625 {
		t.Fatalf("delivered qty: %v %v", q, err)
	}
	if q, err := r.DeliveredQty(ctx, 10, 9, 9); err != nil || q != 0 {
		t.Fatalf("delivered none: %v %v", q, err)
	}
	m, err := r.CommittedQtyBySO(ctx, 10)
	if err != nil || !reflect.DeepEqual(m, map[string]float64{"1/1": 6.625, "2/1": 1}) {
		t.Fatalf("committed: %v %v", m, err)
	}
	if m, err := r.CommittedQtyBySO(ctx, 999); err != nil || m == nil || len(m) != 0 {
		t.Fatalf("committed empty: %v %v", m, err)
	}
	if n, err := r.ActiveNoteCount(ctx, 10); err != nil || n != 3 {
		t.Fatalf("active count: %d %v", n, err)
	}
	if n, err := r.ActiveNoteCount(ctx, 999); err != nil || n != 0 {
		t.Fatalf("active count none: %d %v", n, err)
	}
	// bulk sums only confirmed order-kind notes (F draft and E replacement excluded).
	bm, err := r.DeliveredQtyBulk(ctx, []int64{10, 11, 999})
	if err != nil || !reflect.DeepEqual(bm, map[string]float64{"10/1/1": 2.625, "10/2/1": 1, "11/9/9": 7}) {
		t.Fatalf("bulk: %v %v", bm, err)
	}
	if bm, err := r.DeliveredQtyBulk(ctx, nil); err != nil || bm == nil || len(bm) != 0 {
		t.Fatalf("bulk empty: %v %v", bm, err)
	}
	dc, err := r.DraftCounts(ctx, []int64{10, 11, 999})
	if err != nil || !reflect.DeepEqual(dc, map[int64]int64{10: 1, 11: 1}) {
		t.Fatalf("drafts: %v %v", dc, err)
	}
	if dc, err := r.DraftCounts(ctx, nil); err != nil || dc == nil || len(dc) != 0 {
		t.Fatalf("drafts empty: %v %v", dc, err)
	}
	// ConfirmedQtyBefore accumulates uom spellings that trim equal.
	p1 := sqlcMkNote(t, r, "P1", 1, 20, "2026-10-05", "confirmed", func(n *contracts.DeliveryNote) {
		n.Items = []*contracts.Item{dlLine(1, 0, 1, "pcs", 1, 3, 3), dlLine(1, 0, 1, " pcs ", 1, 1, 1)}
	})
	p2 := sqlcMkNote(t, r, "P2", 1, 20, "2026-10-05", "confirmed", func(n *contracts.DeliveryNote) {
		n.Items = []*contracts.Item{dlLine(1, 0, 1, "pcs", 1, 100, 100)}
	})
	_ = p1
	pq, err := r.ConfirmedQtyBefore(ctx, 20, p2+1)
	if err != nil || pq["1/0/pcs"] != 104 || len(pq) != 1 {
		t.Fatalf("qty before: %v %v", pq, err)
	}
	if pq, _ := r.ConfirmedQtyBefore(ctx, 20, p1); len(pq) != 0 {
		t.Fatalf("qty before exclusive: %v", pq)
	}
}

func TestSqlcDeliveryTransitions(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcDLRepo()
	id := sqlcMkNote(t, r, "T1", 1, 1, "2026-10-01", "draft", nil)

	if ok, err := r.Transition(ctx, id, nil, "confirmed", 1); err != nil || ok {
		t.Fatalf("empty from: %v %v", ok, err)
	}
	if ok, err := r.Transition(ctx, id, []string{"confirmed"}, "cancelled", 1); err != nil || ok {
		t.Fatalf("wrong origin: %v %v", ok, err)
	}
	if ok, err := r.Transition(ctx, id, []string{"draft"}, "cancelled", 8); err != nil || !ok {
		t.Fatalf("cas: %v %v", ok, err)
	}
	var st string
	var ub int64
	_ = testMods.db.QueryRow("SELECT status, updated_by FROM delivery_notes WHERE id=?", id).Scan(&st, &ub)
	if st != "cancelled" || ub != 8 {
		t.Fatalf("state %s %d", st, ub)
	}
	if ok, err := r.Transition(ctx, 9999, []string{"draft"}, "confirmed", 1); err != nil || ok {
		t.Fatalf("missing: %v %v", ok, err)
	}

	// Confirm wins once, stamps evidence, runs the hook; losers write nothing.
	cid := sqlcMkNote(t, r, "C1", 1, 1, "2026-10-01", "draft", nil)
	hooks := 0
	ev := &contracts.DeliveryNote{RecipientName: "Toko", RecipientSignatureStatus: "signed"}
	ok, err := r.Confirm(ctx, cid, ev, 9, func(context.Context, *sql.Tx) error { hooks++; return nil })
	if err != nil || !ok || hooks != 1 {
		t.Fatalf("confirm: %v %v %d", ok, err, hooks)
	}
	got, _ := r.GetNote(ctx, cid)
	if got.Status != "confirmed" || got.RecipientName != "Toko" || got.DispatchedBy != 9 || got.ConfirmedBy != 9 ||
		got.DispatchedAt == "" || got.ConfirmedAt == "" {
		t.Fatalf("evidence: %+v", got)
	}
	if ok, err := r.Confirm(ctx, cid, ev, 9, func(context.Context, *sql.Tx) error { hooks++; return nil }); err != nil || ok || hooks != 1 {
		t.Fatalf("replay must lose: %v %v %d", ok, err, hooks)
	}
	boom := errors.New("boom")
	did := sqlcMkNote(t, r, "C2", 1, 1, "2026-10-01", "draft", nil)
	if ok, err := r.Confirm(ctx, did, ev, 9, func(context.Context, *sql.Tx) error { return boom }); !errors.Is(err, boom) || ok {
		t.Fatalf("hook error: %v %v", ok, err)
	}
	if g, _ := r.GetNote(ctx, did); g.Status != "draft" {
		t.Fatalf("hook failure must roll back: %s", g.Status)
	}

	// SetDispatch stamps without flipping status.
	sid := sqlcMkNote(t, r, "S1", 1, 1, "2026-10-01", "draft", nil)
	if err := r.SetDispatch(ctx, sid, &contracts.DeliveryNote{RecipientName: "Gudang"}, 3); err != nil {
		t.Fatal(err)
	}
	if g, _ := r.GetNote(ctx, sid); g.Status != "draft" || g.RecipientName != "Gudang" || g.DispatchedBy != 3 || g.DispatchedAt == "" {
		t.Fatalf("dispatch: %+v", g)
	}

	// NoteIDByClientRef.
	rid := sqlcMkNote(t, r, "R1", 1, 1, "2026-10-01", "draft", func(n *contracts.DeliveryNote) { n.ClientRef = "ref-7" })
	if got, err := r.NoteIDByClientRef(ctx, 1, "ref-7"); err != nil || got != rid {
		t.Fatalf("ref: %d %v", got, err)
	}
	if got, err := r.NoteIDByClientRef(ctx, 2, "ref-7"); err != nil || got != 0 {
		t.Fatalf("ref other branch: %d %v", got, err)
	}
	if got, err := r.NoteIDByClientRef(ctx, 1, "nope"); err != nil || got != 0 {
		t.Fatalf("ref missing: %d %v", got, err)
	}
}

func TestSqlcDeliveryLists(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcDLRepo()
	today := timeutil.NowUTC().In(timeutil.Jakarta)
	day := func(off int) string { return today.AddDate(0, 0, off).Format("2006-01-02") }
	a := sqlcMkNote(t, r, "DL-A-001", 1, 10, day(0), "draft", func(n *contracts.DeliveryNote) {
		n.DriverName = "Budi Santoso"
		n.Notes = "Toko Maju"
	})
	b := sqlcMkNote(t, r, "DL-B-002", 1, 10, day(-3), "draft", func(n *contracts.DeliveryNote) { n.VehiclePlate = "B 99 XYZ" })
	c := sqlcMkNote(t, r, "DL-C-003", 1, 11, day(-10), "draft", nil)
	d := sqlcMkNote(t, r, "DL-D-004", 2, 10, day(0), "draft", nil) // other branch
	e := sqlcMkNote(t, r, "DL-E-005", 1, 11, day(-10), "confirmed", nil)
	_ = d
	_ = e
	ids := func(ps []*contracts.DeliveryNote) []int64 {
		var o []int64
		for _, p := range ps {
			o = append(o, p.ID)
		}
		return o
	}
	cases := []struct {
		name string
		f    dlinfra.ListFilter
		want []int64
	}{
		{"none", dlinfra.ListFilter{}, []int64{e, c, b, a}},
		{"status", dlinfra.ListFilter{Statuses: []string{"confirmed"}}, []int64{e}},
		{"so", dlinfra.ListFilter{SalesOrderID: 11}, []int64{e, c}},
		{"kind", dlinfra.ListFilter{DocumentKind: "replacement"}, nil},
		{"from", dlinfra.ListFilter{From: day(-3)}, []int64{b, a}},
		{"to", dlinfra.ListFilter{To: day(-3)}, []int64{e, c, b}},
		{"search driver", dlinfra.ListFilter{Search: "santoso"}, []int64{a}},
		{"search plate", dlinfra.ListFilter{Search: "99 xyz"}, []int64{b}},
		{"search none", dlinfra.ListFilter{Search: "zzz"}, nil},
		{"combined", dlinfra.ListFilter{Statuses: []string{"draft"}, SalesOrderID: 10, Search: "santoso"}, []int64{a}},
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
	l, err := r.List(ctx, 1, 11, "draft,confirmed", 50, 0)
	if err != nil || !reflect.DeepEqual(ids(l), []int64{e, c}) {
		t.Fatalf("list csv: %v %v", ids(l), err)
	}
	if n, err := r.Count(ctx, 1, 0, ""); err != nil || n != 4 {
		t.Fatalf("count: %d %v", n, err)
	}

	// WaitingReturn age bands (draft order-kind only).
	w, err := r.WaitingReturn(ctx, 1, "", nil, "", "", "today", 50)
	if err != nil || !reflect.DeepEqual(ids(w), []int64{a}) {
		t.Fatalf("today: %v %v", ids(w), err)
	}
	w, _ = r.WaitingReturn(ctx, 1, "", nil, "", "", "gt_2", 50)
	if !reflect.DeepEqual(ids(w), []int64{c, b}) {
		t.Fatalf("gt_2: %v", ids(w))
	}
	w, _ = r.WaitingReturn(ctx, 1, "", nil, "", "", "gt_7", 50)
	if !reflect.DeepEqual(ids(w), []int64{c}) {
		t.Fatalf("gt_7: %v", ids(w))
	}
	w, _ = r.WaitingReturn(ctx, 1, "", nil, day(-3), day(-3), "", 50)
	if !reflect.DeepEqual(ids(w), []int64{b}) {
		t.Fatalf("range: %v", ids(w))
	}
	w, _ = r.WaitingReturn(ctx, 1, "", nil, "", "", "", 1)
	if len(w) != 1 || w[0].ID != c {
		t.Fatalf("limit oldest first: %+v", w)
	}

	// ConfirmedAfter.
	ca, err := r.ConfirmedAfter(ctx, 1, 0, 10)
	if err != nil || len(ca) != 1 || ca[0].ID != e || ca[0].Number != "DL-E-005" || ca[0].Status != "confirmed" {
		t.Fatalf("confirmed after: %+v %v", ca, err)
	}
	if ca, _ := r.ConfirmedAfter(ctx, 1, e, 5); len(ca) != 0 {
		t.Fatalf("confirmed end: %+v", ca)
	}

	// OrderIDsByNotes.
	om, err := r.OrderIDsByNotes(ctx, []int64{a, c, a, 0, 999})
	if err != nil || !reflect.DeepEqual(om, map[int64]int64{a: 10, c: 11}) {
		t.Fatalf("order ids: %v %v", om, err)
	}
	if om, err := r.OrderIDsByNotes(ctx, nil); err != nil || len(om) != 0 {
		t.Fatalf("order ids empty: %v %v", om, err)
	}
	big := make([]int64, 0, 2500)
	for i := int64(1); i <= 2500; i++ {
		big = append(big, i)
	}
	if om, err := r.OrderIDsByNotes(ctx, big); err != nil || len(om) != 5 {
		t.Fatalf("order ids big: %d %v", len(om), err)
	}
}
