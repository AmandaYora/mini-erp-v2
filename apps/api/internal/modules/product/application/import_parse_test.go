package application

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestParseFlexibleNumber(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"10000", 10000, true},
		{"10,000", 10000, true},
		{"10.000", 10000, true},
		{"1.234,50", 1234.5, true},
		{"1,234.50", 1234.5, true},
		{"1.234.567", 1234567, true},
		{"Rp 10.000", 10000, true},
		{"Rp. 2.500.000", 2500000, true},
		{"12,5", 12.5, true},
		{"0.250", 0.25, true}, // leading zero: a fraction, never thousands
		{"0,25", 0.25, true},
		{"10000.5", 10000.5, true},
		{"1e-05", 0.00001, true},
		{"(500)", -500, true},
		{"-1.500", -1500, true},
		{" 7 ", 7, true},
		{numericMark + "1.125", 1.125, true}, // a real number cell, not thousands
		{"", 0, false},
		{"abc", 0, false},
		{"1.2.3", 0, false},
		{"1,2,3", 0, false},
		{"12-5", 0, false},
	}
	for _, c := range cases {
		got, ok := parseFlexibleNumber(c.in)
		if ok != c.ok || (ok && (got-c.want > 1e-9 || c.want-got > 1e-9)) {
			t.Errorf("parseFlexibleNumber(%q) = %v,%v want %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestNormNameAndAliases(t *testing.T) {
	for in, want := range map[string]string{
		"Pantau Stok?":             "pantau stok",
		"Kode Produk *":            "kode produk",
		"purchase_price":           "purchase price",
		"  Harga   Jual (wajib)":   "harga jual",
		"Isi Jual per Satuan Stok": "isi jual per satuan stok",
	} {
		if got := normName(in); got != want {
			t.Errorf("normName(%q) = %q want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"Daftar Produk": sheetProducts, "KATEGORI PRODUK": sheetCategories,
		"Varian Produk": sheetVariants, "info tambahan produk": sheetAttributes, "products": sheetProducts,
	} {
		if got := sheetAliases[normName(in)]; got != want {
			t.Errorf("sheet %q -> %q want %q", in, got, want)
		}
	}
	if _, ok := sheetAliases[normName("Panduan")]; ok {
		t.Error("Panduan must be ignored")
	}
	if v, ok := parseType("Non Stok"); !ok || v != "non_stok" {
		t.Errorf("parseType non stok = %q,%v", v, ok)
	}
	if v, ok := parseType("physical"); !ok || v != "barang" {
		t.Errorf("parseType physical = %q,%v", v, ok)
	}
}

func xlsx(t *testing.T, sheets map[string][][]any) []byte {
	t.Helper()
	f := excelize.NewFile()
	first := true
	for name, rows := range sheets {
		if first {
			_ = f.SetSheetName("Sheet1", name)
			first = false
		} else if _, err := f.NewSheet(name); err != nil {
			t.Fatal(err)
		}
		for r, row := range rows {
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

func TestReadWorkbookAndClassify(t *testing.T) {
	raw := xlsx(t, map[string][][]any{
		"Daftar Produk": {
			{"Kode Produk", "Nama Produk", "Satuan Stok", "Pantau Stok?", "Harga Jual", "Isi Jual per Satuan Stok"},
			{"A-1", "Barang A", "pcs", "Ya", 15000.5, "4"},
			{},
			{"A-2", "Barang B", "pcs", "Tidak", "10,000", ""},
		},
		"Panduan": {{"abaikan"}},
	})
	sheets, err := readWorkbook(raw)
	if err != nil {
		t.Fatal(err)
	}
	parsed := classifySheets(sheets)
	ps := parsed[sheetProducts]
	if ps == nil || len(parsed) != 1 {
		t.Fatalf("classified = %v", parsed)
	}
	if len(ps.Missing) != 0 || !ps.Has["tracked"] || !ps.Has["sales_inverse"] {
		t.Fatalf("headers not mapped: has=%v missing=%v", ps.Has, ps.Missing)
	}
	if len(ps.Rows) != 2 || ps.Rows[1].Line != 4 {
		t.Fatalf("rows (blank skipped, line numbers kept) = %+v", ps.Rows)
	}
	if f, ok := parseFlexibleNumber(ps.Rows[0].Get("selling_price")); !ok || f != 15000.5 {
		t.Errorf("raw numeric cell = %q", ps.Rows[0].Get("selling_price"))
	}
	if f, ok := parseFlexibleNumber(ps.Rows[1].Get("selling_price")); !ok || f != 10000 {
		t.Errorf("text 10,000 = %v %v", f, ok)
	}
	if _, err := readWorkbook([]byte("bukan excel")); err == nil {
		t.Error("garbage must be rejected")
	}
	ole := append([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, make([]byte, 64)...)
	if _, err := readWorkbook(ole); err == nil {
		t.Error(".xls (OLE2) must be reported, not misread")
	}
}

func TestTemplateReadsBackThroughTheParser(t *testing.T) {
	f, err := buildTemplate()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	sheets, err := readWorkbook(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	parsed := classifySheets(sheets)
	for _, kind := range []string{sheetCategories, sheetProducts, sheetVariants, sheetAttributes} {
		ps := parsed[kind]
		if ps == nil || len(ps.Missing) > 0 || len(ps.Rows) == 0 {
			t.Fatalf("template sheet %s not readable: %+v", kind, ps)
		}
	}
}

// PR-08: legacy inline "Info tambahan:" / "Atribut:" columns are detected on
// the products sheet (raw header, so the ':' survives) and only non-blank
// cells are reported; ordinary and other-sheet columns are untouched.
func TestInlineAttributeColumnsDetected(t *testing.T) {
	for h, want := range map[string]bool{
		"Info tambahan: Warna": true, "  info   tambahan:Ukuran ": true, "Atribut: Merek": true,
		"ATRIBUT:Bahan?": true, "Info Tambahan": false, "Catatan Produk": false, "Atribut": false,
	} {
		if got := inlineAttrHeader(h); got != want {
			t.Errorf("inlineAttrHeader(%q) = %v want %v", h, got, want)
		}
	}
	sh := rawSheet{name: "Daftar Produk", rows: [][]string{
		{"Kode Produk", "Nama Produk", "Info tambahan: Warna", "Atribut: Merek"},
		{"P1", "Kaos", "Merah", ""},
		{"P2", "Celana", "", ""},
	}}
	ps := parseSheet(sheetProducts, sh)
	if len(ps.Rows) != 2 {
		t.Fatalf("rows = %d", len(ps.Rows))
	}
	if got := ps.Rows[0].Inline; len(got) != 1 || got["Info tambahan: Warna"] != "Merah" {
		t.Fatalf("row 1 inline = %v", got)
	}
	if ps.Rows[1].Inline != nil {
		t.Fatalf("blank inline cells must not be reported: %v", ps.Rows[1].Inline)
	}
	// Other sheets never collect inline cells.
	if v := parseSheet(sheetVariants, rawSheet{name: "Varian", rows: [][]string{{"Kode Produk", "Kode Varian", "Atribut: X"}, {"P1", "V1", "1"}}}); v.Rows[0].Inline != nil {
		t.Fatalf("variants sheet must not collect inline: %v", v.Rows[0].Inline)
	}
}
