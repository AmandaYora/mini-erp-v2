package application

import (
	"io"

	"github.com/xuri/excelize/v2"

	"mini-erp/internal/shared/apperror"
)

// templateColumn is one header of the downloadable workbook.
type templateColumn struct {
	Header   string
	Required bool
	Width    float64
}

var templateSheets = []struct {
	Kind    string
	Columns []templateColumn
	Example [][]any
}{
	{sheetCategories, []templateColumn{
		{"Kode Kategori", true, 18}, {"Nama Kategori", true, 28}, {"Subkategori Dari", false, 22},
		{"Urutan Tampil", false, 14}, {"Status", false, 12}},
		[][]any{{"MNM", "Minuman", "", 1, "Aktif"}, {"MNM-DNG", "Minuman Dingin", "MNM", 2, "Aktif"}}},
	{sheetProducts, []templateColumn{
		{"Kode Produk", true, 16}, {"Nama Produk", true, 30}, {"Kategori", false, 18}, {"Tipe Produk", false, 14},
		{"Pantau Stok?", false, 13}, {"Satuan Stok", true, 12}, {"Satuan Beli", false, 12}, {"Satuan Jual", false, 12},
		{"Isi per Satuan Beli", false, 18}, {"Isi Jual per Satuan Stok", false, 22}, {"Harga Beli", false, 14},
		{"Harga Jual", false, 14}, {"Harga Jual Minimum", false, 18}, {"Minimum Stok", false, 14},
		{"Status", false, 12}, {"Catatan Produk", false, 30}},
		[][]any{{"BRG-001", "Contoh Barang", "MNM-DNG", "Barang", "Ya", "pcs", "dus", "pcs", 12, 1, 10000, 15000, 14000, 5, "Aktif", ""}}},
	{sheetVariants, []templateColumn{
		{"Kode Produk", true, 16}, {"Kode Varian", true, 18}, {"Nama Varian", false, 24},
		{"Status", false, 12}, {"Urutan Tampil", false, 14}, {"Barcode", false, 20}},
		[][]any{{"BRG-001", "BRG-001-A", "Varian A", "Aktif", 1, ""}}},
	{sheetAttributes, []templateColumn{
		{"Kode Produk", true, 16}, {"Nama Info", true, 24}, {"Isi", true, 36}},
		[][]any{{"BRG-001", "Merek", "Contoh"}}},
}

// buildTemplate generates the Indonesian-headed import workbook: a guide sheet
// plus the four data sheets, required headers marked. The first revamp's
// English headers stay accepted on upload as aliases.
func buildTemplate() (*excelize.File, error) {
	f := excelize.NewFile()
	guide := "Panduan"
	if err := f.SetSheetName("Sheet1", guide); err != nil {
		return nil, err
	}
	lines := []string{
		"Panduan Impor Produk",
		"",
		"1. Isi sheet sesuai kebutuhan. Hanya 'Daftar Produk' yang wajib; sheet lain opsional.",
		"2. Kolom bertanda merah wajib diisi. Kolom lain boleh dikosongkan.",
		"3. Kode produk yang sudah ada akan DIPERBARUI (impor = upsert); kode baru dibuat.",
		"4. Kategori boleh ditulis dengan kode atau nama. 'Subkategori Dari' mengisi induk kategori.",
		"5. 'Tipe Produk': Barang, Jasa, Paket, atau Non Stok. Hanya Barang yang bisa Pantau Stok.",
		"6. 'Isi Jual per Satuan Stok' = berapa satuan jual dalam 1 satuan stok (4 berarti faktor jual 0,25).",
		"7. Angka boleh ditulis 10000, 10,000, 10.000, atau 1.234,50.",
		"8. Jika ada satu galat, seluruh file ditolak dan tidak ada data yang tertulis. Pakai Pratinjau dulu.",
		"9. Satuan, tipe, dan pantau stok terkunci bila produk sudah punya riwayat stok.",
		"10. Varian hanya untuk produk Barang dengan Pantau Stok. Info Tambahan menimpa isi sebelumnya.",
	}
	for i, l := range lines {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		_ = f.SetCellValue(guide, cell, l)
	}
	_ = f.SetColWidth(guide, "A", "A", 100)
	title, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 14}})
	_ = f.SetCellStyle(guide, "A1", "A1", title)

	required, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"C0392B"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	optional, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"2C3E50"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	for _, sh := range templateSheets {
		name := sheetTitle[sh.Kind]
		if _, err := f.NewSheet(name); err != nil {
			return nil, err
		}
		for i, c := range sh.Columns {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			_ = f.SetCellValue(name, cell, c.Header)
			st := optional
			if c.Required {
				st = required
			}
			_ = f.SetCellStyle(name, cell, cell, st)
			col, _ := excelize.ColumnNumberToName(i + 1)
			_ = f.SetColWidth(name, col, col, c.Width)
		}
		for r, row := range sh.Example {
			for i, v := range row {
				cell, _ := excelize.CoordinatesToCellName(i+1, r+2)
				_ = f.SetCellValue(name, cell, v)
			}
		}
		_ = f.SetRowHeight(name, 1, 30)
	}
	return f, nil
}

// Template writes a fresh import workbook to w.
func (s *Service) Template(w io.Writer) error {
	f, err := buildTemplate()
	if err != nil {
		return apperror.Internal(err)
	}
	defer func() { _ = f.Close() }()
	if err := f.Write(w); err != nil {
		return apperror.Internal(err)
	}
	return nil
}
