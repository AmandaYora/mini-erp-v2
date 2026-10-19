package integration

import (
	"strings"
	"testing"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	productapp "mini-erp/internal/modules/product/application"
	salesapp "mini-erp/internal/modules/sales/application"
)

func i64(v int64) *int64 { return &v }

// minPriceProduct creates a product whose catalog price is 15000 and whose
// minimum selling price is 12000.
func minPriceProduct(t *testing.T, code string) (productID, variantID int64) {
	t.Helper()
	p, err := testMods.product.Service().CreateProduct(ctx(), 0, productapp.ProductInput{
		Code: code, Name: "Barang " + code, Type: "barang", Tracked: true,
		BaseUOM: "pcs", PurchaseUOM: "pcs", SalesUOM: "pcs", PurchaseFactor: 1, SalesFactor: 1,
		PurchasePrice: 8000, SellingPrice: 15000, MinSellingPrice: 12000,
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	return p.ID, p.Variants[0].ID
}

func orderWith(fx shopFixture, line salesapp.LineInput) (int64, error) {
	so, err := testMods.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none",
		Items: []salesapp.LineInput{line},
	})
	if err != nil {
		return 0, err
	}
	return so.ID, nil
}

// F2a-10a: a manual price is accepted above the floor, refused below it, and
// is recorded as manual with the catalog price as its base.
func TestManualPriceAndMinimumFloor(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	pid, vid := minPriceProduct(t, "MIN-001")
	line := func(price int64) salesapp.LineInput {
		return salesapp.LineInput{ProductID: pid, VariantID: vid, UOM: "pcs", Qty: 2, UnitPrice: i64(price)}
	}
	_, err := orderWith(fx, line(11000))
	ae := mustAppErr(t, err)
	if len(ae.Fields) == 0 || !strings.Contains(ae.Fields[0].Message, "minimum") {
		t.Fatalf("below floor = %+v, want a minimum-price rejection", ae)
	}
	id, err := orderWith(fx, line(13000))
	if err != nil {
		t.Fatalf("above floor: %v", err)
	}
	so, _ := testMods.sales.Service().GetByID(ctx(), id)
	it := so.Items[0]
	if it.UnitPrice != 13000 || it.PricingSource != "manual" || it.BasePrice != 15000 {
		t.Fatalf("item = price %d source %s base %d", it.UnitPrice, it.PricingSource, it.BasePrice)
	}
	// Catalog line keeps the catalog source.
	cid, err := orderWith(fx, salesapp.LineInput{ProductID: pid, VariantID: vid, UOM: "pcs", Qty: 1})
	if err != nil {
		t.Fatalf("catalog line: %v", err)
	}
	cso, _ := testMods.sales.Service().GetByID(ctx(), cid)
	if cso.Items[0].PricingSource != "catalog" {
		t.Fatalf("source = %s, want catalog", cso.Items[0].PricingSource)
	}
}

// F2a-10c: per-line discount reason, note and taxable=false are stored; an
// exempt line adds no tax; tax base/amount are frozen on the line.
func TestLineSnapshotAndTaxableExemption(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	taxable, exempt := true, false
	p2 := newProduct(t, "EX-002")
	order, err := testMods.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "exclude", TaxRate: 10,
		Items: []salesapp.LineInput{
			{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 2, Taxable: &taxable,
				DiscountNominal: 1000, DiscountReason: "pelanggan lama", Note: "kirim pagi"},
			{ProductID: p2.ID, VariantID: p2.Variants[0].ID, UOM: "pcs", Qty: 1, Taxable: &exempt},
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	a, b := order.Items[0], order.Items[1]
	if a.DiscountReason != "pelanggan lama" || a.Note != "kirim pagi" || a.TaxAmount != 2900 || a.TaxBase != 29000 {
		t.Fatalf("taxable line = %+v", a)
	}
	if b.Taxable || b.TaxAmount != 0 || b.LineTotal != 15000 {
		t.Fatalf("exempt line = %+v", b)
	}
	if order.TaxTotal != 2900 || order.GrandTotal != 29000+2900+15000 {
		t.Fatalf("header tax %d grand %d", order.TaxTotal, order.GrandTotal)
	}
}

// F2a-7: a service product on an order ships without a location, moves no
// stock, and the order still completes; the goods line moves stock normally.
func TestServiceLineMovesNoStock(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc, err := testMods.product.Service().CreateProduct(ctx(), 0, productapp.ProductInput{
		Code: "JASA-001", Name: "Jasa Pasang", Type: "jasa", Tracked: false,
		BaseUOM: "pcs", PurchaseUOM: "pcs", SalesUOM: "pcs", PurchaseFactor: 1, SalesFactor: 1,
		PurchasePrice: 0, SellingPrice: 50000,
	})
	if err != nil {
		t.Fatalf("create service product: %v", err)
	}
	so, err := testMods.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none",
		Items: []salesapp.LineInput{
			{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 2},
			{ProductID: svc.ID, VariantID: svc.Variants[0].ID, UOM: "pcs", Qty: 1},
		},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if _, err := testMods.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	for _, it := range so.Items {
		if it.ProductID == svc.ID && it.StockTracked {
			t.Fatal("service line snapshot says stock-tracked")
		}
	}
	note, err := testMods.delivery.Service().Create(ctx(), 0, fx.branchID, so.ID, "2026-09-10", "", deliveryapp.DocInput{},
		[]deliveryapp.LineInput{
			{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 2},
			{ProductID: svc.ID, VariantID: svc.Variants[0].ID, LocationID: 0, UOM: "pcs", Qty: 1},
		})
	if err != nil {
		t.Fatalf("draft shipment with a service line: %v", err)
	}
	if _, err := testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, note.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); err != nil {
		t.Fatalf("confirm shipment: %v", err)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 48 {
		t.Fatalf("goods stock = %v, want 48", got)
	}
	var n int
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM stock_movements WHERE product_id = ?`, svc.ID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("service movements = %d, %v; want 0", n, err)
	}
	if got := orderStatus(t, so.ID); got != "completed" {
		t.Fatalf("order = %s, want completed", got)
	}
}
