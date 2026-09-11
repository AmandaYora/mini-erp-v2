package application

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/shared/apperror"
)

// OpeningSheet is the single sheet of the opening-balance workbook.
const OpeningSheet = "opening"

// OpeningHeaders defines the template column order.
var OpeningHeaders = []string{"product_code", "variant_code", "location_code", "qty"}

// OpeningRowError pinpoints one rejected row.
type OpeningRowError struct {
	Row    int    `json:"row"`
	Column string `json:"column"`
	Value  string `json:"value"`
	Reason string `json:"reason"`
}

// OpeningPreview is the dry-run result: nothing is written.
type OpeningPreview struct {
	ValidRows int               `json:"validRows"`
	Errors    []OpeningRowError `json:"errors"`
}

// OpeningResult is the commit outcome.
type OpeningResult struct {
	Posted   int               `json:"posted"`
	Skipped  int               `json:"skipped"`
	Failures []OpeningRowError `json:"failures"`
}

type openingRow struct {
	line         int
	productCode  string
	variantCode  string
	locationCode string
	qtyRaw       string
}

// Template writes a fresh opening workbook to w.
func (s *Service) OpeningTemplate(w io.Writer) error {
	f := excelize.NewFile()
	if err := f.SetSheetName("Sheet1", OpeningSheet); err != nil {
		return apperror.Internal(err)
	}
	for i, h := range OpeningHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(OpeningSheet, cell, h)
	}
	example := []string{"BRG-001", "", "GDG", "100"}
	for i, v := range example {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		_ = f.SetCellValue(OpeningSheet, cell, v)
	}
	_ = f.DeleteSheet("Sheet1")
	if err := f.Write(w); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func openOpeningBook(r io.Reader) (*excelize.File, error) {
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

func readOpeningRows(f *excelize.File) ([]openingRow, error) {
	rows, err := f.GetRows(OpeningSheet)
	if err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "sheet opening tidak ditemukan"}})
	}
	if len(rows) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "sheet opening kosong"}})
	}
	idx := map[string]int{}
	for i, h := range rows[0] {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	var out []openingRow
	for n, row := range rows[1:] {
		blank := true
		for _, c := range row {
			if strings.TrimSpace(c) != "" {
				blank = false
				break
			}
		}
		if blank {
			continue
		}
		get := func(h string) string {
			if i, ok := idx[h]; ok && i < len(row) {
				return strings.TrimSpace(row[i])
			}
			return ""
		}
		out = append(out, openingRow{
			line: n + 2, productCode: strings.ToUpper(get("product_code")),
			variantCode:  strings.ToUpper(get("variant_code")),
			locationCode: strings.ToUpper(get("location_code")), qtyRaw: get("qty"),
		})
	}
	return out, nil
}

type resolvedOpening struct {
	line       int
	branchID   int64
	productID  int64
	variantID  int64
	locationID int64
	qty        float64
}

// resolveOpeningRow validates one row against live master data.
func (s *Service) resolveOpeningRow(ctx context.Context, branchID int64, row openingRow) (*resolvedOpening, *OpeningRowError) {
	fail := func(col, reason string) *OpeningRowError {
		val := row.productCode
		if col == "location_code" {
			val = row.locationCode
		} else if col == "qty" {
			val = row.qtyRaw
		} else if col == "variant_code" {
			val = row.variantCode
		}
		return &OpeningRowError{Row: row.line, Column: col, Value: val, Reason: reason}
	}
	if row.productCode == "" {
		return nil, fail("product_code", "wajib diisi")
	}
	product, err := s.products.GetByCode(ctx, row.productCode)
	if err != nil {
		return nil, &OpeningRowError{Row: row.line, Column: "product_code", Value: row.productCode, Reason: "gagal membaca produk"}
	}
	if product == nil {
		return nil, fail("product_code", "produk tidak ditemukan")
	}
	if product.Status != "active" {
		return nil, fail("product_code", "produk sudah diarsipkan")
	}
	variantID := int64(0)
	if row.variantCode != "" {
		found := false
		for _, v := range product.Variants {
			if v.Code == row.variantCode {
				variantID, found = v.ID, true
				break
			}
		}
		if !found {
			return nil, fail("variant_code", "varian tidak dikenal")
		}
	} else {
		for _, v := range product.Variants {
			if v.IsDefault {
				variantID = v.ID
				break
			}
		}
		if variantID == 0 && len(product.Variants) > 0 {
			variantID = product.Variants[0].ID
		}
	}
	if row.locationCode == "" {
		return nil, fail("location_code", "wajib diisi")
	}
	loc, err := s.repo.LocationByCode(ctx, branchID, row.locationCode)
	if err != nil {
		return nil, &OpeningRowError{Row: row.line, Column: "location_code", Value: row.locationCode, Reason: "gagal membaca lokasi"}
	}
	if loc == nil || loc.Status != "active" {
		return nil, fail("location_code", "lokasi tidak ditemukan")
	}
	leaf, err := s.repo.IsLeaf(ctx, loc.ID)
	if err != nil {
		return nil, &OpeningRowError{Row: row.line, Column: "location_code", Value: row.locationCode, Reason: "gagal membaca lokasi"}
	}
	if !leaf {
		return nil, fail("location_code", "bukan lokasi daun")
	}
	qty, err := strconv.ParseFloat(row.qtyRaw, 64)
	if err != nil || qty < 0 {
		return nil, fail("qty", "harus angka ≥ 0")
	}
	return &resolvedOpening{line: row.line, branchID: branchID, productID: product.ID,
		variantID: variantID, locationID: loc.ID, qty: qty}, nil
}

// PreviewOpening dry-runs the workbook: every row validated, nothing written.
func (s *Service) PreviewOpening(ctx context.Context, branchID int64, r io.Reader) (*OpeningPreview, error) {
	f, err := openOpeningBook(r)
	if err != nil {
		if appErr, ok := err.(*apperror.AppError); ok {
			return nil, appErr
		}
		return nil, apperror.Internal(err)
	}
	defer func() { _ = f.Close() }()
	rows, err := readOpeningRows(f)
	if err != nil {
		if appErr, ok := err.(*apperror.AppError); ok {
			return nil, appErr
		}
		return nil, apperror.Internal(err)
	}
	prev := &OpeningPreview{Errors: []OpeningRowError{}}
	for _, row := range rows {
		if _, roerr := s.resolveOpeningRow(ctx, branchID, row); roerr != nil {
			prev.Errors = append(prev.Errors, *roerr)
			continue
		}
		prev.ValidRows++
	}
	return prev, nil
}

// CommitOpening posts every valid row as an opening inflow. Rows whose
// position already has movements are skipped (resumable), never double-posted.
func (s *Service) CommitOpening(ctx context.Context, actorID, branchID int64, r io.Reader) (*OpeningResult, error) {
	f, err := openOpeningBook(r)
	if err != nil {
		if appErr, ok := err.(*apperror.AppError); ok {
			return nil, appErr
		}
		return nil, apperror.Internal(err)
	}
	defer func() { _ = f.Close() }()
	rows, err := readOpeningRows(f)
	if err != nil {
		if appErr, ok := err.(*apperror.AppError); ok {
			return nil, appErr
		}
		return nil, apperror.Internal(err)
	}
	if _, _, err := s.ensureDefaults(ctx, branchID); err != nil {
		return nil, err
	}
	res := &OpeningResult{Failures: []OpeningRowError{}}
	for _, row := range rows {
		ro, roerr := s.resolveOpeningRow(ctx, branchID, row)
		if roerr != nil {
			res.Failures = append(res.Failures, *roerr)
			continue
		}
		moved, err := s.repo.HasMovementsAt(ctx, branchID, ro.productID, ro.variantID, ro.locationID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if moved {
			// This exact position already has history: skip rather than
			// double-post (resumable).
			res.Skipped++
			continue
		}
		tx, err := s.repo.Begin(ctx)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		post := func() error {
			defer func() { _ = tx.Rollback() }()
			if err := postLocked(ctx, tx, s.repo, branchID, ro.productID, ro.variantID, ro.locationID,
				ro.qty, "in", "in", "opening", 0, "Saldo awal", actorID); err != nil {
				return err
			}
			return tx.Commit()
		}
		if err := post(); err != nil {
			return nil, apperror.Internal(err)
		}
		res.Posted++
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "stock_openings.commit", Entity: "stock_opening", EntityID: 0, BranchID: branchID, ActorID: actorID})
	return res, nil
}
