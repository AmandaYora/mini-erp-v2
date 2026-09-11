package application

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"mini-erp/internal/shared/apperror"
)

// Import sheets of the product bulk template.
const (
	SheetProducts = "products"
	SheetVariants = "variants"
)

// ProductSheetHeaders defines the template column order (stable contract —
// / the template download, preview, and commit all share it).
var ProductSheetHeaders = []string{
	"code", "name", "category_code", "type", "tracked",
	"base_uom", "purchase_uom", "sales_uom", "purchase_factor", "sales_factor",
	"purchase_price", "selling_price", "min_selling_price", "min_stock",
}

// VariantSheetHeaders defines the variants sheet column order.
var VariantSheetHeaders = []string{
	"product_code", "variant_code", "variant_name", "barcode",
}

// RowError pinpoints one rejected spreadsheet row.
type RowError struct {
	Sheet  string `json:"sheet"`
	Row    int    `json:"row"`
	Column string `json:"column"`
	Value  string `json:"value"`
	Reason string `json:"reason"`
}

// ImportPreview is the dry-run result: nothing is written.
type ImportPreview struct {
	ValidProducts int        `json:"validProducts"`
	ValidVariants int        `json:"validVariants"`
	Errors        []RowError `json:"errors"`
}

// ImportResult is the commit outcome: valid rows inserted, the rest reported.
// Re-uploading the same file later commits only the remaining rows (already
// imported codes are reported as skipped, not failures).
type ImportResult struct {
	Inserted int        `json:"inserted"`
	Skipped  int        `json:"skipped"`
	Failures []RowError `json:"failures"`
}

type importRow struct {
	line   int
	values map[string]string
}

func readSheet(f *excelize.File, sheet string, headers []string) ([]importRow, error) {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("sheet %s kosong", sheet)
	}
	idx := map[string]int{}
	for i, h := range rows[0] {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	var out []importRow
	for n, row := range rows[1:] {
		if isBlankRow(row) {
			continue
		}
		values := map[string]string{}
		for _, h := range headers {
			if i, ok := idx[h]; ok && i < len(row) {
				values[h] = strings.TrimSpace(row[i])
			}
		}
		out = append(out, importRow{line: n + 2, values: values})
	}
	return out, nil
}

func isBlankRow(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func parseIntCell(s string) (int64, bool) {
	if s == "" {
		return 0, true
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil
}

func parseFloatCell(s string) (float64, bool) {
	if s == "" {
		return 0, true
	}
	n, err := strconv.ParseFloat(s, 64)
	return n, err == nil
}

func parseBoolCell(s string) (bool, bool) {
	switch strings.ToLower(s) {
	case "ya", "yes", "true", "1":
		return true, true
	case "tidak", "no", "false", "0":
		return false, true
	default:
		return false, false
	}
}

// buildTemplate generates the import workbook with headers + one example row.
func buildTemplate() (*excelize.File, error) {
	f := excelize.NewFile()
	if err := f.SetSheetName("Sheet1", SheetProducts); err != nil {
		return nil, err
	}
	for i, h := range ProductSheetHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(SheetProducts, cell, h)
	}
	example := []string{"BRG-001", "Contoh Barang", "", "barang", "ya",
		"pcs", "dus", "pcs", "12", "1", "10000", "15000", "14000", "0"}
	for i, v := range example {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		_ = f.SetCellValue(SheetProducts, cell, v)
	}
	if _, err := f.NewSheet(SheetVariants); err != nil {
		return nil, err
	}
	for i, h := range VariantSheetHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(SheetVariants, cell, h)
	}
	vex := []string{"BRG-001", "BRG-001-A", "Varian A", ""}
	for i, v := range vex {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		_ = f.SetCellValue(SheetVariants, cell, v)
	}
	_ = f.DeleteSheet("Sheet1")
	return f, nil
}

// Template writes a fresh import workbook to w.
func (s *Service) Template(w io.Writer) error {
	f, err := buildTemplate()
	if err != nil {
		return apperror.Internal(err)
	}
	if err := f.Write(w); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func openWorkbook(r io.Reader) (*excelize.File, error) {
	raw, err := io.ReadAll(io.LimitReader(r, 10<<20))
	if err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "gagal membaca file"}})
	}
	if len(raw) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "file kosong"}})
	}
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "bukan file Excel yang valid"}})
	}
	return f, nil
}

// rowToInput converts one product sheet row, collecting cell errors.
// It enforces the SAME guards as the form (OQ-A23): unknown categories,
// bad types/UOMs/factors/prices, and unknown tracked flags are rejected,
// never silently coerced (A26).
func (s *Service) rowToInput(ctx context.Context, row importRow, seen map[string]bool) (ProductInput, []RowError) {
	var errs []RowError
	fail := func(col, reason string) {
		errs = append(errs, RowError{Sheet: SheetProducts, Row: row.line, Column: col, Value: row.values[col], Reason: reason})
	}
	v := row.values
	code := strings.ToUpper(v["code"])
	if code == "" {
		fail("code", "wajib diisi")
	} else {
		if seen[code] {
			fail("code", "kode ganda di file")
		}
		seen[code] = true
		if dup, _ := s.repo.ProductByCode(ctx, code); dup != nil {
			fail("code", "kode sudah dipakai produk lain")
		}
	}
	in := ProductInput{Code: code, Name: v["name"]}
	if in.Name == "" {
		fail("name", "wajib diisi")
	}
	if cat := v["category_code"]; cat != "" {
		c, _ := s.repo.CategoryByCode(ctx, strings.ToUpper(cat))
		if c == nil {
			fail("category_code", "kategori tidak ditemukan")
		} else if c.Status != "active" {
			fail("category_code", "kategori sudah diarsipkan")
		} else {
			in.CategoryID = &c.ID
		}
	}
	in.Type = strings.ToLower(v["type"])
	if in.Type == "" {
		in.Type = "barang"
	}
	if in.Type != "barang" && in.Type != "jasa" {
		fail("type", "harus barang atau jasa")
	}
	tracked, ok := parseBoolCell(v["tracked"])
	if v["tracked"] != "" && !ok {
		fail("tracked", "harus ya atau tidak")
	} else if v["tracked"] == "" {
		tracked = true
	}
	in.Tracked = tracked
	in.BaseUOM, in.PurchaseUOM, in.SalesUOM = v["base_uom"], v["purchase_uom"], v["sales_uom"]
	for _, u := range []struct {
		col, val string
	}{{"base_uom", in.BaseUOM}, {"purchase_uom", in.PurchaseUOM}, {"sales_uom", in.SalesUOM}} {
		if u.val == "" {
			fail(u.col, "wajib diisi")
		}
	}
	var good bool
	if in.PurchaseFactor, good = parseFloatCell(v["purchase_factor"]); !good || (v["purchase_factor"] != "" && in.PurchaseFactor <= 0) {
		fail("purchase_factor", "harus angka lebih dari 0")
	} else if v["purchase_factor"] == "" {
		in.PurchaseFactor = 1
	}
	if in.SalesFactor, good = parseFloatCell(v["sales_factor"]); !good || (v["sales_factor"] != "" && in.SalesFactor <= 0) {
		fail("sales_factor", "harus angka lebih dari 0")
	} else if v["sales_factor"] == "" {
		in.SalesFactor = 1
	}
	for _, m := range []struct {
		col string
		dst *int64
	}{{"purchase_price", &in.PurchasePrice}, {"selling_price", &in.SellingPrice}, {"min_selling_price", &in.MinSellingPrice}} {
		n, good := parseIntCell(v[m.col])
		if !good || n < 0 {
			fail(m.col, "harus angka bulat ≥ 0")
		} else {
			*m.dst = n
		}
	}
	if in.MinSellingPrice > in.SellingPrice {
		fail("min_selling_price", "tidak boleh melebihi harga jual")
	}
	if in.Tracked && in.PurchasePrice <= 0 {
		fail("purchase_price", "harga beli wajib diisi untuk produk terpantau")
	}
	if in.MinStock, good = parseFloatCell(v["min_stock"]); !good || in.MinStock < 0 {
		fail("min_stock", "harus angka ≥ 0")
	}
	return in, errs
}

// PreviewImport dry-runs the workbook: every row validated, nothing written.
func (s *Service) PreviewImport(ctx context.Context, r io.Reader) (*ImportPreview, error) {
	f, err := openWorkbook(r)
	if err != nil {
		if appErr, ok := err.(*apperror.AppError); ok {
			return nil, appErr
		}
		return nil, apperror.Internal(err)
	}
	defer func() { _ = f.Close() }()
	prows, err := readSheet(f, SheetProducts, ProductSheetHeaders)
	if err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: err.Error()}})
	}
	vrows, err := readSheet(f, SheetVariants, VariantSheetHeaders)
	if err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: err.Error()}})
	}
	byProduct := map[string][]VariantInput{}
	for _, vr := range vrows {
		pc := strings.ToUpper(vr.values["product_code"])
		vc := strings.ToUpper(vr.values["variant_code"])
		if pc == "" || vc == "" {
			continue
		}
		byProduct[pc] = append(byProduct[pc], VariantInput{
			Code: vc, Name: vr.values["variant_name"], Barcode: vr.values["barcode"],
		})
	}
	seen := map[string]bool{}
	prev := &ImportPreview{Errors: []RowError{}}
	// Variant rows pointing at no product row are reported, never silently
	// dropped — a typo'd product_code must not vaporize variants.
	fileCodes := map[string]int{}
	for _, row := range prows {
		fileCodes[strings.ToUpper(row.values["code"])] = row.line
	}
	for _, vr := range vrows {
		pc := strings.ToUpper(vr.values["product_code"])
		if pc == "" || strings.ToUpper(vr.values["variant_code"]) == "" {
			continue
		}
		if _, ok := fileCodes[pc]; !ok {
			prev.Errors = append(prev.Errors, RowError{Sheet: SheetVariants, Row: vr.line, Column: "product_code", Value: vr.values["product_code"], Reason: "produk tidak ada di file"})
		}
	}
	for _, row := range prows {
		in, errs := s.rowToInput(ctx, row, seen)
		if len(errs) > 0 {
			prev.Errors = append(prev.Errors, errs...)
			continue
		}
		in.Variants = byProduct[in.Code]
		if fields, err := s.validateProduct(ctx, in, 0); err != nil {
			return nil, apperror.Internal(err)
		} else if len(fields) > 0 {
			for _, fe := range fields {
				prev.Errors = append(prev.Errors, RowError{Sheet: SheetProducts, Row: row.line, Column: fe.Field, Reason: fe.Message})
			}
			continue
		}
		prev.ValidProducts++
		prev.ValidVariants += len(in.Variants)
	}
	if prev.Errors == nil {
		prev.Errors = []RowError{}
	}
	return prev, nil
}

// CommitImport inserts every valid row, reporting failures per row. Codes
// already in the database are skipped (resumable), never double-inserted.
func (s *Service) CommitImport(ctx context.Context, actorID int64, r io.Reader) (*ImportResult, error) {
	f, err := openWorkbook(r)
	if err != nil {
		if appErr, ok := err.(*apperror.AppError); ok {
			return nil, appErr
		}
		return nil, apperror.Internal(err)
	}
	defer func() { _ = f.Close() }()
	prows, err := readSheet(f, SheetProducts, ProductSheetHeaders)
	if err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: err.Error()}})
	}
	vrows, err := readSheet(f, SheetVariants, VariantSheetHeaders)
	if err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: err.Error()}})
	}
	byProduct := map[string][]VariantInput{}
	for _, vr := range vrows {
		pc := strings.ToUpper(vr.values["product_code"])
		vc := strings.ToUpper(vr.values["variant_code"])
		if pc == "" || vc == "" {
			continue
		}
		byProduct[pc] = append(byProduct[pc], VariantInput{
			Code: vc, Name: vr.values["variant_name"], Barcode: vr.values["barcode"],
		})
	}
	res := &ImportResult{Failures: []RowError{}}
	fileCodes := map[string]bool{}
	for _, row := range prows {
		fileCodes[strings.ToUpper(row.values["code"])] = true
	}
	for _, vr := range vrows {
		pc := strings.ToUpper(vr.values["product_code"])
		if pc == "" || strings.ToUpper(vr.values["variant_code"]) == "" {
			continue
		}
		if !fileCodes[pc] {
			res.Failures = append(res.Failures, RowError{Sheet: SheetVariants, Row: vr.line, Column: "product_code", Value: vr.values["product_code"], Reason: "produk tidak ada di file (untuk produk yang sudah ada, tambah varian lewat ubah produk)"})
		}
	}
	seen := map[string]bool{}
	for _, row := range prows {
		// Already imported by an earlier commit of the same file: skip,
		// never double-insert and never report as failure (resumable).
		if code := strings.ToUpper(strings.TrimSpace(row.values["code"])); code != "" {
			if dup, _ := s.repo.ProductByCode(ctx, code); dup != nil {
				res.Skipped++
				continue
			}
		}
		in, errs := s.rowToInput(ctx, row, seen)
		if len(errs) > 0 {
			res.Failures = append(res.Failures, errs...)
			continue
		}
		in.Variants = byProduct[in.Code]
		if fields, err := s.validateProduct(ctx, in, 0); err != nil {
			return nil, apperror.Internal(err)
		} else if len(fields) > 0 {
			for _, fe := range fields {
				res.Failures = append(res.Failures, RowError{Sheet: SheetProducts, Row: row.line, Column: fe.Field, Reason: fe.Message})
			}
			continue
		}
		if _, err := s.CreateProduct(ctx, actorID, in); err != nil {
			// Lost race (same code committed concurrently): count as skipped.
			if dup, _ := s.repo.ProductByCode(ctx, in.Code); dup != nil {
				res.Skipped++
				continue
			}
			res.Failures = append(res.Failures, RowError{Sheet: SheetProducts, Row: row.line, Reason: "gagal menyimpan"})
			continue
		}
		res.Inserted++
	}
	if res.Failures == nil {
		res.Failures = []RowError{}
	}
	return res, nil
}
