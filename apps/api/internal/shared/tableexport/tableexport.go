// Package tableexport builds .xlsx and .pdf downloads from a paged row source.
// Technical utility (no domain knowledge): the caller supplies a Pager that
// walks its data by keyset, so cost per page is constant and memory stays
// O(page) on the data side (F4-11).
//
// XLSX uses excelize's stream writer (sheet data spills to a temp file, the
// zip is streamed to the response). PDF needs the whole document before the
// cross-reference table can be written, so it is capped at MaxPDFRows.
package tableexport

import (
	"context"
	"fmt"
	"io"

	"github.com/go-pdf/fpdf"
	"github.com/xuri/excelize/v2"

	"mini-erp/internal/shared/apperror"
)

// MaxPDFRows is the largest PDF export; beyond it the caller must narrow the
// filter or pick XLSX.
const MaxPDFRows = 10000

// XLSXContentType / PDFContentType are the response media types.
const (
	XLSXContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	PDFContentType  = "application/pdf"
)

// Pager returns the next page of rows; an empty page ends the walk.
type Pager func(ctx context.Context) ([][]string, error)

// File is a finished export, ready to be streamed to the client.
type File struct {
	Name        string
	ContentType string
	write       func(w io.Writer) error
	closer      func()
}

// Send streams the document.
func (f *File) Send(w io.Writer) error { return f.write(w) }

// Close releases temporary resources; always call it after WriteTo.
func (f *File) Close() {
	if f.closer != nil {
		f.closer()
	}
}

// XLSX walks the pager into a streamed workbook. Cells are written as strings
// (the legacy export did the same).
func XLSX(ctx context.Context, name string, header []string, next Pager) (*File, error) {
	f := excelize.NewFile()
	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		_ = f.Close()
		return nil, apperror.Internal(err)
	}
	hdr := make([]any, len(header))
	for i, h := range header {
		hdr[i] = h
	}
	if err := sw.SetRow("A1", hdr); err != nil {
		_ = f.Close()
		return nil, apperror.Internal(err)
	}
	row := 2
	for {
		page, err := next(ctx)
		if err != nil {
			_ = f.Close()
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		for _, cells := range page {
			vals := make([]any, len(cells))
			for i, c := range cells {
				vals[i] = c
			}
			if err := sw.SetRow(fmt.Sprintf("A%d", row), vals); err != nil {
				_ = f.Close()
				return nil, apperror.Internal(err)
			}
			row++
		}
	}
	if err := sw.Flush(); err != nil {
		_ = f.Close()
		return nil, apperror.Internal(err)
	}
	return &File{
		Name:        name,
		ContentType: XLSXContentType,
		write:       func(w io.Writer) error { return f.Write(w) },
		closer:      func() { _ = f.Close() },
	}, nil
}

// PDFSpec describes a landscape A4 table.
type PDFSpec struct {
	Title    string
	Header   []string
	Widths   []float64
	RightCol map[int]bool // right-aligned columns (money)
	MaxChars int          // cell truncation, default 28
}

// PDF walks the pager into a PDF; more than MaxPDFRows rows is rejected.
func PDF(ctx context.Context, name string, spec PDFSpec, next Pager) (*File, error) {
	if spec.MaxChars == 0 {
		spec.MaxChars = 28
	}
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetFont("Arial", "B", 12)
	pdf.AddPage()
	pdf.CellFormat(0, 8, spec.Title, "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 8)
	pdf.SetFillColor(230, 230, 230)
	for i, h := range spec.Header {
		pdf.CellFormat(spec.Widths[i], 6, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)
	total := 0
	for {
		page, err := next(ctx)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		total += len(page)
		if total > MaxPDFRows {
			return nil, apperror.Validation("Terlalu banyak baris untuk PDF (maksimal 10.000). Persempit filter atau pilih format XLSX.",
				[]apperror.FieldError{{Field: "format", Message: "PDF dibatasi 10.000 baris"}})
		}
		for _, cells := range page {
			for i, c := range cells {
				align := "L"
				if spec.RightCol[i] {
					align = "R"
				}
				pdf.CellFormat(spec.Widths[i], 5, Truncate(c, spec.MaxChars), "1", 0, align, false, 0, "")
			}
			pdf.Ln(-1)
		}
	}
	return &File{
		Name:        name,
		ContentType: PDFContentType,
		write:       func(w io.Writer) error { return pdf.Output(w) },
	}, nil
}

// Truncate shortens s to max runes, marking the cut.
func Truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
