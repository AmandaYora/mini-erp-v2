package presentation

import (
	"fmt"
	"net/http"
	"strings"

	"mini-erp/internal/modules/finance/application"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/response"
)

// Closing, opening, queue, and worksheet handlers (G/H-revisi): derived
// state and typed journals, no new tables. Bodies mirror the Manual handler
// shape so clients reuse one leg editor.

// legInput is one account leg by code (opening + tax adjustments share it).
type legInput struct {
	AccountCode string `json:"accountCode" validate:"required"`
	Debit       int64  `json:"debit"`
	Credit      int64  `json:"credit"`
}

func toManualLines(raw []legInput) []application.ManualLine {
	lines := make([]application.ManualLine, 0, len(raw))
	for _, l := range raw {
		lines = append(lines, application.ManualLine{
			AccountCode: l.AccountCode, Debit: l.Debit, Credit: l.Credit,
		})
	}
	return lines
}

// OpeningStatus handles GET /api/v1/finance/opening: the posted cutover
// journal, or null when the branch has not cut over yet.
func (h *Handler) OpeningStatus(w http.ResponseWriter, r *http.Request) {
	e, err := h.svc.OpeningStatus(r.Context(), branchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	if e == nil {
		response.OK(w, "Belum ada saldo awal", nil)
		return
	}
	response.OK(w, "Saldo awal", entryView(e, true))
}

// PreviewOpening handles POST /api/v1/finance/opening/preview.
func (h *Handler) PreviewOpening(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Date  string     `json:"date"`
		Memo  string     `json:"memo"`
		Lines []legInput `json:"lines" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	e, err := h.svc.PreviewOpening(r.Context(), branchID(r), in.Date, in.Memo, toManualLines(in.Lines))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Pratinjau saldo awal", entryView(e, true))
}

// PostOpening handles POST /api/v1/finance/opening/post (idempotent: a
// second call returns the existing cutover journal).
func (h *Handler) PostOpening(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Date  string     `json:"date"`
		Memo  string     `json:"memo"`
		Lines []legInput `json:"lines" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	e, err := h.svc.PostOpening(r.Context(), actorID(r), branchID(r), in.Date, in.Memo, toManualLines(in.Lines))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Saldo awal diposting", entryView(e, true))
}

// PostingSources handles GET /api/v1/finance/posting-sources: confirmed
// but unjournaled documents, derived live (no queue table to sync).
func (h *Handler) PostingSources(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.UnpostedSources(r.Context(), branchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Dokumen belum posting", items)
}

// PostBatch handles POST /api/v1/finance/posting/post-batch.
func (h *Handler) PostBatch(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Items []struct {
			DocType string `json:"docType" validate:"required"`
			DocID   int64  `json:"docId" validate:"required"`
		} `json:"items" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	items := make([]application.BatchItem, 0, len(in.Items))
	for _, it := range in.Items {
		items = append(items, application.BatchItem{DocType: it.DocType, DocID: it.DocID})
	}
	res, err := h.svc.PostBatch(r.Context(), actorID(r), branchID(r), items)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	ok := 0
	for _, x := range res {
		if x.Error == "" {
			ok++
		}
	}
	response.OK(w, fmt.Sprintf("Diposting %d dari %d dokumen", ok, len(res)), res)
}

// CloseDay handles POST /api/v1/finance/posting/close-day: post everything
// queued on or before the date.
func (h *Handler) CloseDay(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Date string `json:"date" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	res, err := h.svc.CloseDay(r.Context(), actorID(r), branchID(r), in.Date)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, fmt.Sprintf("Tutup harian: %d dokumen diposting", len(res)), res)
}

// Readiness handles GET /api/v1/finance/close/readiness?year=&month=.
func (h *Handler) Readiness(w http.ResponseWriter, r *http.Request) {
	year, month, appErr := yearMonth(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	res, err := h.svc.Readiness(r.Context(), branchID(r), year, month)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Kesiapan tutup periode", res)
}

// SafeClose handles POST /api/v1/finance/close/safe-close: seal the month
// only when every gate passes.
func (h *Handler) SafeClose(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Year  int `json:"year" validate:"required"`
		Month int `json:"month" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	if err := h.svc.SafeClose(r.Context(), actorID(r), branchID(r), in.Year, in.Month); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Periode ditutup", nil)
}

// CreateTaxAdjustment handles POST /api/v1/finance/tax-adjustments.
func (h *Handler) CreateTaxAdjustment(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Date  string     `json:"date"`
		Memo  string     `json:"memo" validate:"required"`
		Lines []legInput `json:"lines" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	e, err := h.svc.CreateTaxAdjustment(r.Context(), actorID(r), branchID(r), in.Date, in.Memo, toManualLines(in.Lines))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Koreksi fiskal dicatat", entryView(e, true))
}

// ListTaxAdjustments handles GET /api/v1/finance/tax-adjustments.
func (h *Handler) ListTaxAdjustments(w http.ResponseWriter, r *http.Request) {
	entries, err := h.svc.ListTaxAdjustments(r.Context(), branchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(entries))
	for _, e := range entries {
		items = append(items, entryView(e, true))
	}
	response.OK(w, "Koreksi fiskal", items)
}

// FiscalSummary handles GET /api/v1/finance/reports/fiscal-summary?from=&to=.
func (h *Handler) FiscalSummary(w http.ResponseWriter, r *http.Request) {
	from, to, appErr := dateRange(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	res, err := h.svc.FiscalSummary(r.Context(), branchID(r), from, to)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Rekonsiliasi fiskal", res)
}

// CashSummary handles GET /api/v1/finance/reports/cash-summary?year=&month=.
func (h *Handler) CashSummary(w http.ResponseWriter, r *http.Request) {
	year, month, appErr := yearMonth(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	rows, total, err := h.svc.CashSummary(r.Context(), branchID(r), year, month)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Ringkas kas", map[string]any{"rows": rows, "total": total})
}

// TaxPackage handles
// GET /api/v1/finance/export/tax-package?year=&month=&variant=.
//
// variant selects WHICH working paper is produced (default: the actual
// books). The parameter is an application-level switch — the produced file
// names itself by its content, never by the switch.
func (h *Handler) TaxPackage(w http.ResponseWriter, r *http.Request) {
	year, month, appErr := yearMonth(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	variant := strings.TrimSpace(r.URL.Query().Get("variant"))
	if variant == "" {
		variant = application.PackageActual
	}
	raw, name, err := h.svc.TaxPackage(r.Context(), branchID(r), year, month, variant)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	w.Header().Set("Content-Type",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}
