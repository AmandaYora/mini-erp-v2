package integration

import (
	"context"
	"database/sql"
	"reflect"
	"strconv"
	"testing"
	"time"

	stcontracts "mini-erp/internal/modules/stock/contracts"
	stinfra "mini-erp/internal/modules/stock/infrastructure"
)

// stockTx begins a module transaction, rolling it back at test end unless
// the caller commits first (a second rollback is harmless).
func stockTx(t *testing.T, r *stinfra.Repository) (context.Context, *sql.Tx) {
	t.Helper()
	ctx := context.Background()
	tx, err := r.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	return ctx, tx
}

// Characterization tests for the stock Repository (F8-18): locations,
// balances, movements, transfers, reservations, product state, housekeeping.
// Money/stock flows through Post/Reverse/Hold are covered by the service
// tests; this file pins the repository's own read/write shapes.

func sqlcStockRepo() *stinfra.Repository { return stinfra.NewRepository(testMods.db) }

func sqlcMkLoc(t *testing.T, r *stinfra.Repository, branch int64, code, name string) int64 {
	t.Helper()
	id, err := r.CreateLocation(context.Background(), &stcontracts.Location{BranchID: branch, Code: code, Name: name}, 1)
	if err != nil {
		t.Fatalf("create location %s: %v", code, err)
	}
	return id
}

func TestSqlcStockLocations(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcStockRepo()

	a := sqlcMkLoc(t, r, 1, "A", "Gudang A")
	b := sqlcMkLoc(t, r, 1, "B", "Gudang B")
	if _, err := r.CreateLocation(ctx, &stcontracts.Location{BranchID: 1, Code: "A", Name: "dup"}, 1); err == nil {
		t.Fatal("duplicate (branch, code) must fail")
	}
	// same code in another branch is fine.
	_ = sqlcMkLoc(t, r, 2, "A", "Gudang A2")

	got, err := r.LocationByID(ctx, a)
	if err != nil || got == nil || got.Code != "A" || got.Name != "Gudang A" || got.BranchID != 1 ||
		got.Kind != "normal" || got.Status != "active" || got.IsSystem || got.ParentID != nil {
		t.Fatalf("location: %+v %v", got, err)
	}
	if got, err := r.LocationByID(ctx, 99999); err != nil || got != nil {
		t.Fatalf("missing: %v %v", got, err)
	}
	if got, err := r.LocationByCode(ctx, 1, "B"); err != nil || got == nil || got.ID != b {
		t.Fatalf("by code: %+v %v", got, err)
	}
	if got, err := r.LocationByCode(ctx, 1, "ZZZ"); err != nil || got != nil {
		t.Fatalf("code missing: %v %v", got, err)
	}

	// List: name order, optional status filter.
	ls, err := r.ListLocations(ctx, 1, "")
	if err != nil || len(ls) != 2 || ls[0].Code != "A" || ls[1].Code != "B" {
		t.Fatalf("list: %+v %v", ls, err)
	}
	if err := r.SetLocationStatus(ctx, b, "archived", 2); err != nil {
		t.Fatal(err)
	}
	if ls, _ := r.ListLocations(ctx, 1, "active"); len(ls) != 1 || ls[0].Code != "A" {
		t.Fatalf("list active: %+v", ls)
	}
	if ls, _ := r.ListLocations(ctx, 1, "archived"); len(ls) != 1 {
		t.Fatalf("list archived: %+v", ls)
	}
	if ls, _ := r.ListLocations(ctx, 99, ""); len(ls) != 0 {
		t.Fatalf("list unknown branch: %+v", ls)
	}

	// Leaf / children.
	if leaf, _ := r.IsLeaf(ctx, a); !leaf {
		t.Fatal("a must be a leaf")
	}
	child := sqlcMkLoc(t, r, 1, "A-1", "Bin")
	tx, err := r.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	lc := &stcontracts.Location{ID: child, Name: "Bin"}
	// parent assignment goes through the Tx variant.
	_ = tx.Rollback()
	if err := func() error {
		tx2, err := r.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = tx2.Rollback() }()
		parentID := a
		lc.ParentID = &parentID
		if err := r.UpdateLocationTx(ctx, tx2, lc, 3); err != nil {
			return err
		}
		return tx2.Commit()
	}(); err != nil {
		t.Fatal(err)
	}
	if leaf, _ := r.IsLeaf(ctx, a); leaf {
		t.Fatal("a must not be a leaf anymore")
	}
	if has, _ := r.HasActiveChildren(ctx, a); !has {
		t.Fatal("a must have active children")
	}
	// archiving the child restores leafness.
	if err := r.SetLocationStatus(ctx, child, "archived", 1); err != nil {
		t.Fatal(err)
	}
	if leaf, _ := r.IsLeaf(ctx, a); !leaf {
		t.Fatal("a must be a leaf again")
	}
	// standalone update + rename.
	if err := r.UpdateLocation(ctx, &stcontracts.Location{ID: b, Name: "Gudang B2"}, 4); err != nil {
		t.Fatal(err)
	}
	if err := r.RenameLocation(ctx, b, "Gudang B3"); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.LocationByID(ctx, b); got.Name != "Gudang B3" {
		t.Fatalf("renamed: %+v", got)
	}

	// Balances gate the parent: nonzero position blocks, zero does not.
	if has, _ := r.LocationHasBalances(ctx, a); has {
		t.Fatal("a must have no balances yet")
	}
	tx3, err := r.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.EnsureBalanceTx(ctx, tx3, 1, 1, 0, a); err != nil {
		_ = tx3.Rollback()
		t.Fatal(err)
	}
	if err := r.AddBalanceTx(ctx, tx3, 1, 1, 0, a, 5, 0); err != nil {
		_ = tx3.Rollback()
		t.Fatal(err)
	}
	if err := tx3.Commit(); err != nil {
		t.Fatal(err)
	}
	if has, _ := r.LocationHasBalances(ctx, a); !has {
		t.Fatal("a must have balances now")
	}

	// System locations are idempotent (second call returns the same row).
	s1, err := r.EnsureSystemLocation(ctx, 1, "damaged", "RUSAK", "Rusak")
	if err != nil || s1 == nil || s1.Kind != "damaged" {
		t.Fatalf("system: %+v %v", s1, err)
	}
	s2, err := r.EnsureSystemLocation(ctx, 1, "damaged", "RUSAK", "Rusak")
	if err != nil || s2.ID != s1.ID {
		t.Fatalf("system idempotent: %+v %v", s2, err)
	}
	s3, err := r.EnsureSystemLocation(ctx, 1, "default", "GDG-1", "Utama")
	if err != nil || s3.Kind != "normal" {
		t.Fatalf("default: %+v %v", s3, err)
	}
}

func TestSqlcStockBalances(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcStockRepo()
	l1 := sqlcMkLoc(t, r, 1, "L1", "L1")
	l2 := sqlcMkLoc(t, r, 1, "L2", "L2")
	dmg, err := r.EnsureSystemLocation(ctx, 1, "damaged", "RUSAK", "Rusak")
	if err != nil {
		t.Fatal(err)
	}

	add := func(branch, prod, variant, loc int64, onHand, reserved float64) {
		t.Helper()
		tx, err := r.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		if err := r.EnsureBalanceTx(ctx, tx, branch, prod, variant, loc); err != nil {
			t.Fatal(err)
		}
		// ensure twice is a no-op (no deadlock, no duplicate).
		if err := r.EnsureBalanceTx(ctx, tx, branch, prod, variant, loc); err != nil {
			t.Fatal(err)
		}
		if err := r.AddBalanceTx(ctx, tx, branch, prod, variant, loc, onHand, reserved); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	add(1, 1, 0, l1, 10.5, 2)
	add(1, 1, 0, l2, 4, 0)
	add(1, 2, 3, l1, 0, 0) // zero row stays readable but is not listed
	add(1, 1, 0, dmg.ID, 100, 0)

	// GetBalance: exact position; missing reads zero (never nil, never error).
	b, err := r.GetBalance(ctx, 1, 1, 0, l1)
	if err != nil || b.OnHand != 10.5 || b.Reserved != 2 || b.Available() != 8.5 {
		t.Fatalf("balance: %+v %v", b, err)
	}
	if b, err := r.GetBalance(ctx, 1, 9, 9, l1); err != nil || b == nil || b.OnHand != 0 {
		t.Fatalf("missing balance: %+v %v", b, err)
	}
	// Tx read of a missing row also reads zero.
	tx, _ := r.Begin(ctx)
	if b, err := r.GetBalanceTx(ctx, tx, 1, 9, 9, l1); err != nil || b.OnHand != 0 {
		_ = tx.Rollback()
		t.Fatalf("tx missing: %+v %v", b, err)
	}
	_ = tx.Rollback()

	// Availability merges variants/locations and skips damaged.
	av, err := r.ProductAvailability(ctx, 1)
	if err != nil || !reflect.DeepEqual(av, map[int64]float64{1: 12.5, 2: 0}) {
		t.Fatalf("availability: %v %v", av, err)
	}
	pt, err := r.ProductTotals(ctx, 1)
	if err != nil || !reflect.DeepEqual(pt, map[int64][2]float64{1: {14.5, 2}, 2: {0, 0}}) {
		t.Fatalf("totals: %v %v", pt, err)
	}

	// SuggestLocations: active non-damaged with available > 0, location order.
	sg, err := r.SuggestLocations(ctx, 1, 1, 0)
	if err != nil || len(sg) != 2 || sg[0].LocationID != l1 || sg[1].LocationID != l2 {
		t.Fatalf("suggest: %+v %v", sg, err)
	}
	if sg, _ := r.SuggestLocations(ctx, 1, 9, 9); len(sg) != 0 {
		t.Fatalf("suggest none: %+v", sg)
	}

	// BalancesByLocation / ByProduct skip zero rows.
	bl, err := r.BalancesByLocation(ctx, l1)
	if err != nil || len(bl) != 1 || bl[0].ProductID != 1 {
		t.Fatalf("by location: %+v %v", bl, err)
	}
	// BalancesByProduct does not exclude damaged locations (unlike the
	// availability rollups): l1, l2 and the damaged row.
	bp, err := r.BalancesByProduct(ctx, 1, 1)
	if err != nil || len(bp) != 3 {
		t.Fatalf("by product: %+v %v", bp, err)
	}
	// Tx variant locks and reads the same rows.
	tx2, _ := r.Begin(ctx)
	blt, err := r.BalancesByLocationTx(ctx, tx2, l1)
	_ = tx2.Rollback()
	if err != nil || len(blt) != 1 {
		t.Fatalf("by location tx: %+v %v", blt, err)
	}

	// ListBalances matrix with exact totals.
	all, total, err := r.ListBalances(ctx, 1, stinfra.BalanceFilter{})
	if err != nil || total != 2 || len(all) != 2 {
		t.Fatalf("list all: %d %+v %v", total, all, err)
	}
	if _, total, _ := r.ListBalances(ctx, 1, stinfra.BalanceFilter{ProductID: 2}); total != 0 {
		t.Fatalf("zero rows are not listed: %d", total)
	}
	if _, total, _ := r.ListBalances(ctx, 1, stinfra.BalanceFilter{IncludeDamaged: true}); total != 3 {
		t.Fatalf("damaged: %d", total)
	}
	if _, total, _ := r.ListBalances(ctx, 1, stinfra.BalanceFilter{LocationID: l2}); total != 1 {
		t.Fatalf("location: %d", total)
	}
	if _, total, _ := r.ListBalances(ctx, 1, stinfra.BalanceFilter{Negative: true}); total != 0 {
		t.Fatalf("negative: %d", total)
	}
	if _, total, _ := r.ListBalances(ctx, 1, stinfra.BalanceFilter{RestrictIDs: true}); total != 0 {
		t.Fatalf("restrict empty: %d", total)
	}
	if _, total, _ := r.ListBalances(ctx, 1, stinfra.BalanceFilter{RestrictIDs: true, ProductIDs: []int64{1, 2}}); total != 2 {
		t.Fatalf("restrict: %d", total)
	}
	pg, total, err := r.ListBalances(ctx, 1, stinfra.BalanceFilter{Page: 2, Limit: 1})
	if err != nil || total != 2 || len(pg) != 1 || pg[0].LocationID != l2 {
		t.Fatalf("page: %d %+v %v", total, pg, err)
	}
	if pg, total, _ := r.ListBalances(ctx, 1, stinfra.BalanceFilter{Page: 9, Limit: 1}); total != 2 || len(pg) != 0 {
		t.Fatalf("beyond: %d %+v", total, pg)
	}
}

func TestSqlcStockMovements(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcStockRepo()
	loc := sqlcMkLoc(t, r, 1, "M", "M")
	at := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)

	post := func(refType string, refID int64, dir string, qty float64, movedAt time.Time) {
		t.Helper()
		tx, err := r.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		if err := r.EnsureBalanceTx(ctx, tx, 1, 1, 0, loc); err != nil {
			t.Fatal(err)
		}
		if err := r.InsertMovementTx(ctx, tx, stinfra.MovementRow{
			BranchID: 1, ProductID: 1, VariantID: 0, LocationID: loc,
			Direction: dir, MovementType: dir, Qty: qty, Before: 0, After: qty,
			RefType: refType, RefID: refID, ActorID: 1, MovedAt: movedAt,
		}); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	post("opening", 1, "in", 5, at)
	post("delivery", 2, "out", 2, at.Add(time.Hour))

	// Duplicate (ref, position, direction) is rejected (1062: replay signal).
	tx, _ := r.Begin(ctx)
	dupErr := r.InsertMovementTx(ctx, tx, stinfra.MovementRow{
		BranchID: 1, ProductID: 1, VariantID: 0, LocationID: loc,
		Direction: "in", MovementType: "in", Qty: 5, RefType: "opening", RefID: 1,
	})
	_ = tx.Rollback()
	if dupErr == nil {
		t.Fatal("duplicate movement must fail")
	}

	_, txe := stockTx(t, r)
	if ok, _ := r.MovementExistsTx(ctx, txe, "opening", 1, 1, 0, loc, "in"); !ok {
		t.Fatal("movement must exist")
	}
	if ok, _ := r.MovementExistsTx(ctx, txe, "opening", 1, 1, 0, loc, "out"); ok {
		t.Fatal("other direction must not exist")
	}
	if ok, _ := r.HasRefMovements(ctx, 1, "delivery", 2); !ok {
		t.Fatal("ref must have movements")
	}
	if ok, _ := r.HasRefMovements(ctx, 1, "delivery", 999); ok {
		t.Fatal("unknown ref must not")
	}
	_, txb := stockTx(t, r)
	byRef, err := r.MovementsByRefTx(ctx, txb, 1, "opening", 1)
	if err != nil || len(byRef) != 1 || byRef[0].Qty != 5 || byRef[0].Direction != "in" {
		t.Fatalf("by ref: %+v %v", byRef, err)
	}
	if got := stinfra.PositionKey(1, 0, loc); got != "1:"+strconv.FormatInt(0, 10)+":"+strconv.FormatInt(loc, 10) {
		t.Fatalf("position key: %q", got)
	}
	mp, err := r.MovedPositions(ctx, 1)
	if err != nil || !mp[stinfra.PositionKey(1, 0, loc)] || len(mp) != 1 {
		t.Fatalf("moved: %v %v", mp, err)
	}
	if ok, _ := r.HasMovements(ctx, 1, 1, 0); !ok {
		t.Fatal("has movements")
	}
	if ok, _ := r.HasMovements(ctx, 1, 9, 9); ok {
		t.Fatal("no movements")
	}
	if ok, _ := r.HasMovementsAnywhere(ctx, 1); !ok {
		t.Fatal("anywhere")
	}

	// Cost cursors: explicit moved_at keeps its microseconds text.
	cm, err := r.ListCostMovements(ctx, 1, 0, 10)
	if err != nil || len(cm) != 2 || cm[0].MovedAt != "2026-10-05 10:00:00.000000" || cm[0].UnitCost != 0 || cm[0].RefID != 1 {
		var got []stcontracts.CostMovement
		for _, m := range cm {
			got = append(got, *m)
		}
		t.Fatalf("cost list: %+v %v", got, err)
	}
	if cm, _ := r.ListCostMovements(ctx, 1, cm[0].ID, 10); len(cm) != 1 {
		t.Fatalf("cost cursor: %+v", cm)
	}
	rc, err := r.RecentCostMovements(ctx, 1, 60, 10)
	if err != nil || len(rc) != 2 {
		t.Fatalf("recent: %+v %v", rc, err)
	}
	if rc, _ := r.RecentCostMovements(ctx, 1, 60, 1); len(rc) != 1 {
		t.Fatalf("recent limit: %+v", rc)
	}

	// History list matrix + count.
	lm, err := r.ListMovements(ctx, 1, 1, 0, loc, "", "", "", 10, 0)
	if err != nil || len(lm) != 2 || lm[0].Direction != "out" || lm[0].Qty != 2 || lm[0].RefID != 2 {
		t.Fatalf("movements: %+v %v", lm, err)
	}
	if n, _ := r.CountMovements(ctx, 1, 1, 0, loc, "", "", ""); n != 2 {
		t.Fatalf("count: %d", n)
	}
	if lm, _ := r.ListMovements(ctx, 1, 1, 0, 0, "in", "", "", 10, 0); len(lm) != 1 {
		t.Fatalf("type filter: %+v", lm)
	}
	if lm, _ := r.ListMovements(ctx, 1, 0, 0, 0, "", "2026-10-05", "2026-10-05", 10, 0); len(lm) != 0 {
		t.Fatalf("date day bounds exclude timed rows: %+v", lm)
	}
	if lm, _ := r.ListMovements(ctx, 0, 0, 0, 0, "", "", "", 1, 1); len(lm) != 1 {
		t.Fatalf("page: %+v", lm)
	}
}

func TestSqlcStockTransfersReservations(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcStockRepo()
	l1 := sqlcMkLoc(t, r, 1, "T1", "T1")
	l2 := sqlcMkLoc(t, r, 1, "T2", "T2")

	mkTransfer := func(num string, fromBranch, toBranch int64) int64 {
		t.Helper()
		id, err := r.CreateTransfer(ctx, &stinfra.Transfer{
			Number: num, FromBranchID: fromBranch, ToBranchID: toBranch,
			FromLocation: l1, ToLocation: l2, Notes: "n-" + num,
			Items: []stinfra.TransferItem{
				{ProductID: 1, VariantID: 0, Qty: 3, Notes: "a", ToLocation: l2},
				{ProductID: 2, VariantID: 1, Qty: 1},
			},
		}, 7)
		if err != nil {
			t.Fatalf("create %s: %v", num, err)
		}
		return id
	}
	t1 := mkTransfer("TR-1", 1, 2)
	if _, err := r.CreateTransfer(ctx, &stinfra.Transfer{Number: "TR-1", FromBranchID: 1, ToBranchID: 1, FromLocation: l1, ToLocation: l2}, 1); err == nil {
		t.Fatal("duplicate number must fail")
	}
	t2 := mkTransfer("TR-2", 1, 1) // same-branch: listed once
	got, err := r.GetTransfer(ctx, t1)
	if err != nil || got == nil || got.Number != "TR-1" || got.Status != "draft" || got.Notes != "n-TR-1" ||
		len(got.Items) != 2 || got.Items[0].Qty != 3 || got.Items[0].ToLocation != l2 || got.Items[1].ToLocation != 0 {
		t.Fatalf("transfer: %+v %v", got, err)
	}
	var cb, ub int64
	_ = testMods.db.QueryRow("SELECT created_by, updated_by FROM stock_transfers WHERE id=?", t1).Scan(&cb, &ub)
	if cb != 7 || ub != 7 {
		t.Fatalf("audit %d %d", cb, ub)
	}
	if got, err := r.GetTransfer(ctx, 99999); err != nil || got != nil {
		t.Fatalf("missing: %v %v", got, err)
	}

	// Lock + CAS transitions with stamps.
	tx, _ := r.Begin(ctx)
	st, err := r.LockTransferStatusTx(ctx, tx, t1)
	_ = tx.Rollback()
	if err != nil || st != "draft" {
		t.Fatalf("lock: %q %v", st, err)
	}
	tx2, _ := r.Begin(ctx)
	if st, _ := r.LockTransferStatusTx(ctx, tx2, 99999); st != "" {
		_ = tx2.Rollback()
		t.Fatalf("lock missing: %q", st)
	}
	_ = tx2.Rollback()
	tx3, _ := r.Begin(ctx)
	if ok, _ := r.TransitionTransferTx(ctx, tx3, t1, nil, "dispatched", 1); ok {
		_ = tx3.Rollback()
		t.Fatal("empty from must lose")
	}
	_ = tx3.Rollback()
	tx4, _ := r.Begin(ctx)
	ok, err := r.TransitionTransferTx(ctx, tx4, t1, []string{"received"}, "dispatched", 8)
	if err != nil || ok {
		_ = tx4.Rollback()
		t.Fatalf("wrong origin: %v %v", ok, err)
	}
	_ = tx4.Rollback()
	tx5, _ := r.Begin(ctx)
	if ok, err := r.TransitionTransferTx(ctx, tx5, t1, []string{"draft"}, "dispatched", 8); err != nil || !ok {
		_ = tx5.Rollback()
		t.Fatalf("cas: %v %v", ok, err)
	}
	if err := tx5.Commit(); err != nil {
		t.Fatal(err)
	}
	var stamped int
	_ = testMods.db.QueryRow("SELECT dispatched_at IS NOT NULL FROM stock_transfers WHERE id=?", t1).Scan(&stamped)
	var ub2 int64
	_ = testMods.db.QueryRow("SELECT updated_by FROM stock_transfers WHERE id=?", t1).Scan(&ub2)
	if stamped != 1 || ub2 != 8 {
		t.Fatalf("stamp %d by %d", stamped, ub2)
	}

	// One-step received transfer carries both stamps.
	tx6, _ := r.Begin(ctx)
	oneID, err := r.CreateTransferTx(ctx, tx6, &stinfra.Transfer{Number: "TR-ONE", FromBranchID: 1, ToBranchID: 1, FromLocation: l1, ToLocation: l2}, "received", 9)
	if err != nil {
		_ = tx6.Rollback()
		t.Fatal(err)
	}
	if err := tx6.Commit(); err != nil {
		t.Fatal(err)
	}
	var both int
	_ = testMods.db.QueryRow("SELECT dispatched_at IS NOT NULL AND received_at IS NOT NULL FROM stock_transfers WHERE id=?", oneID).Scan(&both)
	if both != 1 {
		t.Fatal("one-step must carry both stamps")
	}

	// Standalone status moves stamp the same way.
	if err := r.SetTransferStatus(ctx, t2, "received", 3); err != nil {
		t.Fatal(err)
	}
	_ = testMods.db.QueryRow("SELECT received_at IS NOT NULL AND dispatched_at IS NULL FROM stock_transfers WHERE id=?", t2).Scan(&both)
	if both != 1 {
		t.Fatal("received stamps only received_at")
	}
	tx7, _ := r.Begin(ctx)
	if err := r.SetTransferStatusTx(ctx, tx7, t2, "dispatched", 3); err != nil {
		_ = tx7.Rollback()
		t.Fatal(err)
	}
	_ = tx7.Commit()

	// List/count: same-branch transfer appears once; to-branch is visible.
	lt, err := r.ListTransfers(ctx, 1, "", 10, 0)
	if err != nil || len(lt) != 3 {
		t.Fatalf("list: %+v %v", lt, err)
	}
	if n, _ := r.CountTransfers(ctx, 1, ""); n != 3 {
		t.Fatalf("count: %d", n)
	}
	// t2 was moved to dispatched above too: newest first.
	if lt, _ := r.ListTransfers(ctx, 1, "dispatched", 10, 0); len(lt) != 2 || lt[0].ID != t2 || lt[1].ID != t1 {
		t.Fatalf("status: %+v", lt)
	}
	if n, _ := r.CountTransfers(ctx, 2, ""); n != 1 {
		t.Fatalf("branch 2 sees TR-1: %d", n)
	}
	if lt, _ := r.ListTransfers(ctx, 1, "", 2, 2); len(lt) != 1 {
		t.Fatalf("page: %+v", lt)
	}

	// Reservations.
	future := time.Now().UTC().Add(time.Hour).Format("2006-01-02 15:04:05")
	past := time.Now().UTC().Add(-time.Hour).Format("2006-01-02 15:04:05")
	resID, err := r.CreateReservation(ctx, &stinfra.Reservation{Key: "k-1", BranchID: 1, ProductID: 1, VariantID: 0, LocationID: l1, HasLocation: true, Total: 5, Remaining: 5, ExpiresAt: future}, 4)
	if err != nil || resID == 0 {
		t.Fatalf("reserve: %d %v", resID, err)
	}
	if _, err := r.CreateReservation(ctx, &stinfra.Reservation{Key: "k-1", BranchID: 1, ProductID: 1, VariantID: 0, Total: 1, Remaining: 1, ExpiresAt: future}, 4); err == nil {
		t.Fatal("duplicate key must fail")
	}
	gotRes, err := r.ReservationByKey(ctx, "k-1")
	if err != nil || gotRes == nil || gotRes.Total != 5 || !gotRes.HasLocation || gotRes.LocationID != l1 || gotRes.Status != "active" {
		t.Fatalf("reservation: %+v %v", gotRes, err)
	}
	if gotRes, _ := r.ReservationByKey(ctx, "nope"); gotRes != nil {
		t.Fatalf("unknown key: %+v", gotRes)
	}
	tx8, _ := r.Begin(ctx)
	probe, err := r.ReservationProbeTx(ctx, tx8, "k-1")
	_ = tx8.Rollback()
	if err != nil || probe != resID {
		t.Fatalf("probe: %d %v", probe, err)
	}
	tx9, _ := r.Begin(ctx)
	locked, err := r.ReservationByKeyTx(ctx, tx9, "k-1")
	_ = tx9.Rollback()
	if err != nil || locked == nil || locked.ID != resID {
		t.Fatalf("by key tx: %+v %v", locked, err)
	}
	_, txr := stockTx(t, r)
	if err := r.RewriteReservationTx(ctx, txr, resID, 5, 4, "active", future); err != nil {
		t.Fatal(err)
	}
	if err := txr.Commit(); err != nil {
		t.Fatal(err)
	}
	if gotRes, _ := r.ReservationByKey(ctx, "k-1"); gotRes.Remaining != 4 {
		t.Fatalf("rewritten: %+v", gotRes)
	}
	if err := r.UpdateReservation(ctx, resID, 3, "active"); err != nil {
		t.Fatal(err)
	}
	tx10, _ := r.Begin(ctx)
	if err := r.UpdateReservationTx(ctx, tx10, resID, 2, "active"); err != nil {
		_ = tx10.Rollback()
		t.Fatal(err)
	}
	_ = tx10.Commit()
	if gotRes, _ := r.ReservationByKey(ctx, "k-1"); gotRes.Remaining != 2 {
		t.Fatalf("updated: %+v", gotRes)
	}
	ar, err := r.ActiveReservations(ctx, 1, 1, 0)
	if err != nil || len(ar) != 1 || ar[0].ID != resID {
		t.Fatalf("active: %+v %v", ar, err)
	}
	// An expired hold is invisible to ActiveReservations but listed for sweep.
	expID, err := r.CreateReservation(ctx, &stinfra.Reservation{Key: "k-2", BranchID: 1, ProductID: 1, VariantID: 0, LocationID: l1, HasLocation: true, Total: 1, Remaining: 1, ExpiresAt: past}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if ar, _ := r.ActiveReservations(ctx, 1, 1, 0); len(ar) != 1 {
		t.Fatalf("expired hidden: %+v", ar)
	}
	ex, err := r.ExpiredReservations(ctx, 10)
	if err != nil || len(ex) != 1 || ex[0].ID != expID {
		t.Fatalf("expired list: %+v %v", ex, err)
	}
	// Sweep returns the expired hold's reserved qty to its balance.
	tx11, _ := r.Begin(ctx)
	if err := r.EnsureBalanceTx(ctx, tx11, 1, 1, 0, l1); err != nil {
		_ = tx11.Rollback()
		t.Fatal(err)
	}
	if err := r.AddBalanceTx(ctx, tx11, 1, 1, 0, l1, 0, 1); err != nil {
		_ = tx11.Rollback()
		t.Fatal(err)
	}
	if err := tx11.Commit(); err != nil {
		t.Fatal(err)
	}
	tx12, _ := r.Begin(ctx)
	if err := r.ExpireReservationsTx(ctx, tx12, 1, 1, 0, l1); err != nil {
		_ = tx12.Rollback()
		t.Fatal(err)
	}
	if err := tx12.Commit(); err != nil {
		t.Fatal(err)
	}
	if b, _ := r.GetBalance(ctx, 1, 1, 0, l1); b.Reserved != 0 {
		t.Fatalf("swept reserved: %+v", b)
	}
	if gotRes, _ := r.ReservationByKey(ctx, "k-2"); gotRes.Status != "expired" {
		t.Fatalf("swept status: %+v", gotRes)
	}

	// Product state + housekeeping.
	if ok, _ := r.ProductHasHistory(ctx, 1); ok {
		t.Fatal("no movements for product 1")
	}
	if q, _ := r.ProductOnHand(ctx, 1); q != 0 {
		t.Fatalf("on hand: %v", q)
	}
	if ok, _ := r.ProductHasHolds(ctx, 1); !ok {
		t.Fatal("k-1 is an active hold")
	}
	vs, err := r.VariantStockStates(ctx, 1)
	if err != nil || len(vs) != 1 || vs[0].VariantID != 0 || vs[0].OnHand != 0 || vs[0].HasHistory {
		t.Fatalf("variant states: %+v %v", vs, err)
	}
	// Purge deletes only finished holds past cutoff, up to limit.
	cutoff := time.Now().UTC().Add(-time.Minute)
	if n, err := r.PurgeReservations(ctx, cutoff, 10); err != nil || n != 1 {
		t.Fatalf("purge: %d %v", n, err)
	}
	if gotRes, _ := r.ReservationByKey(ctx, "k-1"); gotRes == nil {
		t.Fatal("active hold must survive purge")
	}
	if n, _ := r.PurgeReservations(ctx, cutoff, 10); n != 0 {
		t.Fatalf("purge again: %d", n)
	}
}
