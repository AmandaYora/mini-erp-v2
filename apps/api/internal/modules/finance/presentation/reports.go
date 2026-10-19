package presentation

import (
	"encoding/csv"
	"net/http"
	"strconv"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/finance/application"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
)

// maxDisplayLimit bounds ?limit= display caps (top-N), never a page size.
const maxDisplayLimit = 1000

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
	ctx, appErr := h.branchScope(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	var rows []application.TrialRow
	var err error
	if asOf := r.URL.Query().Get("asOf"); asOf != "" {
		rows, err = h.svc.TrialBalanceAsOf(ctx, authcontracts.BranchID(r), asOf)
	} else {
		year, month, appErr := yearMonth(r)
		if appErr != nil {
			response.Fail(w, appErr)
			return
		}
		rows, err = h.svc.TrialBalance(ctx, authcontracts.BranchID(r), year, month)
	}
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
	ctx, scopeErr := h.branchScope(r)
	if scopeErr != nil {
		response.Fail(w, scopeErr)
		return
	}
	from, to, appErr := dateRange(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	rep, err := h.svc.ProfitLoss(ctx, authcontracts.BranchID(r), from, to)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Laba rugi", rep)
}

// BalanceSheet handles GET /api/v1/finance/reports/balance-sheet?date=.
func (h *Handler) BalanceSheet(w http.ResponseWriter, r *http.Request) {
	ctx, scopeErr := h.branchScope(r)
	if scopeErr != nil {
		response.Fail(w, scopeErr)
		return
	}
	date := r.URL.Query().Get("date")
	if date == "" {
		response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "date", Message: "wajib diisi"}}))
		return
	}
	bs, err := h.svc.BalanceSheet(ctx, authcontracts.BranchID(r), date)
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
	ctx, appErr := h.branchScope(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	if q.Get("account") == "" {
		// No account picked (FN-08): the ledger of every account — opening
		// balance, movement, closing — paginated, matching the trial balance.
		page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
		ov, err := h.svc.GeneralLedgerOverview(ctx, authcontracts.BranchID(r), from, to, page, limit)
		if err != nil {
			response.FailErr(w, err)
			return
		}
		response.Paginated(w, "Buku besar semua akun", map[string]any{
			"rows": ov.Rows, "totalDebit": ov.TotalDebit, "totalCredit": ov.TotalCredit,
		}, response.Meta{Page: page, Limit: limit, Total: ov.Total, TotalPages: pagination.TotalPages(ov.Total, limit)})
		return
	}
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 1000
	}
	if limit > 5000 {
		limit = 5000
	}
	if page <= 0 {
		page = 1
	}
	res, err := h.svc.GeneralLedgerPage(ctx, authcontracts.BranchID(r), q.Get("account"), from, to, page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(res.Lines)+1)
	if res.Opening != 0 && page == 1 {
		items = append(items, map[string]any{"entryId": 0, "number": "", "date": from, "memo": "Saldo awal",
			"debit": 0, "credit": 0, "balance": res.Opening})
	}
	for _, l := range res.Lines {
		items = append(items, map[string]any{
			"entryId": l.EntryID, "number": l.Number, "date": l.Date, "memo": l.Memo,
			"debit": l.Debit, "credit": l.Credit, "balance": l.Balance,
			"partyId": l.PartyID, "partyName": l.PartyName, "productId": l.ProductID, "productName": l.ProductName,
			"description": l.Description,
		})
	}
	totalPages := int((res.Total + int64(limit) - 1) / int64(limit))
	response.Paginated(w, "Buku besar", items, response.Meta{Page: page, Limit: limit, Total: res.Total, TotalPages: totalPages})
}

// TaxSummary handles GET /api/v1/finance/reports/tax-summary?year=&month=.
func (h *Handler) TaxSummary(w http.ResponseWriter, r *http.Request) {
	ctx, scopeErr := h.branchScope(r)
	if scopeErr != nil {
		response.Fail(w, scopeErr)
		return
	}
	year, month, appErr := yearMonth(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	// The exceptions list (taxable documents without an invoice number) is
	// paginated; the totals are always the whole month.
	page, limit := pagination.Parse(r.URL.Query().Get("page"), r.URL.Query().Get("limit"))
	sum, err := h.svc.TaxSummaryPage(ctx, authcontracts.BranchID(r), year, month, page, limit)
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
	ctx, scopeErr := h.branchScope(r)
	if scopeErr != nil {
		response.Fail(w, scopeErr)
		return
	}
	if r.URL.Query().Get("format") != "csv" {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		rows, total, err := h.svc.TaxDetailReport(ctx, authcontracts.BranchID(r), year, month, page, limit)
		if err != nil {
			response.FailErr(w, err)
			return
		}
		if limit < 1 || limit > 2000 {
			limit = 500
		}
		if page < 1 {
			page = 1
		}
		items := make([]any, 0, len(rows))
		for _, row := range rows {
			items = append(items, map[string]any{
				"entryId": row.EntryID, "orderId": row.OrderID, "orderNumber": row.OrderNumber, "partyName": row.PartyName,
				"date": row.Date, "number": row.Number, "memo": row.Memo, "kind": row.Kind,
				"revenue": row.Revenue, "ppn": row.Ppn, "ppnIn": row.PpnIn, "dpp": row.Dpp,
				"taxInvoiceNumber": row.TaxInvoiceNumber, "taxInvoiceDate": row.TaxInvoiceDate,
				"missingInvoice": row.MissingInvoice,
			})
		}
		response.Paginated(w, "Rincian PPN", items, response.Meta{Page: page, Limit: limit, Total: total,
			TotalPages: int((total + int64(limit) - 1) / int64(limit))})
		return
	}
	rows, err := h.svc.TaxDetail(ctx, authcontracts.BranchID(r), year, month)
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
	// ?branch=all|<id> (finance.report.all_branches) values several branches and
	// returns the per-branch split (FN-09).
	ctx, scopeErr := h.branchScope(r)
	if scopeErr != nil {
		response.Fail(w, scopeErr)
		return
	}
	rep, err := h.svc.InventoryReport(ctx, authcontracts.BranchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	positions, total := rep.Positions, rep.Total
	items := make([]any, 0, len(positions))
	for _, p := range positions {
		items = append(items, map[string]any{
			"productId": p.ProductID, "variantId": p.VariantID,
			"productCode": p.ProductCode, "productName": p.ProductName,
			"qty": p.Qty, "avgCost": p.AvgCost, "value": p.Value,
		})
	}
	byBranch := make([]any, 0, len(rep.ByBranch))
	for _, bv := range rep.ByBranch {
		byBranch = append(byBranch, bv)
	}
	response.OK(w, "Nilai persediaan", map[string]any{"positions": items, "total": total, "byBranch": byBranch})
}

// Margin handles GET /api/v1/finance/reports/margin?from=&to=.
func (h *Handler) Margin(w http.ResponseWriter, r *http.Request) {
	ctx, scopeErr := h.branchScope(r)
	if scopeErr != nil {
		response.Fail(w, scopeErr)
		return
	}
	from, to, appErr := dateRange(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	m, err := h.svc.Margin(ctx, authcontracts.BranchID(r), from, to)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Marjin", m)
}

// MarginByProduct handles GET /api/v1/finance/reports/margin-by-product
// ?from=&to=&limit=. Ranked by realised gross margin, richest first.
func (h *Handler) MarginByProduct(w http.ResponseWriter, r *http.Request) {
	ctx, scopeErr := h.branchScope(r)
	if scopeErr != nil {
		response.Fail(w, scopeErr)
		return
	}
	from, to, appErr := dateRange(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	// limit is a display cap, not a page: the ranking is only meaningful over
	// the whole range, so it is applied after sorting, never in SQL.
	limit := pagination.ParseDisplayLimit(r.URL.Query().Get("limit"), 0, maxDisplayLimit)
	rows, err := h.svc.MarginByProduct(ctx, authcontracts.BranchID(r), from, to, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Marjin per produk", rows)
}

// agingDocuments serves the per-document aging view (?view=documents).
func (h *Handler) agingDocuments(w http.ResponseWriter, r *http.Request, kind string) {
	rep, err := h.svc.AgingDocuments(r.Context(), authcontracts.BranchID(r), kind, r.URL.Query().Get("asOf"))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Umur dokumen", rep)
}

// Receivables handles GET /api/v1/finance/reports/receivables?asOf=.
func (h *Handler) Receivables(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("view") == "documents" {
		h.agingDocuments(w, r, "receivable")
		return
	}
	ctx, scopeErr := h.branchScope(r)
	if scopeErr != nil {
		response.Fail(w, scopeErr)
		return
	}
	rows, total, err := h.svc.Receivables(ctx, authcontracts.BranchID(r), r.URL.Query().Get("asOf"))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Piutang usaha", map[string]any{"parties": rows, "total": total})
}

// Payables handles GET /api/v1/finance/reports/payables?asOf=.
func (h *Handler) Payables(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("view") == "documents" {
		h.agingDocuments(w, r, "payable")
		return
	}
	ctx, scopeErr := h.branchScope(r)
	if scopeErr != nil {
		response.Fail(w, scopeErr)
		return
	}
	rows, total, err := h.svc.Payables(ctx, authcontracts.BranchID(r), r.URL.Query().Get("asOf"))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Hutang usaha", map[string]any{"parties": rows, "total": total})
}
