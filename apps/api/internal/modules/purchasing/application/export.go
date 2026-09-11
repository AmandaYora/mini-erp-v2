package application

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-pdf/fpdf"
	"github.com/xuri/excelize/v2"
	"mini-erp/internal/modules/purchasing/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
)

// Order list export (J1): server-side .xlsx (streaming writer — the whole
// workbook is never assembled in memory, P3) and .pdf. Rows stream from the
// list query in 500-row pages, so heap stays O(page) for the source data
// regardless of N.

const exportPageSize = 500

var purchExportHeader = []string{
	"Nomor", "Tanggal", "Supplier", "Termin", "Jatuh Tempo",
	"Subtotal", "Diskon", "Pajak", "Grand Total", "Status", "No Faktur Supplier",
}

func purchExportRow(o *contracts.PurchaseOrder) []string {
	return []string{
		o.Number, o.OrderDate, o.PartyName, o.PaymentTerms, o.DueDate,
		strconv.FormatInt(o.Subtotal, 10),
		strconv.FormatInt(o.DiscountTotal, 10),
		strconv.FormatInt(o.TaxTotal, 10),
		strconv.FormatInt(o.GrandTotal, 10),
		o.Status, o.SupplierInvoiceNumber,
	}
}

// Export dumps the filtered order list. format is "xlsx" or "pdf".
func (s *Service) Export(ctx context.Context, branchID int64, search, status, format string) ([]byte, string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format != "xlsx" && format != "pdf" {
		return nil, "", apperror.Validation("", []apperror.FieldError{{Field: "format", Message: "harus xlsx atau pdf"}})
	}
	stamp := timeutil.NowUTC().In(timeutil.Jakarta).Format("20060102-150405")
	if format == "xlsx" {
		raw, err := s.exportXLSX(ctx, branchID, search, status)
		if err != nil {
			return nil, "", err
		}
		return raw, fmt.Sprintf("order-beli-%s.xlsx", stamp), nil
	}
	raw, err := s.exportPDF(ctx, branchID, search, status)
	if err != nil {
		return nil, "", err
	}
	return raw, fmt.Sprintf("order-beli-%s.pdf", stamp), nil
}

func (s *Service) exportXLSX(ctx context.Context, branchID int64, search, status string) ([]byte, error) {
	f := excelize.NewFile()
	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	header := make([]any, len(purchExportHeader))
	for i, h := range purchExportHeader {
		header[i] = h
	}
	if err := sw.SetRow("A1", header); err != nil {
		return nil, apperror.Internal(err)
	}
	row := 2
	for page := 1; ; page++ {
		res, err := s.repo.List(ctx, branchID, search, status, exportPageSize, (page-1)*exportPageSize)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if len(res) == 0 {
			break
		}
		for _, o := range res {
			cells := purchExportRow(o)
			vals := make([]any, len(cells))
			for i, c := range cells {
				vals[i] = c
			}
			if err := sw.SetRow(fmt.Sprintf("A%d", row), vals); err != nil {
				return nil, apperror.Internal(err)
			}
			row++
		}
		if len(res) < exportPageSize {
			break
		}
	}
	if err := sw.Flush(); err != nil {
		return nil, apperror.Internal(err)
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, apperror.Internal(err)
	}
	_ = f.Close()
	return buf.Bytes(), nil
}

func (s *Service) exportPDF(ctx context.Context, branchID int64, search, status string) ([]byte, error) {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetFont("Arial", "B", 12)
	pdf.AddPage()
	pdf.CellFormat(0, 8, "Daftar Order Beli", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 8)
	widths := []float64{38, 22, 44, 16, 22, 24, 24, 24, 26, 20, 34}
	pdf.SetFillColor(230, 230, 230)
	for i, h := range purchExportHeader {
		pdf.CellFormat(widths[i], 6, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)
	for page := 1; ; page++ {
		res, err := s.repo.List(ctx, branchID, search, status, exportPageSize, (page-1)*exportPageSize)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if len(res) == 0 {
			break
		}
		for _, o := range res {
			cells := purchExportRow(o)
			for i, c := range cells {
				align := "L"
				if i >= 5 && i <= 8 {
					align = "R"
				}
				pdf.CellFormat(widths[i], 5, truncateCell(c, 28), "1", 0, align, false, 0, "")
			}
			pdf.Ln(-1)
		}
		if len(res) < exportPageSize {
			break
		}
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, apperror.Internal(err)
	}
	return buf.Bytes(), nil
}

func truncateCell(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
