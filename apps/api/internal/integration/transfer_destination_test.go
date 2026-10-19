package integration

import (
	"testing"

	stockapp "mini-erp/internal/modules/stock/application"
)

// F2a-5: a transfer line may carry its own destination; it receives there while
// lines without one follow the document destination.
func TestTransferItemDestination(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.stock.Service()
	header, err := svc.CreateLocation(ctx(), 0, fx.branchID, stockapp.LocationInput{Code: "HDR", Name: "Header"})
	if err != nil {
		t.Fatalf("location: %v", err)
	}
	perItem, err := svc.CreateLocation(ctx(), 0, fx.branchID, stockapp.LocationInput{Code: "ITM", Name: "Per baris"})
	if err != nil {
		t.Fatalf("location: %v", err)
	}
	tr, err := svc.CreateTransfer(ctx(), 0, fx.branchID, fx.branchID, fx.locationID, header.ID, "uji", []stockapp.TransferItemInput{
		{ProductID: fx.productID, VariantID: fx.variantID, Qty: 3},
		{ProductID: fx.productID, VariantID: fx.variantID, Qty: 4, ToLocationID: perItem.ID},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.DispatchTransfer(ctx(), 0, fx.branchID, tr.ID); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if _, err := svc.ReceiveTransfer(ctx(), 0, fx.branchID, tr.ID); err != nil {
		t.Fatalf("receive: %v", err)
	}
	if on, _ := balanceAt(t, fx, header.ID); on != 3 {
		t.Fatalf("header destination on_hand = %v, want 3", on)
	}
	if on, _ := balanceAt(t, fx, perItem.ID); on != 4 {
		t.Fatalf("per-line destination on_hand = %v, want 4", on)
	}
	if on, _ := balanceAt(t, fx, fx.locationID); on != 43 {
		t.Fatalf("source on_hand = %v, want 43", on)
	}
}
