package integration

import (
	"database/sql"
	"math"
	"strings"
	"testing"
	"time"

	"mini-erp/internal/shared/apperror"

	stockapp "mini-erp/internal/modules/stock/application"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
)

// The tests below make a race deterministic: a raw transaction (rawTx) takes
// the lock the racing flow will need, the flow is started in a goroutine and
// parked on that lock (waitLockWaits), then the raw transaction commits.

func rawTx(t *testing.T) *sql.Tx {
	t.Helper()
	tx, err := testMods.db.BeginTx(ctx(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	return tx
}

func rawExec(t *testing.T, tx *sql.Tx, q string, args ...any) {
	t.Helper()
	if _, err := tx.Exec(q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

// waitLockWaits blocks until n of the test user's other connections have sat
// on a statement (a lock wait) for four polls in a row. The test user
// has no PROCESS privilege, so information_schema.innodb_trx is closed; the
// processlist still shows the user's own threads.
func waitLockWaits(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	stable := 0
	for time.Now().Before(deadline) {
		var got int
		if err := testMods.db.QueryRow(
			`SELECT COUNT(*) FROM information_schema.PROCESSLIST
			 WHERE ID <> CONNECTION_ID() AND DB = DATABASE() AND COMMAND IN ('Query', 'Execute') AND INFO IS NOT NULL
			   AND INFO NOT LIKE '%GET_LOCK%'`).Scan(&got); err != nil {
			t.Fatalf("processlist: %v", err)
		}
		if got >= n {
			if stable++; stable >= 4 {
				return
			}
		} else {
			stable = 0
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("no %d connection(s) parked on a lock", n)
}

func holdState(t *testing.T, key string) (status string, remaining float64) {
	t.Helper()
	if err := testMods.db.QueryRow(
		`SELECT status, qty_remaining FROM stock_reservations WHERE reservation_key = ?`, key).Scan(&status, &remaining); err != nil {
		t.Fatalf("hold %s: %v", key, err)
	}
	return
}

func ageHold(t *testing.T, key string) {
	t.Helper()
	if _, err := testMods.db.Exec(
		`UPDATE stock_reservations SET expires_at = UTC_TIMESTAMP(6) - INTERVAL 1 MINUTE WHERE reservation_key = ?`, key); err != nil {
		t.Fatalf("age hold: %v", err)
	}
}

// A hold revived by its cart (expire + re-hold in one transaction) while a
// sweep of the same product already listed it as expired is not killed by
// that sweep, and its reserved quantity is not leaked: the sweep re-checks
// the hold under its row lock and finds it live.
func TestSweepSparesRevivedHold(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.stock.Service()
	loc2, err := svc.CreateLocation(ctx(), 0, fx.branchID, stockapp.LocationInput{Code: "L2", Name: "Gudang 2"})
	if err != nil {
		t.Fatalf("location: %v", err)
	}
	seedIn(t, fx, fx.productID, fx.variantID, loc2.ID, 10)
	if _, err := svc.Hold(ctx(), 0, fx.branchID, fx.productID, fx.variantID, fx.locationID, 5, "cart-revive", 20); err != nil {
		t.Fatalf("hold: %v", err)
	}
	ageHold(t, "cart-revive")

	// The cart's Hold transaction: the lapsed 5 go back (-5) and the hold is
	// revived at 7 (+7) with a fresh expiry — not committed yet.
	rev := rawTx(t)
	rawExec(t, rev, `SELECT id FROM stock_reservations WHERE reservation_key = 'cart-revive' FOR UPDATE`)
	rawExec(t, rev, `UPDATE stock_reservations SET qty_total = 7, qty_remaining = 7,
		expires_at = UTC_TIMESTAMP(6) + INTERVAL 20 MINUTE WHERE reservation_key = 'cart-revive'`)
	rawExec(t, rev, `UPDATE stock_balances SET reserved = reserved + 2
		WHERE branch_id = ? AND product_id = ? AND variant_id = ? AND location_id = ?`,
		fx.branchID, fx.productID, fx.variantID, fx.locationID)

	// Another cart holds the same product elsewhere: its sweep lists the
	// lapsed hold and parks on its row.
	done := make(chan error, 1)
	go func() {
		_, err := svc.Hold(ctx(), 0, fx.branchID, fx.productID, fx.variantID, loc2.ID, 1, "cart-other", 20)
		done <- err
	}()
	waitLockWaits(t, 1)
	if err := rev.Commit(); err != nil {
		t.Fatalf("revive commit: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("other hold: %v", err)
	}
	if st, rem := holdState(t, "cart-revive"); st != "active" || rem != 7 {
		t.Fatalf("revived hold = %s %v, want active 7", st, rem)
	}
	if _, r := balanceAt(t, fx, fx.locationID); r != 7 {
		t.Fatalf("reserved at the revived position = %v, want 7 (nothing leaked)", r)
	}
	if _, r := balanceAt(t, fx, loc2.ID); r != 1 {
		t.Fatalf("reserved at loc2 = %v, want 1", r)
	}
}

// The background sweeper (SweepExpired) lists a lapsed hold, then the cart
// revives it before the sweeper locks it: the sweeper must leave it alone.
func TestSweepExpiredSparesRevivedHold(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.stock.Service()
	if _, err := svc.Hold(ctx(), 0, fx.branchID, fx.productID, fx.variantID, fx.locationID, 5, "cart-sweep", 20); err != nil {
		t.Fatalf("hold: %v", err)
	}
	ageHold(t, "cart-sweep")

	// Revived at the same quantity: the expire/re-hold nets to zero reserved.
	rev := rawTx(t)
	rawExec(t, rev, `SELECT id FROM stock_reservations WHERE reservation_key = 'cart-sweep' FOR UPDATE`)
	rawExec(t, rev, `UPDATE stock_reservations SET expires_at = UTC_TIMESTAMP(6) + INTERVAL 20 MINUTE
		WHERE reservation_key = 'cart-sweep'`)

	type result struct {
		n   int
		err error
	}
	done := make(chan result, 1)
	go func() {
		n, err := svc.SweepExpired(ctx(), 500)
		done <- result{n, err}
	}()
	waitLockWaits(t, 1)
	if err := rev.Commit(); err != nil {
		t.Fatalf("revive commit: %v", err)
	}
	res := <-done
	if res.err != nil || res.n != 0 {
		t.Fatalf("sweep = %d, %v; want 0 swept", res.n, res.err)
	}
	if st, rem := holdState(t, "cart-sweep"); st != "active" || rem != 5 {
		t.Fatalf("revived hold = %s %v, want active 5", st, rem)
	}
	if _, r := balanceAt(t, fx, fx.locationID); r != 5 {
		t.Fatalf("reserved = %v, want 5", r)
	}
}

// An archive racing a post into the same location never strands stock in an
// archived location, whichever takes the location row first.
func TestArchiveLocationRacesInflow(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.stock.Service()

	// 1. A post is in flight (location row held shared, stock written, not
	// committed): the archive waits for it, then sees the stock.
	loc, err := svc.CreateLocation(ctx(), 0, fx.branchID, stockapp.LocationInput{Code: "ARC1", Name: "Arsip 1"})
	if err != nil {
		t.Fatalf("location: %v", err)
	}
	post := rawTx(t)
	rawExec(t, post, `SELECT status FROM stock_locations WHERE id = ? FOR SHARE`, loc.ID)
	rawExec(t, post, `INSERT INTO stock_balances (branch_id, product_id, variant_id, location_id, on_hand, reserved)
		VALUES (?, ?, ?, ?, 4, 0)`, fx.branchID, fx.productID, fx.variantID, loc.ID)
	done := make(chan error, 1)
	go func() { done <- svc.ArchiveLocation(ctx(), 0, fx.branchID, loc.ID) }()
	waitLockWaits(t, 1)
	if err := post.Commit(); err != nil {
		t.Fatalf("post commit: %v", err)
	}
	if ae := mustAppErr(t, <-done); ae.Code != apperror.CodeConflict || !strings.Contains(ae.Message, "saldo") {
		t.Fatalf("archive = %+v, want the has-stock conflict", ae)
	}
	var status string
	if err := testMods.db.QueryRow(`SELECT status FROM stock_locations WHERE id = ?`, loc.ID).Scan(&status); err != nil || status != "active" {
		t.Fatalf("location status = %q %v, want still active", status, err)
	}

	// 2. The archive holds the row first: the post (validated before) waits,
	// then finds the location archived and writes nothing.
	loc2, err := svc.CreateLocation(ctx(), 0, fx.branchID, stockapp.LocationInput{Code: "ARC2", Name: "Arsip 2"})
	if err != nil {
		t.Fatalf("location 2: %v", err)
	}
	arc := rawTx(t)
	rawExec(t, arc, `SELECT id FROM stock_locations WHERE id = ? FOR UPDATE`, loc2.ID)
	rawExec(t, arc, `UPDATE stock_locations SET status = 'archived' WHERE id = ?`, loc2.ID)
	go func() {
		_, err := svc.Post(ctx(), stockcontracts.PostRequest{
			BranchID: fx.branchID, Direction: "in", RefType: "opening", RefID: 910001,
			Lines: []stockcontracts.PostLine{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: loc2.ID, Qty: 3}},
		})
		done <- err
	}()
	waitLockWaits(t, 1)
	if err := arc.Commit(); err != nil {
		t.Fatalf("archive commit: %v", err)
	}
	if ae := mustAppErr(t, <-done); ae.Code != apperror.CodeConflict {
		t.Fatalf("post = %+v, want a conflict", ae)
	}
	if on, _ := balanceAt(t, fx, loc2.ID); on != 0 {
		t.Fatalf("stock stranded in the archived location: %v", on)
	}
}

// Two damaged moves of one product interleave (both out-legs written before
// either in-leg). Each in-leg is still priced from its OWN out-leg — paired
// by the shared ref_id — so the moves stay value-neutral to the rupiah.
func TestDamagedMovesPairExactly(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.stock.Service()
	p := productWith(t, "PAIR", 0, 1)
	pid, vid := p.ID, p.Variants[0].ID
	loc2, err := svc.CreateLocation(ctx(), 0, fx.branchID, stockapp.LocationInput{Code: "L2", Name: "Gudang 2"})
	if err != nil {
		t.Fatalf("location: %v", err)
	}
	// 3 units worth 10.000 in total: an uneven 3.333,33 per unit, so a leg
	// priced from the wrong partner shows up as a rounding difference.
	third := 10000.0 / 3
	seedInCost(t, fx, pid, vid, fx.locationID, 1, third)
	seedInCost(t, fx, pid, vid, loc2.ID, 2, third)
	damaged, err := svc.DamagedLocation(ctx(), fx.branchID)
	if err != nil {
		t.Fatalf("damaged location: %v", err)
	}
	if _, err := testMods.db.Exec(`INSERT IGNORE INTO stock_balances (branch_id, product_id, variant_id, location_id, on_hand, reserved)
		VALUES (?, ?, ?, ?, 0, 0)`, fx.branchID, pid, vid, damaged.ID); err != nil {
		t.Fatalf("damaged balance: %v", err)
	}
	// Park both moves on the damaged position: each writes its out-leg, then
	// waits to write its in-leg.
	gate := rawTx(t)
	rawExec(t, gate, `SELECT on_hand FROM stock_balances
		WHERE branch_id = ? AND product_id = ? AND variant_id = ? AND location_id = ? FOR UPDATE`,
		fx.branchID, pid, vid, damaged.ID)
	done := make(chan error, 2)
	move := func(loc int64, qty float64) {
		done <- svc.MoveToDamaged(ctx(), 0, fx.branchID, stockapp.DamagedInput{
			ProductID: pid, VariantID: vid, LocationID: loc, Qty: qty, Reason: "pecah"})
	}
	go move(fx.locationID, 1)
	waitLockWaits(t, 1) // the first move parks before the second starts
	go move(loc2.ID, 2)
	waitLockWaits(t, 2)
	if err := gate.Rollback(); err != nil {
		t.Fatalf("release: %v", err)
	}
	for range 2 {
		if err := <-done; err != nil {
			t.Fatalf("damaged move: %v", err)
		}
	}

	// Both legs of a move carry one ref_id (the out-leg's id), distinct per move.
	rows, err := testMods.db.Query(`SELECT id, direction, ref_id FROM stock_movements
		WHERE product_id = ? AND ref_type = 'damaged' ORDER BY id`, pid)
	if err != nil {
		t.Fatalf("movements: %v", err)
	}
	type mvRow struct {
		id    int64
		dir   string
		refID int64
	}
	var mvs []mvRow
	for rows.Next() {
		var r mvRow
		if err := rows.Scan(&r.id, &r.dir, &r.refID); err != nil {
			t.Fatal(err)
		}
		mvs = append(mvs, r)
	}
	_ = rows.Close()
	if len(mvs) != 4 || mvs[0].dir != "out" || mvs[1].dir != "out" {
		t.Fatalf("movements = %+v, want two out-legs before two in-legs", mvs)
	}
	legs := map[int64]int{}
	for _, r := range mvs {
		if r.dir == "out" && r.refID != r.id {
			t.Fatalf("out-leg %d carries ref_id %d, want its own id", r.id, r.refID)
		}
		legs[r.refID]++
	}
	if len(legs) != 2 || legs[mvs[0].id] != 2 || legs[mvs[1].id] != 2 {
		t.Fatalf("ref_id pairs = %v, want two pairs", legs)
	}

	// Finance prices every in-leg at exactly its own out-leg's value.
	if _, err := testMods.finance.Service().Sync(ctx(), 1); err != nil {
		t.Fatalf("sync: %v", err)
	}
	crows, err := testMods.db.Query(`SELECT ref_id, direction, total_cost FROM finance_inventory_cost_movements
		WHERE product_id = ? AND ref_type = 'damaged'`, pid)
	if err != nil {
		t.Fatalf("cost rows: %v", err)
	}
	value := map[int64]map[string]float64{}
	for crows.Next() {
		var ref int64
		var dir string
		var total float64
		if err := crows.Scan(&ref, &dir, &total); err != nil {
			t.Fatal(err)
		}
		if value[ref] == nil {
			value[ref] = map[string]float64{}
		}
		value[ref][dir] = math.Abs(total)
	}
	_ = crows.Close()
	if len(value) != 2 {
		t.Fatalf("cost rows = %v, want two pairs", value)
	}
	var moved float64
	for ref, v := range value {
		if v["out"] == 0 || v["in"] != v["out"] {
			t.Fatalf("pair %d priced out=%v in=%v, want equal", ref, v["out"], v["in"])
		}
		moved += v["in"]
	}
	if moved != 10000 {
		t.Fatalf("value moved to damaged = %v, want exactly 10000", moved)
	}
}
