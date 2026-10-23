package integration

import (
	"errors"
	"strings"
	"testing"
	"time"

	"mini-erp/cmd/stockfix/fix"
	stockapp "mini-erp/internal/modules/stock/application"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
)

// Temuan 6 (2026-10-23): a wrong +50 correction (made because a POS hold bug
// hid stock) is neutralized by cmd/stockfix with a counter adjustment through
// the stock service — never by editing rows.

type stockfixFixture struct {
	fx    shopFixture
	deps  fix.Deps
	refID int64
}

func newStockfixFixture(t *testing.T) stockfixFixture {
	t.Helper()
	freshDB(t)
	fx := newShop(t, ctx())
	if err := testMods.user.Service().EnsurePermissions(ctx(), stockcontracts.PermissionCatalog()); err != nil {
		t.Fatal(err)
	}
	opRole := makeRole(t, "OPS-STOK", "Operator stok")
	if err := testMods.user.Service().SetRolePermissions(ctx(), 0, opRole, []string{"stock.adjust"}); err != nil {
		t.Fatal(err)
	}
	makeUserWithRole(t, fx.branchID, "gudangops", opRole)
	approverFixture(t, fx, "bosstok")
	// Production default: every adjustment needs an approver.
	if _, err := testMods.db.Exec(`UPDATE company_settings SET setting_value = 'true' WHERE setting_key = 'stock_adjust_always_approve'`); err != nil {
		t.Fatal(err)
	}
	// The wrong correction: +50 on top of the fixture's 50.
	if err := testMods.stock.Service().Adjust(ctx(), 0, fx.branchID, stockapp.AdjustInput{
		ProductID: fx.productID, VariantID: fx.variantID, Location: fx.locationID, Mode: "in", QtyDelta: 50,
		Reason: "stok tidak tersedia di POS", Approver: "bosstok", ApproverP: "rahasia-kuat-1",
	}); err != nil {
		t.Fatalf("wrong correction: %v", err)
	}
	var ref int64
	if err := testMods.db.QueryRow(`SELECT MAX(id) FROM stock_movements WHERE movement_type = 'adjustment' AND product_id = ?`, fx.productID).Scan(&ref); err != nil {
		t.Fatal(err)
	}
	return stockfixFixture{fx: fx, refID: ref, deps: fix.Deps{DB: testMods.db, Stock: testMods.stock.Service(),
		Products: testMods.product.Products(), Users: testMods.user.Users(), Roles: testMods.user.Roles(),
		Sleep: func(time.Duration) {}}}
}

func (s stockfixFixture) onHand(t *testing.T) float64 {
	t.Helper()
	b, err := testMods.stock.Service().GetBalance(ctx(), s.fx.branchID, s.fx.productID, s.fx.variantID, s.fx.locationID)
	if err != nil {
		t.Fatal(err)
	}
	return b.OnHand
}

func (s stockfixFixture) opts(commit bool) fix.Options {
	return fix.Options{BranchID: s.fx.branchID, Actor: "gudangops", Approver: "bosstok", ApproverPassword: "rahasia-kuat-1", Commit: commit}
}

func TestStockfixNeutralizesWrongCorrection(t *testing.T) {
	s := newStockfixFixture(t)
	if got := s.onHand(t); got != 100 {
		t.Fatalf("on hand before = %v, want 100", got)
	}
	lines := []fix.Line{{Direction: "out", Qty: 50, RefMovementID: s.refID, Reason: "netralkan koreksi bug tahanan POS"}}
	movements := func() int {
		return countRows(t, `SELECT COUNT(*) FROM stock_movements WHERE product_id = ?`, s.fx.productID)
	}
	audits := func() int {
		return countRows(t, `SELECT COUNT(*) FROM audit_logs WHERE action_key = 'stock_adjustments.create'`)
	}
	m0, a0 := movements(), audits()

	// Dry run: the plan, nothing written.
	rep, err := fix.Run(ctx(), s.deps, s.opts(false), lines)
	if err != nil || rep.Planned != 1 || rep.Lines[0].OnHandBefore != 100 || rep.Lines[0].OnHandAfter != 50 {
		t.Fatalf("dry run = %+v err=%v", rep, err)
	}
	if movements() != m0 || audits() != a0 || s.onHand(t) != 100 {
		t.Fatal("the dry run wrote something")
	}

	rep, err = fix.Run(ctx(), s.deps, s.opts(true), lines)
	if err != nil || rep.Applied != 1 {
		t.Fatalf("commit = %+v err=%v", rep, err)
	}
	if got := s.onHand(t); got != 50 {
		t.Fatalf("on hand after = %v, want 50 (the wrong +50 neutralized)", got)
	}
	// Stock card: the wrong +50 and the counter -50, both adjustments; the
	// counter names its reference and the approver is trailed.
	if n := countRows(t, `SELECT COUNT(*) FROM stock_movements WHERE product_id = ? AND movement_type = 'adjustment' AND direction = 'out'
		AND qty_base = 50 AND notes LIKE ?`, s.fx.productID, "%"+fix.Marker(s.refID)+"%"); n != 1 || movements() != m0+1 {
		t.Fatalf("counter movements = %d (total %d -> %d)", n, m0, movements())
	}
	if n := countRows(t, `SELECT COUNT(*) FROM audit_logs WHERE action_key = 'stock_adjustments.create' AND note LIKE ? AND note LIKE '%approver: bosstok%'`,
		"%"+fix.Marker(s.refID)+"%"); n != 1 {
		t.Fatalf("audit entries with the marker = %d, want 1", n)
	}

	// Second run: recognised, nothing written.
	rep, err = fix.Run(ctx(), s.deps, s.opts(true), lines)
	if err != nil || rep.Done != 1 || rep.Applied != 0 || movements() != m0+1 || s.onHand(t) != 50 {
		t.Fatalf("rerun = %+v err=%v", rep, err)
	}
}

func TestStockfixRefusesUnsafeLines(t *testing.T) {
	s := newStockfixFixture(t)
	m0 := countRows(t, `SELECT COUNT(*) FROM stock_movements`)
	// Stock already sold down to 30 (+5 opening below): neutralizing 50 would go negative.
	if err := testMods.stock.Service().Adjust(ctx(), 0, s.fx.branchID, stockapp.AdjustInput{
		ProductID: s.fx.productID, VariantID: s.fx.variantID, Location: s.fx.locationID, Mode: "out", QtyDelta: 70,
		Reason: "terjual", Approver: "bosstok", ApproverP: "rahasia-kuat-1",
	}); err != nil {
		t.Fatal(err)
	}
	seedInCost(t, s.fx, s.fx.productID, s.fx.variantID, s.fx.locationID, 5, 7000)
	var opening int64
	if err := testMods.db.QueryRow(`SELECT MAX(id) FROM stock_movements WHERE movement_type <> 'adjustment'`).Scan(&opening); err != nil {
		t.Fatal(err)
	}
	m0 += 2
	cases := []struct {
		name string
		line fix.Line
		want string
	}{
		{"negatif", fix.Line{Direction: "out", Qty: 50, RefMovementID: s.refID, Reason: "x"}, "negatif"},
		{"arah sama", fix.Line{Direction: "in", Qty: 10, RefMovementID: s.refID, Reason: "x"}, "berlawanan"},
		{"qty lebih", fix.Line{Direction: "out", Qty: 60, RefMovementID: s.refID, Reason: "x"}, "melebihi"},
		{"lokasi lain", fix.Line{Direction: "out", Qty: 10, RefMovementID: s.refID, LocationID: s.fx.locationID + 999, Reason: "x"}, "tidak cocok"},
		{"produk lain", fix.Line{ProductCode: "BUKAN-INI", Direction: "out", Qty: 10, RefMovementID: s.refID, Reason: "x"}, "tidak cocok"},
		{"tanpa alasan", fix.Line{Direction: "out", Qty: 10, RefMovementID: s.refID}, "reason"},
		{"bukan koreksi", fix.Line{Direction: "out", Qty: 1, RefMovementID: opening, Reason: "x"}, "adjustment"},
		{"tidak ada", fix.Line{Direction: "out", Qty: 1, RefMovementID: 99999999, Reason: "x"}, "tidak ditemukan"},
	}
	for _, c := range cases {
		rep, err := fix.Run(ctx(), s.deps, s.opts(true), []fix.Line{c.line})
		if !errors.Is(err, fix.ErrRejected) || rep.Rejected != 1 || !strings.Contains(rep.Lines[0].Problem, c.want) {
			t.Errorf("%s: err=%v report=%+v, want rejected with %q", c.name, err, rep.Lines, c.want)
		}
	}
	// One bad line blocks the whole file, the good one included.
	rep, err := fix.Run(ctx(), s.deps, s.opts(true), []fix.Line{
		{Direction: "out", Qty: 10, RefMovementID: s.refID, Reason: "sebagian"},
		{Direction: "out", Qty: 10, RefMovementID: s.refID, Reason: "dobel"},
	})
	if !errors.Is(err, fix.ErrRejected) || rep.Lines[1].Status != fix.StatusRejected || rep.Applied != 0 {
		t.Fatalf("duplicate ref = %+v err=%v", rep, err)
	}
	if n := countRows(t, `SELECT COUNT(*) FROM stock_movements`); n != m0 {
		t.Fatalf("movements %d -> %d: a refused run wrote", m0, n)
	}
	// The actor must be real, allowed and on the branch; the approver rule is the service's.
	o := s.opts(true)
	o.Actor = "bosstok" // holds stock.approve, not stock.adjust
	if _, err := fix.Run(ctx(), s.deps, o, []fix.Line{{Direction: "out", Qty: 10, RefMovementID: s.refID, Reason: "x"}}); err == nil || !strings.Contains(err.Error(), "stock.adjust") {
		t.Fatalf("actor without stock.adjust = %v", err)
	}
	o = s.opts(true)
	o.ApproverPassword = "salah-salah"
	if _, err := fix.Run(ctx(), s.deps, o, []fix.Line{{Direction: "out", Qty: 10, RefMovementID: s.refID, Reason: "x"}}); err == nil || !strings.Contains(err.Error(), "Persetujuan tidak valid") {
		t.Fatalf("wrong approver password = %v", err)
	}
	if n := countRows(t, `SELECT COUNT(*) FROM stock_movements`); n != m0 {
		t.Fatalf("movements %d -> %d after refused approvals", m0, n)
	}
}
