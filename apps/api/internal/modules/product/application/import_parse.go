package application

import (
	"bytes"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/xuri/excelize/v2"

	"mini-erp/internal/shared/apperror"
)

// numericMark prefixes a cell that the workbook stores as a NUMBER but whose
// raw text would read as a thousands-grouped value ("1.125"): the parser
// takes it literally instead of guessing.
const numericMark = "\x01"

// Sheet kinds of the product workbook.
const (
	sheetCategories = "categories"
	sheetProducts   = "products"
	sheetVariants   = "variants"
	sheetAttributes = "attributes"
)

// Display names used in issues and the downloadable template.
var sheetTitle = map[string]string{
	sheetCategories: "Kategori Produk",
	sheetProducts:   "Daftar Produk",
	sheetVariants:   "Varian Produk",
	sheetAttributes: "Info Tambahan Produk",
}

// Sheet name aliases (legacy workbook + the English names of the first
// revamp template), compared after normName.
var sheetAliases = map[string]string{
	"kategori produk": sheetCategories, "kategori": sheetCategories, "categories": sheetCategories,
	"category": sheetCategories, "product categories": sheetCategories,
	"daftar produk": sheetProducts, "produk": sheetProducts, "products": sheetProducts, "product": sheetProducts,
	"varian produk": sheetVariants, "varian": sheetVariants, "variants": sheetVariants, "variant": sheetVariants,
	"info tambahan produk": sheetAttributes, "info tambahan": sheetAttributes,
	"atribut produk": sheetAttributes, "attributes": sheetAttributes,
}

// Header aliases per sheet: canonical field -> accepted headers (normalised).
var headerAliases = map[string]map[string][]string{
	sheetCategories: {
		"code":   {"kode kategori", "category code", "kode", "code"},
		"name":   {"nama kategori", "category name", "nama", "name"},
		"parent": {"subkategori dari", "kategori induk", "induk", "parent", "parent code"},
		"sort":   {"urutan tampil", "urutan", "sort order", "sort"},
		"status": {"status"},
	},
	sheetProducts: {
		"code":            {"kode produk", "product code", "kode", "code"},
		"name":            {"nama produk", "product name", "nama", "name"},
		"category":        {"kategori", "kategori produk", "kode kategori", "category code", "category"},
		"type":            {"tipe produk", "tipe", "jenis produk", "jenis", "type"},
		"tracked":         {"pantau stok", "stok dipantau", "tracked", "stock tracked"},
		"base_uom":        {"satuan stok", "satuan dasar", "satuan", "base uom"},
		"purchase_uom":    {"satuan beli", "purchase uom"},
		"sales_uom":       {"satuan jual", "sales uom"},
		"purchase_factor": {"isi per satuan beli", "isi satuan beli", "purchase factor", "faktor beli"},
		"sales_factor":    {"isi per satuan jual", "isi satuan jual", "sales factor", "faktor jual"},
		"sales_inverse":   {"isi jual per satuan stok"},
		"purchase_price":  {"harga beli", "purchase price"},
		"selling_price":   {"harga jual", "selling price"},
		"min_selling":     {"harga jual minimum", "harga minimum", "min selling price", "minimum harga jual"},
		"min_stock":       {"minimum stok", "stok minimum", "min stock"},
		"status":          {"status", "status produk"},
		"summary":         {"catatan produk", "ringkasan", "catatan", "summary"},
	},
	sheetVariants: {
		"product_code": {"kode produk", "product code"},
		"code":         {"kode varian", "variant code"},
		"name":         {"nama varian", "variant name"},
		"status":       {"status", "status varian"},
		"sort":         {"urutan tampil", "urutan", "sort order"},
		"barcode":      {"barcode", "kode barcode"},
	},
	sheetAttributes: {
		"product_code": {"kode produk", "product code"},
		"key":          {"nama info", "nama info tambahan", "nama atribut", "info tambahan", "atribut", "nama", "key"},
		"value":        {"isi", "isi info", "nilai", "value"},
	},
}

// Required fields per sheet (a missing header is a file error, not a row error).
var requiredFields = map[string][]string{
	sheetCategories: {"code", "name"},
	sheetProducts:   {"code", "name"},
	sheetVariants:   {"product_code", "code"},
	sheetAttributes: {"product_code", "key", "value"},
}

var (
	reParen     = regexp.MustCompile(`\([^)]*\)`)
	reSpaces    = regexp.MustCompile(`\s+`)
	reThousand  = regexp.MustCompile(`^[1-9]\d{0,2}(\.\d{3})+$`)
	reThousandC = regexp.MustCompile(`^[1-9]\d{0,2}(,\d{3})+$`)
)

// normName lower-cases and flattens a sheet or header name: separators and
// annotation marks ("*", "?", "(wajib)") never decide a match.
func normName(s string) string {
	s = strings.ToLower(s)
	s = reParen.ReplaceAllString(s, " ")
	s = strings.Map(func(r rune) rune {
		switch r {
		case '_', '-', '.', '*', '?', ':', '/':
			return ' '
		}
		return r
	}, s)
	return strings.TrimSpace(reSpaces.ReplaceAllString(s, " "))
}

// rawSheet is one worksheet as text rows.
type rawSheet struct {
	name string
	rows [][]string
}

// readWorkbook opens an .xlsx and returns every sheet as text rows. Numeric
// cells are read RAW (the number, not the cell's display format), so a price
// formatted "10,000" or "Rp 10.000" arrives as 10000.
func readWorkbook(raw []byte) ([]rawSheet, error) {
	if len(raw) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "file kosong"}})
	}
	// OLE2 compound document: the legacy .xls (BIFF) container.
	if bytes.HasPrefix(raw, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file",
			Message: "format .xls lama belum bisa dibaca: buka di Excel lalu Simpan Sebagai .xlsx"}})
	}
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "bukan file Excel (.xlsx) yang valid"}})
	}
	defer func() { _ = f.Close() }()
	var out []rawSheet
	for _, name := range f.GetSheetList() {
		rows, err := f.GetRows(name, excelize.Options{RawCellValue: true})
		if err != nil {
			continue
		}
		for r, row := range rows {
			for c, cell := range row {
				if reThousand.MatchString(strings.TrimSpace(cell)) {
					if t, err := f.GetCellType(name, mustCell(c, r)); err == nil && t == excelize.CellTypeNumber {
						rows[r][c] = numericMark + strings.TrimSpace(cell)
					}
				}
			}
		}
		out = append(out, rawSheet{name: name, rows: rows})
	}
	return out, nil
}

func mustCell(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col+1, row+1)
	return name
}

// sheetRow is one data row: canonical field -> trimmed text.
type sheetRow struct {
	Line   int
	Values map[string]string
	// Inline holds the non-blank legacy inline attribute cells of the row
	// (header as typed -> value); only the products sheet fills it.
	Inline map[string]string
}

// Get returns a field's text ("" when the column is missing or blank).
func (r sheetRow) Get(field string) string { return r.Values[field] }

// parsedSheet is a worksheet mapped to canonical fields.
type parsedSheet struct {
	Kind   string
	Name   string
	Has    map[string]bool // canonical fields present as a column
	Rows   []sheetRow
	Header int // 1-based header line
	// Missing lists required fields without a column.
	Missing []string
}

// inlineAttrHeader reports whether a raw header is a legacy inline attribute
// column ("Info tambahan: Warna" / "Atribut: Warna", case and "?" ignored).
// normName cannot be used: it flattens the ':' that marks these columns.
func inlineAttrHeader(h string) bool {
	n := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(h, "?", "")))
	n = reSpaces.ReplaceAllString(n, " ")
	return strings.HasPrefix(n, "info tambahan:") || strings.HasPrefix(n, "atribut:")
}

// classifySheets maps workbook sheets to kinds (first match wins; "Panduan"
// and anything unknown is ignored) and reads their headers.
func classifySheets(sheets []rawSheet) map[string]*parsedSheet {
	out := map[string]*parsedSheet{}
	for _, sh := range sheets {
		kind, ok := sheetAliases[normName(sh.name)]
		if !ok || out[kind] != nil {
			continue
		}
		out[kind] = parseSheet(kind, sh)
	}
	return out
}

func parseSheet(kind string, sh rawSheet) *parsedSheet {
	ps := &parsedSheet{Kind: kind, Name: sh.name, Has: map[string]bool{}}
	// Header = first row with any non-blank cell.
	hi := -1
	for i, row := range sh.rows {
		if !blankRow(row) {
			hi = i
			break
		}
	}
	if hi < 0 {
		ps.Missing = requiredFields[kind]
		return ps
	}
	ps.Header = hi + 1
	colOf := map[string]int{}
	for ci, h := range sh.rows[hi] {
		n := normName(h)
		if n == "" {
			continue
		}
		for field, names := range headerAliases[kind] {
			for _, a := range names {
				if a == n {
					if _, dup := colOf[field]; !dup {
						colOf[field] = ci
					}
				}
			}
		}
	}
	var inlineCols []int
	if kind == sheetProducts {
		for ci, h := range sh.rows[hi] {
			if inlineAttrHeader(h) {
				inlineCols = append(inlineCols, ci)
			}
		}
	}
	for f := range colOf {
		ps.Has[f] = true
	}
	for _, f := range requiredFields[kind] {
		if !ps.Has[f] {
			ps.Missing = append(ps.Missing, f)
		}
	}
	for ri := hi + 1; ri < len(sh.rows); ri++ {
		row := sh.rows[ri]
		if blankRow(row) {
			continue
		}
		vals := make(map[string]string, len(colOf))
		for f, ci := range colOf {
			if ci < len(row) {
				vals[f] = strings.TrimSpace(row[ci])
			}
		}
		sr := sheetRow{Line: ri + 1, Values: vals}
		for _, ci := range inlineCols {
			if ci < len(row) && strings.TrimSpace(row[ci]) != "" {
				if sr.Inline == nil {
					sr.Inline = map[string]string{}
				}
				sr.Inline[strings.TrimSpace(sh.rows[hi][ci])] = strings.TrimSpace(row[ci])
			}
		}
		ps.Rows = append(ps.Rows, sr)
	}
	return ps
}

func blankRow(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// parseFlexibleNumber reads the number spellings of real spreadsheets:
// "10000", "10,000", "10.000", "1.234,50", "1,234.50", "Rp 10.000", "12,5",
// "(500)". A lone separator followed by exactly three digits is a thousands
// separator when the integer part has no leading zero ("10.000" = 10000,
// "0.250" = 0.25). Cells the workbook stores as numbers arrive marked and are
// taken literally.
func parseFlexibleNumber(in string) (float64, bool) {
	s := strings.TrimSpace(in)
	if strings.HasPrefix(s, numericMark) {
		f, err := strconv.ParseFloat(strings.TrimPrefix(s, numericMark), 64)
		return f, err == nil && !math.IsNaN(f) && !math.IsInf(f, 0)
	}
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == ' ' {
			return -1
		}
		return r
	}, s)
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "rp") {
		low = strings.TrimPrefix(strings.TrimPrefix(low, "rp"), ".")
	}
	s = low
	if s == "" {
		return 0, false
	}
	neg := false
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		neg, s = true, s[1:len(s)-1]
	}
	if strings.HasPrefix(s, "-") {
		neg, s = true, s[1:]
	} else if strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	if strings.ContainsAny(s, "e") {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, false
		}
		if neg {
			f = -f
		}
		return f, true
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9') && r != '.' && r != ',' {
			return 0, false
		}
	}
	dots, commas := strings.Count(s, "."), strings.Count(s, ",")
	var norm string
	switch {
	case dots > 0 && commas > 0:
		last := strings.LastIndexAny(s, ".,")
		dec := s[last]
		thou := byte('.')
		if dec == '.' {
			thou = ','
		}
		intPart := strings.ReplaceAll(s[:last], string(thou), "")
		if strings.ContainsRune(intPart, rune(dec)) {
			return 0, false
		}
		norm = intPart + "." + s[last+1:]
	case commas > 0:
		if reThousandC.MatchString(s) {
			norm = strings.ReplaceAll(s, ",", "")
		} else if commas == 1 {
			norm = strings.Replace(s, ",", ".", 1)
		} else {
			return 0, false
		}
	case dots > 0:
		if reThousand.MatchString(s) {
			norm = strings.ReplaceAll(s, ".", "")
		} else if dots == 1 {
			norm = s
		} else {
			return 0, false
		}
	default:
		norm = s
	}
	f, err := strconv.ParseFloat(norm, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	if neg {
		f = -f
	}
	return f, true
}

// parseBool reads ya/tidak style flags.
func parseBool(s string) (val, ok bool) {
	switch normName(s) {
	case "ya", "y", "yes", "true", "1", "aktif", "ok":
		return true, true
	case "tidak", "t", "n", "no", "false", "0", "nonaktif":
		return false, true
	}
	return false, false
}

// parseType maps legacy and Indonesian type names to revamp types.
func parseType(s string) (string, bool) {
	switch normName(s) {
	case "barang", "physical", "fisik":
		return "barang", true
	case "jasa", "service", "layanan":
		return "jasa", true
	case "paket", "bundle":
		return "paket", true
	case "non stok", "nonstok", "non stock", "nonstock":
		return "non_stok", true
	}
	return "", false
}

// parseProductStatus reads product/variant status words.
func parseProductStatus(s string) (string, bool) {
	switch normName(s) {
	case "aktif", "active":
		return "active", true
	case "nonaktif", "non aktif", "tidak aktif", "inactive":
		return "inactive", true
	case "arsip", "diarsipkan", "archived":
		return "archived", true
	}
	return "", false
}
