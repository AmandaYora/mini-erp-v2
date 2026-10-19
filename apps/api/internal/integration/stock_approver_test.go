package integration

import (
	"context"
	"sync/atomic"
	"testing"

	"mini-erp/internal/modules/stock"
	stockapp "mini-erp/internal/modules/stock/application"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/apperror"
)

// countingUsers decorates the real UserClient and counts credential
// verifications (F1-8: proves the approver check runs exactly once per
// Adjust, before the transaction opens).
type countingUsers struct {
	usercontracts.UserClient
	calls *int32
}

func (c *countingUsers) VerifyCredentials(ctx context.Context, username, password string) (*usercontracts.User, error) {
	atomic.AddInt32(c.calls, 1)
	return c.UserClient.VerifyCredentials(ctx, username, password)
}

func approverFixture(t *testing.T, fx shopFixture, username string) {
	t.Helper()
	if err := testMods.user.Service().EnsurePermissions(ctx(), stockcontracts.PermissionCatalog()); err != nil {
		t.Fatalf("ensure stock catalog: %v", err)
	}
	roleID := makeRole(t, "APV-"+username, "Approver "+username)
	if err := testMods.user.Service().SetRolePermissions(ctx(), 0, roleID, []string{"stock.approve"}); err != nil {
		t.Fatalf("grant approve: %v", err)
	}
	makeUserWithRole(t, fx.branchID, username, roleID)
}

// TestApproverRateLimited (F1-8): 5 tebakan salah boleh (403), tebakan ke-6
// dalam semenit ditolak 429 — per pasangan actor|approver.
func TestApproverRateLimited(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	approverFixture(t, fx, "penyetuju")

	in := stockapp.AdjustInput{
		ProductID: fx.productID, VariantID: fx.variantID, Location: fx.locationID,
		Mode: "in", QtyDelta: 150, Reason: "uji rate limit",
		Approver: "penyetuju", ApproverP: "salah-salah",
	}
	for i := 0; i < 5; i++ {
		err := testMods.stock.Service().Adjust(ctx(), 0, fx.branchID, in)
		if mustAppErr(t, err).Code != apperror.CodeForbidden {
			t.Fatalf("tebakan %d: want 403, got %v", i+1, err)
		}
	}
	if err := testMods.stock.Service().Adjust(ctx(), 0, fx.branchID, in); mustAppErr(t, err).Code != apperror.CodeRateLimited {
		t.Fatalf("tebakan 6: want 429, got %v", err)
	}
}

// TestApproverVerifiedOnceOutsideTx (F1-8): approver benar diverifikasi tepat
// sekali (sebelum tx) dan koreksi di atas ambang lolos; di bawah ambang
// tanpa approver tetap lolos tanpa verifikasi.
func TestApproverVerifiedOnceOutsideTx(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	approverFixture(t, fx, "bosstok")

	// Rebuild the stock service over the counting decorator.
	var calls int32
	counting := &countingUsers{UserClient: testMods.user.Users(), calls: &calls}
	svc := stock.NewModule(testMods.db, testMods.product.Products(), testMods.branch.Branches(),
		counting, testMods.user.Roles(), testMods.company.Company(), testMods.audit.Audit()).Service()

	big := stockapp.AdjustInput{
		ProductID: fx.productID, VariantID: fx.variantID, Location: fx.locationID,
		Mode: "in", QtyDelta: 150, Reason: "koreksi besar",
		Approver: "bosstok", ApproverP: "rahasia-kuat-1",
	}
	if err := svc.Adjust(ctx(), 0, fx.branchID, big); err != nil {
		t.Fatalf("adjust dengan approver benar: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("verifikasi = %d, want tepat 1 (sebelum tx, bukan di dalam)", got)
	}
	bal, err := testMods.stock.Service().GetBalance(ctx(), fx.branchID, fx.productID, fx.variantID, fx.locationID)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	if bal.OnHand != 200 {
		t.Fatalf("onhand = %v, want 200 (50 awal + 150)", bal.OnHand)
	}

	// Tanpa approver di atas ambang: 403 tanpa menyentuh verifikasi.
	before := atomic.LoadInt32(&calls)
	anon := stockapp.AdjustInput{
		ProductID: fx.productID, VariantID: fx.variantID, Location: fx.locationID,
		Mode: "in", QtyDelta: 150, Reason: "tanpa approver",
	}
	if err := svc.Adjust(ctx(), 0, fx.branchID, anon); mustAppErr(t, err).Code != apperror.CodeForbidden {
		t.Fatalf("tanpa approver: want 403, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != before {
		t.Fatalf("tanpa approver tak boleh verifikasi: %d -> %d", before, got)
	}

	// Di bawah ambang tanpa approver: lolos.
	small := stockapp.AdjustInput{
		ProductID: fx.productID, VariantID: fx.variantID, Location: fx.locationID,
		Mode: "in", QtyDelta: 10, Reason: "koreksi kecil",
	}
	if err := svc.Adjust(ctx(), 0, fx.branchID, small); err != nil {
		t.Fatalf("koreksi kecil: %v", err)
	}
}
