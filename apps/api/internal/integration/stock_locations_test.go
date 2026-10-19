package integration

import (
	"strings"
	"testing"

	stockapp "mini-erp/internal/modules/stock/application"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
)

// F2a-9a #1: the damaged location is recognised by its kind. It cannot be a
// posting target for sales/receipts, its stock is not available stock and
// never suggested for allocation, yet the damaged flows keep working.
func TestDamagedLocationKind(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.stock.Service()

	locs, err := svc.ListLocations(ctx(), fx.branchID, "active")
	if err != nil {
		t.Fatalf("list locations: %v", err)
	}
	var damaged *stockcontracts.Location
	for _, l := range locs {
		if l.Kind == stockcontracts.KindDamaged {
			damaged = l
		}
	}
	if damaged == nil || damaged.Code != "RUSAK" {
		t.Fatalf("no damaged-kind location among %+v", locs)
	}
	if err := svc.CheckLocation(ctx(), fx.branchID, damaged.ID); err == nil {
		t.Fatal("CheckLocation accepted the damaged location")
	}

	if err := svc.MoveToDamaged(ctx(), 0, fx.branchID, stockapp.DamagedInput{
		ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, Qty: 5, Reason: "pecah",
	}); err != nil {
		t.Fatalf("move to damaged: %v", err)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, damaged.ID); got != 5 {
		t.Fatalf("damaged stock = %v, want 5", got)
	}
	plan, shortfall, err := svc.Suggest(ctx(), fx.branchID, fx.productID, fx.variantID, 100)
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	for _, p := range plan {
		if p.LocationID == damaged.ID {
			t.Fatalf("suggestion allocated damaged stock: %+v", plan)
		}
	}
	if shortfall != 55 {
		t.Fatalf("shortfall = %v, want 55 (only 45 non-damaged stock)", shortfall)
	}
	crit, err := svc.CriticalStock(ctx(), fx.branchID, 50)
	if err != nil {
		t.Fatalf("critical: %v", err)
	}
	_ = crit
	if err := svc.RestoreDamaged(ctx(), 0, fx.branchID, stockapp.DamagedInput{
		ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, Qty: 5, Reason: "ternyata utuh",
	}); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 50 {
		t.Fatalf("restored stock = %v, want 50", got)
	}
}

// F2a-9a #2: a sub-location under a parent that holds stock is refused unless
// the stock is moved down with it, in one transaction, leaving the parent empty.
func TestChildUnderStockedParent(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx()) // fx.locationID holds 50
	svc := testMods.stock.Service()
	parent := fx.locationID

	_, err := svc.CreateLocation(ctx(), 0, fx.branchID, stockapp.LocationInput{
		Code: "RAK1", Name: "Rak 1", ParentID: &parent,
	})
	if !isConflict(err) || !strings.Contains(err.Error(), "PARENT_HAS_STOCK") {
		t.Fatalf("child under stocked parent = %v, want PARENT_HAS_STOCK conflict", err)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, parent); got != 50 {
		t.Fatalf("parent changed by a refused create: %v", got)
	}

	child, err := svc.CreateLocation(ctx(), 0, fx.branchID, stockapp.LocationInput{
		Code: "RAK1", Name: "Rak 1", ParentID: &parent, ForceTransfer: true,
	})
	if err != nil {
		t.Fatalf("forced create: %v", err)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, parent); got != 0 {
		t.Fatalf("parent after forced create = %v, want 0", got)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, child.ID); got != 50 {
		t.Fatalf("child after forced create = %v, want 50", got)
	}
	var docs int
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM stock_transfers WHERE status = 'received'`).Scan(&docs); err != nil || docs != 1 {
		t.Fatalf("relocation documents = %d, %v; want 1", docs, err)
	}
}
