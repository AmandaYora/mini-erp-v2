package application

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/xuri/excelize/v2"
)

// Tax working-paper package variants.
//
// These identifiers live at the APPLICATION boundary only — the words
// "layer 1" / "layer 2" never reach a file name, a sheet name, or a cell.
// A working paper handed to a tax consultant has to describe ITSELF ("data
// riil", "dibatasi plafon peredaran bruto"), not an internal toggle nobody
// outside this codebase can interpret.
const (
	// PackageActual is the full commercial books, nothing removed.
	PackageActual = "actual"
	// PackageCapped is the PP23 view: turnover limited to the yearly
	// gross-turnover ceiling, with the excluded orders itemised.
	PackageCapped = "capped"
)

// packageMeta describes one variant's identity in the produced artefact.
type packageMeta struct {
	fileSuffix string
	dataLabel  string
}

var packageMetas = map[string]packageMeta{
	PackageActual: {
		fileSuffix: "data-riil",
		dataLabel:  "Data riil — seluruh transaksi terbukukan",
	},
	PackageCapped: {
		fileSuffix: "peredaran-terbatas",
		dataLabel:  "Dibatasi plafon peredaran bruto Rp4.800.000.000 per tahun pajak — seluruh angka sudah mengeluarkan order di luar plafon",
	},
}

// ValidPackageVariant reports whether v names a known package variant.
func ValidPackageVariant(v string) bool {
	_, ok := packageMetas[v]
	return ok
}

// sheet is a tiny writer over excelize that keeps the builders below
// declarative: header once, then rows, then column widths.
type sheet struct {
	f    *excelize.File
	name string
	row  int
	err  error
}

func newSheet(f *excelize.File, name string) *sheet {
	if _, err := f.NewSheet(name); err != nil {
		return &sheet{f: f, name: name, err: err}
	}
	return &sheet{f: f, name: name, row: 0}
}

func (s *sheet) write(values ...any) {
	if s.err != nil {
		return
	}
	s.row++
	for i, v := range values {
		cell, err := excelize.CoordinatesToCellName(i+1, s.row)
		if err != nil {
			s.err = err
			return
		}
		if err := s.f.SetCellValue(s.name, cell, v); err != nil {
			s.err = err
			return
		}
	}
}

func (s *sheet) blank() { s.write("") }

func (s *sheet) widths(w ...float64) {
	if s.err != nil {
		return
	}
	for i, width := range w {
		col, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			s.err = err
			return
		}
		if err := s.f.SetColWidth(s.name, col, col, width); err != nil {
			s.err = err
			return
		}
	}
}

// monthLabel renders an Indonesian month name for the cover sheet.
func monthLabel(month int) string {
	names := []string{"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember"}
	if month < 1 || month >= len(names) {
		return strconv.Itoa(month)
	}
	return names[month]
}

// buildWorkbook renders the finished file, dropping excelize's default
// "Sheet1" so the cover sheet is what opens first.
func buildWorkbook(f *excelize.File, first string) ([]byte, error) {
	idx, err := f.GetSheetIndex(first)
	if err != nil {
		return nil, err
	}
	f.SetActiveSheet(idx)
	if def, err := f.GetSheetIndex("Sheet1"); err == nil && def >= 0 && def != idx {
		if err := f.DeleteSheet("Sheet1"); err != nil {
			return nil, err
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// packageFileName names the artefact after what it CONTAINS.
func packageFileName(variant string, year, month int) string {
	return fmt.Sprintf("paket-pajak-%04d-%02d-%s.xlsx", year, month, packageMetas[variant].fileSuffix)
}
