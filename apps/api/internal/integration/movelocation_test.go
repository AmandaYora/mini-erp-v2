package integration

import (
	"testing"

	stockapp "mini-erp/internal/modules/stock/application"
)

// TestMoveLocationOneStep (J4): same-branch relocation in one call lands
// received with paired out/in movements and exact balances.
func TestMoveLocationOneStep(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	to, err := m.stock.Service().CreateLocation(ctx(), 0, fx.branchID, stockapp.LocationInput{
		Code: "TST2", Name: "Gudang Uji 2",
	})
	if err != nil {
		t.Fatalf("create location: %v", err)
	}
	done, err := m.stock.Service().MoveLocation(ctx(), 0, fx.branchID,
		fx.locationID, to.ID, "pindah uji", []stockapp.TransferItemInput{
			{ProductID: fx.productID, VariantID: fx.variantID, Qty: 10},
		})
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if done.Status != "received" {
		t.Fatalf("status = %q, want received", done.Status)
	}
	fromBal, err := m.stock.Service().GetBalance(ctx(), fx.branchID, fx.productID, fx.variantID, fx.locationID)
	if err != nil {
		t.Fatalf("from balance: %v", err)
	}
	toBal, err := m.stock.Service().GetBalance(ctx(), fx.branchID, fx.productID, fx.variantID, to.ID)
	if err != nil {
		t.Fatalf("to balance: %v", err)
	}
	if fromBal.OnHand != 40 || toBal.OnHand != 10 {
		t.Fatalf("balances = %v/%v, want 40/10", fromBal.OnHand, toBal.OnHand)
	}
}

// TestMoveLocationShortAbortsWhole: a short line moves nothing — the draft
// stays behind for review, balances untouched.
func TestMoveLocationShortAbortsWhole(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	to, err := m.stock.Service().CreateLocation(ctx(), 0, fx.branchID, stockapp.LocationInput{
		Code: "TST2", Name: "Gudang Uji 2",
	})
	if err != nil {
		t.Fatalf("create location: %v", err)
	}
	if _, err := m.stock.Service().MoveLocation(ctx(), 0, fx.branchID,
		fx.locationID, to.ID, "pindah gagal", []stockapp.TransferItemInput{
			{ProductID: fx.productID, VariantID: fx.variantID, Qty: 500},
		}); err == nil {
		t.Fatal("expected short-stock rejection, got nil")
	}
	bal, err := m.stock.Service().GetBalance(ctx(), fx.branchID, fx.productID, fx.variantID, fx.locationID)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	if bal.OnHand != 50 {
		t.Fatalf("onhand = %v, want 50 (nothing moved)", bal.OnHand)
	}
	res, err := m.stock.Service().ListTransfers(ctx(), fx.branchID, "draft", 1, 10)
	if err != nil {
		t.Fatalf("list drafts: %v", err)
	}
	if res.Total != 1 {
		t.Fatalf("drafts = %d, want 1 left behind", res.Total)
	}
}

// TestMoveLocationSameLocation: asal = tujuan ditolak, seperti dokumen.
func TestMoveLocationSameLocation(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	if _, err := m.stock.Service().MoveLocation(ctx(), 0, fx.branchID,
		fx.locationID, fx.locationID, "sama", []stockapp.TransferItemInput{
			{ProductID: fx.productID, VariantID: fx.variantID, Qty: 1},
		}); err == nil {
		t.Fatal("expected same-location rejection, got nil")
	}
}
