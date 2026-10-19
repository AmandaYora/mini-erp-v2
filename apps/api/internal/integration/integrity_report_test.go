package integration

import (
	"testing"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
)

// F2a-4: the consistency report names the documents a failed confirm/posting
// left half-done, and repeating the action clears them.
func TestIntegrityReportFindsHalfDoneDocuments(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 5)
	dn, err := draftNote(t, fx, soID, 2)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	// "Crash" between the stock posting and the status flip.
	if _, err := testMods.stock.Service().Post(ctx(), stockcontracts.PostRequest{
		BranchID: fx.branchID, Direction: "out", RefType: stockcontracts.RefDelivery, RefID: dn.ID,
		Lines: []stockcontracts.PostLine{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, Qty: 2}},
	}); err != nil {
		t.Fatalf("post: %v", err)
	}
	poID := confirmedPO(t, fx, 4)
	rcpt := insertPendingReceipt(t, fx, poID, 4, "GR-OPS-1")

	issues := func() map[string]int64 {
		t.Helper()
		got := map[string]int64{}
		d, err := testMods.delivery.Service().IntegrityIssues(ctx(), fx.branchID)
		if err != nil {
			t.Fatalf("delivery issues: %v", err)
		}
		for _, i := range d {
			got[i.Kind] = i.DocID
		}
		g, err := testMods.goodsreceipt.Service().IntegrityIssues(ctx(), fx.branchID)
		if err != nil {
			t.Fatalf("receipt issues: %v", err)
		}
		for _, i := range g {
			got[i.Kind] = i.DocID
		}
		return got
	}
	got := issues()
	if got["draft_with_stock"] != dn.ID || got["pending_receipt"] != rcpt {
		t.Fatalf("report = %v; want the draft note %d and the pending receipt %d", got, dn.ID, rcpt)
	}
	if _, err := testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); err != nil {
		t.Fatalf("repeat confirm: %v", err)
	}
	if _, err := testMods.goodsreceipt.Service().Complete(ctx(), 0, fx.branchID, rcpt, nil); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if got := issues(); len(got) != 0 {
		t.Fatalf("report after repair = %v; want empty", got)
	}
}
