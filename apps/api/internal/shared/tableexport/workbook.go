package tableexport

import (
	"fmt"
	"io"

	"github.com/xuri/excelize/v2"

	"mini-erp/internal/shared/apperror"
)

// Workbook is a streamed multi-sheet .xlsx: every sheet has its own stream
// writer, rows are appended per sheet in any interleaving, and nothing but the
// current row is held in memory (sheet data spills to a temp file inside
// excelize). The caller walks its data once and feeds several sheets from it —
// a summary sheet is simply written last, once the totals are known.
type Workbook struct {
	f    *excelize.File
	sw   map[string]*excelize.StreamWriter
	next map[string]int
	done bool
}

// NewWorkbook creates the sheets in order; the first one is the opening tab.
func NewWorkbook(sheets ...string) (*Workbook, error) {
	if len(sheets) == 0 {
		return nil, apperror.Internal(fmt.Errorf("workbook needs at least one sheet"))
	}
	f := excelize.NewFile()
	w := &Workbook{f: f, sw: map[string]*excelize.StreamWriter{}, next: map[string]int{}}
	for i, name := range sheets {
		if i == 0 {
			if err := f.SetSheetName("Sheet1", name); err != nil {
				_ = f.Close()
				return nil, apperror.Internal(err)
			}
		} else if _, err := f.NewSheet(name); err != nil {
			_ = f.Close()
			return nil, apperror.Internal(err)
		}
		sw, err := f.NewStreamWriter(name)
		if err != nil {
			_ = f.Close()
			return nil, apperror.Internal(err)
		}
		w.sw[name] = sw
		w.next[name] = 1
	}
	return w, nil
}

// Widths sets the column widths of a sheet; call it before the first row.
func (w *Workbook) Widths(sheet string, widths ...float64) error {
	sw, ok := w.sw[sheet]
	if !ok {
		return apperror.Internal(fmt.Errorf("unknown sheet %q", sheet))
	}
	for i, width := range widths {
		if err := sw.SetColWidth(i+1, i+1, width); err != nil {
			return apperror.Internal(err)
		}
	}
	return nil
}

// Row appends one row to a sheet. Cells may be strings or numbers (money and
// quantities stay numeric so the sheet can be summed).
func (w *Workbook) Row(sheet string, cells ...any) error {
	sw, ok := w.sw[sheet]
	if !ok {
		return apperror.Internal(fmt.Errorf("unknown sheet %q", sheet))
	}
	cell, err := excelize.CoordinatesToCellName(1, w.next[sheet])
	if err != nil {
		return apperror.Internal(err)
	}
	if err := sw.SetRow(cell, cells); err != nil {
		return apperror.Internal(err)
	}
	w.next[sheet]++
	return nil
}

// Finish flushes every sheet and returns the downloadable file. The caller must
// not use the workbook afterwards; File.Close releases it.
func (w *Workbook) Finish(name string) (*File, error) {
	for sheet, sw := range w.sw {
		if err := sw.Flush(); err != nil {
			_ = w.f.Close()
			return nil, apperror.Internal(fmt.Errorf("flush %s: %w", sheet, err))
		}
	}
	w.done = true
	return &File{
		Name:        name,
		ContentType: XLSXContentType,
		write:       func(out io.Writer) error { return w.f.Write(out) },
		closer:      func() { _ = w.f.Close() },
	}, nil
}

// Abort releases a workbook that will not be finished (an error mid-walk).
func (w *Workbook) Abort() {
	if !w.done {
		_ = w.f.Close()
	}
}
