package integration

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"sort"
	"testing"

	"mini-erp/internal/modules/payment/contracts"
	paymentinfra "mini-erp/internal/modules/payment/infrastructure"
)

// Characterization tests for the payment Repository (F8-18).

func sqlcPayRepo() *paymentinfra.Repository { return paymentinfra.NewRepository(testMods.db) }

func sqlcMkPay(t *testing.T, r *paymentinfra.Repository, number string, branch, party int64, status string, paidAt string, allocs ...*contracts.Allocation) int64 {
	t.Helper()
	p := &contracts.Payment{
		Number: number, BranchID: branch, PartyID: party, PartyType: "customer", Direction: "in",
		Amount: 1000, Method: "cash", PaidAt: paidAt, Allocations: allocs,
	}
	id, err := r.CreatePayment(context.Background(), p, 3, nil)
	if err != nil {
		t.Fatalf("create %s: %v", number, err)
	}
	if status != "active" {
		if err := r.SetStatus(context.Background(), id, status, 3); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func TestSqlcPaymentCreateAndGet(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPayRepo()

	full := &contracts.Payment{
		Number: "PAY-1", BranchID: 1, PartyID: 10, PartyType: "customer", Direction: "in", Amount: 5000,
		Method: "transfer", PaidAt: "2026-10-05 13:30:15", Notes: "catatan", ClientRef: "ref-1",
		AmountTendered: 6000, Reference: "TRF-9",
		Allocations: []*contracts.Allocation{{OrderType: "sales", OrderID: 7, Amount: 3000}, {OrderType: "sales", OrderID: 8, Amount: 2000}},
	}
	id, err := r.CreatePayment(ctx, full, 4, nil)
	if err != nil || id == 0 {
		t.Fatalf("create: %d %v", id, err)
	}
	got, err := r.GetPayment(ctx, id)
	if err != nil || got == nil {
		t.Fatalf("get: %v %v", got, err)
	}
	want := &contracts.Payment{
		ID: id, Number: "PAY-1", BranchID: 1, PartyID: 10, PartyType: "customer", Direction: "in", Amount: 5000,
		Method: "transfer", PaidAt: "2026-10-05 13:30:15", Notes: "catatan", Status: "active", ClientRef: "ref-1",
		AmountTendered: 6000, Reference: "TRF-9",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("payment mismatch:\n got %+v\nwant %+v", got, want)
	}
	var cb, ub int64
	if err := testMods.db.QueryRow("SELECT created_by, updated_by FROM payments WHERE id=?", id).Scan(&cb, &ub); err != nil || cb != 4 || ub != 4 {
		t.Fatalf("audit cols %d %d %v", cb, ub, err)
	}

	// date-only paid_at (what the service sends) and NULL columns
	id2 := sqlcMkPay(t, r, "PAY-2", 1, 10, "active", "2026-10-06")
	g2, _ := r.GetPayment(ctx, id2)
	if g2.PaidAt != "2026-10-06 00:00:00" || g2.Notes != "" || g2.ClientRef != "" || g2.Reference != "" || g2.AmountTendered != 0 || g2.PairID != 0 {
		t.Fatalf("null columns mismatch: %+v", g2)
	}

	if g, err := r.GetPayment(ctx, 99999); err != nil || g != nil {
		t.Fatalf("missing: %v %v", g, err)
	}
	if g, err := r.PaymentByClientRef(ctx, 1, "ref-1"); err != nil || g == nil || g.ID != id {
		t.Fatalf("by client ref: %v %v", g, err)
	}
	if g, err := r.PaymentByClientRef(ctx, 2, "ref-1"); err != nil || g != nil {
		t.Fatalf("other branch must miss: %v %v", g, err)
	}
	if g, err := r.PaymentByClientRef(ctx, 1, "nope"); err != nil || g != nil {
		t.Fatalf("unknown ref: %v %v", g, err)
	}

	// allocations in row order
	al, err := r.AllocationsByPayment(ctx, id)
	if err != nil || len(al) != 2 || al[0].OrderID != 7 || al[1].OrderID != 8 || al[0].Amount != 3000 || al[0].OrderType != "sales" {
		t.Fatalf("allocations: %v %v", al, err)
	}
	if al, err := r.AllocationsByPayment(ctx, id2); err != nil || len(al) != 0 {
		t.Fatalf("no allocations: %v %v", al, err)
	}

	// duplicate number / client ref are rejected and leave nothing behind
	dup := *full
	dup.ClientRef = "other"
	if _, err := r.CreatePayment(ctx, &dup, 4, nil); err == nil {
		t.Fatal("duplicate number must fail")
	}
	var n int
	_ = testMods.db.QueryRow("SELECT COUNT(*) FROM payment_allocations").Scan(&n)
	if n != 2 {
		t.Fatalf("allocations after failed create: %d", n)
	}
}

func TestSqlcPaymentCreateHookAtomicity(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPayRepo()
	var hookID int64
	id, err := r.CreatePayment(ctx, &contracts.Payment{
		Number: "H-1", BranchID: 1, PartyID: 1, PartyType: "customer", Direction: "in", Amount: 10, Method: "cash", PaidAt: "2026-10-01",
		Allocations: []*contracts.Allocation{{OrderType: "sales", OrderID: 1, Amount: 10}},
	}, 1, func(ctx context.Context, tx *sql.Tx, id int64) error {
		hookID = id
		var n int
		// the hook sees the uncommitted rows on its own tx
		return tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM payment_allocations WHERE payment_id = ?", id).Scan(&n)
	})
	if err != nil || hookID != id {
		t.Fatalf("hook id %d vs %d err %v", hookID, id, err)
	}
	boom := errors.New("boom")
	if _, err := r.CreatePayment(ctx, &contracts.Payment{
		Number: "H-2", BranchID: 1, PartyID: 1, PartyType: "customer", Direction: "in", Amount: 10, Method: "cash", PaidAt: "2026-10-01",
		Allocations: []*contracts.Allocation{{OrderType: "sales", OrderID: 2, Amount: 10}},
	}, 1, func(context.Context, *sql.Tx, int64) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("hook error must surface: %v", err)
	}
	var np, na int
	_ = testMods.db.QueryRow("SELECT COUNT(*) FROM payments").Scan(&np)
	_ = testMods.db.QueryRow("SELECT COUNT(*) FROM payment_allocations").Scan(&na)
	if np != 1 || na != 1 {
		t.Fatalf("hook failure must roll back: payments=%d allocs=%d", np, na)
	}
}

func TestSqlcPaymentPairAndCancel(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPayRepo()
	a := &contracts.Payment{Number: "P-A", BranchID: 1, PartyID: 5, PartyType: "customer", Direction: "in", Amount: 100, Method: "offset", PaidAt: "2026-10-01",
		Allocations: []*contracts.Allocation{{OrderType: "sales", OrderID: 1, Amount: 100}}}
	b := &contracts.Payment{Number: "P-B", BranchID: 1, PartyID: 5, PartyType: "customer", Direction: "out", Amount: 100, Method: "offset", PaidAt: "2026-10-01",
		Allocations: []*contracts.Allocation{{OrderType: "sales_return", OrderID: 2, Amount: 100}}}
	id1, id2, err := r.CreatePaymentPair(ctx, a, b, 2)
	if err != nil || id1 == 0 || id2 == 0 || id1 == id2 {
		t.Fatalf("pair: %d %d %v", id1, id2, err)
	}
	g1, _ := r.GetPayment(ctx, id1)
	g2, _ := r.GetPayment(ctx, id2)
	if g1.PairID != id2 || g2.PairID != id1 {
		t.Fatalf("pair links: %d %d", g1.PairID, g2.PairID)
	}
	if p, err := r.PairedPayment(ctx, id2); err != nil || p == nil || p.ID != id2 {
		t.Fatalf("paired: %v %v", p, err)
	}
	if p, err := r.PairedPayment(ctx, 0); err != nil || p != nil {
		t.Fatalf("paired 0: %v %v", p, err)
	}

	// a failing second leg rolls the first back
	dupNum := &contracts.Payment{Number: "P-A", BranchID: 1, PartyID: 5, PartyType: "customer", Direction: "in", Amount: 1, Method: "offset", PaidAt: "2026-10-01"}
	okLeg := &contracts.Payment{Number: "P-C", BranchID: 1, PartyID: 5, PartyType: "customer", Direction: "in", Amount: 1, Method: "offset", PaidAt: "2026-10-01"}
	if _, _, err := r.CreatePaymentPair(ctx, okLeg, dupNum, 2); err == nil {
		t.Fatal("pair with duplicate second number must fail")
	}
	if c, _ := r.Count(ctx, 1, 0, ""); c != 2 {
		t.Fatalf("failed pair must leave nothing: %d", c)
	}

	// CancelWithPair cancels both legs, hook runs once, CAS second call = 0
	hooks := 0
	n, err := r.CancelWithPair(ctx, id1, 8, func(context.Context, *sql.Tx) error { hooks++; return nil })
	if err != nil || n != 2 || hooks != 1 {
		t.Fatalf("cancel: n=%d hooks=%d err=%v", n, hooks, err)
	}
	g1, _ = r.GetPayment(ctx, id1)
	g2, _ = r.GetPayment(ctx, id2)
	if g1.Status != "cancelled" || g2.Status != "cancelled" {
		t.Fatalf("status: %s %s", g1.Status, g2.Status)
	}
	var ub int64
	_ = testMods.db.QueryRow("SELECT updated_by FROM payments WHERE id=?", id2).Scan(&ub)
	if ub != 8 {
		t.Fatalf("updated_by %d", ub)
	}
	n, err = r.CancelWithPair(ctx, id2, 8, func(context.Context, *sql.Tx) error { hooks++; return nil })
	if err != nil || n != 0 || hooks != 1 {
		t.Fatalf("second cancel: n=%d hooks=%d err=%v", n, hooks, err)
	}

	// hook failure rolls back the cancellation; single (unpaired) payment cancels alone
	solo := sqlcMkPay(t, r, "SOLO", 1, 5, "active", "2026-10-02")
	boom := errors.New("boom")
	if _, err := r.CancelWithPair(ctx, solo, 1, func(context.Context, *sql.Tx) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("hook err: %v", err)
	}
	if g, _ := r.GetPayment(ctx, solo); g.Status != "active" {
		t.Fatal("rolled back cancel must stay active")
	}
	if n, err := r.CancelWithPair(ctx, solo, 1, nil); err != nil || n != 1 {
		t.Fatalf("solo cancel (nil hook): %d %v", n, err)
	}
	if n, err := r.CancelWithPair(ctx, 424242, 1, nil); err != nil || n != 0 {
		t.Fatalf("missing: %d %v", n, err)
	}
}

func TestSqlcPaymentPaidForOrders(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPayRepo()
	sqlcMkPay(t, r, "A", 1, 1, "active", "2026-10-01", &contracts.Allocation{OrderType: "sales", OrderID: 100, Amount: 300}, &contracts.Allocation{OrderType: "sales", OrderID: 101, Amount: 50})
	sqlcMkPay(t, r, "B", 1, 1, "active", "2026-10-01", &contracts.Allocation{OrderType: "sales", OrderID: 100, Amount: 200})
	sqlcMkPay(t, r, "C", 1, 1, "cancelled", "2026-10-01", &contracts.Allocation{OrderType: "sales", OrderID: 100, Amount: 9999})
	sqlcMkPay(t, r, "D", 1, 1, "active", "2026-10-01", &contracts.Allocation{OrderType: "purchase", OrderID: 100, Amount: 7})

	if n, err := r.PaidForOrder(ctx, "sales", 100); err != nil || n != 500 {
		t.Fatalf("paid sales/100: %d %v", n, err)
	}
	if n, err := r.PaidForOrder(ctx, "purchase", 100); err != nil || n != 7 {
		t.Fatalf("paid purchase/100: %d %v", n, err)
	}
	if n, err := r.PaidForOrder(ctx, "sales", 555); err != nil || n != 0 {
		t.Fatalf("paid none: %d %v", n, err)
	}
	m, err := r.PaidForOrders(ctx, "sales", []int64{100, 101, 555})
	if err != nil || len(m) != 2 || m[100] != 500 || m[101] != 50 {
		t.Fatalf("batch: %v %v", m, err)
	}
	if m, err := r.PaidForOrders(ctx, "sales", nil); err != nil || m == nil || len(m) != 0 {
		t.Fatalf("empty batch must be empty non-nil map: %v %v", m, err)
	}
	// large IN list
	big := make([]int64, 0, 3000)
	for i := int64(1); i <= 3000; i++ {
		big = append(big, i)
	}
	m, err = r.PaidForOrders(ctx, "sales", big)
	if err != nil || m[100] != 500 || m[101] != 50 || len(m) != 2 {
		t.Fatalf("big batch: %v %v", m, err)
	}
}

func TestSqlcPaymentListCountPaging(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPayRepo()
	ids := []int64{}
	ids = append(ids, sqlcMkPay(t, r, "L1", 1, 10, "active", "2026-10-01")) // 1
	ids = append(ids, sqlcMkPay(t, r, "L2", 1, 11, "active", "2026-10-02")) // 2
	ids = append(ids, sqlcMkPay(t, r, "L3", 1, 10, "cancelled", "2026-10-03"))
	ids = append(ids, sqlcMkPay(t, r, "L4", 2, 10, "active", "2026-10-04")) // other branch
	ids = append(ids, sqlcMkPay(t, r, "L5", 1, 10, "active", "2026-10-05"))

	idsOf := func(ps []*contracts.Payment) []int64 {
		var out []int64
		for _, p := range ps {
			out = append(out, p.ID)
		}
		return out
	}
	cases := []struct {
		name          string
		party         int64
		status        string
		limit, offset int
		want          []int64
		total         int64
	}{
		{"branch only newest first", 0, "", 50, 0, []int64{ids[4], ids[2], ids[1], ids[0]}, 4},
		{"party", 10, "", 50, 0, []int64{ids[4], ids[2], ids[0]}, 3},
		{"status", 0, "cancelled", 50, 0, []int64{ids[2]}, 1},
		{"party+status", 10, "active", 50, 0, []int64{ids[4], ids[0]}, 2},
		{"page 1", 0, "", 2, 0, []int64{ids[4], ids[2]}, 4},
		{"page 2", 0, "", 2, 2, []int64{ids[1], ids[0]}, 4},
		{"beyond", 0, "", 2, 9, nil, 4},
		{"no match", 99, "", 50, 0, nil, 0},
	}
	for _, c := range cases {
		got, err := r.List(ctx, 1, c.party, c.status, c.limit, c.offset)
		if err != nil || !reflect.DeepEqual(idsOf(got), c.want) {
			t.Fatalf("%s: list %v err %v want %v", c.name, idsOf(got), err, c.want)
		}
		n, err := r.Count(ctx, 1, c.party, c.status)
		if err != nil || n != c.total {
			t.Fatalf("%s: count %d err %v want %d", c.name, n, err, c.total)
		}
	}
	// ledger = active payments of the party
	led, err := r.LedgerByParty(ctx, 1, 10, 50, 0)
	if err != nil || !reflect.DeepEqual(idsOf(led), []int64{ids[4], ids[0]}) {
		t.Fatalf("ledger: %v %v", idsOf(led), err)
	}
	// cancelled among
	ca, err := r.CancelledAmong(ctx, 1, []int64{ids[0], ids[2], ids[3], 777})
	if err != nil || !reflect.DeepEqual(ca, []int64{ids[2]}) {
		t.Fatalf("cancelled among: %v %v", ca, err)
	}
	if ca, err := r.CancelledAmong(ctx, 2, []int64{ids[2]}); err != nil || len(ca) != 0 {
		t.Fatalf("other branch: %v %v", ca, err)
	}
	if ca, err := r.CancelledAmong(ctx, 1, nil); err != nil || len(ca) != 0 {
		t.Fatalf("empty: %v %v", ca, err)
	}
	// ActiveAfter cursor paging ascending
	pg, err := r.ActiveAfter(ctx, 1, 0, 2)
	if err != nil || !reflect.DeepEqual(idsOf(pg), []int64{ids[0], ids[1]}) {
		t.Fatalf("active after 0: %v %v", idsOf(pg), err)
	}
	pg, err = r.ActiveAfter(ctx, 1, ids[1], 10)
	if err != nil || !reflect.DeepEqual(idsOf(pg), []int64{ids[4]}) {
		t.Fatalf("active after cursor: %v %v", idsOf(pg), err)
	}
	if pg, err := r.ActiveAfter(ctx, 1, ids[4], 10); err != nil || len(pg) != 0 {
		t.Fatalf("active after end: %v %v", pg, err)
	}
}

func TestSqlcPaymentAllocationsBatchAndOrderIDs(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPayRepo()
	p1 := sqlcMkPay(t, r, "A", 1, 1, "active", "2026-10-01", &contracts.Allocation{OrderType: "sales", OrderID: 5, Amount: 1}, &contracts.Allocation{OrderType: "sales", OrderID: 6, Amount: 2}, &contracts.Allocation{OrderType: "sales_return", OrderID: 9, Amount: 3})
	p2 := sqlcMkPay(t, r, "B", 1, 1, "active", "2026-10-01", &contracts.Allocation{OrderType: "sales", OrderID: 5, Amount: 4})
	p3 := sqlcMkPay(t, r, "C", 1, 1, "active", "2026-10-01")

	m, err := r.AllocationsByPayments(ctx, []int64{p1, p2, p3, 4242})
	if err != nil || len(m) != 2 || len(m[p1]) != 3 || len(m[p2]) != 1 {
		t.Fatalf("batch allocs: %v %v", m, err)
	}
	if m[p1][0].OrderID != 5 || m[p1][1].OrderID != 6 || m[p1][2].OrderType != "sales_return" || m[p2][0].Amount != 4 {
		t.Fatalf("order/contents: %+v %+v", m[p1], m[p2])
	}
	if m, err := r.AllocationsByPayments(ctx, nil); err != nil || m == nil || len(m) != 0 {
		t.Fatalf("empty: %v %v", m, err)
	}
	// more than one chunk (listquery.MaxIn = 1000)
	big := make([]int64, 0, 2500)
	for i := int64(1); i <= 2500; i++ {
		big = append(big, i)
	}
	m, err = r.AllocationsByPayments(ctx, big)
	if err != nil || len(m[p1]) != 3 || len(m[p2]) != 1 {
		t.Fatalf("chunked: %v %v", len(m), err)
	}

	o, err := r.OrderIDsByPayments(ctx, []int64{p1, p1, 0, p2, p3, 4242})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(o[p1], func(i, j int) bool { return o[p1][i] < o[p1][j] })
	if !reflect.DeepEqual(o, map[int64][]int64{p1: {5, 6}, p2: {5}}) {
		t.Fatalf("order ids (sales only): %v", o)
	}
	if o, err := r.OrderIDsByPayments(ctx, nil); err != nil || o == nil || len(o) != 0 {
		t.Fatalf("empty order ids: %v %v", o, err)
	}
	if o, err := r.OrderIDsByPayments(ctx, []int64{0, 0}); err != nil || len(o) != 0 {
		t.Fatalf("zero ids: %v %v", o, err)
	}
}

func TestSqlcPaymentListFiltered(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPayRepo()
	mk := func(num string, branch, party int64, dir, method, paidAt, notes, ref string, allocs ...*contracts.Allocation) int64 {
		id, err := r.CreatePayment(ctx, &contracts.Payment{Number: num, BranchID: branch, PartyID: party, PartyType: "customer",
			Direction: dir, Amount: 10, Method: method, PaidAt: paidAt, Notes: notes, Reference: ref, Allocations: allocs}, 1, nil)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	a := mk("PAY-0001", 1, 10, "in", "cash", "2026-10-01 08:00:00", "lunas tunai", "", &contracts.Allocation{OrderType: "sales", OrderID: 500, Amount: 10})
	b := mk("PAY-0002", 1, 11, "in", "transfer", "2026-10-02 10:00:00", "", "TRF-ABC", &contracts.Allocation{OrderType: "sales", OrderID: 501, Amount: 10})
	c := mk("PAY-0003", 1, 10, "out", "cash", "2026-10-03 23:59:59", "refund 100%", "")
	d := mk("PAY-0004", 2, 10, "in", "cash", "2026-10-03", "", "")
	if err := r.SetStatus(ctx, c, "cancelled", 1); err != nil {
		t.Fatal(err)
	}
	_ = d
	ids := func(ps []*contracts.Payment) []int64 {
		var o []int64
		for _, p := range ps {
			o = append(o, p.ID)
		}
		return o
	}
	cases := []struct {
		name string
		f    paymentinfra.ListFilter
		want []int64
	}{
		{"none", paymentinfra.ListFilter{}, []int64{c, b, a}},
		{"party", paymentinfra.ListFilter{PartyID: 10}, []int64{c, a}},
		{"status", paymentinfra.ListFilter{Status: "cancelled"}, []int64{c}},
		{"method", paymentinfra.ListFilter{Method: "transfer"}, []int64{b}},
		{"direction", paymentinfra.ListFilter{Direction: "out"}, []int64{c}},
		{"from", paymentinfra.ListFilter{From: "2026-10-02"}, []int64{c, b}},
		{"to inclusive upper day", paymentinfra.ListFilter{To: "2026-10-03"}, []int64{c, b, a}},
		{"to excludes later", paymentinfra.ListFilter{To: "2026-10-02"}, []int64{b, a}},
		{"range", paymentinfra.ListFilter{From: "2026-10-02", To: "2026-10-02"}, []int64{b}},
		{"bad dates ignored", paymentinfra.ListFilter{From: "x", To: "2026-13-99"}, []int64{c, b, a}},
		{"search number", paymentinfra.ListFilter{Search: "0002"}, []int64{b}},
		{"search notes", paymentinfra.ListFilter{Search: "lunas"}, []int64{a}},
		{"search reference", paymentinfra.ListFilter{Search: "abc"}, []int64{b}},
		{"search two words AND", paymentinfra.ListFilter{Search: "refund PAY"}, []int64{c}},
		{"search literal percent", paymentinfra.ListFilter{Search: "100%"}, []int64{c}},
		{"search underscore is literal", paymentinfra.ListFilter{Search: "PAY_0001"}, nil},
		{"search or party ids", paymentinfra.ListFilter{Search: "lunas", SearchPartyIDs: []int64{11}}, []int64{b, a}},
		{"search party ids only matter with search", paymentinfra.ListFilter{SearchPartyIDs: []int64{11}}, []int64{c, b, a}},
		{"search no match", paymentinfra.ListFilter{Search: "zzz"}, nil},
		{"order alloc", paymentinfra.ListFilter{OrderType: "sales", OrderID: 501}, []int64{b}},
		{"order alloc wrong type", paymentinfra.ListFilter{OrderType: "purchase", OrderID: 501}, nil},
		{"combined", paymentinfra.ListFilter{PartyID: 10, Status: "active", Method: "cash", Direction: "in", From: "2026-10-01", To: "2026-10-01"}, []int64{a}},
	}
	for _, tc := range cases {
		got, err := r.ListFiltered(ctx, 1, tc.f, 50, 0)
		if err != nil || !reflect.DeepEqual(ids(got), tc.want) {
			t.Fatalf("%s: list %v err %v want %v", tc.name, ids(got), err, tc.want)
		}
		n, err := r.CountFiltered(ctx, 1, tc.f)
		if err != nil || int(n) != len(tc.want) {
			t.Fatalf("%s: count %d err %v want %d", tc.name, n, err, len(tc.want))
		}
	}
	// paging through the deferred join
	pg, err := r.ListFiltered(ctx, 1, paymentinfra.ListFilter{}, 1, 1)
	if err != nil || !reflect.DeepEqual(ids(pg), []int64{b}) {
		t.Fatalf("page: %v %v", ids(pg), err)
	}
	if pg, err := r.ListFiltered(ctx, 1, paymentinfra.ListFilter{}, 5, 10); err != nil || len(pg) != 0 {
		t.Fatalf("beyond: %v %v", pg, err)
	}
	// full row mapping through ListFiltered
	pg, _ = r.ListFiltered(ctx, 1, paymentinfra.ListFilter{Status: "cancelled"}, 5, 0)
	if len(pg) != 1 || pg[0].PaidAt != "2026-10-03 23:59:59" || pg[0].Notes != "refund 100%" {
		t.Fatalf("mapping: %+v", pg)
	}
}
