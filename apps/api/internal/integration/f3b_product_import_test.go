package integration

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	productapp "mini-erp/internal/modules/product/application"
)

type f3bSheets map[string][][]any

func f3bWorkbook(t *testing.T, order []string, sheets f3bSheets) []byte {
	t.Helper()
	f := excelize.NewFile()
	for i, name := range order {
		if i == 0 {
			_ = f.SetSheetName("Sheet1", name)
		} else if _, err := f.NewSheet(name); err != nil {
			t.Fatal(err)
		}
		for r, row := range sheets[name] {
			for c, v := range row {
				cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
				_ = f.SetCellValue(name, cell, v)
			}
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

var f3bProductHeader = []any{"Kode Produk", "Nama Produk", "Kategori", "Tipe Produk", "Pantau Stok?", "Satuan Stok",
	"Satuan Beli", "Satuan Jual", "Isi per Satuan Beli", "Isi Jual per Satuan Stok", "Harga Beli", "Harga Jual",
	"Minimum Stok", "Status"}

func f3bImportFile(t *testing.T, sellPrice any, extraProductRows ...[]any) []byte {
	t.Helper()
	products := [][]any{f3bProductHeader,
		{"imp-1", "Teh Botol", "MNM-DNG", "Barang", "Ya", "pcs", "dus", "pak", "12", 4, "10,000", sellPrice, 5, "Aktif"},
		{"IMP-2", "Servis AC", "Minuman", "Jasa", "Tidak", "jam", "jam", "jam", 1, 1, 0, "Rp 75.000", 0, ""},
	}
	products = append(products, extraProductRows...)
	return f3bWorkbook(t, []string{"Panduan", "Kategori Produk", "Daftar Produk", "Varian Produk", "Info Tambahan Produk"}, f3bSheets{
		"Panduan": {{"abaikan saya"}},
		// The child comes BEFORE its parent and names it by NAME; the parent is
		// the earlier row of a different order in the file.
		"Kategori Produk": {
			{"Kode Kategori", "Nama Kategori", "Subkategori Dari", "Urutan Tampil"},
			{"MNM-DNG", "Minuman Dingin", "Minuman", 2},
			{"MNM", "Minuman", "", 1},
		},
		"Daftar Produk": products,
		"Varian Produk": {
			{"Kode Produk", "Kode Varian", "Nama Varian", "Status", "Urutan Tampil"},
			{"IMP-1", "IMP-1-A", "Kecil", "Aktif", 1},
			{"IMP-1", "IMP-1-B", "Besar", "Aktif", 2},
		},
		"Info Tambahan Produk": {
			{"Kode Produk", "Nama Info", "Isi"},
			{"IMP-1", "Merek", "Contoh"},
		},
	})
}

func f3bCount(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	if err := testMods.db.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

// F3b-4: legacy-shaped workbook (Indonesian sheet and header names, text and
// numeric money, inverse sales factor, hierarchy defined child-first, variants
// and extra info) previews, commits, and re-commits idempotently.
func TestF3bImportLegacyWorkbook(t *testing.T) {
	freshDB(t)
	svc := testMods.product.Service()
	raw := f3bImportFile(t, 15000.5)

	prev, err := svc.PreviewImport(ctx(), bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if !prev.OKToCommit || len(prev.Errors) != 0 || prev.Summary.CategoriesToCreate != 2 ||
		prev.Summary.ProductsToCreate != 2 || prev.Summary.VariantsTotal != 2 || prev.Summary.AttributesTotal != 1 {
		t.Fatalf("preview: ok=%v summary=%+v issues=%+v", prev.OKToCommit, prev.Summary, prev.Issues)
	}
	if got := f3bCount(t, `SELECT COUNT(*) FROM products`); got != 0 {
		t.Fatalf("preview wrote %d products", got)
	}

	res, err := svc.CommitImport(ctx(), 0, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if res.Imported.ProductsCreated != 2 || res.Imported.CategoriesCreated != 2 || len(res.Failures) != 0 {
		t.Fatalf("commit result: %+v", res)
	}
	p, _ := svc.GetByCode(ctx(), "IMP-1")
	if p == nil {
		t.Fatal("IMP-1 missing (code must be upper-cased)")
	}
	if p.PurchasePrice != 10000 || p.SellingPrice != 15001 || p.SalesFactor != 0.25 || p.PurchaseFactor != 12 ||
		p.SalesUOM != "pak" || p.CategoryName != "Minuman Dingin" || p.Attributes["Merek"] != "Contoh" || p.MinStock != 5 {
		t.Fatalf("IMP-1 mapped wrongly: %+v", p)
	}
	// Two visible variants; the implicit default was never created for a variant product.
	visible := 0
	for _, v := range p.Variants {
		if !v.IsHidden {
			visible++
		}
	}
	if visible != 2 || len(p.Variants) != 2 {
		t.Fatalf("variants: %+v", p.Variants)
	}
	svc2, _ := svc.GetByCode(ctx(), "IMP-2")
	if svc2 == nil || svc2.Type != "jasa" || svc2.Tracked || svc2.SellingPrice != 75000 || svc2.CategoryName != "Minuman" {
		t.Fatalf("IMP-2: %+v", svc2)
	}
	var parent int
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM product_categories c JOIN product_categories p ON p.id = c.parent_id
	   WHERE c.code = 'MNM-DNG' AND p.code = 'MNM'`).Scan(&parent); err != nil || parent != 1 {
		t.Fatalf("hierarchy not built (child listed before parent): %d %v", parent, err)
	}

	// Same file again: upsert, nothing duplicated, no failures.
	res, err = svc.CommitImport(ctx(), 0, bytes.NewReader(raw))
	if err != nil || res.Imported.ProductsCreated != 0 || res.Imported.ProductsUpdated != 2 || len(res.Failures) != 0 {
		t.Fatalf("re-commit: %+v %v", res, err)
	}
	if n := f3bCount(t, `SELECT COUNT(*) FROM products`); n != 2 {
		t.Fatalf("products after re-commit = %d", n)
	}
	if n := f3bCount(t, `SELECT COUNT(*) FROM product_variants WHERE product_id = ?`, p.ID); n != 2 {
		t.Fatalf("variant rows after re-commit = %d", n)
	}

	// Price update through the file.
	if _, err := svc.CommitImport(ctx(), 0, bytes.NewReader(f3bImportFile(t, "17.000"))); err != nil {
		t.Fatal(err)
	}
	p, _ = svc.GetByCode(ctx(), "IMP-1")
	if p.SellingPrice != 17000 {
		t.Fatalf("update price = %d (text 17.000 means seventeen thousand)", p.SellingPrice)
	}
}

// The gate is all-or-nothing: one bad row blocks every row.
func TestF3bImportGate(t *testing.T) {
	freshDB(t)
	svc := testMods.product.Service()
	bad := f3bImportFile(t, 15000, []any{"IMP-3", "Rusak", "", "Barang", "Ya", "pcs", "pcs", "pcs", 1, 1, -5, 100, 0, ""})

	prev, err := svc.PreviewImport(ctx(), bytes.NewReader(bad))
	if err != nil {
		t.Fatal(err)
	}
	if prev.OKToCommit || prev.Summary.Errors == 0 || len(prev.Issues) == 0 || len(prev.Errors) == 0 {
		t.Fatalf("preview must flag the file: %+v", prev.Summary)
	}
	is := prev.Issues[0]
	if is.Severity != "error" || is.Sheet != "Daftar Produk" || is.Row != 4 || !strings.Contains(is.Column, "Harga Beli") {
		t.Fatalf("issue must name sheet/row/column: %+v", is)
	}
	if _, err := svc.CommitImport(ctx(), 0, bytes.NewReader(bad)); err == nil {
		t.Fatal("commit must refuse a file with errors")
	}
	if n := f3bCount(t, `SELECT COUNT(*) FROM products`); n != 0 {
		t.Fatalf("gate leaked %d products", n)
	}
	if n := f3bCount(t, `SELECT COUNT(*) FROM product_categories`); n != 0 {
		t.Fatalf("gate leaked %d categories", n)
	}
	// Unknown legacy .xls (OLE2) is reported clearly.
	ole := append([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, make([]byte, 32)...)
	_, err = svc.PreviewImport(ctx(), bytes.NewReader(ole))
	if ae := mustAppErr(t, err); len(ae.Fields) == 0 || !strings.Contains(ae.Fields[0].Message, ".xlsx") {
		t.Fatalf(".xls message: %+v", ae)
	}
}

// An import cannot bypass the stock-history guards the form enforces.
func TestF3bImportRespectsStockGuards(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.product.Service()
	if _, err := svc.CommitImport(ctx(), 0, bytes.NewReader(f3bImportFile(t, 15000))); err != nil {
		t.Fatal(err)
	}
	p, _ := svc.GetByCode(ctx(), "IMP-1")
	f3bStock(t, fx, p.ID, p.Variants[0].ID, 7)

	// Same product, different purchase unit factor: locked by history.
	changed := f3bWorkbook(t, []string{"Daftar Produk"}, f3bSheets{"Daftar Produk": {f3bProductHeader,
		{"IMP-1", "Teh Botol", "", "Barang", "Ya", "pcs", "dus", "pak", 6, 4, 10000, 15000, 5, "Aktif"},
		{"IMP-9", "Baru", "", "Barang", "Ya", "pcs", "pcs", "pcs", 1, 1, 100, 200, 0, ""},
	}})
	prev, err := svc.PreviewImport(ctx(), bytes.NewReader(changed))
	if err != nil {
		t.Fatal(err)
	}
	if prev.OKToCommit {
		t.Fatalf("locked factor must block the file: %+v", prev.Issues)
	}
	found := false
	for _, is := range prev.Issues {
		if is.Row == 2 && strings.Contains(is.Message, "terkunci") {
			found = true
		}
	}
	if !found {
		t.Fatalf("issue must explain the lock on row 2: %+v", prev.Issues)
	}
	if _, err := svc.CommitImport(ctx(), 0, bytes.NewReader(changed)); err == nil {
		t.Fatal("commit must be refused")
	}
	if got, _ := svc.GetByCode(ctx(), "IMP-9"); got != nil {
		t.Fatal("gate must keep the new row out as well")
	}
	// A price-only change on the same product is fine (units untouched).
	ok := f3bWorkbook(t, []string{"Daftar Produk"}, f3bSheets{"Daftar Produk": {f3bProductHeader,
		{"IMP-1", "Teh Botol", "", "Barang", "Ya", "pcs", "dus", "pak", 12, 4, 10000, 15500, 5, "Aktif"}}})
	if _, err := svc.CommitImport(ctx(), 0, bytes.NewReader(ok)); err != nil {
		t.Fatalf("price-only update: %v", err)
	}
	p, _ = svc.GetByCode(ctx(), "IMP-1")
	if p.SellingPrice != 15500 || len(p.Variants) != 2 {
		t.Fatalf("variants must survive a file without a variant sheet: %+v", p)
	}
}

// Scan-key collisions across the file and the catalog are caught in the gate.
func TestF3bImportCollisions(t *testing.T) {
	freshDB(t)
	svc := testMods.product.Service()
	f3bProduct(t, productapp.ProductInput{Code: "EXIST", Name: "Ada", Variants: []productapp.VariantInput{
		{Code: "EX-1", Name: "Ex", Barcode: "BC-EXIST", IsDefault: true}}})
	file := f3bWorkbook(t, []string{"Daftar Produk", "Varian Produk"}, f3bSheets{
		"Daftar Produk": {f3bProductHeader,
			{"BC-EXIST", "Salah", "", "Barang", "Ya", "pcs", "pcs", "pcs", 1, 1, 100, 200, 0, ""},
			{"NEW-1", "Baru", "", "Barang", "Ya", "pcs", "pcs", "pcs", 1, 1, 100, 200, 0, ""}},
		"Varian Produk": {{"Kode Produk", "Kode Varian", "Nama Varian", "Barcode"},
			{"NEW-1", "NEW-1-A", "A", "BC-EXIST"}},
	})
	prev, err := svc.PreviewImport(ctx(), bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	if prev.OKToCommit || prev.Summary.Errors < 2 {
		t.Fatalf("both collisions must be reported: %+v / %+v", prev.Summary, prev.Issues)
	}
}
