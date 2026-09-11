package integration

import (
	"testing"

	branchapp "mini-erp/internal/modules/branch/application"
	stockapp "mini-erp/internal/modules/stock/application"
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
		_, err := m.stock.Service().CancelTransfer(ctx(), 0, branchB, tr2.ID)
		return err
	}(), "cancel from destination")
	if _, err := m.stock.Service().CancelTransfer(ctx(), 0, fx.branchID, tr2.ID); err != nil {
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
