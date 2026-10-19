package application

import (
	"context"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/modules/product/infrastructure"
	"mini-erp/internal/shared/apperror"
)

// maxImportBytes caps a workbook read into memory.
const maxImportBytes = 10 << 20

// ImportIssue is one finding of the validation pass.
type ImportIssue struct {
	Severity string `json:"severity"` // error | warning
	Sheet    string `json:"sheet"`
	Row      int    `json:"row"`
	Column   string `json:"column"`
	Message  string `json:"message"`
}

// RowError is the pre-parity error shape, still returned (preview `errors`,
// commit `failures`) so existing clients keep working.
type RowError struct {
	Sheet  string `json:"sheet"`
	Row    int    `json:"row"`
	Column string `json:"column"`
	Value  string `json:"value"`
	Reason string `json:"reason"`
}

// ImportSummary counts what a file would do.
type ImportSummary struct {
	CategoriesTotal    int `json:"categoriesTotal"`
	CategoriesToCreate int `json:"categoriesToCreate"`
	CategoriesToUpdate int `json:"categoriesToUpdate"`
	ProductsTotal      int `json:"productsTotal"`
	ProductsToCreate   int `json:"productsToCreate"`
	ProductsToUpdate   int `json:"productsToUpdate"`
	VariantsTotal      int `json:"variantsTotal"`
	AttributesTotal    int `json:"attributesTotal"`
	Errors             int `json:"errors"`
	Warnings           int `json:"warnings"`
}

// ImportRow is one planned product line of the preview.
type ImportRow struct {
	Row    int    `json:"row"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	Action string `json:"action"` // create | update
}

// ImportPreview is the dry-run result: nothing is written. OKToCommit is the
// all-or-nothing gate: any error blocks the whole file.
type ImportPreview struct {
	OKToCommit bool          `json:"okToCommit"`
	Summary    ImportSummary `json:"summary"`
	Issues     []ImportIssue `json:"issues"`
	Categories []ImportRow   `json:"categories"`
	Products   []ImportRow   `json:"products"`
	// Legacy fields of the first revamp import (kept for existing clients).
	ValidProducts int        `json:"validProducts"`
	ValidVariants int        `json:"validVariants"`
	Errors        []RowError `json:"errors"`
}

// ImportResult is the commit outcome.
type ImportResult struct {
	Imported struct {
		CategoriesCreated int `json:"categoriesCreated"`
		CategoriesUpdated int `json:"categoriesUpdated"`
		ProductsCreated   int `json:"productsCreated"`
		ProductsUpdated   int `json:"productsUpdated"`
	} `json:"imported"`
	// Inserted/Skipped/Failures are the pre-parity fields.
	Inserted int        `json:"inserted"`
	Skipped  int        `json:"skipped"`
	Failures []RowError `json:"failures"`
	Notes    []string   `json:"notes"`
}

type importCategory struct {
	Row        int
	Code, Name string
	ParentRef  string
	Parent     string // resolved parent code ("" = root)
	Sort       int
	Status     string
	ExistingID int64
	Action     string
}

type importProduct struct {
	Row    int
	Code   string
	Action string
	// CatSet is true when the sheet has a category column: CatCode ("" = none)
	// is then resolved to an id after the categories are written.
	CatSet    bool
	CatCode   string
	Write     *contracts.Product
	VariantN  int
	NameLabel string
}

type importPlan struct {
	Summary    ImportSummary
	Issues     []ImportIssue
	Categories []*importCategory
	Products   []*importProduct
	Restored   []string
}

func (p *importPlan) fail(sheet string, row int, col, msg string) {
	p.Issues = append(p.Issues, ImportIssue{Severity: "error", Sheet: sheetTitle[sheet], Row: row, Column: col, Message: msg})
	p.Summary.Errors++
}

func (p *importPlan) warn(sheet string, row int, col, msg string) {
	p.Issues = append(p.Issues, ImportIssue{Severity: "warning", Sheet: sheetTitle[sheet], Row: row, Column: col, Message: msg})
	p.Summary.Warnings++
}

// columnLabel maps a canonical product field to the header users see.
var productColumnLabel = map[string]string{
	"code": "Kode Produk", "name": "Nama Produk", "category": "Kategori", "type": "Tipe Produk",
	"tracked": "Pantau Stok?", "base_uom": "Satuan Stok", "purchase_uom": "Satuan Beli", "sales_uom": "Satuan Jual",
	"purchase_factor": "Isi per Satuan Beli", "sales_factor": "Isi per Satuan Jual",
	"sales_inverse": "Isi Jual per Satuan Stok", "purchase_price": "Harga Beli", "selling_price": "Harga Jual",
	"min_selling": "Harga Jual Minimum", "min_stock": "Minimum Stok", "status": "Status", "summary": "Catatan Produk",
}

// fieldToColumn translates validateProduct's API field names to columns.
var fieldToColumn = map[string]string{
	"code": "code", "name": "name", "categoryId": "category", "type": "type", "status": "status",
	"baseUom": "base_uom", "purchaseUom": "purchase_uom", "salesUom": "sales_uom",
	"purchaseFactor": "purchase_factor", "salesFactor": "sales_factor", "purchasePrice": "purchase_price",
	"sellingPrice": "selling_price", "minSellingPrice": "min_selling", "minStock": "min_stock",
}

func roundRupiah(f float64) int64 { return int64(math.Round(f)) }

// importBuilt is one product row after parsing, before DB-backed checks.
type importBuilt struct {
	ip       *importProduct
	in       ProductInput
	existing *contracts.Product
	varRow   int // first variant row (error attribution), 0 = none
	bad      bool
}

func errMsg(err error) string {
	if ae, ok := err.(*apperror.AppError); ok {
		if len(ae.Fields) > 0 {
			return ae.Fields[0].Message
		}
		if ae.Message != "" {
			return ae.Message
		}
	}
	return err.Error()
}

// buildImportPlan reads, validates and plans a workbook against the current
// catalog. It reads in batches (codes, keys, variants, stock facts) — never
// per row — and writes nothing.
func (s *Service) buildImportPlan(ctx context.Context, raw []byte) (*importPlan, error) {
	sheets, err := readWorkbook(raw)
	if err != nil {
		return nil, err
	}
	parsed := classifySheets(sheets)
	plan := &importPlan{}
	prodSheet := parsed[sheetProducts]
	if prodSheet == nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file",
			Message: "sheet 'Daftar Produk' tidak ditemukan"}})
	}
	for kind, ps := range parsed {
		if len(ps.Missing) > 0 {
			var cols []string
			for _, f := range ps.Missing {
				cols = append(cols, f)
			}
			plan.fail(kind, ps.Header, strings.Join(cols, ", "), "kolom wajib tidak ditemukan")
		}
	}
	if plan.Summary.Errors > 0 {
		return plan, nil
	}

	finalCats, err := s.planCategories(ctx, plan, parsed[sheetCategories])
	if err != nil {
		return nil, err
	}
	if err := s.planProducts(ctx, plan, parsed, finalCats); err != nil {
		return nil, err
	}
	sort.SliceStable(plan.Issues, func(i, j int) bool { return false })
	return plan, nil
}

type finalCategory struct {
	Code, Name string
	ParentCode string
	Status     string
	ID         int64
	InFile     bool
}

// planCategories resolves the category sheet against existing categories and
// returns the category set as it will be AFTER the import (by upper code).
func (s *Service) planCategories(ctx context.Context, plan *importPlan, ps *parsedSheet) (map[string]*finalCategory, error) {
	existing, err := s.repo.ListCategories(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	idCode := map[int64]string{}
	final := map[string]*finalCategory{}
	for _, c := range existing {
		idCode[c.ID] = strings.ToUpper(c.Code)
	}
	for _, c := range existing {
		fc := &finalCategory{Code: c.Code, Name: c.Name, Status: c.Status, ID: c.ID}
		if c.ParentID != nil {
			fc.ParentCode = idCode[*c.ParentID]
		}
		final[strings.ToUpper(c.Code)] = fc
	}
	if ps == nil {
		return final, nil
	}
	byName := func(name string) (string, int) {
		n, code := 0, ""
		for k, c := range final {
			if c.Status == "active" && strings.EqualFold(c.Name, name) {
				n++
				code = k
			}
		}
		return code, n
	}
	seen := map[string]bool{}
	for _, row := range ps.Rows {
		ic := &importCategory{Row: row.Line, Code: strings.ToUpper(row.Get("code")), Name: row.Get("name"),
			ParentRef: row.Get("parent")}
		if ic.Code == "" {
			plan.fail(sheetCategories, row.Line, "Kode Kategori", "wajib diisi")
			continue
		}
		if len([]rune(ic.Code)) > 50 {
			plan.fail(sheetCategories, row.Line, "Kode Kategori", "maksimal 50 karakter")
			continue
		}
		if ic.Name == "" {
			plan.fail(sheetCategories, row.Line, "Nama Kategori", "wajib diisi")
			continue
		}
		if len([]rune(ic.Name)) > 255 {
			plan.fail(sheetCategories, row.Line, "Nama Kategori", "maksimal 255 karakter")
			continue
		}
		if seen[ic.Code] {
			plan.fail(sheetCategories, row.Line, "Kode Kategori", "kode '"+ic.Code+"' ganda di file")
			continue
		}
		seen[ic.Code] = true
		if v := row.Get("sort"); v != "" {
			n, ok := parseFlexibleNumber(v)
			if !ok {
				plan.fail(sheetCategories, row.Line, "Urutan Tampil", "harus angka")
				continue
			}
			ic.Sort = int(n)
		}
		ic.Status = "active"
		if v := row.Get("status"); v != "" {
			st, ok := parseProductStatus(v)
			if !ok || st == "inactive" {
				plan.fail(sheetCategories, row.Line, "Status", "harus Aktif atau Arsip")
				continue
			}
			ic.Status = st
		}
		if cur := final[ic.Code]; cur != nil {
			ic.ExistingID = cur.ID
			ic.Action = "update"
			if cur.Status == "archived" && ic.Status == "active" {
				plan.Restored = append(plan.Restored, ic.Code)
				plan.warn(sheetCategories, row.Line, "Kode Kategori", "kategori '"+ic.Code+"' ada di arsip dan dipulihkan otomatis")
			}
		} else {
			ic.Action = "create"
		}
		final[ic.Code] = &finalCategory{Code: ic.Code, Name: ic.Name, Status: ic.Status, ID: ic.ExistingID, InFile: true}
		plan.Categories = append(plan.Categories, ic)
	}
	// Parent references resolve against the post-import set (so a parent may
	// be defined later in the sheet).
	for _, ic := range plan.Categories {
		ref := strings.TrimSpace(ic.ParentRef)
		fc := final[ic.Code]
		if ref == "" {
			fc.ParentCode = ""
			continue
		}
		code := strings.ToUpper(ref)
		if _, ok := final[code]; !ok {
			c, n := byName(ref)
			switch {
			case n == 0:
				plan.fail(sheetCategories, ic.Row, "Subkategori Dari", "induk '"+ref+"' tidak ditemukan")
				continue
			case n > 1:
				plan.fail(sheetCategories, ic.Row, "Subkategori Dari", "nama induk '"+ref+"' ambigu: pakai kode kategori")
				continue
			}
			code = c
		}
		if code == ic.Code {
			plan.fail(sheetCategories, ic.Row, "Subkategori Dari", "kategori tidak bisa menjadi induk dirinya sendiri")
			continue
		}
		if final[code].Status == "archived" {
			plan.fail(sheetCategories, ic.Row, "Subkategori Dari", "induk '"+ref+"' sudah diarsipkan")
			continue
		}
		fc.ParentCode = code
		ic.Parent = code
	}
	// Cycle check over the final parent map.
	for _, ic := range plan.Categories {
		cur, steps := final[ic.Code].ParentCode, 0
		for cur != "" && steps < 100 {
			if cur == ic.Code {
				plan.fail(sheetCategories, ic.Row, "Subkategori Dari", "induk membentuk siklus")
				break
			}
			next := final[cur]
			if next == nil {
				break
			}
			cur = next.ParentCode
			steps++
		}
	}
	// Order parents before children (depth in the final map).
	depth := func(code string) int {
		d, cur := 0, final[code].ParentCode
		for cur != "" && d < 100 {
			d++
			next := final[cur]
			if next == nil {
				break
			}
			cur = next.ParentCode
		}
		return d
	}
	sort.SliceStable(plan.Categories, func(i, j int) bool {
		return depth(plan.Categories[i].Code) < depth(plan.Categories[j].Code)
	})
	for _, ic := range plan.Categories {
		plan.Summary.CategoriesTotal++
		if ic.Action == "create" {
			plan.Summary.CategoriesToCreate++
		} else {
			plan.Summary.CategoriesToUpdate++
		}
	}
	return final, nil
}

func (s *Service) planProducts(ctx context.Context, plan *importPlan, parsed map[string]*parsedSheet, cats map[string]*finalCategory) error {
	ps := parsed[sheetProducts]
	vs, as := parsed[sheetVariants], parsed[sheetAttributes]

	// One batch read of everything the file's codes touch.
	codes := make([]string, 0, len(ps.Rows))
	for _, r := range ps.Rows {
		codes = append(codes, r.Get("code"))
	}
	existing, err := s.repo.ProductsByCodes(ctx, codes)
	if err != nil {
		return apperror.Internal(err)
	}
	var existingIDs []int64
	for _, p := range existing {
		existingIDs = append(existingIDs, p.ID)
	}
	allVariants, err := s.repo.AllVariantsByProducts(ctx, existingIDs)
	if err != nil {
		return apperror.Internal(err)
	}

	// Group variant and attribute rows by product code.
	variantRows := map[string][]sheetRow{}
	if vs != nil {
		for _, r := range vs.Rows {
			variantRows[strings.ToUpper(r.Get("product_code"))] = append(variantRows[strings.ToUpper(r.Get("product_code"))], r)
		}
	}
	attrRows := map[string][]sheetRow{}
	if as != nil {
		for _, r := range as.Rows {
			attrRows[strings.ToUpper(r.Get("product_code"))] = append(attrRows[strings.ToUpper(r.Get("product_code"))], r)
		}
	}
	fileCodes := map[string]int{}
	for _, r := range ps.Rows {
		c := strings.ToUpper(r.Get("code"))
		if c != "" {
			if _, dup := fileCodes[c]; !dup {
				fileCodes[c] = r.Line
			}
		}
	}
	for code, rows := range variantRows {
		if _, ok := fileCodes[code]; !ok {
			plan.fail(sheetVariants, rows[0].Line, "Kode Produk", "produk '"+code+"' tidak ada di sheet Daftar Produk")
		}
	}
	for code, rows := range attrRows {
		if _, ok := fileCodes[code]; !ok {
			plan.fail(sheetAttributes, rows[0].Line, "Kode Produk", "produk '"+code+"' tidak ada di sheet Daftar Produk")
		}
	}

	var builds []*importBuilt
	seen := map[string]bool{}
	for _, row := range ps.Rows {
		code := strings.ToUpper(row.Get("code"))
		if code == "" {
			plan.fail(sheetProducts, row.Line, "Kode Produk", "wajib diisi")
			continue
		}
		if seen[code] {
			plan.fail(sheetProducts, row.Line, "Kode Produk", "kode '"+code+"' ganda di file (baris "+fmt.Sprint(fileCodes[code])+")")
			continue
		}
		seen[code] = true
		// Legacy inline "Info tambahan:"/"Atribut:" cells are never read here
		// (attributes live in their own sheet): say so per cell instead of
		// dropping the values silently (PR-08, legacy E-41 / KI-60).
		for _, header := range sortedKeys(row.Inline) {
			plan.fail(sheetProducts, row.Line, header,
				`Info tambahan produk diisi melalui sheet "Info Tambahan Produk". Pindahkan nilai ini ke sheet tersebut.`)
		}
		ex := existing[code]
		before := plan.Summary.Errors
		in, catCode := s.productInputFromRow(plan, ps, row, ex, cats)
		if plan.Summary.Errors > before {
			continue
		}
		b := &importBuilt{ip: &importProduct{Row: row.Line, Code: code, NameLabel: in.Name,
			CatSet: ps.Has["category"], CatCode: catCode}, in: in, existing: ex}
		if ex != nil {
			b.ip.Action = "update"
		} else {
			b.ip.Action = "create"
		}
		// Variants / attributes.
		if rows := variantRows[code]; len(rows) > 0 {
			b.varRow = rows[0].Line
			if in.Type != "barang" || !in.Tracked {
				plan.fail(sheetVariants, rows[0].Line, "Kode Produk", "varian hanya untuk produk bertipe Barang dengan Pantau Stok")
				continue
			}
			seenVar := map[string]bool{}
			ok := true
			for _, vr := range rows {
				vi := VariantInput{Code: strings.ToUpper(vr.Get("code")), Name: vr.Get("name"), Barcode: vr.Get("barcode")}
				if vi.Name == "" {
					vi.Name = vi.Code
				}
				if v := vr.Get("sort"); v != "" {
					n, good := parseFlexibleNumber(v)
					if !good {
						plan.fail(sheetVariants, vr.Line, "Urutan Tampil", "harus angka")
						ok = false
					}
					vi.SortOrder = int(n)
				}
				if v := vr.Get("status"); v != "" {
					st, good := parseProductStatus(v)
					if !good {
						plan.fail(sheetVariants, vr.Line, "Status", "harus Aktif, Nonaktif, atau Arsip")
						ok = false
					}
					vi.Status = st
				}
				if vi.Code == "" {
					plan.fail(sheetVariants, vr.Line, "Kode Varian", "wajib diisi")
					ok = false
				} else if seenVar[vi.Code] {
					plan.fail(sheetVariants, vr.Line, "Kode Varian", "kode varian '"+vi.Code+"' ganda untuk produk ini")
					ok = false
				}
				seenVar[vi.Code] = true
				in.Variants = append(in.Variants, vi)
			}
			if !ok {
				continue
			}
			b.ip.VariantN = len(rows)
			plan.Summary.VariantsTotal += len(rows)
		}
		if rows := attrRows[code]; len(rows) > 0 {
			attrs := map[string]string{}
			ok := true
			for _, ar := range rows {
				k, v := ar.Get("key"), ar.Get("value")
				if k == "" || v == "" {
					plan.fail(sheetAttributes, ar.Line, "Nama Info / Isi", "nama dan isi wajib diisi")
					ok = false
					continue
				}
				if _, dup := attrs[k]; dup {
					plan.fail(sheetAttributes, ar.Line, "Nama Info", "nama '"+k+"' ganda untuk produk ini")
					ok = false
					continue
				}
				attrs[k] = v
			}
			if !ok {
				continue
			}
			in.Attributes = attrs
			plan.Summary.AttributesTotal += len(rows)
		} else if ex != nil && as == nil {
			in.Attributes = ex.Attributes // no Info Tambahan sheet: keep what is stored
		}
		b.in = in
		builds = append(builds, b)
	}

	// Field-level validation (pure) + file-level scan-key collisions.
	fileProductCode := map[string]bool{}
	for _, b := range builds {
		fileProductCode[b.ip.Code] = true
	}
	ownerOfKey := map[string]string{} // upper key -> product code owning it as variant code/barcode in the file
	var good []*importBuilt
	for _, b := range builds {
		fields := s.validateProductFields(b.in)
		bad := false
		for _, f := range fields {
			sheet, row, col := sheetProducts, b.ip.Row, productColumnLabel[fieldToColumn[f.Field]]
			if f.Field == "variants" || f.Field == "attributes" {
				if b.varRow > 0 && f.Field == "variants" {
					sheet, row, col = sheetVariants, b.varRow, "Varian"
				} else {
					col = "Info Tambahan"
				}
			}
			plan.fail(sheet, row, col, f.Message)
			bad = true
		}
		if bad {
			continue
		}
		for _, v := range b.in.Variants {
			for _, k := range []string{strings.ToUpper(strings.TrimSpace(v.Code)), strings.ToUpper(strings.TrimSpace(v.Barcode))} {
				if k == "" {
					continue
				}
				if k != b.ip.Code && fileProductCode[k] {
					plan.fail(sheetVariants, b.varRow, "Kode Varian/Barcode", "'"+k+"' sudah menjadi kode produk lain di file")
					bad = true
				}
			}
			if bc := strings.ToUpper(strings.TrimSpace(v.Barcode)); bc != "" {
				if o, ok := ownerOfKey["B:"+bc]; ok && o != b.ip.Code {
					plan.fail(sheetVariants, b.varRow, "Barcode", "barcode '"+bc+"' dipakai produk '"+o+"' juga")
					bad = true
				}
				ownerOfKey["B:"+bc] = b.ip.Code
			}
		}
		if !bad {
			good = append(good, b)
		}
	}
	builds = good

	// Database-level collisions, in batches.
	if err := s.checkImportCollisions(ctx, plan, builds); err != nil {
		return err
	}

	// Guards + variant plan per product.
	for _, b := range builds {
		if b.bad {
			continue
		}
		var all []*contracts.Variant
		var states map[int64]contracts.VariantStockState
		if b.existing != nil {
			if b.existing.Status == "archived" {
				plan.fail(sheetProducts, b.ip.Row, "Kode Produk", "produk '"+b.ip.Code+"' sudah diarsipkan dan tidak bisa diubah lewat impor")
				continue
			}
			if lockedFieldsChange(b.existing, b.in) {
				state, err := s.stockState(ctx, b.existing.ID)
				if err != nil {
					return apperror.Internal(err)
				}
				if ae := checkUpdateRules(b.existing, b.in, state); ae != nil {
					plan.fail(sheetProducts, b.ip.Row, "Satuan / Tipe / Pantau Stok", ae.Message)
					continue
				}
			}
			all = allVariants[b.existing.ID]
			if b.in.Variants != nil {
				states, err = s.variantStates(ctx, b.existing.ID)
				if err != nil {
					return apperror.Internal(err)
				}
			}
		}
		oldCode := ""
		if b.existing != nil {
			oldCode = b.existing.Code
		}
		variants, err := planVariants(variantPlanInput{
			ProductCode: b.ip.Code, OldProductCode: oldCode, ProductName: b.in.Name,
			Existing: all, Payload: b.in.Variants, States: states,
		})
		if err != nil {
			row, sheet := b.ip.Row, sheetProducts
			if b.varRow > 0 {
				row, sheet = b.varRow, sheetVariants
			}
			plan.fail(sheet, row, "Varian", errMsg(err))
			continue
		}
		p := s.toProduct(b.in)
		if b.existing != nil {
			p.ID = b.existing.ID
			if p.Status == "" {
				p.Status = b.existing.Status
			}
		}
		p.Variants = variants
		b.ip.Write = p
		plan.Products = append(plan.Products, b.ip)
		plan.Summary.ProductsTotal++
		if b.ip.Action == "create" {
			plan.Summary.ProductsToCreate++
		} else {
			plan.Summary.ProductsToUpdate++
		}
	}
	// Products that failed validation still count toward the total so the
	// preview shows how many rows the file holds.
	plan.Summary.ProductsTotal = len(ps.Rows)
	return nil
}

// productInputFromRow reads one product row. Absent COLUMNS keep the stored
// value on update; blank CELLS of present columns overwrite (legacy: the file
// is the source of truth).
func (s *Service) productInputFromRow(plan *importPlan, ps *parsedSheet, row sheetRow, ex *contracts.Product, cats map[string]*finalCategory) (ProductInput, string) {
	catCode := ""
	in := ProductInput{Code: strings.ToUpper(row.Get("code")), Name: row.Get("name")}
	if ex != nil {
		in = ProductInput{Code: in.Code, Name: in.Name, CategoryID: ex.CategoryID, Type: ex.Type, Tracked: ex.Tracked,
			BaseUOM: ex.BaseUOM, PurchaseUOM: ex.PurchaseUOM, SalesUOM: ex.SalesUOM,
			PurchaseFactor: ex.PurchaseFactor, SalesFactor: ex.SalesFactor,
			PurchasePrice: ex.PurchasePrice, SellingPrice: ex.SellingPrice, MinSellingPrice: ex.MinSellingPrice,
			MinStock: ex.MinStock, Summary: ex.Summary}
	} else {
		in.Type, in.Tracked, in.PurchaseFactor, in.SalesFactor = "barang", true, 1, 1
	}
	fail := func(field, msg string) {
		plan.fail(sheetProducts, row.Line, productColumnLabel[field], msg)
	}
	has := func(f string) bool { return ps.Has[f] }
	if has("category") {
		in.CategoryID = nil // resolved from catCode once the categories exist
		if ref := row.Get("category"); ref != "" {
			if c, msg := resolveCategoryRef(ref, cats); msg != "" {
				fail("category", msg)
			} else {
				catCode = strings.ToUpper(c.Code)
			}
		}
	}
	if has("type") {
		if v := row.Get("type"); v != "" {
			t, ok := parseType(v)
			if !ok {
				fail("type", "harus Barang, Jasa, Paket, atau Non Stok")
			}
			in.Type = t
		} else {
			in.Type = "barang"
		}
	}
	if has("tracked") {
		if v := row.Get("tracked"); v != "" {
			b, ok := parseBool(v)
			if !ok {
				fail("tracked", "harus Ya atau Tidak")
			}
			in.Tracked = b
		} else {
			in.Tracked = true
		}
	}
	if has("base_uom") {
		in.BaseUOM = row.Get("base_uom")
	}
	if has("purchase_uom") {
		in.PurchaseUOM = row.Get("purchase_uom")
	}
	if has("sales_uom") {
		in.SalesUOM = row.Get("sales_uom")
	}
	if in.BaseUOM == "" {
		fail("base_uom", "wajib diisi")
	}
	if in.PurchaseUOM == "" {
		in.PurchaseUOM = in.BaseUOM
	}
	if in.SalesUOM == "" {
		in.SalesUOM = in.BaseUOM
	}
	num := func(field string, allowNeg bool) (float64, bool, bool) { // value, present-and-nonblank, ok
		v := row.Get(field)
		if v == "" {
			return 0, false, true
		}
		f, ok := parseFlexibleNumber(v)
		if !ok || (!allowNeg && f < 0) {
			fail(field, "'"+v+"' bukan angka yang valid")
			return 0, true, false
		}
		return f, true, true
	}
	if has("purchase_factor") {
		if f, present, ok := num("purchase_factor", false); ok {
			if !present {
				f = 1
			}
			if f <= 0 {
				fail("purchase_factor", "harus lebih dari 0")
			}
			in.PurchaseFactor = f
		}
	}
	// Sales factor: "Isi Jual per Satuan Stok" is the INVERSE (1 stock unit =
	// N sales units, so factor = 1/N); "Isi per Satuan Jual" is direct.
	if has("sales_inverse") && row.Get("sales_inverse") != "" {
		if f, _, ok := num("sales_inverse", false); ok {
			if f <= 0 {
				fail("sales_inverse", "harus lebih dari 0")
			} else {
				in.SalesFactor = 1 / f
			}
		}
	} else if has("sales_factor") {
		if f, present, ok := num("sales_factor", false); ok {
			if !present {
				f = 1
			}
			if f <= 0 {
				fail("sales_factor", "harus lebih dari 0")
			}
			in.SalesFactor = f
		}
	}
	for _, m := range []struct {
		field string
		dst   *int64
	}{{"purchase_price", &in.PurchasePrice}, {"selling_price", &in.SellingPrice}, {"min_selling", &in.MinSellingPrice}} {
		if !has(m.field) {
			continue
		}
		if f, _, ok := num(m.field, false); ok {
			*m.dst = roundRupiah(f)
		}
	}
	if has("min_stock") {
		if f, _, ok := num("min_stock", false); ok {
			in.MinStock = f
		}
	}
	if has("status") {
		if v := row.Get("status"); v != "" {
			st, ok := parseProductStatus(v)
			if !ok || st == "archived" {
				fail("status", "harus Aktif atau Nonaktif")
			} else {
				in.Status = st
			}
		}
	}
	if has("summary") {
		in.Summary = row.Get("summary")
	}
	return normalizeInput(in), catCode
}

// resolveCategoryRef finds a category by code, then by name, in the
// post-import set.
func resolveCategoryRef(ref string, cats map[string]*finalCategory) (*finalCategory, string) {
	if c, ok := cats[strings.ToUpper(ref)]; ok {
		if c.Status != "active" {
			return nil, "kategori '" + ref + "' sudah diarsipkan"
		}
		return c, ""
	}
	var hit *finalCategory
	n := 0
	for _, c := range cats {
		if c.Status == "active" && strings.EqualFold(c.Name, ref) {
			hit = c
			n++
		}
	}
	switch n {
	case 0:
		return nil, "kategori '" + ref + "' tidak ditemukan"
	case 1:
		return hit, ""
	}
	return nil, "nama kategori '" + ref + "' ambigu: pakai kode kategori"
}

// checkImportCollisions runs the scan-key rules of validateProduct for the
// whole file in a few chunked IN queries instead of per row (product code,
// variant codes and barcodes share one namespace; legacy seenScanKeys).
func (s *Service) checkImportCollisions(ctx context.Context, plan *importPlan, builds []*importBuilt) error {
	keySet := map[string]bool{}
	var keys []string
	add := func(k string) {
		k = strings.ToUpper(strings.TrimSpace(k))
		if k != "" && !keySet[k] {
			keySet[k] = true
			keys = append(keys, k)
		}
	}
	for _, b := range builds {
		add(b.ip.Code)
		for _, v := range b.in.Variants {
			add(v.Code)
			add(v.Barcode)
		}
	}
	codeOwners := map[string][]infrastructure.ScanKeyOwner{}
	barcodeOwners := map[string][]infrastructure.ScanKeyOwner{}
	productCodes := map[string]bool{}
	for start := 0; start < len(keys); start += 400 {
		chunk := keys[start:min(start+400, len(keys))]
		owners, err := s.repo.OtherVariantsUsingKeys(ctx, chunk, 0)
		if err != nil {
			return apperror.Internal(err)
		}
		for _, o := range owners {
			codeOwners[strings.ToUpper(o.Code)] = append(codeOwners[strings.ToUpper(o.Code)], o)
			if o.Barcode != "" {
				barcodeOwners[strings.ToUpper(o.Barcode)] = append(barcodeOwners[strings.ToUpper(o.Barcode)], o)
			}
		}
		pc, err := s.repo.ProductCodesIn(ctx, chunk, 0)
		if err != nil {
			return apperror.Internal(err)
		}
		for k := range pc {
			productCodes[k] = true
		}
	}
	other := func(os []infrastructure.ScanKeyOwner, eid int64) bool {
		for _, o := range os {
			if o.ProductID != eid {
				return true
			}
		}
		return false
	}
	for _, b := range builds {
		var eid int64
		if b.existing != nil {
			eid = b.existing.ID
		}
		before := plan.Summary.Errors
		failP := func(msg string) { plan.fail(sheetProducts, b.ip.Row, "Kode Produk", msg) }
		failV := func(msg string) {
			if b.varRow == 0 {
				plan.fail(sheetProducts, b.ip.Row, "Varian", msg)
				return
			}
			plan.fail(sheetVariants, b.varRow, "Kode Varian/Barcode", msg)
		}
		if other(codeOwners[b.ip.Code], eid) || other(barcodeOwners[b.ip.Code], eid) {
			failP("kode produk '" + b.ip.Code + "' sudah digunakan sebagai kode atau barcode varian")
		}
		for _, v := range b.in.Variants {
			vc, bc := strings.ToUpper(strings.TrimSpace(v.Code)), strings.ToUpper(strings.TrimSpace(v.Barcode))
			if other(barcodeOwners[vc], eid) {
				failV("kode varian '" + vc + "' sama dengan barcode varian produk lain")
			}
			if vc != b.ip.Code && productCodes[vc] {
				failV("kode varian '" + vc + "' sama dengan kode produk lain")
			}
			if bc != "" {
				if other(barcodeOwners[bc], eid) || other(codeOwners[bc], eid) {
					failV("barcode '" + bc + "' sudah dipakai produk lain")
				}
				if bc != b.ip.Code && productCodes[bc] {
					failV("barcode '" + bc + "' sama dengan kode produk lain")
				}
			}
		}
		if plan.Summary.Errors > before {
			b.bad = true
		}
	}
	return nil
}

// PreviewImport dry-runs the workbook: every row validated, nothing written.
func (s *Service) PreviewImport(ctx context.Context, r io.Reader) (*ImportPreview, error) {
	raw, err := io.ReadAll(io.LimitReader(r, maxImportBytes+1))
	if err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "gagal membaca file"}})
	}
	if len(raw) > maxImportBytes {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "maksimal 10 MB"}})
	}
	plan, err := s.buildImportPlan(ctx, raw)
	if err != nil {
		return nil, wrapErr(err)
	}
	prev := &ImportPreview{Issues: plan.Issues, Errors: []RowError{}, OKToCommit: plan.Summary.Errors == 0,
		Summary: plan.Summary, Categories: []ImportRow{}, Products: []ImportRow{}}
	if prev.Issues == nil {
		prev.Issues = []ImportIssue{}
	}
	for _, c := range plan.Categories {
		prev.Categories = append(prev.Categories, ImportRow{Row: c.Row, Code: c.Code, Name: c.Name, Action: c.Action})
	}
	for _, p := range plan.Products {
		prev.Products = append(prev.Products, ImportRow{Row: p.Row, Code: p.Code, Name: p.NameLabel, Action: p.Action})
		prev.ValidVariants += p.VariantN
	}
	prev.ValidProducts = len(plan.Products)
	for _, is := range plan.Issues {
		if is.Severity == "error" {
			prev.Errors = append(prev.Errors, RowError{Sheet: is.Sheet, Row: is.Row, Column: is.Column, Reason: is.Message})
		}
	}
	return prev, nil
}

// sortedKeys returns a string map's keys in a stable order (deterministic issues).
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
