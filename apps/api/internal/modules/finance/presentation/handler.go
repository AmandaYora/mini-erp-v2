package presentation

import (
	"net/http"
	"strconv"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/finance/application"
	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/modules/finance/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
)

// Handler serves the finance module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires finance endpoints.
func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func actorID(r *http.Request) int64 {
	if s := authcontracts.SessionFromContext(r.Context()); s != nil {
		return s.UserID
	}
	return 0
}

func branchID(r *http.Request) int64 {
	if s := authcontracts.SessionFromContext(r.Context()); s != nil {
		return s.BranchID
	}
	return 0
}

func accountView(a *contracts.Account) map[string]any {
	return map[string]any{
		"id": a.ID, "code": a.Code, "name": a.Name,
		"type": a.Type, "isCash": a.IsCash, "status": a.Status,
	}
}

func entryView(e *contracts.JournalEntry, withLines bool) map[string]any {
	v := map[string]any{
		"id": e.ID, "number": e.Number, "branchId": e.BranchID,
		"date": e.Date, "memo": e.Memo, "sourceType": e.SourceType,
		"sourceId": e.SourceID, "status": e.Status, "reversedBy": e.ReversedBy,
	}
	if withLines {
		lines := make([]any, 0, len(e.Lines))
		for _, l := range e.Lines {
			lines = append(lines, map[string]any{
				"accountId": l.AccountID, "accountCode": l.AccountCode,
				"accountName": l.AccountName, "debit": l.Debit, "credit": l.Credit,
				"productId": l.ProductID, "variantId": l.VariantID,
				"partyId": l.PartyID, "description": l.Description,
			})
		}
		v["lines"] = lines
	}
	return v
}

// ListAccounts handles GET /api/v1/finance/accounts.
func (h *Handler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.svc.ListAccounts(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(accounts))
	for _, a := range accounts {
		items = append(items, accountView(a))
	}
	response.OK(w, "Data akun", items)
}

// CreateAccount handles POST /api/v1/finance/accounts.
func (h *Handler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code   string `json:"code" validate:"required"`
		Name   string `json:"name" validate:"required"`
		Type   string `json:"type" validate:"required"`
		IsCash bool   `json:"isCash"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	a, err := h.svc.CreateAccount(r.Context(), actorID(r), in.Code, in.Name, in.Type, in.IsCash)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Akun dibuat", accountView(a))
}

// UpdateAccount handles PUT /api/v1/finance/accounts/{id}.
func (h *Handler) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Akun"))
		return
	}
	var in struct {
		Name   string `json:"name" validate:"required"`
		Type   string `json:"type" validate:"required"`
		IsCash bool   `json:"isCash"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	a, appErr := h.svc.UpdateAccount(r.Context(), actorID(r), id, in.Name, in.Type, in.IsCash)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Akun disimpan", accountView(a))
}

// ArchiveAccount handles POST /api/v1/finance/accounts/{id}/archive.
func (h *Handler) ArchiveAccount(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Akun"))
		return
	}
	if appErr := h.svc.ArchiveAccount(r.Context(), actorID(r), id); appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Akun diarsipkan", nil)
}

// GetMappings handles GET /api/v1/finance/account-mappings.
func (h *Handler) GetMappings(w http.ResponseWriter, r *http.Request) {
	mappings, err := h.svc.GetMappings(r.Context())
	if err != nil {
		response.FailErr(w, err)
		return
	}
	out := map[string]any{}
	for k, a := range mappings {
		out[k] = accountView(a)
	}
	response.OK(w, "Pemetaan akun", out)
}

// SetMapping handles PUT /api/v1/finance/account-mappings.
func (h *Handler) SetMapping(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Key       string `json:"key" validate:"required"`
		AccountID int64  `json:"accountId" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	if err := h.svc.SetMapping(r.Context(), actorID(r), in.Key, in.AccountID); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Pemetaan disimpan", nil)
}

// ListPeriods handles GET /api/v1/finance/periods.
func (h *Handler) ListPeriods(w http.ResponseWriter, r *http.Request) {
	periods, err := h.svc.ListPeriods(r.Context(), branchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(periods))
	for _, p := range periods {
		items = append(items, map[string]any{
			"year": p.Year, "month": p.Month, "status": p.Status,
		})
	}
	response.OK(w, "Data periode", items)
}

func periodInput(r *http.Request) (int, int, *apperror.AppError) {
	var in struct {
		Year  int `json:"year" validate:"required"`
		Month int `json:"month" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		return 0, 0, apperror.Validation("", fields)
	}
	return in.Year, in.Month, nil
}

// ClosePeriod handles POST /api/v1/finance/periods/close.
func (h *Handler) ClosePeriod(w http.ResponseWriter, r *http.Request) {
	year, month, appErr := periodInput(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	if err := h.svc.ClosePeriod(r.Context(), actorID(r), branchID(r), year, month); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Periode ditutup", nil)
}

// ReopenPeriod handles POST /api/v1/finance/periods/reopen.
func (h *Handler) ReopenPeriod(w http.ResponseWriter, r *http.Request) {
	year, month, appErr := periodInput(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	if err := h.svc.ReopenPeriod(r.Context(), actorID(r), branchID(r), year, month); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Periode dibuka", nil)
}

// Preview handles GET /api/v1/finance/posting/preview?docType=&docId=.
func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	docID, err := strconv.ParseInt(q.Get("docId"), 10, 64)
	if err != nil || docID <= 0 || q.Get("docType") == "" {
		response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "docId", Message: "wajib diisi"}}))
		return
	}
	e, appErr := h.svc.Preview(r.Context(), branchID(r), q.Get("docType"), docID)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Pratinjau jurnal", entryView(e, true))
}

// Post handles POST /api/v1/finance/posting/post.
func (h *Handler) Post(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DocType string `json:"docType" validate:"required"`
		DocID   int64  `json:"docId" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	e, err := h.svc.Post(r.Context(), actorID(r), branchID(r), in.DocType, in.DocID)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Jurnal diposting", entryView(e, true))
}

// ListJournals handles GET /api/v1/finance/journals.
func (h *Handler) ListJournals(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	var accountID int64
	if raw := q.Get("accountId"); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			accountID = id
		}
	}
	res, err := h.svc.ListEntries(r.Context(), branchID(r), q.Get("from"), q.Get("to"), accountID, page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(res.Entries))
	for _, e := range res.Entries {
		items = append(items, entryView(e, false))
	}
	response.Paginated(w, "Data jurnal", items, response.Meta{
		Page: page, Limit: limit, Total: res.Total,
		TotalPages: pagination.TotalPages(res.Total, limit),
	})
}

// GetJournal handles GET /api/v1/finance/journals/{id}.
func (h *Handler) GetJournal(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Jurnal"))
		return
	}
	e, appErr := h.svc.GetEntry(r.Context(), branchID(r), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	if e == nil {
		response.Fail(w, apperror.NotFound("Jurnal"))
		return
	}
	response.OK(w, "Data jurnal", entryView(e, true))
}

// Reverse handles POST /api/v1/finance/journals/{id}/reverse.
func (h *Handler) Reverse(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Jurnal"))
		return
	}
	var in struct {
		Reason string `json:"reason" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	e, appErr := h.svc.Reverse(r.Context(), actorID(r), branchID(r), id, in.Reason)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Jurnal dibalik", entryView(e, true))
}

// Manual handles POST /api/v1/finance/journals/manual.
func (h *Handler) Manual(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Date  string `json:"date"`
		Memo  string `json:"memo" validate:"required"`
		Lines []struct {
			AccountCode string `json:"accountCode" validate:"required"`
			Debit       int64  `json:"debit"`
			Credit      int64  `json:"credit"`
		} `json:"lines" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	lines := make([]application.ManualLine, 0, len(in.Lines))
	for _, l := range in.Lines {
		lines = append(lines, application.ManualLine{
			AccountCode: l.AccountCode, Debit: l.Debit, Credit: l.Credit,
		})
	}
	e, err := h.svc.Manual(r.Context(), actorID(r), branchID(r), in.Date, in.Memo, lines)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Jurnal manual dicatat", entryView(e, true))
}

// CreateExpense handles POST /api/v1/finance/expenses.
func (h *Handler) CreateExpense(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ExpenseAccountID int64  `json:"expenseAccountId" validate:"required"`
		PayAccountID     int64  `json:"payAccountId" validate:"required"`
		Amount           int64  `json:"amount" validate:"required"`
		Date             string `json:"date"`
		Notes            string `json:"notes"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	e, err := h.svc.CreateExpense(r.Context(), actorID(r), branchID(r), application.CreateExpenseInput{
		ExpenseAccountID: in.ExpenseAccountID, PayAccountID: in.PayAccountID,
		Amount: in.Amount, Date: in.Date, Notes: in.Notes,
	})
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Biaya dicatat", expenseView(e))
}

func expenseView(e *infrastructure.Expense) map[string]any {
	return map[string]any{
		"id": e.ID, "number": e.Number, "branchId": e.BranchID,
		"expenseAccountId": e.ExpenseAccount, "payAccountId": e.PayAccount,
		"amount": e.Amount, "date": e.Date, "notes": e.Notes,
		"journalEntryId": e.JournalID, "status": e.Status,
	}
}

// ListExpenses handles GET /api/v1/finance/expenses.
func (h *Handler) ListExpenses(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	res, err := h.svc.ListExpenses(r.Context(), branchID(r), page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(res.Expenses))
	for _, e := range res.Expenses {
		items = append(items, expenseView(e))
	}
	response.Paginated(w, "Data biaya", items, response.Meta{
		Page: page, Limit: limit, Total: res.Total,
		TotalPages: pagination.TotalPages(res.Total, limit),
	})
}

// GetExpense handles GET /api/v1/finance/expenses/{id}.
func (h *Handler) GetExpense(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Biaya"))
		return
	}
	e, appErr := h.svc.GetExpense(r.Context(), branchID(r), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	if e == nil {
		response.Fail(w, apperror.NotFound("Biaya"))
		return
	}
	response.OK(w, "Data biaya", expenseView(e))
}

// CancelExpense handles POST /api/v1/finance/expenses/{id}/cancel.
func (h *Handler) CancelExpense(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Biaya"))
		return
	}
	e, appErr := h.svc.CancelExpense(r.Context(), actorID(r), branchID(r), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Biaya dibatalkan", expenseView(e))
}

// CostingSync handles POST /api/v1/finance/costing/sync.
func (h *Handler) CostingSync(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.Sync(r.Context(), branchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Sinkronisasi biaya selesai", res)
}

// Uncosted handles GET /api/v1/finance/costing/uncosted.
func (h *Handler) Uncosted(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.Uncosted(r.Context(), branchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	out := make([]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{
			"productId": it.ProductID, "variantId": it.VariantID,
			"productCode": it.ProductCode, "productName": it.ProductName,
		})
	}
	response.OK(w, "Daftar belum berbiaya", out)
}

// CloseTaxPeriod handles POST /api/v1/finance/tax-periods/close.
func (h *Handler) CloseTaxPeriod(w http.ResponseWriter, r *http.Request) {
	year, month, appErr := periodInput(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	if err := h.svc.CloseTaxPeriod(r.Context(), actorID(r), branchID(r), year, month); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Periode pajak ditutup", nil)
}

// ReopenTaxPeriod handles POST /api/v1/finance/tax-periods/reopen.
func (h *Handler) ReopenTaxPeriod(w http.ResponseWriter, r *http.Request) {
	year, month, appErr := periodInput(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	if err := h.svc.ReopenTaxPeriod(r.Context(), actorID(r), branchID(r), year, month); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Periode pajak dibuka", nil)
}

// ListTaxPeriods handles GET /api/v1/finance/tax-periods.
func (h *Handler) ListTaxPeriods(w http.ResponseWriter, r *http.Request) {
	periods, err := h.svc.ListTaxPeriods(r.Context(), branchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(periods))
	for _, p := range periods {
		items = append(items, map[string]any{
			"year": p.Year, "month": p.Month, "status": p.Status,
		})
	}
	response.OK(w, "Data periode pajak", items)
}

// Permission is injected by the composition root (built from the auth module).
type Permission func(perm string, next func(http.ResponseWriter, *http.Request)) http.Handler

// guarded adapts the branch guard into func-style middleware.
func guarded(next func(http.ResponseWriter, *http.Request), branchGuard func(http.Handler) http.Handler) func(http.ResponseWriter, *http.Request) {
	return branchGuard(http.HandlerFunc(next)).ServeHTTP
}

// RegisterRoutes mounts finance endpoints with explicit permissions.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/finance/accounts", perm("finance.view", guarded(h.ListAccounts, branchGuard)))
	mux.Handle("POST /api/v1/finance/accounts", perm("finance.manage", guarded(h.CreateAccount, branchGuard)))
	mux.Handle("PUT /api/v1/finance/accounts/{id}", perm("finance.manage", guarded(h.UpdateAccount, branchGuard)))
	mux.Handle("POST /api/v1/finance/accounts/{id}/archive", perm("finance.manage", guarded(h.ArchiveAccount, branchGuard)))
	mux.Handle("GET /api/v1/finance/account-mappings", perm("finance.view", guarded(h.GetMappings, branchGuard)))
	mux.Handle("PUT /api/v1/finance/account-mappings", perm("finance.manage", guarded(h.SetMapping, branchGuard)))
	mux.Handle("GET /api/v1/finance/periods", perm("finance.view", guarded(h.ListPeriods, branchGuard)))
	mux.Handle("POST /api/v1/finance/periods/close", perm("finance.manage", guarded(h.ClosePeriod, branchGuard)))
	mux.Handle("POST /api/v1/finance/periods/reopen", perm("finance.manage", guarded(h.ReopenPeriod, branchGuard)))
	mux.Handle("GET /api/v1/finance/posting/preview", perm("finance.post", guarded(h.Preview, branchGuard)))
	mux.Handle("POST /api/v1/finance/posting/post", perm("finance.post", guarded(h.Post, branchGuard)))
	mux.Handle("GET /api/v1/finance/journals", perm("finance.view", guarded(h.ListJournals, branchGuard)))
	mux.Handle("POST /api/v1/finance/journals/manual", perm("finance.manage", guarded(h.Manual, branchGuard)))
	mux.Handle("GET /api/v1/finance/journals/{id}", perm("finance.view", guarded(h.GetJournal, branchGuard)))
	mux.Handle("POST /api/v1/finance/journals/{id}/reverse", perm("finance.manage", guarded(h.Reverse, branchGuard)))
	mux.Handle("GET /api/v1/finance/expenses", perm("finance.view", guarded(h.ListExpenses, branchGuard)))
	mux.Handle("POST /api/v1/finance/expenses", perm("finance.post", guarded(h.CreateExpense, branchGuard)))
	mux.Handle("GET /api/v1/finance/expenses/{id}", perm("finance.view", guarded(h.GetExpense, branchGuard)))
	mux.Handle("POST /api/v1/finance/expenses/{id}/cancel", perm("finance.post", guarded(h.CancelExpense, branchGuard)))
	mux.Handle("POST /api/v1/finance/costing/sync", perm("finance.post", guarded(h.CostingSync, branchGuard)))
	mux.Handle("GET /api/v1/finance/costing/uncosted", perm("finance.view", guarded(h.Uncosted, branchGuard)))
	mux.Handle("GET /api/v1/finance/tax-periods", perm("finance.view", guarded(h.ListTaxPeriods, branchGuard)))
	mux.Handle("POST /api/v1/finance/tax-periods/close", perm("finance.manage", guarded(h.CloseTaxPeriod, branchGuard)))
	mux.Handle("POST /api/v1/finance/tax-periods/reopen", perm("finance.manage", guarded(h.ReopenTaxPeriod, branchGuard)))
	mux.Handle("GET /api/v1/finance/reports/trial-balance", perm("finance.view", guarded(h.TrialBalance, branchGuard)))
	mux.Handle("GET /api/v1/finance/reports/profit-loss", perm("finance.view", guarded(h.ProfitLoss, branchGuard)))
	mux.Handle("GET /api/v1/finance/reports/balance-sheet", perm("finance.view", guarded(h.BalanceSheet, branchGuard)))
	mux.Handle("GET /api/v1/finance/reports/general-ledger", perm("finance.view", guarded(h.GeneralLedger, branchGuard)))
	mux.Handle("GET /api/v1/finance/reports/tax-summary", perm("finance.view", guarded(h.TaxSummary, branchGuard)))
	mux.Handle("GET /api/v1/finance/reports/tax-detail", perm("finance.view", guarded(h.TaxDetail, branchGuard)))
	mux.Handle("GET /api/v1/finance/reports/inventory-value", perm("finance.view", guarded(h.InventoryValue, branchGuard)))
	mux.Handle("GET /api/v1/finance/reports/margin", perm("finance.view", guarded(h.Margin, branchGuard)))
	mux.Handle("GET /api/v1/finance/reports/margin-by-product", perm("finance.view", guarded(h.MarginByProduct, branchGuard)))
	mux.Handle("GET /api/v1/finance/reports/receivables", perm("finance.view", guarded(h.Receivables, branchGuard)))
	mux.Handle("GET /api/v1/finance/reports/payables", perm("finance.view", guarded(h.Payables, branchGuard)))
	mux.Handle("GET /api/v1/finance/reports/cash-summary", perm("finance.view", guarded(h.CashSummary, branchGuard)))
	mux.Handle("GET /api/v1/finance/reports/fiscal-summary", perm("finance.view", guarded(h.FiscalSummary, branchGuard)))
	mux.Handle("GET /api/v1/finance/opening", perm("finance.view", guarded(h.OpeningStatus, branchGuard)))
	mux.Handle("POST /api/v1/finance/opening/preview", perm("finance.manage", guarded(h.PreviewOpening, branchGuard)))
	mux.Handle("POST /api/v1/finance/opening/post", perm("finance.manage", guarded(h.PostOpening, branchGuard)))
	mux.Handle("GET /api/v1/finance/posting-sources", perm("finance.post", guarded(h.PostingSources, branchGuard)))
	mux.Handle("POST /api/v1/finance/posting/post-batch", perm("finance.post", guarded(h.PostBatch, branchGuard)))
	mux.Handle("POST /api/v1/finance/posting/close-day", perm("finance.post", guarded(h.CloseDay, branchGuard)))
	mux.Handle("GET /api/v1/finance/tax-adjustments", perm("finance.view", guarded(h.ListTaxAdjustments, branchGuard)))
	mux.Handle("POST /api/v1/finance/tax-adjustments", perm("finance.manage", guarded(h.CreateTaxAdjustment, branchGuard)))
	mux.Handle("GET /api/v1/finance/close/readiness", perm("finance.view", guarded(h.Readiness, branchGuard)))
	mux.Handle("POST /api/v1/finance/close/safe-close", perm("finance.manage", guarded(h.SafeClose, branchGuard)))
	mux.Handle("GET /api/v1/finance/export/tax-package", perm("finance.view", guarded(h.TaxPackage, branchGuard)))
}
