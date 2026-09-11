package integration

import (
	"testing"
)

// TestBatchResolutionPaths (A3): operational name resolution and balance
// folding travel through the batch reads — asserted by value (the single
// IN queries are visible in code; these tests lock the behavior).
func TestBatchResolutionPaths(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	so1 := sellFixture(t, ctx(), fx, 2)
	so2 := sellFixture(t, ctx(), fx, 3)

	// Both confirmed orders resolve the same customer name in one batch.
	rows, err := m.sales.Service().ListOrders(ctx(), fx.branchID, []string{"confirmed"}, "", "", 10)
	if err != nil {
		t.Fatalf("list orders: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	for _, r := range rows {
		if r.PartyName != "Customer Uji" {
			t.Fatalf("order %s party = %q, want Customer Uji", r.Number, r.PartyName)
		}
	}

	// Two confirmed returns fold into one balance through the batch.
	deliverAll(t, ctx(), fx, so1, 2)
	deliverAll(t, ctx(), fx, so2, 3)
	ret1 := createSalesReturn(t, fx, so1, 2)
	ret2 := createSalesReturn(t, fx, so2, 3)
	if _, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret1.ID); err != nil {
		t.Fatalf("confirm return 1: %v", err)
	}
	if _, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret2.ID); err != nil {
		t.Fatalf("confirm return 2: %v", err)
	}
	bal, err := m.payment.Service().GetPartyBalance(ctx(), fx.branchID, fx.customerID)
	if err != nil {
		t.Fatalf("party balance: %v", err)
	}
	if len(bal.Returns) != 2 {
		t.Fatalf("returns = %d, want 2", len(bal.Returns))
	}
	if bal.TotalReturned != ret1.Total+ret2.Total {
		t.Fatalf("returned = %d, want %d", bal.TotalReturned, ret1.Total+ret2.Total)
	}
}

// TestPurchaseListOrdersBatchNames (A3): purchase summaries resolve supplier
// names through the same batch.
func TestPurchaseListOrdersBatchNames(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	purchaseFixture(t, ctx(), fx, 10000, 2)
	// The fixture receives the PO in full, so it reads completed.
	rows, err := m.purchasing.Service().ListOrders(ctx(), fx.branchID, []string{"completed"}, "", "", 10)
	if err != nil {
		t.Fatalf("list orders: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].PartyName != "Supplier Uji" {
		t.Fatalf("party = %q, want Supplier Uji", rows[0].PartyName)
	}
}
