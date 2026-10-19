package integration

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"mini-erp/internal/modules/product/contracts"
	productinfra "mini-erp/internal/modules/product/infrastructure"
)

// Characterization of the product Repository (F8-18): every public method,
// every filter branch, NULL/empty edges, ordering and pagination.

func sqlcPtr64(v int64) *int64 { return &v }

func sqlcProdRepo() *productinfra.Repository { return productinfra.NewRepository(testMods.db) }

func sqlcMkProduct(code, name string, cat *int64) *contracts.Product {
	return &contracts.Product{
		Code: code, Name: name, CategoryID: cat, Type: "barang", Tracked: true,
		BaseUOM: "pcs", PurchaseUOM: "dus", SalesUOM: "pcs",
		PurchaseFactor: 12.5, SalesFactor: 1.123456789012,
		PurchasePrice: 10000, SellingPrice: 15000, MinSellingPrice: 12000, MinStock: 3.25,
		Status: "active", Summary: "ringkasan " + code, Attributes: map[string]string{"warna": "merah"},
	}
}

func TestSqlcProductCategories(t *testing.T) {
	freshDB(t)
	repo := sqlcProdRepo()

	if c, err := repo.CategoryByID(ctx(), 1); c != nil || err != nil {
		t.Fatalf("missing by id = %v %v", c, err)
	}
	if c, err := repo.CategoryByCode(ctx(), "X"); c != nil || err != nil {
		t.Fatalf("missing by code = %v %v", c, err)
	}
	if l, err := repo.ListCategories(ctx()); err != nil || len(l) != 0 {
		t.Fatalf("empty list = %v %v", l, err)
	}
	root, err := repo.CreateCategory(ctx(), &contracts.Category{Code: "ROOT", Name: "Zeta"}, 5)
	if err != nil || root == 0 {
		t.Fatal(err)
	}
	child, _ := repo.CreateCategory(ctx(), &contracts.Category{Code: "CH", Name: "Alpha", ParentID: sqlcPtr64(root)}, 5)
	grand, _ := repo.CreateCategory(ctx(), &contracts.Category{Code: "GR", Name: "Mid", ParentID: sqlcPtr64(child)}, 5)

	c, _ := repo.CategoryByID(ctx(), child)
	if c == nil || c.Code != "CH" || c.Name != "Alpha" || c.ParentID == nil || *c.ParentID != root || c.Status != "active" {
		t.Fatalf("child = %+v", c)
	}
	c, _ = repo.CategoryByCode(ctx(), "ROOT")
	if c == nil || c.ID != root || c.ParentID != nil {
		t.Fatalf("root = %+v", c)
	}
	l, _ := repo.ListCategories(ctx())
	if len(l) != 3 || l[0].Name != "Alpha" || l[1].Name != "Mid" || l[2].Name != "Zeta" {
		t.Fatalf("order by name = %+v", l)
	}

	if err := repo.UpdateCategory(ctx(), &contracts.Category{ID: grand, Name: "Mid2", ParentID: nil}, 6); err != nil {
		t.Fatal(err)
	}
	c, _ = repo.CategoryByID(ctx(), grand)
	if c.Name != "Mid2" || c.ParentID != nil || c.Code != "GR" {
		t.Fatalf("after update = %+v", c)
	}
	var by int64
	if err := testMods.db.QueryRow(`SELECT updated_by FROM product_categories WHERE id = ?`, grand).Scan(&by); err != nil || by != 6 {
		t.Fatalf("updated_by = %d %v", by, err)
	}
	if err := repo.UpdateCategory(ctx(), &contracts.Category{ID: grand, Name: "Mid2", ParentID: sqlcPtr64(child)}, 6); err != nil {
		t.Fatal(err)
	}

	// HasAncestor: chain grand -> child -> root.
	for _, tc := range []struct {
		start, target int64
		want          bool
	}{
		{grand, child, true}, {grand, root, true}, {child, root, true},
		{root, child, false}, {child, grand, false}, {999, root, false}, {root, 0, false},
	} {
		got, err := repo.HasAncestor(ctx(), tc.start, tc.target)
		if err != nil || got != tc.want {
			t.Fatalf("HasAncestor(%d,%d) = %v %v, want %v", tc.start, tc.target, got, err, tc.want)
		}
	}
	// A cycle (corrupt chain) terminates and reports true.
	if _, err := testMods.db.Exec(`UPDATE product_categories SET parent_id = ? WHERE id = ?`, grand, root); err != nil {
		t.Fatal(err)
	}
	if got, err := repo.HasAncestor(ctx(), grand, 12345); err != nil || !got {
		t.Fatalf("cycle = %v %v", got, err)
	}
	if _, err := testMods.db.Exec(`UPDATE product_categories SET parent_id = NULL WHERE id = ?`, root); err != nil {
		t.Fatal(err)
	}

	if n, err := repo.CountActiveSubcategories(ctx(), root); err != nil || n != 1 {
		t.Fatalf("subcats = %d %v", n, err)
	}
	if err := repo.SetCategoryStatus(ctx(), child, "archived", 9); err != nil {
		t.Fatal(err)
	}
	if n, _ := repo.CountActiveSubcategories(ctx(), root); n != 0 {
		t.Fatalf("archived child counted: %d", n)
	}
	if c, _ := repo.CategoryByID(ctx(), child); c.Status != "archived" {
		t.Fatalf("status = %q", c.Status)
	}
	if n, _ := repo.CountActiveSubcategories(ctx(), 999); n != 0 {
		t.Fatalf("n = %d", n)
	}

	// Names by IDs: dedupe, zeros, empty.
	names, err := repo.CategoryNamesByIDs(ctx(), []int64{root, child, root, 0, 777})
	if err != nil || len(names) != 2 || names[root] != "Zeta" || names[child] != "Alpha" {
		t.Fatalf("names = %v %v", names, err)
	}
	if names, err := repo.CategoryNamesByIDs(ctx(), nil); err != nil || names == nil || len(names) != 0 {
		t.Fatalf("empty names = %v %v", names, err)
	}
	if names, _ := repo.CategoryNamesByIDs(ctx(), []int64{0, 0}); len(names) != 0 {
		t.Fatalf("zero-only names = %v", names)
	}
}

func TestSqlcProductCrud(t *testing.T) {
	freshDB(t)
	repo := sqlcProdRepo()
	cat, _ := repo.CreateCategory(ctx(), &contracts.Category{Code: "C1", Name: "Cat"}, 1)

	if p, err := repo.GetProduct(ctx(), 1); p != nil || err != nil {
		t.Fatalf("missing = %v %v", p, err)
	}
	if p, err := repo.ProductByCode(ctx(), "NOPE"); p != nil || err != nil {
		t.Fatalf("missing by code = %v %v", p, err)
	}

	p := sqlcMkProduct("P-1", "Produk Satu", sqlcPtr64(cat))
	p.Variants = []*contracts.Variant{
		{Code: "P-1", Name: "Default", IsDefault: true, IsHidden: true, Barcode: "", SortOrder: 0},
		{Code: "P-1-L", Name: "Large", Barcode: "899001", SortOrder: 2, Attributes: map[string]string{"size": "L"}},
	}
	id, err := repo.CreateProduct(ctx(), p, 7)
	if err != nil || id == 0 {
		t.Fatalf("create: %d %v", id, err)
	}
	got, err := repo.GetProduct(ctx(), id)
	if err != nil || got == nil {
		t.Fatal(err)
	}
	want := sqlcMkProduct("P-1", "Produk Satu", sqlcPtr64(cat))
	want.ID = id
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
	byCode, _ := repo.ProductByCode(ctx(), "P-1")
	if byCode == nil || byCode.ID != id {
		t.Fatalf("by code = %+v", byCode)
	}
	vs, _ := repo.AllVariantsByProduct(ctx(), id)
	if len(vs) != 2 || vs[0].Code != "P-1" || !vs[0].IsDefault || !vs[0].IsHidden || vs[0].Barcode != "" ||
		vs[1].Barcode != "899001" || vs[1].SortOrder != 2 || vs[1].Attributes["size"] != "L" || len(vs[0].Attributes) != 0 ||
		vs[0].Attributes == nil || vs[0].Status != "active" || vs[0].ProductID != id {
		t.Fatalf("variants = %+v", vs)
	}
	var createdBy int64
	if err := testMods.db.QueryRow(`SELECT created_by FROM products WHERE id = ?`, id).Scan(&createdBy); err != nil || createdBy != 7 {
		t.Fatalf("created_by = %d %v", createdBy, err)
	}

	// NULL columns: no category, no summary, no attributes, empty status -> active.
	bare := sqlcMkProduct("P-2", "Bare", nil)
	bare.Summary, bare.Attributes, bare.Status, bare.Tracked = "", nil, "", false
	bare.Type = "jasa"
	id2, err := repo.CreateProduct(ctx(), bare, 0)
	if err != nil {
		t.Fatal(err)
	}
	g2, _ := repo.GetProduct(ctx(), id2)
	if g2.CategoryID != nil || g2.Summary != "" || g2.Attributes == nil || len(g2.Attributes) != 0 || g2.Status != "active" || g2.Tracked || g2.Type != "jasa" {
		t.Fatalf("bare = %+v", g2)
	}
	var attrNull, sumNull int
	if err := testMods.db.QueryRow(`SELECT attributes IS NULL, summary IS NULL FROM products WHERE id = ?`, id2).Scan(&attrNull, &sumNull); err != nil || attrNull != 1 || sumNull != 1 {
		t.Fatalf("NULL storage = %d %d %v", attrNull, sumNull, err)
	}
	// Corrupt JSON and non-string values read leniently.
	if _, err := testMods.db.Exec(`UPDATE products SET attributes = '{"n": 5, "b": true, "z": null, "s": "x"}' WHERE id = ?`, id2); err != nil {
		t.Fatal(err)
	}
	g2, _ = repo.GetProduct(ctx(), id2)
	if !reflect.DeepEqual(g2.Attributes, map[string]string{"n": "5", "b": "true", "s": "x"}) {
		t.Fatalf("lenient attrs = %v", g2.Attributes)
	}

	// Duplicate code is rejected and the tx rolled back (no stray rows).
	if _, err := repo.CreateProduct(ctx(), sqlcMkProduct("P-1", "Dup", nil), 1); err == nil {
		t.Fatal("duplicate code must fail")
	}
	dupVar := sqlcMkProduct("P-3", "DupVariant", nil)
	dupVar.Variants = []*contracts.Variant{{Code: "A", Name: "A"}, {Code: "A", Name: "A again"}}
	if _, err := repo.CreateProduct(ctx(), dupVar, 1); err == nil {
		t.Fatal("duplicate variant code must fail")
	}
	if p, _ := repo.ProductByCode(ctx(), "P-3"); p != nil {
		t.Fatal("failed create must roll back the product row")
	}

	// UpdateProduct: product fields + variants (update in place, insert new).
	upd := sqlcMkProduct("P-1B", "Satu Baru", nil)
	upd.ID = id
	upd.Summary, upd.Attributes, upd.Tracked, upd.Status = "", map[string]string{"k": "v"}, false, "archived"
	upd.PurchasePrice, upd.SellingPrice, upd.MinSellingPrice, upd.MinStock = 1, 2, 3, 0.5
	upd.PurchaseFactor, upd.SalesFactor = 0.000000000001, 24
	upd.Variants = []*contracts.Variant{
		{ID: vs[1].ID, Code: "P-1-XL", Name: "XL", Barcode: "", Status: "inactive", SortOrder: 5},
		{Code: "P-1-NEW", Name: "Baru", Barcode: "899002", Attributes: map[string]string{"a": "b"}},
		{ID: 99999, Code: "GHOST", Name: "Wrong product"}, // id of no row: no effect
	}
	if err := repo.UpdateProduct(ctx(), upd, 8); err != nil {
		t.Fatal(err)
	}
	g, _ := repo.GetProduct(ctx(), id)
	if g.Code != "P-1B" || g.Name != "Satu Baru" || g.CategoryID != nil || g.Summary != "" || g.Tracked || g.Status != "archived" ||
		g.PurchasePrice != 1 || g.SellingPrice != 2 || g.MinSellingPrice != 3 || g.MinStock != 0.5 ||
		g.PurchaseFactor != 0.000000000001 || g.SalesFactor != 24 || !reflect.DeepEqual(g.Attributes, map[string]string{"k": "v"}) {
		t.Fatalf("after update = %+v", g)
	}
	vs, _ = repo.AllVariantsByProduct(ctx(), id)
	if len(vs) != 3 || vs[1].Code != "P-1-XL" || vs[1].Status != "inactive" || vs[1].Barcode != "" || vs[1].SortOrder != 5 || len(vs[1].Attributes) != 0 ||
		vs[2].Code != "P-1-NEW" || vs[2].Barcode != "899002" || vs[2].Attributes["a"] != "b" || vs[0].Code != "P-1" {
		t.Fatalf("variants after update = %+v", vs)
	}
	// An update whose variant write fails rolls the product back too.
	bad := *upd
	bad.Name = "Should Not Stick"
	bad.Variants = []*contracts.Variant{{Code: "P-1-NEW", Name: "clash"}}
	if err := repo.UpdateProduct(ctx(), &bad, 8); err == nil {
		t.Fatal("variant code clash must fail")
	}
	if g, _ := repo.GetProduct(ctx(), id); g.Name != "Satu Baru" {
		t.Fatalf("update not rolled back: %q", g.Name)
	}

	if err := repo.SetProductStatus(ctx(), id, "active", 4); err != nil {
		t.Fatal(err)
	}
	if g, _ := repo.GetProduct(ctx(), id); g.Status != "active" {
		t.Fatalf("status = %q", g.Status)
	}
	if n, _ := repo.CountActiveProductsByCategory(ctx(), cat); n != 0 {
		t.Fatalf("category count (product moved out) = %d", n)
	}
	if _, err := testMods.db.Exec(`UPDATE products SET category_id = ? WHERE id = ?`, cat, id); err != nil {
		t.Fatal(err)
	}
	if n, _ := repo.CountActiveProductsByCategory(ctx(), cat); n != 1 {
		t.Fatalf("category count = %d", n)
	}
	if err := repo.SetProductStatus(ctx(), id, "archived", 4); err != nil {
		t.Fatal(err)
	}
	if n, _ := repo.CountActiveProductsByCategory(ctx(), cat); n != 0 {
		t.Fatalf("archived counted: %d", n)
	}
}

func TestSqlcProductBulkWrites(t *testing.T) {
	freshDB(t)
	repo := sqlcProdRepo()

	// WriteProducts: create + update in one tx; failure rolls back everything.
	a := sqlcMkProduct("W-1", "Write 1", nil)
	a.Variants = []*contracts.Variant{{Code: "W-1", Name: "d", IsDefault: true, IsHidden: true}}
	if err := repo.WriteProducts(ctx(), []productinfra.ProductWrite{{Product: a, Row: 2}}, 3); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.ProductByCode(ctx(), "W-1")
	b := sqlcMkProduct("W-2", "Write 2", nil)
	up := sqlcMkProduct("W-1", "Write 1 renamed", nil)
	up.ID = got.ID
	up.Variants = nil
	if err := repo.WriteProducts(ctx(), []productinfra.ProductWrite{{Product: b, Row: 3}, {Product: up, Row: 4}}, 3); err != nil {
		t.Fatal(err)
	}
	if g, _ := repo.GetProduct(ctx(), got.ID); g.Name != "Write 1 renamed" {
		t.Fatalf("name = %q", g.Name)
	}
	if p, _ := repo.ProductByCode(ctx(), "W-2"); p == nil {
		t.Fatal("W-2 missing")
	}
	dup := sqlcMkProduct("W-3", "Write 3", nil)
	clash := sqlcMkProduct("W-2", "Clash", nil)
	if err := repo.WriteProducts(ctx(), []productinfra.ProductWrite{{Product: dup}, {Product: clash}}, 3); err == nil {
		t.Fatal("clash must fail")
	}
	if p, _ := repo.ProductByCode(ctx(), "W-3"); p != nil {
		t.Fatal("W-3 must be rolled back")
	}
	if err := repo.WriteProducts(ctx(), nil, 3); err != nil {
		t.Fatalf("empty write = %v", err)
	}

	// ApplyCategories: parents before children, parent by code (in-batch and
	// pre-existing), update re-activates, sort_order stored.
	pre, _ := repo.CreateCategory(ctx(), &contracts.Category{Code: "PRE", Name: "Pre"}, 1)
	if err := repo.SetCategoryStatus(ctx(), pre, "archived", 1); err != nil {
		t.Fatal(err)
	}
	writes := []productinfra.CategoryWrite{
		{Code: "N1", Name: "New1", SortOrder: 4},
		{Code: "N2", Name: "New2", ParentCode: "n1", SortOrder: 2, Status: "archived"},
		{ID: pre, Code: "PRE", Name: "Pre Renamed", ParentCode: "N2", SortOrder: 9},
		{Code: "N3", Name: "New3", ParentCode: "PRE"},
	}
	if err := repo.ApplyCategories(ctx(), writes, 5); err != nil {
		t.Fatal(err)
	}
	n1, _ := repo.CategoryByCode(ctx(), "N1")
	n2, _ := repo.CategoryByCode(ctx(), "N2")
	n3, _ := repo.CategoryByCode(ctx(), "N3")
	p0, _ := repo.CategoryByID(ctx(), pre)
	if n1 == nil || n1.ParentID != nil || n1.Status != "active" ||
		n2 == nil || n2.ParentID == nil || *n2.ParentID != n1.ID || n2.Status != "archived" ||
		p0.Name != "Pre Renamed" || p0.Status != "active" || p0.ParentID == nil || *p0.ParentID != n2.ID ||
		n3 == nil || n3.ParentID == nil || *n3.ParentID != pre {
		t.Fatalf("categories: %+v %+v %+v %+v", n1, n2, n3, p0)
	}
	var so int
	if err := testMods.db.QueryRow(`SELECT sort_order FROM product_categories WHERE code = 'N1'`).Scan(&so); err != nil || so != 4 {
		t.Fatalf("sort_order = %d %v", so, err)
	}
	// Unknown parent code fails and rolls back the whole batch.
	err := repo.ApplyCategories(ctx(), []productinfra.CategoryWrite{
		{Code: "OK1", Name: "ok"}, {Code: "BAD", Name: "bad", ParentCode: "NOPE"},
	}, 5)
	if err == nil {
		t.Fatal("unknown parent must fail")
	}
	if c, _ := repo.CategoryByCode(ctx(), "OK1"); c != nil {
		t.Fatal("OK1 must be rolled back")
	}
}

func TestSqlcProductBatchReads(t *testing.T) {
	freshDB(t)
	repo := sqlcProdRepo()
	cat, _ := repo.CreateCategory(ctx(), &contracts.Category{Code: "C", Name: "C"}, 1)
	var ids []int64
	for i := 1; i <= 3; i++ {
		p := sqlcMkProduct(fmt.Sprintf("B-%d", i), fmt.Sprintf("Batch %d", i), sqlcPtr64(cat))
		p.PurchasePrice = int64(i * 1000)
		p.PurchaseFactor = float64(i * 2)
		id, err := repo.CreateProduct(ctx(), p, 1)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	// product 3 has purchase price 0 (excluded from base costs).
	if _, err := testMods.db.Exec(`UPDATE products SET purchase_price = 0 WHERE id = ?`, ids[2]); err != nil {
		t.Fatal(err)
	}
	// product 2 has a zero factor (-> 1).
	if _, err := testMods.db.Exec(`UPDATE products SET purchase_to_base_factor = 0 WHERE id = ?`, ids[1]); err != nil {
		t.Fatal(err)
	}

	m, err := repo.ProductsByIDs(ctx(), []int64{ids[0], ids[0], 0, ids[2], 9999})
	if err != nil || len(m) != 2 || m[ids[0]].Code != "B-1" || m[ids[2]].Code != "B-3" || m[ids[0]].Variants != nil {
		t.Fatalf("ProductsByIDs = %v %v", m, err)
	}
	if m, err := repo.ProductsByIDs(ctx(), nil); err != nil || m == nil || len(m) != 0 {
		t.Fatalf("empty = %v %v", m, err)
	}
	if m, _ := repo.ProductsByIDs(ctx(), []int64{0}); len(m) != 0 {
		t.Fatal("zero ids")
	}

	pm, err := repo.ProductsByCodes(ctx(), []string{" b-1 ", "B-2", "b-2", "", "nope"})
	if err != nil || len(pm) != 2 || pm["B-1"] == nil || pm["B-2"] == nil {
		t.Fatalf("ProductsByCodes = %v %v", pm, err)
	}
	if pm, err := repo.ProductsByCodes(ctx(), nil); err != nil || pm == nil || len(pm) != 0 {
		t.Fatalf("empty codes = %v %v", pm, err)
	}
	if pm, _ := repo.ProductsByCodes(ctx(), []string{"", "  "}); len(pm) != 0 {
		t.Fatal("blank codes")
	}
	ex, err := repo.ExistingCodes(ctx(), []string{"b-1", "B-3", "zz", "", "B-3"})
	if err != nil || len(ex) != 2 || !ex["B-1"] || !ex["B-3"] {
		t.Fatalf("ExistingCodes = %v %v", ex, err)
	}
	if ex, _ := repo.ExistingCodes(ctx(), nil); ex == nil || len(ex) != 0 {
		t.Fatal("empty existing")
	}

	pr, err := repo.PricingByIDs(ctx(), []int64{ids[0], ids[0], 0, 777, ids[1]})
	if err != nil || len(pr) != 2 {
		t.Fatalf("PricingByIDs = %v %v", pr, err)
	}
	if p := pr[ids[0]]; p.Code != "B-1" || p.Name != "Batch 1" || p.Status != "active" || p.SellingPrice != 15000 || p.MinSellingPrice != 12000 ||
		p.PurchasePrice != 1000 || p.PurchaseFactor != 2 || p.SalesFactor != 1.123456789012 {
		t.Fatalf("pricing = %+v", p)
	}
	if pr, _ := repo.PricingByIDs(ctx(), nil); pr == nil || len(pr) != 0 {
		t.Fatal("empty pricing")
	}

	costs, err := repo.PurchaseBaseCostsByIDs(ctx(), []int64{ids[0], ids[1], ids[2], 0, ids[0]})
	if err != nil || len(costs) != 2 || costs[ids[0]] != 500 || costs[ids[1]] != 2000 {
		t.Fatalf("costs = %v %v", costs, err)
	}
	if c, _ := repo.PurchaseBaseCostsByIDs(ctx(), nil); c == nil || len(c) != 0 {
		t.Fatal("empty costs")
	}
	pp, err := repo.PurchasePricesByIDs(ctx(), []int64{ids[0], ids[2], 0, 4242})
	if err != nil || len(pp) != 2 || pp[ids[0]] != 1000 || pp[ids[2]] != 0 {
		t.Fatalf("prices = %v %v", pp, err)
	}
	if c, _ := repo.PurchasePricesByIDs(ctx(), []int64{0}); c == nil || len(c) != 0 {
		t.Fatal("zero prices")
	}
	lb, err := repo.LabelsByIDs(ctx(), []int64{ids[0], ids[0], 0, ids[1], 31337})
	if err != nil || len(lb) != 2 || lb[ids[0]] != (contracts.Label{Code: "B-1", Name: "Batch 1"}) {
		t.Fatalf("labels = %v %v", lb, err)
	}
	if lb, _ := repo.LabelsByIDs(ctx(), nil); lb == nil || len(lb) != 0 {
		t.Fatal("empty labels")
	}

	// Large IN lists: more ids than one chunk (1000) for the chunked methods
	// and the plain ones.
	big := make([]int64, 0, 2500)
	for i := 0; i < 2500; i++ {
		big = append(big, int64(1_000_000+i))
	}
	big = append(big, ids...)
	if m, err := repo.ProductsByIDs(ctx(), big); err != nil || len(m) != 3 {
		t.Fatalf("big ProductsByIDs = %d %v", len(m), err)
	}
	if m, err := repo.PricingByIDs(ctx(), big); err != nil || len(m) != 3 {
		t.Fatalf("big PricingByIDs = %d %v", len(m), err)
	}
	if m, err := repo.LabelsByIDs(ctx(), big); err != nil || len(m) != 3 {
		t.Fatalf("big LabelsByIDs = %d %v", len(m), err)
	}
	if m, err := repo.PurchasePricesByIDs(ctx(), big); err != nil || len(m) != 3 {
		t.Fatalf("big PurchasePricesByIDs = %d %v", len(m), err)
	}
	if m, err := repo.PurchaseBaseCostsByIDs(ctx(), big); err != nil || len(m) != 2 {
		t.Fatalf("big PurchaseBaseCostsByIDs = %d %v", len(m), err)
	}
	if m, err := repo.CategoryNamesByIDs(ctx(), append(big, cat)); err != nil || len(m) != 1 {
		t.Fatalf("big CategoryNamesByIDs = %d %v", len(m), err)
	}
	codes := make([]string, 0, 2500)
	for i := 0; i < 2500; i++ {
		codes = append(codes, fmt.Sprintf("ZZ-%d", i))
	}
	codes = append(codes, "b-1", "B-2", "B-3")
	if m, err := repo.ProductsByCodes(ctx(), codes); err != nil || len(m) != 3 {
		t.Fatalf("big ProductsByCodes = %d %v", len(m), err)
	}
	if m, err := repo.ExistingCodes(ctx(), codes); err != nil || len(m) != 3 {
		t.Fatalf("big ExistingCodes = %d %v", len(m), err)
	}

	// Scan keys / code collisions against OTHER products.
	for i, id := range ids {
		if _, err := testMods.db.Exec(`INSERT INTO product_variants (product_id, code, name, barcode) VALUES (?, ?, 'v', ?)`,
			id, fmt.Sprintf("V-%d", i+1), []any{"BC1", nil, "BC3"}[i]); err != nil {
			t.Fatal(err)
		}
	}
	owners, err := repo.OtherVariantsUsingKeys(ctx(), []string{"V-1", "V-2", "BC3", "nokey"}, ids[0])
	if err != nil || len(owners) != 2 {
		t.Fatalf("owners = %+v %v", owners, err)
	}
	sort.Slice(owners, func(i, j int) bool { return owners[i].ProductID < owners[j].ProductID })
	if owners[0] != (productinfra.ScanKeyOwner{ProductID: ids[1], Code: "V-2", Barcode: ""}) ||
		owners[1] != (productinfra.ScanKeyOwner{ProductID: ids[2], Code: "V-3", Barcode: "BC3"}) {
		t.Fatalf("owners = %+v", owners)
	}
	all, _ := repo.OtherVariantsUsingKeys(ctx(), []string{"V-1", "BC1"}, 0)
	if len(all) != 1 || all[0].ProductID != ids[0] {
		t.Fatalf("exclude 0 = %+v", all)
	}
	if o, err := repo.OtherVariantsUsingKeys(ctx(), nil, 0); err != nil || len(o) != 0 {
		t.Fatalf("empty keys = %v %v", o, err)
	}
	pc, err := repo.ProductCodesIn(ctx(), []string{"B-1", "B-2", "XX"}, ids[0])
	if err != nil || len(pc) != 1 || !pc["B-2"] {
		t.Fatalf("ProductCodesIn = %v %v", pc, err)
	}
	if pc, _ := repo.ProductCodesIn(ctx(), nil, 0); pc == nil || len(pc) != 0 {
		t.Fatal("empty codes in")
	}
	if pc, _ := repo.ProductCodesIn(ctx(), []string{"B-1", "B-2"}, 0); len(pc) != 2 {
		t.Fatalf("exclude 0 = %v", pc)
	}
}

func TestSqlcProductVariants(t *testing.T) {
	freshDB(t)
	repo := sqlcProdRepo()
	mk := func(code string, vs ...*contracts.Variant) int64 {
		p := sqlcMkProduct(code, code, nil)
		p.Variants = vs
		id, err := repo.CreateProduct(ctx(), p, 1)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	p1 := mk("A", &contracts.Variant{Code: "A-Z", Name: "z", SortOrder: 2, Barcode: "111"},
		&contracts.Variant{Code: "A-D", Name: "d", IsDefault: true, IsHidden: true},
		&contracts.Variant{Code: "A-B", Name: "b", SortOrder: 2},
		&contracts.Variant{Code: "A-X", Name: "x", SortOrder: 1, Status: "inactive", Barcode: "222"})
	p2 := mk("B", &contracts.Variant{Code: "A-D", Name: "dup code other product", Barcode: "333"})
	p3 := mk("C") // no variants

	// Active only; default first, then sort_order, then code.
	vs, err := repo.VariantsByProduct(ctx(), p1)
	if err != nil || len(vs) != 3 || vs[0].Code != "A-D" || vs[1].Code != "A-B" || vs[2].Code != "A-Z" {
		t.Fatalf("VariantsByProduct = %+v %v", vs, err)
	}
	if vs, err := repo.VariantsByProduct(ctx(), p3); err != nil || len(vs) != 0 {
		t.Fatalf("none = %v %v", vs, err)
	}
	all, _ := repo.AllVariantsByProduct(ctx(), p1)
	if len(all) != 4 || all[0].Code != "A-Z" || all[3].Code != "A-X" || all[3].Status != "inactive" {
		t.Fatalf("AllVariantsByProduct = %+v", all)
	}

	bp, err := repo.VariantsByProducts(ctx(), []int64{p2, p1, p3, 9999})
	if err != nil || len(bp) != 2 || len(bp[p1]) != 3 || len(bp[p2]) != 1 || bp[p1][0].Code != "A-D" {
		t.Fatalf("VariantsByProducts = %v %v", bp, err)
	}
	if bp, err := repo.VariantsByProducts(ctx(), nil); err != nil || bp == nil || len(bp) != 0 {
		t.Fatalf("empty = %v %v", bp, err)
	}
	abp, err := repo.AllVariantsByProducts(ctx(), []int64{p2, p1, p3})
	if err != nil || len(abp) != 2 || len(abp[p1]) != 4 || abp[p1][0].Code != "A-Z" {
		t.Fatalf("AllVariantsByProducts = %v %v", abp, err)
	}
	if abp, err := repo.AllVariantsByProducts(ctx(), nil); err != nil || abp == nil || len(abp) != 0 {
		t.Fatalf("empty all = %v %v", abp, err)
	}
	// More ids than one chunk; includes duplicates and a zero.
	big := make([]int64, 0, 2100)
	for i := 0; i < 2100; i++ {
		big = append(big, int64(500_000+i))
	}
	big = append(big, p1, p1, 0)
	if abp, err := repo.AllVariantsByProducts(ctx(), big); err != nil || len(abp) != 1 || len(abp[p1]) != 4 {
		t.Fatalf("big AllVariantsByProducts = %d %v (dups collapse in IN)", len(abp[p1]), err)
	}
	if bp, err := repo.VariantsByProducts(ctx(), big); err != nil || len(bp[p1]) != 3 {
		t.Fatalf("big VariantsByProducts = %d %v", len(bp[p1]), err)
	}

	v, err := repo.VariantByBarcode(ctx(), "111")
	if err != nil || v == nil || v.Code != "A-Z" || v.Barcode != "111" {
		t.Fatalf("by barcode = %+v %v", v, err)
	}
	if v, err := repo.VariantByBarcode(ctx(), "222"); v != nil || err != nil {
		t.Fatalf("inactive barcode must not resolve: %v %v", v, err)
	}
	if v, err := repo.VariantByBarcode(ctx(), "nope"); v != nil || err != nil {
		t.Fatalf("missing barcode: %v %v", v, err)
	}
	if v, err := repo.VariantByBarcode(ctx(), ""); v != nil || err != nil {
		t.Fatalf("empty barcode: %v %v", v, err)
	}

	byCode, err := repo.ActiveVariantsByCode(ctx(), "A-D", 10)
	if err != nil || len(byCode) != 2 || byCode[0].ProductID != p1 || byCode[1].ProductID != p2 {
		t.Fatalf("ActiveVariantsByCode = %+v %v", byCode, err)
	}
	if one, _ := repo.ActiveVariantsByCode(ctx(), "A-D", 1); len(one) != 1 || one[0].ProductID != p1 {
		t.Fatalf("limit 1 = %+v", one)
	}
	if none, err := repo.ActiveVariantsByCode(ctx(), "A-X", 10); err != nil || len(none) != 0 {
		t.Fatalf("inactive code = %v %v", none, err)
	}

	d, err := repo.DefaultVariant(ctx(), p1)
	if err != nil || d == nil || d.Code != "A-D" || !d.IsDefault || !d.IsHidden {
		t.Fatalf("default = %+v %v", d, err)
	}
	// No default flag: first active by id.
	d2, _ := repo.DefaultVariant(ctx(), p2)
	if d2 == nil || d2.Code != "A-D" || d2.ProductID != p2 {
		t.Fatalf("fallback default = %+v", d2)
	}
	if d, err := repo.DefaultVariant(ctx(), p3); d != nil || err != nil {
		t.Fatalf("no variants default = %v %v", d, err)
	}
}

func TestSqlcProductSearchAndLists(t *testing.T) {
	freshDB(t)
	repo := sqlcProdRepo()
	cat1, _ := repo.CreateCategory(ctx(), &contracts.Category{Code: "K1", Name: "Kat Satu"}, 1)
	cat2, _ := repo.CreateCategory(ctx(), &contracts.Category{Code: "K2", Name: "Kat Dua"}, 1)
	type spec struct {
		code, name, typ, status string
		tracked                 bool
		cat                     *int64
		min                     float64
	}
	specs := []spec{
		{"SKU-001", "Beras Premium", "barang", "active", true, sqlcPtr64(cat1), 5},
		{"SKU-002", "Beras Medium", "barang", "active", true, sqlcPtr64(cat2), 0},
		{"SKU-003", "Gula Pasir", "barang", "archived", true, sqlcPtr64(cat1), 2},
		{"SVC-001", "Jasa Antar", "jasa", "active", false, nil, 0},
		{"SKU-004", "Beras Merah 100%", "barang", "active", false, sqlcPtr64(cat1), 7},
		{"SKU-005", "Beras Premium", "barang", "active", true, nil, 1}, // same name as 001: id tie-break
		{"under_score", "Under_score", "barang", "active", true, nil, 4},
	}
	idOf := map[string]int64{}
	for _, s := range specs {
		p := sqlcMkProduct(s.code, s.name, s.cat)
		p.Type, p.Status, p.Tracked, p.MinStock = s.typ, s.status, s.tracked, s.min
		id, err := repo.CreateProduct(ctx(), p, 1)
		if err != nil {
			t.Fatal(err)
		}
		idOf[s.code] = id
	}
	codesOf := func(ps []*contracts.Product) []string {
		out := []string{}
		for _, p := range ps {
			out = append(out, p.Code)
		}
		return out
	}
	tr, fa := true, false
	list := func(f productinfra.ProductFilter, limit, offset int) []string {
		t.Helper()
		ps, err := repo.ListProducts(ctx(), f, limit, offset)
		if err != nil {
			t.Fatalf("list %+v: %v", f, err)
		}
		return codesOf(ps)
	}
	eq := func(name string, got, want []string) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: got %v want %v", name, got, want)
		}
	}
	// Name order: Beras Medium, Beras Merah 100%, Beras Premium(001), Beras Premium(005), Gula Pasir, Jasa Antar, Under_score
	eq("all", list(productinfra.ProductFilter{}, 100, 0), []string{"SKU-002", "SKU-004", "SKU-001", "SKU-005", "SKU-003", "SVC-001", "under_score"})
	eq("status", list(productinfra.ProductFilter{Status: "archived"}, 100, 0), []string{"SKU-003"})
	eq("type", list(productinfra.ProductFilter{Type: "jasa"}, 100, 0), []string{"SVC-001"})
	eq("tracked", list(productinfra.ProductFilter{Tracked: &fa}, 100, 0), []string{"SKU-004", "SVC-001"})
	eq("tracked true+status", list(productinfra.ProductFilter{Tracked: &tr, Status: "active"}, 100, 0), []string{"SKU-002", "SKU-001", "SKU-005", "under_score"})
	eq("category", list(productinfra.ProductFilter{CategoryID: sqlcPtr64(cat1)}, 100, 0), []string{"SKU-004", "SKU-001", "SKU-003"})
	eq("search code", list(productinfra.ProductFilter{Search: "svc"}, 100, 0), []string{"SVC-001"})
	eq("search name", list(productinfra.ProductFilter{Search: "beras"}, 100, 0), []string{"SKU-002", "SKU-004", "SKU-001", "SKU-005"})
	eq("search words AND", list(productinfra.ProductFilter{Search: "  beras   premium "}, 100, 0), []string{"SKU-001", "SKU-005"})
	eq("search word across code+name", list(productinfra.ProductFilter{Search: "sku-004 merah"}, 100, 0), []string{"SKU-004"})
	eq("search percent literal-ish", list(productinfra.ProductFilter{Search: "100%"}, 100, 0), []string{"SKU-004"})
	eq("search underscore wildcard", list(productinfra.ProductFilter{Search: "under_score"}, 100, 0), []string{"under_score"})
	eq("search none", list(productinfra.ProductFilter{Search: "zzz"}, 100, 0), []string{})
	eq("blank search", list(productinfra.ProductFilter{Search: "   "}, 100, 0), []string{"SKU-002", "SKU-004", "SKU-001", "SKU-005", "SKU-003", "SVC-001", "under_score"})
	eq("combo", list(productinfra.ProductFilter{Search: "beras", CategoryID: sqlcPtr64(cat1), Status: "active", Type: "barang", Tracked: &tr}, 100, 0), []string{"SKU-001"})
	eq("page 1", list(productinfra.ProductFilter{}, 3, 0), []string{"SKU-002", "SKU-004", "SKU-001"})
	eq("page 2", list(productinfra.ProductFilter{}, 3, 3), []string{"SKU-005", "SKU-003", "SVC-001"})
	eq("page 3", list(productinfra.ProductFilter{}, 3, 6), []string{"under_score"})
	eq("beyond", list(productinfra.ProductFilter{}, 3, 99), []string{})
	// Joined category name; NULL category -> "".
	ps, _ := repo.ListProducts(ctx(), productinfra.ProductFilter{Search: "premium"}, 10, 0)
	if len(ps) != 2 || ps[0].CategoryName != "Kat Satu" || ps[1].CategoryName != "" || ps[1].CategoryID != nil ||
		ps[0].Attributes["warna"] != "merah" || ps[0].MinStock != 5 || ps[0].PurchaseFactor != 12.5 {
		t.Fatalf("list row = %+v / %+v", ps[0], ps[1])
	}
	// List/count parity for every filter combo.
	for _, f := range []productinfra.ProductFilter{
		{}, {Status: "archived"}, {Type: "jasa"}, {Tracked: &fa}, {Tracked: &tr, Status: "active"},
		{CategoryID: sqlcPtr64(cat1)}, {Search: "beras"}, {Search: "beras premium"}, {Search: "zzz"},
		{Search: "beras", CategoryID: sqlcPtr64(cat1), Status: "active", Type: "barang", Tracked: &tr}, {CategoryID: sqlcPtr64(9999)},
	} {
		n, err := repo.CountProducts(ctx(), f)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(list(f, 1000, 0)); int64(got) != n {
			t.Fatalf("count %+v = %d, list = %d", f, n, got)
		}
	}

	// SearchProductIDs: active only, code or name fragment, code order, limit.
	ids, err := repo.SearchProductIDs(ctx(), "Beras", 10)
	if err != nil || !reflect.DeepEqual(ids, []int64{idOf["SKU-001"], idOf["SKU-002"], idOf["SKU-004"], idOf["SKU-005"]}) {
		t.Fatalf("SearchProductIDs = %v %v", ids, err)
	}
	if ids, _ := repo.SearchProductIDs(ctx(), "SKU", 2); len(ids) != 2 || ids[0] != idOf["SKU-001"] {
		t.Fatalf("limit = %v", ids)
	}
	if ids, err := repo.SearchProductIDs(ctx(), "Gula", 10); err != nil || len(ids) != 0 {
		t.Fatalf("archived must not match: %v %v", ids, err)
	}
	if ids, _ := repo.SearchProductIDs(ctx(), "", 3); len(ids) != 3 {
		t.Fatalf("empty query matches all active: %v", ids)
	}

	// ListStocked / ListStockedAfter: tracked + active, id order paging.
	after, err := repo.ListStockedAfter(ctx(), 0, 100)
	if err != nil || len(after) != 4 {
		t.Fatalf("stocked = %+v %v", after, err)
	}
	for i := 1; i < len(after); i++ {
		if after[i].ID <= after[i-1].ID {
			t.Fatalf("not id ordered: %+v", after)
		}
	}
	if after[0].Code != "SKU-001" || after[0].MinStock != 5 {
		t.Fatalf("first = %+v", after[0])
	}
	page, _ := repo.ListStockedAfter(ctx(), after[0].ID, 2)
	if len(page) != 2 || page[0].ID != after[1].ID {
		t.Fatalf("page = %+v", page)
	}
	if page, err := repo.ListStockedAfter(ctx(), after[len(after)-1].ID, 5); err != nil || page == nil || len(page) != 0 {
		t.Fatalf("past end = %v %v (must be non-nil empty)", page, err)
	}
	st, err := repo.ListStocked(ctx())
	if err != nil || len(st) != 4 || st[0].Code != "SKU-001" || st[1].Code != "SKU-002" || st[2].Code != "SKU-005" || st[3].Code != "under_score" {
		t.Fatalf("ListStocked = %+v %v", st, err)
	}
	freshDB(t)
	if st, err := repo.ListStocked(ctx()); err != nil || st == nil || len(st) != 0 {
		t.Fatalf("empty ListStocked = %v %v", st, err)
	}
}

func TestSqlcProductQRSources(t *testing.T) {
	freshDB(t)
	repo := sqlcProdRepo()
	cat, _ := repo.CreateCategory(ctx(), &contracts.Category{Code: "K", Name: "Kategori"}, 1)
	a := sqlcMkProduct("QA", "Qa", sqlcPtr64(cat))
	a.Variants = []*contracts.Variant{{Code: "QA", Name: "d", IsDefault: true, IsHidden: true},
		{Code: "QA-1", Name: "one", Barcode: "9001"}, {Code: "QA-2", Name: "gone", Status: "inactive"}}
	b := sqlcMkProduct("QB", "Qb", nil)
	jasa := sqlcMkProduct("QJ", "Jasa", nil)
	jasa.Type = "jasa"
	gone := sqlcMkProduct("QG", "Gone", nil)
	gone.Status = "archived"
	for _, p := range []*contracts.Product{a, b, jasa, gone} {
		if _, err := repo.CreateProduct(ctx(), p, 1); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := repo.QRSources(ctx())
	if err != nil || len(rows) != 3 {
		t.Fatalf("rows = %+v %v", rows, err)
	}
	if rows[0].ProductCode != "QA" || rows[0].Category != "Kategori" || rows[0].VariantCode != "QA" || !rows[0].VariantHidden || rows[0].Barcode != "" || rows[0].VariantID == 0 {
		t.Fatalf("row0 = %+v", rows[0])
	}
	if rows[1].VariantCode != "QA-1" || rows[1].Barcode != "9001" || rows[1].VariantHidden || rows[1].ProductName != "Qa" {
		t.Fatalf("row1 = %+v", rows[1])
	}
	// Product without variants: LEFT JOIN yields zero values.
	if r := rows[2]; r.ProductCode != "QB" || r.Category != "" || r.VariantID != 0 || r.VariantCode != "" || r.VariantName != "" || r.Barcode != "" || r.VariantHidden {
		t.Fatalf("row2 = %+v", r)
	}
	freshDB(t)
	if rows, err := repo.QRSources(ctx()); err != nil || rows == nil || len(rows) != 0 {
		t.Fatalf("empty QR = %v %v", rows, err)
	}
}
