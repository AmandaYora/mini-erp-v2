package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"mini-erp/internal/bootstrap"
	branchapp "mini-erp/internal/modules/branch/application"
	stockapp "mini-erp/internal/modules/stock/application"
	userapp "mini-erp/internal/modules/user/application"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/apperror"
)

// TestTransferBranchScope (A1): per-ID transfer endpoints are branch-scoped
// with direction rules — dispatch/cancel from source only, receive at
// destination only, GET from either side. Foreign branches read as missing
// (not_found, never forbidden) so document existence does not leak.
func TestTransferBranchScope(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	mkBranch := func(code string) int64 {
		t.Helper()
		br, err := m.branch.Service().CreateBranch(ctx(), 0, branchapp.CreateBranchInput{
			Code: code, Name: "Cabang " + code,
		})
		if err != nil {
			t.Fatalf("create branch %s: %v", code, err)
		}
		return br.ID
	}
	mkLocation := func(branchID int64, code string) int64 {
		t.Helper()
		loc, err := m.stock.Service().CreateLocation(ctx(), 0, branchID, stockapp.LocationInput{
			Code: code, Name: "Gudang " + code,
		})
		if err != nil {
			t.Fatalf("create location %s: %v", code, err)
		}
		return loc.ID
	}

	branchB := mkBranch("CB2")
	branchC := mkBranch("CB3")
	locB := mkLocation(branchB, "CB2")

	tr, err := m.stock.Service().CreateTransfer(ctx(), 0, fx.branchID, branchB,
		fx.locationID, locB, "antar cabang", []stockapp.TransferItemInput{
			{ProductID: fx.productID, VariantID: fx.variantID, Qty: 5},
		})
	if err != nil {
		t.Fatalf("create transfer: %v", err)
	}

	notFound := func(err error, what string) {
		t.Helper()
		if got := mustAppErr(t, err).Code; got != apperror.CodeNotFound {
			t.Fatalf("%s: code = %q, want not_found", what, got)
		}
	}

	// GET: destination side OK, third branch missing.
	if _, err := m.stock.Service().GetTransfer(ctx(), branchB, tr.ID); err != nil {
		t.Fatalf("get from destination: %v", err)
	}
	notFound(func() error {
		_, err := m.stock.Service().GetTransfer(ctx(), branchC, tr.ID)
		return err
	}(), "get from foreign branch")

	// Dispatch: destination rejected, source succeeds.
	notFound(func() error {
		_, err := m.stock.Service().DispatchTransfer(ctx(), 0, branchB, tr.ID)
		return err
	}(), "dispatch from destination")
	if _, err := m.stock.Service().DispatchTransfer(ctx(), 0, fx.branchID, tr.ID); err != nil {
		t.Fatalf("dispatch from source: %v", err)
	}

	// Cancel is source-only too: destination rejected on a fresh draft.
	tr2, err := m.stock.Service().CreateTransfer(ctx(), 0, fx.branchID, branchB,
		fx.locationID, locB, "batal uji", []stockapp.TransferItemInput{
			{ProductID: fx.productID, VariantID: fx.variantID, Qty: 1},
		})
	if err != nil {
		t.Fatalf("create transfer 2: %v", err)
	}
	notFound(func() error {
		_, err := m.stock.Service().CancelTransfer(ctx(), 0, branchB, tr2.ID, "")
		return err
	}(), "cancel from destination")
	if _, err := m.stock.Service().CancelTransfer(ctx(), 0, fx.branchID, tr2.ID, ""); err != nil {
		t.Fatalf("cancel from source: %v", err)
	}

	// Receive: source rejected, destination succeeds (cross-branch alive).
	notFound(func() error {
		_, err := m.stock.Service().ReceiveTransfer(ctx(), 0, fx.branchID, tr.ID)
		return err
	}(), "receive from source")
	done, err := m.stock.Service().ReceiveTransfer(ctx(), 0, branchB, tr.ID)
	if err != nil {
		t.Fatalf("receive at destination: %v", err)
	}
	if done.Status != "received" {
		t.Fatalf("status = %q, want received", done.Status)
	}
}

// TestLocationBranchScope (A1.5): locations cannot be renamed or archived
// from another branch.
func TestLocationBranchScope(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	br, err := m.branch.Service().CreateBranch(ctx(), 0, branchapp.CreateBranchInput{
		Code: "CB2", Name: "Cabang CB2",
	})
	if err != nil {
		t.Fatalf("create branch: %v", err)
	}
	loc, err := m.stock.Service().CreateLocation(ctx(), 0, fx.branchID, stockapp.LocationInput{
		Code: "ASN", Name: "Asing",
	})
	if err != nil {
		t.Fatalf("create location: %v", err)
	}
	if _, err := m.stock.Service().UpdateLocation(ctx(), 0, br.ID, loc.ID,
		stockapp.LocationInput{Name: "Direbut"}); mustAppErr(t, err).Code != apperror.CodeNotFound {
		t.Fatalf("update from foreign branch: code = %q, want not_found", mustAppErr(t, err).Code)
	}
	if err := m.stock.Service().ArchiveLocation(ctx(), 0, br.ID, loc.ID); mustAppErr(t, err).Code != apperror.CodeNotFound {
		t.Fatalf("archive from foreign branch: code = %q, want not_found", mustAppErr(t, err).Code)
	}
}

// TestTransferDestinationLocationsAccess (K-06): GET /stock/locations?branchId=
// serves another branch's locations only to a user holding access to that
// branch; a branch without access reads 403 (the pickers must not leak foreign
// locations) and creating a transfer toward it is refused the same way.
func TestTransferDestinationLocationsAccess(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	mkBranch := func(code string) int64 {
		t.Helper()
		br, err := m.branch.Service().CreateBranch(ctx(), 0, branchapp.CreateBranchInput{Code: code, Name: "Cabang " + code})
		if err != nil {
			t.Fatalf("create branch %s: %v", code, err)
		}
		return br.ID
	}
	branchB, branchC := mkBranch("K6B"), mkBranch("K6C")
	locB, err := m.stock.Service().CreateLocation(ctx(), 0, branchB, stockapp.LocationInput{Code: "K6B", Name: "Gudang K6B"})
	if err != nil {
		t.Fatalf("location B: %v", err)
	}
	if _, err := m.stock.Service().CreateLocation(ctx(), 0, branchC, stockapp.LocationInput{Code: "K6C", Name: "Gudang K6C"}); err != nil {
		t.Fatalf("location C: %v", err)
	}

	if err := bootstrap.EnsureAuthz(ctx(), m.user.Service()); err != nil {
		t.Fatalf("seed authz: %v", err)
	}
	role, err := m.user.Service().CreateRole(ctx(), 0, userapp.CreateRoleInput{Code: "K6-role", Name: "K6"})
	if err != nil {
		t.Fatalf("role: %v", err)
	}
	if err := m.user.Service().SyncRolePermissions(ctx(), 0, role.ID, []string{"stock.view", "stock.transfer"}); err != nil {
		t.Fatalf("perms: %v", err)
	}
	mkUser := func(name string, branches ...usercontracts.BranchAccess) string {
		t.Helper()
		if _, err := m.user.Service().CreateUser(ctx(), 0, userapp.CreateUserInput{
			Username: name, Password: "rahasia-kuat-1", RoleIDs: []int64{role.ID}, Branches: branches,
		}); err != nil {
			t.Fatalf("user %s: %v", name, err)
		}
		return login(t, name, "rahasia-kuat-1")
	}
	both := mkUser("k6-both", usercontracts.BranchAccess{BranchID: fx.branchID, IsDefault: true}, usercontracts.BranchAccess{BranchID: branchB})
	only := mkUser("k6-only", usercontracts.BranchAccess{BranchID: fx.branchID, IsDefault: true})

	h := routeServer(t).Handler()
	locIDs := func(body []byte) (ids []int64, branches map[int64]bool) {
		var env struct {
			Data []struct {
				ID       int64 `json:"id"`
				BranchID int64 `json:"branchId"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &env); err != nil {
			t.Fatalf("decode: %v", err)
		}
		branches = map[int64]bool{}
		for _, d := range env.Data {
			ids = append(ids, d.ID)
			branches[d.BranchID] = true
		}
		return
	}

	// With access: destination branch's locations, and only those.
	rec := doReq(h, "GET", fmt.Sprintf("/api/v1/stock/locations?branchId=%d&status=active", branchB), both, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("with access: status %d body %s", rec.Code, rec.Body)
	}
	ids, brs := locIDs(rec.Body.Bytes())
	if len(brs) != 1 || !brs[branchB] {
		t.Fatalf("with access: branches = %v, want only %d", brs, branchB)
	}
	found := false
	for _, id := range ids {
		found = found || id == locB.ID
	}
	if !found {
		t.Fatalf("with access: destination location %d missing from %v", locB.ID, ids)
	}

	// Own branch explicitly and implicitly stay unchanged.
	for _, q := range []string{"", fmt.Sprintf("?branchId=%d", fx.branchID)} {
		rec = doReq(h, "GET", "/api/v1/stock/locations"+q, only, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("own branch %q: status %d", q, rec.Code)
		}
		if _, brs := locIDs(rec.Body.Bytes()); len(brs) != 1 || !brs[fx.branchID] {
			t.Fatalf("own branch %q: branches = %v", q, brs)
		}
	}

	// Without access (B for "only", C for "both"): 403, no data.
	for _, c := range []struct {
		tok    string
		branch int64
	}{{only, branchB}, {both, branchC}} {
		rec = doReq(h, "GET", fmt.Sprintf("/api/v1/stock/locations?branchId=%d", c.branch), c.tok, "")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("no access to %d: status %d body %s, want 403", c.branch, rec.Code, rec.Body)
		}
	}
	// Garbage branchId is a validation error, not a silent fallback.
	if rec = doReq(h, "GET", "/api/v1/stock/locations?branchId=abc", both, ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad branchId: status %d, want 400", rec.Code)
	}

	// Creating a transfer: allowed toward B (access), 403 toward B without access.
	body := fmt.Sprintf(`{"toBranchId":%d,"fromLocationId":%d,"toLocationId":%d,"driverName":"Joko","items":[{"productId":%d,"variantId":%d,"qty":1}]}`,
		branchB, fx.locationID, locB.ID, fx.productID, fx.variantID)
	if rec = doReq(h, "POST", "/api/v1/stock/transfers", both, body); rec.Code != http.StatusCreated {
		t.Fatalf("create with access: status %d body %s", rec.Code, rec.Body)
	}
	if rec = doReq(h, "POST", "/api/v1/stock/transfers", only, body); rec.Code != http.StatusForbidden {
		t.Fatalf("create without access: status %d body %s, want 403", rec.Code, rec.Body)
	}
}
