package application

import (
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestParseFlexibleQty(t *testing.T) {
	cases := map[string]float64{
		"12": 12, "12,5": 12.5, "1.000,5": 1000.5, "1,000.5": 1000.5, " 7 ": 7, "10 pcs": 10,
	}
	for in, want := range cases {
		got, ok := parseFlexibleQty(in)
		if !ok || got != want {
			t.Fatalf("parseFlexibleQty(%q) = %v,%v want %v", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "abc", "-"} {
		if _, ok := parseFlexibleQty(bad); ok {
			t.Fatalf("%q must not parse", bad)
		}
	}
}

func book(t *testing.T, sheet string, rows [][]string) *excelize.File {
	t.Helper()
	f := excelize.NewFile()
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		t.Fatal(err)
	}
	for r, row := range rows {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			_ = f.SetCellValue(sheet, cell, v)
		}
	}
	return f
}

// TestReadOpeningRowsAcceptsLegacyWorkbook: the legacy sheet name and Indonesian
// headers are read; so are the revamp's own English ones (ST-05).
func TestReadOpeningRowsAcceptsLegacyWorkbook(t *testing.T) {
	legacy := book(t, "Stok Awal", [][]string{
		{"Kode Barang", "Nama Barang", "Kode Lokasi", "Jumlah Stok Awal"},
		{"brg-1", "Semen", "gdg", "1.500,5"},
		{"", "", "", ""},
		{"brg-2", "Paku", "rak-a", ""},
	})
	rows, err := readOpeningRows(legacy)
	if err != nil || len(rows) != 2 {
		t.Fatalf("legacy: %v %+v", err, rows)
	}
	if rows[0].productCode != "BRG-1" || rows[0].locationCode != "GDG" || rows[0].qtyRaw != "1.500,5" || rows[0].line != 2 {
		t.Fatalf("row0 = %+v", rows[0])
	}
	if rows[1].qtyRaw != "" || rows[1].line != 4 {
		t.Fatalf("row1 = %+v", rows[1])
	}

	english := book(t, "opening", [][]string{
		{"product_code", "variant_code", "location_code", "qty"},
		{"A", "V1", "GDG", "3"},
	})
	rows, err = readOpeningRows(english)
	if err != nil || len(rows) != 1 || rows[0].variantCode != "V1" {
		t.Fatalf("english: %v %+v", err, rows)
	}

	// a single sheet with any name is read
	odd := book(t, "Sheet Saya", [][]string{{"SKU", "Kode Gudang", "Stok Awal"}, {"A", "GDG", "5"}})
	if rows, err = readOpeningRows(odd); err != nil || len(rows) != 1 || rows[0].qtyRaw != "5" {
		t.Fatalf("single sheet: %v %+v", err, rows)
	}

	// a missing required column is named, not silently empty
	noQty := book(t, "Stok Awal", [][]string{{"Kode Barang", "Kode Lokasi"}, {"A", "GDG"}})
	if _, err = readOpeningRows(noQty); err == nil {
		t.Fatal("missing qty column must be rejected")
	}
}
