package integration

import (
	"strings"
	"testing"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	goodsreceiptapp "mini-erp/internal/modules/goodsreceipt/application"
	partyapp "mini-erp/internal/modules/party/application"
	productapp "mini-erp/internal/modules/product/application"
	purchasereturnapp "mini-erp/internal/modules/purchasereturn/application"
	purchasingapp "mini-erp/internal/modules/purchasing/application"
	salesapp "mini-erp/internal/modules/sales/application"
	salesreturnapp "mini-erp/internal/modules/salesreturn/application"
	stockapp "mini-erp/internal/modules/stock/application"
)

// Temuan 1 (2026-10-23): a line bought/sold per DUS (12 PCS) can be returned
// per PCS. The context reports what is left in the line's own unit and in base
// units; a mixed return (1 DUS + 6 PCS = 18 PCS) is valued pro-rata from the
// source line, and 25 PCS against 24 is refused.

func dusProduct(t *testing.T, fx shopFixture, code string) shopFixture {
	t.Helper()
	p, err := testMods.product.Service().CreateProduct(ctx(), 0, productapp.ProductInput{
		Code: code, Name: "Pipa Dus " + code, Type: "barang", Tracked: true,
		BaseUOM: "pcs", PurchaseUOM: "dus", SalesUOM: "dus",
		PurchaseFactor: 12, SalesFactor: 12, PurchasePrice: 120000, SellingPrice: 180000,
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	fx.productID, fx.variantID = p.ID, p.Variants[0].ID
	return fx
}

func contextLine(t *testing.T, ctxMap map[string]any) map[string]any {
	t.Helper()
	lines, _ := ctxMap["lines"].([]any)
	if len(lines) != 1 {
		t.Fatalf("context lines = %v", ctxMap["lines"])
	}
	return lines[0].(map[string]any)
}

func TestPurchaseReturnPerBaseUnitOfDusLine(t *testing.T) {
	freshDB(t)
	fx := dusProduct(t, newShop(t, ctx()), "DUS-PO")
	m := testMods
	sup, err := m.party.Service().CreateSupplier(ctx(), 0, partyapp.PartyInput{Name: "Supplier Dus"})
	if err != nil {
		t.Fatal(err)
	}
	po, err := m.purchasing.Service().CreateOrder(ctx(), 0, fx.branchID, purchasingapp.OrderInput{
		PartyID: sup.ID, PaymentTerms: "net", DueDate: "2026-10-10", TaxType: "none",
		Items: []purchasingapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "dus", Qty: 2, UnitPrice: 120000}},
	})
	if err != nil {
		t.Fatalf("create PO: %v", err)
	}
	if _, err := m.purchasing.Service().ConfirmOrder(ctx(), 0, fx.branchID, po.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.goodsreceipt.Service().Create(ctx(), 0, fx.branchID, po.ID, "2026-09-10", "",
		[]goodsreceiptapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "dus", Qty: 2}}); err != nil {
		t.Fatalf("receive: %v", err)
	}
	cx, err := m.purchasereturn.Service().ReturnContext(ctx(), fx.branchID, po.ID)
	if err != nil {
		t.Fatal(err)
	}
	l := contextLine(t, cx)
	if l["remaining"] != 2.0 || l["remainingBase"] != 24.0 || l["baseUom"] != "pcs" || len(l["uoms"].([]map[string]any)) != 2 {
		t.Fatalf("context line = %v; want 2 dus / 24 pcs, units dus+pcs", l)
	}
	svc := m.purchasereturn.Service()
	_, err = svc.Create(ctx(), 0, fx.branchID, po.ID, "2026-09-11", "", []purchasereturnapp.LineInput{
		{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 25}})
	if err == nil || !strings.Contains(err.Error(), "Melebihi") {
		t.Fatalf("25 pcs of 24 = %v, want refused", err)
	}
	if _, err := svc.Create(ctx(), 0, fx.branchID, po.ID, "2026-09-11", "", []purchasereturnapp.LineInput{
		{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "kg", Qty: 1}}); err == nil {
		t.Fatal("a unit the product does not have was accepted")
	}
	ret, err := svc.Create(ctx(), 0, fx.branchID, po.ID, "2026-09-11", "", []purchasereturnapp.LineInput{
		{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "dus", Qty: 1},
		{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 6},
	})
	if err != nil {
		t.Fatalf("1 dus + 6 pcs: %v", err)
	}
	var base float64
	var total int64
	for _, it := range ret.Items {
		base += it.QtyBase
		total += it.LineTotal
		if it.UOM == "pcs" && (it.UnitPrice != 10000 || it.LineTotal != 60000) {
			t.Errorf("pcs line = %+v, want 10.000/pcs pro-rata, total 60.000", it)
		}
	}
	if base != 18 || total != 180000 {
		t.Fatalf("return = %v base / Rp%d, want 18 pcs / Rp180.000", base, total)
	}
	cx, _ = svc.ReturnContext(ctx(), fx.branchID, po.ID)
	if l := contextLine(t, cx); l["remainingBase"] != 6.0 || l["remaining"] != 0.5 {
		t.Fatalf("remaining after the draft = %v", l)
	}
}

func TestSalesReturnPerBaseUnitOfDusLine(t *testing.T) {
	freshDB(t)
	fx := dusProduct(t, newShop(t, ctx()), "DUS-SO")
	m := testMods
	if err := m.stock.Service().Adjust(ctx(), 0, fx.branchID, stockapp.AdjustInput{
		ProductID: fx.productID, VariantID: fx.variantID, Location: fx.locationID, Mode: "in", QtyDelta: 48, Reason: "stok uji dus",
	}); err != nil {
		t.Fatal(err)
	}
	so, err := m.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "net", DueDate: "2026-12-31", TaxType: "none",
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "dus", Qty: 2}},
	})
	if err != nil {
		t.Fatalf("create SO: %v", err)
	}
	if _, err := m.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
		t.Fatal(err)
	}
	dn, err := m.delivery.Service().Create(ctx(), 0, fx.branchID, so.ID, "", "", deliveryapp.DocInput{},
		[]deliveryapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "dus", Qty: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{}); err != nil {
		t.Fatal(err)
	}
	cx, err := m.salesreturn.Service().ReturnContext(ctx(), fx.branchID, so.ID)
	if err != nil {
		t.Fatal(err)
	}
	if l := contextLine(t, cx); l["remaining"] != 2.0 || l["remainingBase"] != 24.0 || l["delivered"] != 2.0 {
		t.Fatalf("context line = %v; want 2 dus delivered / 24 pcs left", l)
	}
	line := func(uom string, qty float64) salesreturnapp.LineInput {
		return salesreturnapp.LineInput{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: uom, Qty: qty}
	}
	if _, err := m.salesreturn.Service().Create(ctx(), 0, fx.branchID, so.ID, "", "", "return_only", []salesreturnapp.LineInput{line("pcs", 25)}, nil); err == nil ||
		!strings.Contains(err.Error(), "Melebihi") {
		t.Fatalf("25 pcs of 24 = %v, want refused", err)
	}
	ret, err := m.salesreturn.Service().Create(ctx(), 0, fx.branchID, so.ID, "", "", "return_only", []salesreturnapp.LineInput{line("dus", 1), line("pcs", 6)}, nil)
	if err != nil {
		t.Fatalf("1 dus + 6 pcs: %v", err)
	}
	var base float64
	var total int64
	for _, it := range ret.Items {
		base += it.QtyBase
		total += it.LineTotal
	}
	if base != 18 || total != so.GrandTotal*18/24 {
		t.Fatalf("return = %v base / Rp%d, want 18 pcs / Rp%d (pro-rata of %d)", base, total, so.GrandTotal*18/24, so.GrandTotal)
	}
}
