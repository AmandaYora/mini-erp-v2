package presentation

import (
	"encoding/csv"
	"net/http"
	"strconv"

	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/response"
)

func yearMonth(r *http.Request) (int, int, *apperror.AppError) {
	q := r.URL.Query()
	year, err1 := strconv.Atoi(q.Get("year"))
	month, err2 := strconv.Atoi(q.Get("month"))
	if err1 != nil || err2 != nil || year < 2000 || year > 2100 || month < 1 || month > 12 {
		return 0, 0, apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "tahun & bulan wajib valid"}})
	}
	return year, month, nil
}

func dateRange(r *http.Request) (string, string, *apperror.AppError) {
	q := r.URL.Query()
	if q.Get("from") == "" || q.Get("to") == "" {
		return "", "", apperror.Validation("", []apperror.FieldError{{Field: "from", Message: "rentang from-to wajib diisi"}})
	}
	return q.Get("from"), q.Get("to"), nil
}

// TrialBalance handles GET /api/v1/finance/reports/trial-balance?year=&month=.
func (h *Handler) TrialBalance(w http.ResponseWriter, r *http.Request) {
	year, month, appErr := yearMonth(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	rows, err := h.svc.TrialBalance(r.Context(), branchID(r), year, month)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(rows))
	var td, tc int64
	for _, row := range rows {
		items = append(items, map[string]any{
			"code": row.Code, "name": row.Name, "type": row.Type,
			"debit": row.Debit, "credit": row.Credit,
		})
		td += row.Debit
		tc += row.Credit
	}
	response.OK(w, "Neraca saldo", map[string]any{
		"rows": items, "totalDebit": td, "totalCredit": tc, "balanced": td == tc,
	})
}

// ProfitLoss handles GET /api/v1/finance/reports/profit-loss?from=&to=.
func (h *Handler) ProfitLoss(w http.ResponseWriter, r *http.Request) {
	from, to, appErr := dateRange(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	rep, err := h.svc.ProfitLoss(r.Context(), branchID(r), from, to)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Laba rugi", rep)
}

// BalanceSheet handles GET /api/v1/finance/reports/balance-sheet?date=.
func (h *Handler) BalanceSheet(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "date", Message: "wajib diisi"}}))
		return
	}
	bs, err := h.svc.BalanceSheet(r.Context(), branchID(r), date)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Neraca", bs)
}

// GeneralLedger handles GET /api/v1/finance/reports/general-ledger?account=&from=&to=.
func (h *Handler) GeneralLedger(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, to, appErr := dateRange(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	if q.Get("account") == "" {
		response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "account", Message: "kode akun wajib diisi"}}))
		return
	}
	lines, err := h.svc.GeneralLedger(r.Context(), branchID(r), q.Get("account"), from, to)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(lines))
	for _, l := range lines {
		items = append(items, map[string]any{
			"entryId": l.EntryID, "number": l.Number, "date": l.Date, "memo": l.Memo,
			"debit": l.Debit, "credit": l.Credit, "balance": l.Balance,
		})
	}
	response.OK(w, "Buku besar", items)
}

// TaxSummary handles GET /api/v1/finance/reports/tax-summary?year=&month=.
func (h *Handler) TaxSummary(w http.ResponseWriter, r *http.Request) {
	year, month, appErr := yearMonth(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	sum, err := h.svc.TaxSummary(r.Context(), branchID(r), year, month)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Ringkasan pajak", sum)
}

// TaxDetail handles GET /api/v1/finance/reports/tax-detail?year=&month=
// (&format=csv for the SPT working-paper download).
func (h *Handler) TaxDetail(w http.ResponseWriter, r *http.Request) {
	year, month, appErr := yearMonth(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	rows, err := h.svc.TaxDetail(r.Context(), branchID(r), year, month)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	if r.URL.Query().Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="ppn-detail.csv"`)
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"tanggal", "nomor", "keterangan", "no_faktur", "tgl_faktur", "omzet", "ppn"})
		for _, row := range rows {
			_ = cw.Write([]string{row.Date, row.Number, row.Memo, row.TaxInvoiceNumber, row.TaxInvoiceDate,
				strconv.FormatInt(row.Revenue, 10), strconv.FormatInt(row.Ppn, 10)})
		}
		cw.Flush()
		return
	}
	items := make([]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"date": row.Date, "number": row.Number, "memo": row.Memo,
			"revenue": row.Revenue, "ppn": row.Ppn,
			"taxInvoiceNumber": row.TaxInvoiceNumber, "taxInvoiceDate": row.TaxInvoiceDate,
		})
	}
	response.OK(w, "Rincian PPN", items)
}

// InventoryValue handles GET /api/v1/finance/reports/inventory-value.
func (h *Handler) InventoryValue(w http.ResponseWriter, r *http.Request) {
	positions, total, err := h.svc.InventoryValue(r.Context(), branchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(positions))
	for _, p := range positions {
		items = append(items, map[string]any{
			"productId": p.ProductID, "variantId": p.VariantID,
			"productCode": p.ProductCode, "productName": p.ProductName,
			"qty": p.Qty, "avgCost": p.AvgCost, "value": p.Value,
		})
	}
	response.OK(w, "Nilai persediaan", map[string]any{"positions": items, "total": total})
}

// Margin handles GET /api/v1/finance/reports/margin?from=&to=.
func (h *Handler) Margin(w http.ResponseWriter, r *http.Request) {
	from, to, appErr := dateRange(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	m, err := h.svc.Margin(r.Context(), branchID(r), from, to)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Marjin", m)
}

// MarginByProduct handles GET /api/v1/finance/reports/margin-by-product
// ?from=&to=&limit=. Ranked by realised gross margin, richest first.
func (h *Handler) MarginByProduct(w http.ResponseWriter, r *http.Request) {
	from, to, appErr := dateRange(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	// limit is a display cap, not a page: the ranking is only meaningful over
	// the whole range, so it is applied after sorting, never in SQL.
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := h.svc.MarginByProduct(r.Context(), branchID(r), from, to, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Marjin per produk", rows)
}

// Receivables handles GET /api/v1/finance/reports/receivables?asOf=.
func (h *Handler) Receivables(w http.ResponseWriter, r *http.Request) {
	rows, total, err := h.svc.Receivables(r.Context(), branchID(r), r.URL.Query().Get("asOf"))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Piutang usaha", map[string]any{"parties": rows, "total": total})
}

// Payables handles GET /api/v1/finance/reports/payables?asOf=.
func (h *Handler) Payables(w http.ResponseWriter, r *http.Request) {
	rows, total, err := h.svc.Payables(r.Context(), branchID(r), r.URL.Query().Get("asOf"))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Hutang usaha", map[string]any{"parties": rows, "total": total})
}
