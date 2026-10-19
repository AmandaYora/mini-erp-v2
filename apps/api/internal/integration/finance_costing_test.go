package integration

import (
	"strings"
	"testing"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	productapp "mini-erp/internal/modules/product/application"
	productcontracts "mini-erp/internal/modules/product/contracts"
	salesapp "mini-erp/internal/modules/sales/application"
	salesreturnapp "mini-erp/internal/modules/salesreturn/application"
	salesreturncontracts "mini-erp/internal/modules/salesreturn/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
)

func seedInCost(t *testing.T, fx shopFixture, productID, variantID, locationID int64, qty, unit float64) {
	t.Helper()
	seedRef++
	if _, err := testMods.stock.Service().Post(ctx(), stockcontracts.PostRequest{
		BranchID: fx.branchID, Direction: "in", RefType: "opening", RefID: seedRef,
		Lines: []stockcontracts.PostLine{{ProductID: productID, VariantID: variantID, LocationID: locationID, Qty: qty, UnitCost: unit}},
	}); err != nil {
		t.Fatalf("seed costed stock: %v", err)
	}
}

func productWith(t *testing.T, code string, purchasePrice int64, purchaseFactor float64) *productcontracts.Product {
	t.Helper()
	create := purchasePrice
	if create == 0 {
		create = 1 // the catalog refuses a product without a purchase price
	}
	p, err := testMods.product.Service().CreateProduct(ctx(), 0, productapp.ProductInput{
		Code: code, Name: "Barang " + code, Type: "barang", Tracked: true,
		BaseUOM: "pcs", PurchaseUOM: "dus", SalesUOM: "pcs",
		PurchaseFactor: purchaseFactor, SalesFactor: 1, PurchasePrice: create, SellingPrice: 30000,
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	if purchasePrice == 0 { // legacy/imported data can still carry no price
		if _, err := testMods.db.Exec(`UPDATE products SET purchase_price = 0 WHERE id = ?`, p.ID); err != nil {
			t.Fatalf("clear price: %v", err)
		}
	}
	return p
}

func sellAndDeliver(t *testing.T, fx shopFixture, pid, vid int64, qty float64) int64 {
	t.Helper()
	so, err := testMods.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none",
		Items: []salesapp.LineInput{{ProductID: pid, VariantID: vid, UOM: "pcs", Qty: qty}},
	})
	if err != nil {
		t.Fatalf("create SO: %v", err)
	}
	if _, err := testMods.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
		t.Fatalf("confirm SO: %v", err)
	}
	n, err := testMods.delivery.Service().Create(ctx(), 0, fx.branchID, so.ID, "2026-09-10", "", deliveryapp.DocInput{},
		[]deliveryapp.LineInput{{ProductID: pid, VariantID: vid, LocationID: fx.locationID, UOM: "pcs", Qty: qty}})
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if _, err := testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, n.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); err != nil {
		t.Fatalf("confirm shipment: %v", err)
	}
	return n.ID
}

func costRows(t *testing.T, pid int64) (unit float64, estimated int) {
	t.Helper()
	if _, err := testMods.finance.Service().Sync(ctx(), 1); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if err := testMods.db.QueryRow(
		`SELECT unit_cost, is_estimated FROM finance_inventory_cost_movements WHERE product_id = ? AND direction='in' ORDER BY id LIMIT 1`, pid).Scan(&unit, &estimated); err != nil {
		t.Fatalf("cost row: %v", err)
	}
	return
}

// F2b-11: a product nobody can price is NEVER booked at zero cost — the
// shipment's journal is refused with an actionable message.
func TestShipmentWithoutKnownCostIsBlocked(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	p := productWith(t, "NOCOST", 0, 1)                     // no catalog purchase price
	seedIn(t, fx, p.ID, p.Variants[0].ID, fx.locationID, 5) // no cost hint either
	noteID := sellAndDeliver(t, fx, p.ID, p.Variants[0].ID, 2)
	_, err := testMods.finance.Service().Post(ctx(), 0, fx.branchID, "delivery", noteID)
	ae := mustAppErr(t, err)
	if !strings.Contains(ae.Message, "Harga modal") {
		t.Fatalf("post = %+v, want a missing-cost conflict", ae)
	}
}

// F2b-11: an explicit cost on opening stock is exact (not estimated) and is
// what the shipment's COGS uses.
func TestOpeningWithExplicitCostIsExact(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	p := productWith(t, "COSTED", 0, 1)
	seedInCost(t, fx, p.ID, p.Variants[0].ID, fx.locationID, 10, 7000)
	unit, est := costRows(t, p.ID)
	if unit != 7000 || est != 0 {
		t.Fatalf("cost row = %v estimated=%d, want 7000 exact", unit, est)
	}
	noteID := sellAndDeliver(t, fx, p.ID, p.Variants[0].ID, 3)
	legs := postedLegs(t, "delivery", noteID)
	if legs["5000"][0] != 21000 || legs["1200"][1] != 21000 {
		t.Fatalf("legs = %v, want COGS/inventory 21000", legs)
	}
}

// F2b-11 (N5): the catalog purchase price is per PURCHASE unit and is divided
// by the purchase→base factor: 120.000 per dus of 12 = 10.000 per pcs.
func TestCatalogCostDividesByPurchaseFactor(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	p := productWith(t, "DUS12", 120000, 12)
	seedIn(t, fx, p.ID, p.Variants[0].ID, fx.locationID, 12) // no hint: falls to the catalog
	unit, est := costRows(t, p.ID)
	if unit != 10000 || est != 1 {
		t.Fatalf("cost row = %v estimated=%d, want 10000 estimated", unit, est)
	}
}

// F2b-11 (N4): a return into a position the sale emptied is valued at the last
// known cost, not zero — inventory and COGS are restored.
func TestReturnIntoEmptiedPositionKeepsCost(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	p := productWith(t, "EMPTYRET", 0, 1)
	seedInCost(t, fx, p.ID, p.Variants[0].ID, fx.locationID, 1, 10000)
	so, err := testMods.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none",
		Items: []salesapp.LineInput{{ProductID: p.ID, VariantID: p.Variants[0].ID, UOM: "pcs", Qty: 1}},
	})
	if err != nil {
		t.Fatalf("SO: %v", err)
	}
	_, _ = testMods.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, so.ID)
	n, _ := testMods.delivery.Service().Create(ctx(), 0, fx.branchID, so.ID, "2026-09-10", "", deliveryapp.DocInput{},
		[]deliveryapp.LineInput{{ProductID: p.ID, VariantID: p.Variants[0].ID, LocationID: fx.locationID, UOM: "pcs", Qty: 1}})
	if _, err := testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, n.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	ret := createSalesReturnFor(t, fx, so.ID, p.ID, p.Variants[0].ID, 1)
	if _, err := testMods.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm return: %v", err)
	}
	legs := postedLegs(t, "sales_return", ret.ID)
	if legs["1200"][0] != 10000 || legs["5000"][1] != 10000 {
		t.Fatalf("return legs = %v, want Dr 1200 / Cr 5000 10000", legs)
	}
}

func createSalesReturnFor(t *testing.T, fx shopFixture, soID, pid, vid int64, qty float64) *salesreturncontracts.SalesReturn {
	t.Helper()
	ret, err := testMods.salesreturn.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", "return_only",
		[]salesreturnapp.LineInput{{ProductID: pid, VariantID: vid, LocationID: fx.locationID, UOM: "pcs", Qty: qty}}, nil,
		salesreturnapp.WithReason("uji"))
	if err != nil {
		t.Fatalf("create return: %v", err)
	}
	return ret
}
