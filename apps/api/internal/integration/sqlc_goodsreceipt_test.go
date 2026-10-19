package integration

import (
	"context"
	"testing"

	grcontracts "mini-erp/internal/modules/goodsreceipt/contracts"
	grinfra "mini-erp/internal/modules/goodsreceipt/infrastructure"
)

// Characterization tests for the goodsreceipt Repository (F8-18). The
// repository only touches its own tables, so receipts are created directly.
func TestSqlcGoodsReceiptRepository(t *testing.T) {
	c := context.Background()
	freshDB(t)
	repo := grinfra.NewRepository(testMods.db)

	item := func(prod, variant, loc int64, uom string, factor, qty, base float64) *grcontracts.Item {
		return &grcontracts.Item{ProductID: prod, VariantID: variant, LocationID: loc, UOM: uom, UOMFactor: factor, Qty: qty, QtyBase: base}
	}
	create := func(number string, branch, po int64, at, ref string, items ...*grcontracts.Item) int64 {
		t.Helper()
		id, err := repo.CreateReceipt(c, &grcontracts.GoodsReceipt{Number: number, BranchID: branch, PurchaseOrderID: po,
			ReceivedAt: at, Notes: "", SupplierDeliveryNumber: "", ClientRef: ref, Items: items}, 7)
		if err != nil {
			t.Fatalf("create %s: %v", number, err)
		}
		return id
	}

	// r1: PO 10 branch 1; r2: PO 10 branch 1; r3: PO 10 branch 2; r4: PO 11 branch 1.
	r1 := create("GR-1", 1, 10, "2024-05-10", "ref-1",
		item(1, 0, 5, "pcs", 1, 3, 3), item(2, 3, 5, "box", 12, 1.5, 18), item(1, 0, 6, " pcs ", 1, 0.000000000001, 0.000000000001))
	r2 := create("GR-2", 1, 10, "2024-05-11 13:14:15", "", item(1, 0, 5, "pcs", 1, 4, 4), item(2, 3, 5, "box", 12, 2, 24))
	r3 := create("", 2, 10, "2024-05-12T01:02:03", "ref-1", item(1, 0, 5, "pcs", 1, 100, 100))
	r4 := create("GR-4", 1, 11, "2024-05-13", "", item(9, 0, 5, "pcs", 1, 1, 1))
	if _, err := repo.CreateReceipt(c, &grcontracts.GoodsReceipt{Number: "GR-1", BranchID: 1, PurchaseOrderID: 10, ReceivedAt: "2024-05-10"}, 1); err == nil {
		t.Fatal("duplicate number must fail")
	}
	if _, err := repo.CreateReceipt(c, &grcontracts.GoodsReceipt{Number: "GR-9", BranchID: 1, PurchaseOrderID: 10, ReceivedAt: "2024-05-10", ClientRef: "ref-1"}, 1); err == nil {
		t.Fatal("duplicate (branch, client_ref) must fail")
	}
	// A failed insert (bad item) rolls the header back.
	var before int
	_ = testMods.db.QueryRow("SELECT COUNT(*) FROM goods_receipts").Scan(&before)
	if _, err := repo.CreateReceipt(c, &grcontracts.GoodsReceipt{Number: "GR-ROLL", BranchID: 1, PurchaseOrderID: 10, ReceivedAt: "2024-05-10",
		Items: []*grcontracts.Item{item(1, 0, 5, "pcs", 1, 1, 1), item(1, 0, 5, "this-uom-is-way-longer-than-thirty-chars-for-sure", 1, 1, 1)}}, 1); err == nil {
		t.Fatal("oversize uom must fail")
	}
	var after int
	_ = testMods.db.QueryRow("SELECT COUNT(*) FROM goods_receipts").Scan(&after)
	if after != before {
		t.Fatalf("header must roll back: %d -> %d", before, after)
	}

	// GetReceipt: all fields; NULL optionals read ""; status starts pending.
	g, err := repo.GetReceipt(c, r1)
	if err != nil || g == nil || g.ID != r1 || g.Number != "GR-1" || g.BranchID != 1 || g.PurchaseOrderID != 10 ||
		g.ReceivedAt != "2024-05-10 00:00:00" || g.Notes != "" || g.SupplierDeliveryNumber != "" || g.Status != "pending" || g.ClientRef != "ref-1" {
		t.Fatalf("get r1: %+v %v", g, err)
	}
	g, _ = repo.GetReceipt(c, r2)
	if g.ReceivedAt != "2024-05-11 13:14:15" || g.ClientRef != "" {
		t.Fatalf("get r2: %+v", g)
	}
	g, _ = repo.GetReceipt(c, r3)
	if g.Number != "" || g.ReceivedAt != "2024-05-12 01:02:03" {
		t.Fatalf("get r3 (NULL number): %+v", g)
	}
	if g, err := repo.GetReceipt(c, 999999); err != nil || g != nil {
		t.Fatalf("missing: %v %v", g, err)
	}
	// notes / supplier delivery number persisted when given.
	rn, err := repo.CreateReceipt(c, &grcontracts.GoodsReceipt{Number: "GR-N", BranchID: 3, PurchaseOrderID: 50, ReceivedAt: "2024-06-01",
		Notes: "catatan", SupplierDeliveryNumber: "SJ-77"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if g, _ := repo.GetReceipt(c, rn); g.Notes != "catatan" || g.SupplierDeliveryNumber != "SJ-77" {
		t.Fatalf("notes/sj: %+v", g)
	}

	// ReceiptByClientRef is branch scoped.
	if g, err := repo.ReceiptByClientRef(c, 1, "ref-1"); err != nil || g == nil || g.ID != r1 {
		t.Fatalf("ref b1: %v %v", g, err)
	}
	if g, _ := repo.ReceiptByClientRef(c, 2, "ref-1"); g == nil || g.ID != r3 {
		t.Fatalf("ref b2: %v", g)
	}
	if g, err := repo.ReceiptByClientRef(c, 3, "ref-1"); err != nil || g != nil {
		t.Fatalf("ref b3: %v %v", g, err)
	}

	// ItemsByReceipt: row order, 12-decimal precision, empty receipt.
	its, err := repo.ItemsByReceipt(c, r1)
	if err != nil || len(its) != 3 || its[0].ProductID != 1 || its[1].VariantID != 3 || its[1].UOMFactor != 12 || its[1].Qty != 1.5 || its[1].QtyBase != 18 ||
		its[2].Qty != 0.000000000001 || its[2].UOM != " pcs " || its[0].ID >= its[1].ID || its[1].ID >= its[2].ID || its[0].LocationID != 5 {
		t.Fatalf("items: %+v %v", its, err)
	}
	if its, err := repo.ItemsByReceipt(c, rn); err != nil || len(its) != 0 {
		t.Fatalf("no items: %v %v", its, err)
	}

	// Transition: CAS from allowed origins only.
	if ok, err := repo.Transition(c, r1, []string{"posted"}, "discarded"); err != nil || ok {
		t.Fatalf("wrong origin: %v %v", ok, err)
	}
	if ok, err := repo.Transition(c, r1, nil, "posted"); err != nil || ok {
		t.Fatalf("empty from: %v %v", ok, err)
	}
	if ok, err := repo.Transition(c, 999999, []string{"pending"}, "posted"); err != nil || ok {
		t.Fatalf("unknown id: %v %v", ok, err)
	}
	for _, id := range []int64{r1, r3, r4} {
		if ok, err := repo.Transition(c, id, []string{"pending", "x"}, "posted"); err != nil || !ok {
			t.Fatalf("post %d: %v %v", id, ok, err)
		}
	}
	if ok, _ := repo.Transition(c, r1, []string{"pending"}, "posted"); ok {
		t.Fatal("second transition must lose the CAS")
	}
	// r2 stays pending; rn discarded.
	if ok, _ := repo.Transition(c, rn, []string{"pending"}, "discarded"); !ok {
		t.Fatal("discard rn")
	}

	// ReceivedQty: posted only; per product/variant; none -> 0.
	q, err := repo.ReceivedQty(c, 10, 1, 0)
	if err != nil || q != 3.000000000001+100 {
		t.Fatalf("received qty: %v %v", q, err)
	}
	if q, _ := repo.ReceivedQty(c, 10, 2, 3); q != 18 {
		t.Fatalf("received v3: %v", q)
	}
	if q, err := repo.ReceivedQty(c, 10, 77, 0); err != nil || q != 0 {
		t.Fatalf("received none: %v %v", q, err)
	}

	// QtyByPO: statuses filter, empty statuses, keys.
	m, err := repo.QtyByPO(c, 10, "posted")
	if err != nil || len(m) != 2 || m["1/0"] != 103.000000000001 || m["2/3"] != 18 {
		t.Fatalf("qtybypo posted: %v %v", m, err)
	}
	m, _ = repo.QtyByPO(c, 10, "posted", "pending")
	if m["1/0"] != 107.000000000001 || m["2/3"] != 42 {
		t.Fatalf("qtybypo both: %v", m)
	}
	if m, err := repo.QtyByPO(c, 10); err != nil || len(m) != 0 {
		t.Fatalf("qtybypo no statuses: %v %v", m, err)
	}
	if m, _ := repo.QtyByPO(c, 10, "discarded"); len(m) != 0 {
		t.Fatalf("qtybypo none: %v", m)
	}

	// ActiveReceiptCount: pending + posted only.
	if n, err := repo.ActiveReceiptCount(c, 10); err != nil || n != 3 {
		t.Fatalf("active count: %d %v", n, err)
	}
	if n, _ := repo.ActiveReceiptCount(c, 50); n != 0 {
		t.Fatalf("active count discarded: %d", n)
	}

	// PostedAfter: cursor, branch, limit, ordering.
	pa, err := repo.PostedAfter(c, 1, 0, 10)
	if err != nil || len(pa) != 2 || pa[0].ID != r1 || pa[1].ID != r4 {
		t.Fatalf("posted after 0: %+v %v", pa, err)
	}
	if pa, _ := repo.PostedAfter(c, 1, r1, 10); len(pa) != 1 || pa[0].ID != r4 {
		t.Fatalf("posted after r1: %+v", pa)
	}
	if pa, _ := repo.PostedAfter(c, 1, 0, 1); len(pa) != 1 || pa[0].ID != r1 {
		t.Fatalf("posted limit 1: %+v", pa)
	}
	if pa, _ := repo.PostedAfter(c, 1, r4, 10); len(pa) != 0 {
		t.Fatalf("posted after last: %+v", pa)
	}

	// List / Count filters, ordering and pagination.
	ls, err := repo.List(c, grinfra.ListFilter{BranchID: 1}, 10, 0)
	if err != nil || len(ls) != 3 || ls[0].ID != r4 || ls[1].ID != r2 || ls[2].ID != r1 {
		t.Fatalf("list b1: %+v %v", ls, err)
	}
	if n, _ := repo.Count(c, grinfra.ListFilter{BranchID: 1}); n != 3 {
		t.Fatalf("count b1: %d", n)
	}
	if ls, _ := repo.List(c, grinfra.ListFilter{BranchID: 1, POID: 10}, 10, 0); len(ls) != 2 || ls[0].ID != r2 {
		t.Fatalf("list po: %+v", ls)
	}
	if n, _ := repo.Count(c, grinfra.ListFilter{BranchID: 1, POID: 10}); n != 2 {
		t.Fatalf("count po: %d", n)
	}
	if ls, _ := repo.List(c, grinfra.ListFilter{BranchID: 1, PostedOnly: true}, 10, 0); len(ls) != 2 || ls[0].ID != r4 || ls[1].ID != r1 {
		t.Fatalf("list posted: %+v", ls)
	}
	if n, _ := repo.Count(c, grinfra.ListFilter{BranchID: 1, PostedOnly: true, POID: 10}); n != 1 {
		t.Fatalf("count posted po: %d", n)
	}
	if ls, _ := repo.List(c, grinfra.ListFilter{BranchID: 1}, 1, 1); len(ls) != 1 || ls[0].ID != r2 {
		t.Fatalf("list page 2: %+v", ls)
	}
	if ls, _ := repo.List(c, grinfra.ListFilter{BranchID: 1}, 10, 10); len(ls) != 0 {
		t.Fatalf("list beyond: %+v", ls)
	}
	if ls, _ := repo.List(c, grinfra.ListFilter{BranchID: 99}, 10, 0); len(ls) != 0 {
		t.Fatalf("list unknown branch: %+v", ls)
	}
	if n, _ := repo.Count(c, grinfra.ListFilter{BranchID: 99}); n != 0 {
		t.Fatalf("count unknown branch: %d", n)
	}

	// PostedQtyBefore: key uses trimmed uom; id strictly below the cursor.
	pq, err := repo.PostedQtyBefore(c, 10, r3+1)
	if err != nil || pq["1/0/pcs"] != 103.000000000001 || pq["2/3/box"] != 1.5 || len(pq) != 2 {
		t.Fatalf("posted before: %v %v", pq, err)
	}
	if pq, _ := repo.PostedQtyBefore(c, 10, r1); len(pq) != 0 {
		t.Fatalf("posted before r1 (exclusive): %v", pq)
	}
	if pq, _ := repo.PostedQtyBefore(c, 10, r3); pq["1/0/pcs"] != 3.000000000001 {
		t.Fatalf("posted before r3: %v", pq)
	}

	// PurchaseOrderIDsByReceipts: dedupe, zero ids, unknown, empty, large.
	if m, err := repo.PurchaseOrderIDsByReceipts(c, nil); err != nil || len(m) != 0 {
		t.Fatalf("po ids empty: %v %v", m, err)
	}
	if m, _ := repo.PurchaseOrderIDsByReceipts(c, []int64{0, 0}); len(m) != 0 {
		t.Fatalf("po ids zeros: %v", m)
	}
	m2, err := repo.PurchaseOrderIDsByReceipts(c, []int64{r1, r1, r4, 0, 999999})
	if err != nil || len(m2) != 2 || m2[r1] != 10 || m2[r4] != 11 {
		t.Fatalf("po ids: %v %v", m2, err)
	}
	big := make([]int64, 0, 3000)
	for i := int64(1); i <= 3000; i++ {
		big = append(big, i)
	}
	if m, err := repo.PurchaseOrderIDsByReceipts(c, big); err != nil || len(m) != 5 {
		t.Fatalf("po ids big: %d %v", len(m), err)
	}
}
