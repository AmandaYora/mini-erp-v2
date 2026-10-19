package integration

import (
	"strings"
	"testing"

	productapp "mini-erp/internal/modules/product/application"
	"mini-erp/internal/modules/product/contracts"
	salesapp "mini-erp/internal/modules/sales/application"
	stockapp "mini-erp/internal/modules/stock/application"
)

func f3bProduct(t *testing.T, in productapp.ProductInput) *contracts.Product {
	t.Helper()
	if in.Type == "" {
		in.Type = "barang"
		in.Tracked = true
	}
	if in.BaseUOM == "" {
		in.BaseUOM, in.PurchaseUOM, in.SalesUOM = "pcs", "pcs", "pcs"
	}
	in.PurchaseFactor, in.SalesFactor = 1, 1
	if in.PurchasePrice == 0 {
		in.PurchasePrice = 8000
	}
	if in.SellingPrice == 0 {
		in.SellingPrice = 12000
	}
	p, err := testMods.product.Service().CreateProduct(ctx(), 0, in)
	if err != nil {
		t.Fatalf("create product %s: %v", in.Code, err)
	}
	return p
}

func f3bUpdateInput(p *contracts.Product) productapp.ProductInput {
	return productapp.ProductInput{
		Code: p.Code, Name: p.Name, CategoryID: p.CategoryID, Type: p.Type, Tracked: p.Tracked,
		BaseUOM: p.BaseUOM, PurchaseUOM: p.PurchaseUOM, SalesUOM: p.SalesUOM,
		PurchaseFactor: p.PurchaseFactor, SalesFactor: p.SalesFactor,
		PurchasePrice: p.PurchasePrice, SellingPrice: p.SellingPrice, MinSellingPrice: p.MinSellingPrice,
		MinStock: p.MinStock,
	}
}

func f3bStock(t *testing.T, fx shopFixture, productID, variantID int64, qty float64) {
	t.Helper()
	if err := testMods.stock.Service().Adjust(ctx(), 0, fx.branchID, stockapp.AdjustInput{
		ProductID: productID, VariantID: variantID, Location: fx.locationID,
		Mode: "in", QtyDelta: qty, Reason: "uji F3b",
	}); err != nil {
		t.Fatalf("adjust: %v", err)
	}
}

// F3b-3.1: the stock facts reach the product guards through the wired
// provider; a product with stock/history cannot be archived, re-unit'ed or
// untracked, and a code change leaves the scannable default variant intact.
func TestF3bProductGuardsWithStock(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.product.Service()
	p, err := svc.GetByID(ctx(), fx.productID)
	if err != nil || p == nil {
		t.Fatalf("get: %v", err)
	}
	if len(p.Variants) != 1 || !p.Variants[0].IsHidden || !p.Variants[0].IsDefault || p.Variants[0].Code != "TST-001" {
		t.Fatalf("a plain product owns one hidden default variant coded by the product code: %+v", p.Variants)
	}

	// Archive refused while stock exists anywhere.
	if ae := mustAppErr(t, svc.ArchiveProduct(ctx(), 0, p.ID)); !strings.Contains(ae.Message, "stok") {
		t.Fatalf("archive with stock: %+v", ae)
	}
	// Units / factors lock with history.
	in := f3bUpdateInput(p)
	in.PurchaseFactor = 6
	if _, err := svc.UpdateProduct(ctx(), 0, p.ID, in); err == nil {
		t.Fatal("factor change with stock history must be refused")
	}
	in = f3bUpdateInput(p)
	in.Tracked = false
	if _, err := svc.UpdateProduct(ctx(), 0, p.ID, in); err == nil {
		t.Fatal("untracking a product with stock must be refused")
	}
	// Explicit variants that would orphan the default's stock are refused.
	in = f3bUpdateInput(p)
	in.Variants = []productapp.VariantInput{{Code: "RED", Name: "Merah", IsDefault: true}}
	if _, err := svc.UpdateProduct(ctx(), 0, p.ID, in); err == nil {
		t.Fatal("switching to explicit variants while the default holds stock must be refused")
	}

	// A harmless edit with the stored variants echoed back keeps the default hidden.
	in = f3bUpdateInput(p)
	in.SellingPrice = 16000
	in.Variants = []productapp.VariantInput{{ID: p.Variants[0].ID, Code: "TST-001", Name: "Barang Uji", IsDefault: true}}
	up, err := svc.UpdateProduct(ctx(), 0, p.ID, in)
	if err != nil {
		t.Fatalf("echoed update: %v", err)
	}
	if up.SellingPrice != 16000 || len(up.Variants) != 1 || !up.Variants[0].IsHidden {
		t.Fatalf("echoed update changed the variant set: %+v", up.Variants)
	}

	// Renaming the product code: the default variant keeps its code (stock
	// history froze it) and both codes still scan to the same variant.
	in = f3bUpdateInput(up)
	in.Code = "TST-NEW"
	renamed, err := svc.UpdateProduct(ctx(), 0, p.ID, in)
	if err != nil {
		t.Fatalf("rename product code: %v", err)
	}
	if len(renamed.Variants) != 1 || renamed.Variants[0].ID != fx.variantID || renamed.Variants[0].Code != "TST-001" {
		t.Fatalf("variant with history must keep id and code: %+v", renamed.Variants)
	}
	for _, code := range []string{"TST-NEW", "TST-001"} {
		m, err := svc.ResolveScanCode(ctx(), code)
		if err != nil || m == nil || m.Variant == nil || m.Variant.ID != fx.variantID || m.Product.ID != p.ID {
			t.Fatalf("scan %q = %+v %v", code, m, err)
		}
	}
	// The stock scan shows the same answer.
	res, err := testMods.stock.Service().Scan(ctx(), fx.branchID, "TST-NEW")
	if err != nil || res.Status != "ok" || len(res.Balances) == 0 {
		t.Fatalf("stock scan by product code: %+v %v", res, err)
	}

	// Without stock or history the same operations are allowed.
	free := f3bProduct(t, productapp.ProductInput{Code: "FREE-1", Name: "Bebas"})
	in = f3bUpdateInput(free)
	in.Code = "FREE-2"
	in.PurchaseFactor, in.PurchaseUOM = 6, "dus"
	fr, err := svc.UpdateProduct(ctx(), 0, free.ID, in)
	if err != nil || fr.Code != "FREE-2" || fr.Variants[0].Code != "FREE-2" {
		t.Fatalf("free product: default must follow the code: %+v %v", fr, err)
	}
	if err := svc.ArchiveProduct(ctx(), 0, free.ID); err != nil {
		t.Fatalf("archive free product: %v", err)
	}
}

// Variant retirement is judged per variant against stock facts.
func TestF3bVariantStockGuards(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.product.Service()
	p := f3bProduct(t, productapp.ProductInput{Code: "VAR-1", Name: "Berwarna", Variants: []productapp.VariantInput{
		{Code: "VAR-1-R", Name: "Merah", Barcode: "8990001", IsDefault: true},
		{Code: "VAR-1-B", Name: "Biru"},
	}})
	if len(p.Variants) != 2 {
		t.Fatalf("variants: %+v", p.Variants)
	}
	var red, blue *contracts.Variant
	for _, v := range p.Variants {
		switch v.Code {
		case "VAR-1-R":
			red = v
		case "VAR-1-B":
			blue = v
		}
	}
	f3bStock(t, fx, p.ID, red.ID, 5)

	// Dropping the red variant (stock) is refused with the legacy message.
	in := f3bUpdateInput(p)
	in.Variants = []productapp.VariantInput{{ID: blue.ID, Code: "VAR-1-B", Name: "Biru", IsDefault: true}}
	ae := mustAppErr(t, errOnly(svc.UpdateProduct(ctx(), 0, p.ID, in)))
	if len(ae.Fields) == 0 || !strings.Contains(ae.Fields[0].Message, "masih punya stok") {
		t.Fatalf("variant with stock must not be dropped: %+v", ae)
	}
	// So is hiding it.
	in.Variants = []productapp.VariantInput{
		{ID: red.ID, Code: "VAR-1-R", Name: "Merah", Barcode: "8990001", Status: "inactive"},
		{ID: blue.ID, Code: "VAR-1-B", Name: "Biru", IsDefault: true},
	}
	if _, err := svc.UpdateProduct(ctx(), 0, p.ID, in); err == nil {
		t.Fatal("hiding a variant with stock must be refused")
	}
	// Renaming a variant with history is refused; its sibling can still change.
	in.Variants = []productapp.VariantInput{
		{ID: red.ID, Code: "VAR-1-MERAH", Name: "Merah", Barcode: "8990001", IsDefault: true},
		{ID: blue.ID, Code: "VAR-1-B", Name: "Biru"},
	}
	if _, err := svc.UpdateProduct(ctx(), 0, p.ID, in); err == nil {
		t.Fatal("renaming a variant with stock history must be refused")
	}
	// A payload that simply omits variants never touches them.
	in = f3bUpdateInput(p)
	in.Name = "Berwarna 2"
	up, err := svc.UpdateProduct(ctx(), 0, p.ID, in)
	if err != nil || len(up.Variants) != 2 {
		t.Fatalf("omitted variants must keep the set: %+v %v", up, err)
	}
	// The never-used blue variant can be dropped.
	in.Variants = []productapp.VariantInput{{ID: red.ID, Code: "VAR-1-R", Name: "Merah", Barcode: "8990001", IsDefault: true}}
	up, err = svc.UpdateProduct(ctx(), 0, p.ID, in)
	if err != nil || len(up.Variants) != 1 || up.Variants[0].ID != red.ID {
		t.Fatalf("dropping an unused variant: %+v %v", up, err)
	}
	var status string
	if err := testMods.db.QueryRow(`SELECT status FROM product_variants WHERE id = ?`, blue.ID).Scan(&status); err != nil || status != "archived" {
		t.Fatalf("dropped variant must be archived, not deleted: %q %v", status, err)
	}
}

func errOnly(_ *contracts.Product, err error) error { return err }

// F3b-3.2: barcode, variant code, product code resolve in that order.
func TestF3bScanResolution(t *testing.T) {
	freshDB(t)
	svc := testMods.product.Service()
	a := f3bProduct(t, productapp.ProductInput{Code: "SC-A", Name: "Produk A", Variants: []productapp.VariantInput{
		{Code: "S", Name: "Kecil", Barcode: "7770001", IsDefault: true}, {Code: "L", Name: "Besar"}}})
	b := f3bProduct(t, productapp.ProductInput{Code: "SC-B", Name: "Produk B", Variants: []productapp.VariantInput{
		{Code: "S", Name: "Kecil B"}}})
	plain := f3bProduct(t, productapp.ProductInput{Code: "SC-P", Name: "Polos"})

	m, err := svc.ResolveScanCode(ctx(), "7770001")
	if err != nil || m == nil || m.Via != contracts.ViaBarcode || m.Variant.Code != "S" || m.Product.ID != a.ID {
		t.Fatalf("barcode: %+v %v", m, err)
	}
	if m, _ = svc.ResolveScanCode(ctx(), "l"); m == nil || m.Via != contracts.ViaVariantCode || m.Product.ID != a.ID {
		t.Fatalf("unique variant code (case-insensitive): %+v", m)
	}
	// "S" exists on two products: ambiguous, never guessed.
	if m, _ = svc.ResolveScanCode(ctx(), "S"); m != nil {
		t.Fatalf("ambiguous variant code must not resolve: %+v", m)
	}
	// A product code with several visible variants asks for a choice.
	if m, _ = svc.ResolveScanCode(ctx(), "SC-A"); m == nil || !m.VariantRequired || m.Variant != nil || m.Via != contracts.ViaProductCode {
		t.Fatalf("product code with variants: %+v", m)
	}
	// One visible variant: resolves to it.
	if m, _ = svc.ResolveScanCode(ctx(), "SC-B"); m == nil || m.VariantRequired || m.Variant == nil || m.Variant.ProductID != b.ID {
		t.Fatalf("single variant product: %+v", m)
	}
	// Plain product: the hidden default.
	if m, _ = svc.ResolveScanCode(ctx(), "SC-P"); m == nil || m.Variant == nil || !m.Variant.IsHidden || m.Product.ID != plain.ID {
		t.Fatalf("plain product: %+v", m)
	}
	if m, _ = svc.ResolveScanCode(ctx(), "NOPE"); m != nil {
		t.Fatalf("unknown code: %+v", m)
	}
	// ResolveScan (variant-only contract) mirrors it.
	if v, _ := svc.ResolveScan(ctx(), "SC-A"); v != nil {
		t.Fatal("ResolveScan must not guess a variant")
	}

	// Inactive products answer Inactive, and stop selling.
	in := f3bUpdateInput(plain)
	in.Status = "inactive"
	if _, err := svc.UpdateProduct(ctx(), 0, plain.ID, in); err != nil {
		t.Fatal(err)
	}
	if m, _ = svc.ResolveScanCode(ctx(), "SC-P"); m == nil || !m.Inactive {
		t.Fatalf("inactive product scan: %+v", m)
	}
	if v, _ := svc.ResolveScan(ctx(), "SC-P"); v != nil {
		t.Fatal("ResolveScan must not resolve an inactive product")
	}
	fx := newShop(t, ctx())
	_, err = testMods.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none",
		Items: []salesapp.LineInput{{ProductID: plain.ID, VariantID: plain.Variants[0].ID, UOM: "pcs", Qty: 1}},
	})
	if err == nil {
		t.Fatal("an inactive product must not be sellable")
	}
	// The stock scan surfaces the status instead of an error.
	res, err := testMods.stock.Service().Scan(ctx(), fx.branchID, "SC-A")
	if err != nil || res.Status != "variant_required" || len(res.Variants) != 2 {
		t.Fatalf("stock scan variant_required: %+v %v", res, err)
	}
	res, err = testMods.stock.Service().Scan(ctx(), fx.branchID, "SC-P")
	if err != nil || res.Status != "inactive" {
		t.Fatalf("stock scan inactive: %+v %v", res, err)
	}
}

// F3b-3.3: product code, variant code and barcode share one namespace.
func TestF3bScanKeyCollisions(t *testing.T) {
	freshDB(t)
	svc := testMods.product.Service()
	f3bProduct(t, productapp.ProductInput{Code: "COL-A", Name: "A", Variants: []productapp.VariantInput{
		{Code: "COL-A-1", Name: "A1", Barcode: "BAR-1", IsDefault: true}}})

	create := func(in productapp.ProductInput) error {
		in.Type, in.Tracked = "barang", true
		in.BaseUOM, in.PurchaseUOM, in.SalesUOM = "pcs", "pcs", "pcs"
		in.PurchaseFactor, in.SalesFactor, in.PurchasePrice, in.SellingPrice = 1, 1, 100, 200
		_, err := svc.CreateProduct(ctx(), 0, in)
		return err
	}
	// A product code equal to another product's variant barcode / code.
	for _, code := range []string{"BAR-1", "COL-A-1"} {
		ae := mustAppErr(t, create(productapp.ProductInput{Code: code, Name: "X"}))
		if !strings.Contains(ae.Message, "varian") {
			t.Errorf("product code %q vs variant key: %+v", code, ae)
		}
	}
	// A variant barcode equal to another product's code / variant barcode / code.
	for _, key := range []string{"COL-A", "BAR-1", "COL-A-1"} {
		err := create(productapp.ProductInput{Code: "COL-B", Name: "B", Variants: []productapp.VariantInput{
			{Code: "COL-B-1", Name: "B1", Barcode: key, IsDefault: true}}})
		if err == nil {
			t.Errorf("variant barcode %q must collide", key)
		}
	}
	// A variant code equal to another product's code or barcode.
	for _, key := range []string{"COL-A", "BAR-1"} {
		err := create(productapp.ProductInput{Code: "COL-C", Name: "C", Variants: []productapp.VariantInput{
			{Code: key, Name: "C1", IsDefault: true}}})
		if err == nil {
			t.Errorf("variant code %q must collide", key)
		}
	}
	// Inside one payload: barcode equal to a sibling's code.
	if err := create(productapp.ProductInput{Code: "COL-D", Name: "D", Variants: []productapp.VariantInput{
		{Code: "D1", Name: "D1", IsDefault: true}, {Code: "D2", Name: "D2", Barcode: "D1"}}}); err == nil {
		t.Error("barcode equal to a sibling variant code must collide")
	}
	// The same variant code on different products is allowed.
	if err := create(productapp.ProductInput{Code: "COL-E", Name: "E", Variants: []productapp.VariantInput{
		{Code: "COL-A-1X", Name: "E1", IsDefault: true}, {Code: "SHARED", Name: "E2"}}}); err != nil {
		t.Errorf("distinct keys: %v", err)
	}
	if err := create(productapp.ProductInput{Code: "COL-F", Name: "F", Variants: []productapp.VariantInput{
		{Code: "SHARED", Name: "F1", IsDefault: true}}}); err != nil {
		t.Errorf("a variant code may repeat across products: %v", err)
	}
}

// F3b-3.4: product types and status.
func TestF3bProductTypesAndStatus(t *testing.T) {
	freshDB(t)
	svc := testMods.product.Service()
	for _, typ := range []string{"paket", "non_stok", "jasa"} {
		p, err := svc.CreateProduct(ctx(), 0, productapp.ProductInput{
			Code: "T-" + typ, Name: typ, Type: typ, Tracked: true,
			BaseUOM: "unit", PurchaseUOM: "dus", SalesUOM: "dus", PurchaseFactor: 12, SalesFactor: 12,
			PurchasePrice: 10, SellingPrice: 20,
		})
		if err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		if p.Tracked || p.PurchaseFactor != 1 || p.SalesFactor != 1 || p.PurchaseUOM != "unit" || p.SalesUOM != "unit" {
			t.Errorf("%s must be untracked, factor 1, one unit: %+v", typ, p)
		}
	}
	if _, err := svc.CreateProduct(ctx(), 0, productapp.ProductInput{Code: "BAD", Name: "x", Type: "physical",
		BaseUOM: "pcs", PurchaseUOM: "pcs", SalesUOM: "pcs", PurchaseFactor: 1, SalesFactor: 1}); err == nil {
		t.Error("unknown type must be rejected")
	}
	p := f3bProduct(t, productapp.ProductInput{Code: "ST-1", Name: "Status", Summary: "ringkas",
		Attributes: map[string]string{"Merek": "Contoh"}})
	if p.Summary != "ringkas" || p.Attributes["Merek"] != "Contoh" {
		t.Fatalf("summary/attributes round trip: %+v", p)
	}
	// List filters: type, tracked, status.
	l, err := svc.List(ctx(), productapp.ListFilter{Type: "paket"}, 1, 20)
	if err != nil || l.Total != 1 || l.Products[0].Code != "T-PAKET" {
		t.Fatalf("type filter: %+v %v", l, err)
	}
	tr := true
	if l, _ = svc.List(ctx(), productapp.ListFilter{Tracked: &tr}, 1, 20); l.Total != 1 {
		t.Fatalf("tracked filter total = %d", l.Total)
	}
	in := f3bUpdateInput(p)
	in.Status = "inactive"
	if _, err := svc.UpdateProduct(ctx(), 0, p.ID, in); err != nil {
		t.Fatal(err)
	}
	if l, _ = svc.List(ctx(), productapp.ListFilter{Status: "inactive"}, 1, 20); l.Total != 1 {
		t.Fatalf("status filter total = %d", l.Total)
	}
	opts, err := svc.SearchOptions(ctx(), productapp.ListFilter{Search: "ST-1"}, 5)
	if err != nil || len(opts) != 0 {
		t.Fatalf("search-options hides inactive by default: %+v %v", opts, err)
	}
	opts, _ = svc.SearchOptions(ctx(), productapp.ListFilter{Search: "T-PAKET"}, 5)
	if len(opts) != 1 || len(opts[0].Variants) != 1 {
		t.Fatalf("search-options carries variants: %+v", opts)
	}
}
