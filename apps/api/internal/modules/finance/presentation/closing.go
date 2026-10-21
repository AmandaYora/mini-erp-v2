package presentation

import (
	"fmt"
	authcontracts "mini-erp/internal/modules/auth/contracts"
	"net/http"
	"strings"

	"mini-erp/internal/modules/finance/application"
	mediacontracts "mini-erp/internal/modules/media/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/response"
)

// Closing, opening, queue, and worksheet handlers (G/H-revisi): derived
// state and typed journals, no new tables. Bodies mirror the Manual handler
// shape so clients reuse one leg editor.

// legInput is one account leg by code (manual, opening and tax adjustments
// share it). partyId names the customer/supplier of a Piutang/Hutang/Uang Muka
// leg; the service decides where it is required (F5).
type legInput struct {
	AccountCode string `json:"accountCode" validate:"required"`
	Debit       int64  `json:"debit"`
	Credit      int64  `json:"credit"`
	PartyID     int64  `json:"partyId"`
}

func toManualLines(raw []legInput) []application.ManualLine {
	lines := make([]application.ManualLine, 0, len(raw))
	for _, l := range raw {
		lines = append(lines, application.ManualLine{
			AccountCode: l.AccountCode, Debit: l.Debit, Credit: l.Credit, PartyID: l.PartyID,
		})
	}
	return lines
}

// OpeningStatus handles GET /api/v1/finance/opening: the posted cutover
// journal, or null when the branch has not cut over yet.
func (h *Handler) OpeningStatus(w http.ResponseWriter, r *http.Request) {
	e, err := h.svc.OpeningStatus(r.Context(), authcontracts.BranchID(r))
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
	e, err := h.svc.PreviewOpening(r.Context(), authcontracts.BranchID(r), in.Date, in.Memo, toManualLines(in.Lines))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	view := entryView(e, true)
	if inv, err := h.svc.OpeningInventory(r.Context(), authcontracts.BranchID(r)); err == nil {
		view["inventory"] = inv
	}
	response.OK(w, "Pratinjau saldo awal", view)
}

// OpeningInventory handles GET /api/v1/finance/opening/inventory: the
// server-computed Persediaan total for the cutover, the products held without
// a cost, and whether the opening is already posted ("lengkapi" is then open).
func (h *Handler) OpeningInventory(w http.ResponseWriter, r *http.Request) {
	inv, err := h.svc.OpeningInventory(r.Context(), authcontracts.BranchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Persediaan saldo awal", inv)
}

// CompleteOpening handles POST /api/v1/finance/opening/complete: price
// held-but-uncosted positions after the opening is locked.
func (h *Handler) CompleteOpening(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Items []struct {
			ProductID int64 `json:"productId" validate:"required"`
			VariantID int64 `json:"variantId"`
			UnitCost  int64 `json:"unitCost"`
		} `json:"items" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	items := make([]application.CompleteItem, 0, len(in.Items))
	for _, it := range in.Items {
		items = append(items, application.CompleteItem{ProductID: it.ProductID, VariantID: it.VariantID, UnitCost: it.UnitCost})
	}
	res, err := h.svc.CompleteOpening(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), items)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	view := entryView(res.Entry, true)
	view["total"] = res.Total
	response.Created(w, "Harga modal saldo awal dilengkapi", view)
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
	e, err := h.svc.PostOpening(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), in.Date, in.Memo, toManualLines(in.Lines))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Saldo awal diposting", entryView(e, true))
}

// PostingSources handles GET /api/v1/finance/posting-sources: confirmed
// but unjournaled documents, derived live (no queue table to sync).
func (h *Handler) PostingSources(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.UnpostedSources(r.Context(), authcontracts.BranchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Dokumen belum posting", items)
}

// Cutover handles GET /api/v1/finance/cutover: the per-document-type boundary
// below which documents count as already journalled.
func (h *Handler) Cutover(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.GetCutovers(r.Context(), authcontracts.BranchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Batas cutover", c)
}

// SetCutover handles PUT /api/v1/finance/cutover.
func (h *Handler) SetCutover(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DocType          string `json:"docType" validate:"required"`
		CoveredThroughID int64  `json:"coveredThroughId"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	if err := h.svc.SetCutover(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), in.DocType, in.CoveredThroughID); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Batas cutover disimpan", nil)
}

// Integrity handles GET /api/v1/finance/integrity: rollups vs journal lines.
func (h *Handler) Integrity(w http.ResponseWriter, r *http.Request) {
	rep, err := h.svc.VerifyRollups(r.Context(), authcontracts.BranchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Pemeriksaan integritas", rep)
}

// RebuildRollups handles POST /api/v1/finance/integrity/rebuild.
func (h *Handler) RebuildRollups(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RebuildRollups(r.Context(), authcontracts.BranchID(r)); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Rollup dihitung ulang", nil)
}

// Orphans handles GET /api/v1/finance/posting/orphans.
func (h *Handler) Orphans(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.Orphans(r.Context(), authcontracts.BranchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Jurnal dokumen batal", items)
}

// PostBatch handles POST /api/v1/finance/posting/post-batch.
func (h *Handler) PostBatch(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Items []struct {
			DocType string `json:"docType" validate:"required"`
			DocID   int64  `json:"docId" validate:"required"`
			Action  string `json:"action"`
		} `json:"items" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	items := make([]application.BatchItem, 0, len(in.Items))
	for _, it := range in.Items {
		if it.Action == "catchup" {
			// Journalling into a period other than the document's own is a
			// deliberate override of the close.
			if sess := authcontracts.SessionFromContext(r.Context()); sess == nil || !sess.HasPermission("finance.close.force") {
				response.Fail(w, apperror.Forbidden())
				return
			}
		}
		items = append(items, application.BatchItem{DocType: it.DocType, DocID: it.DocID, Action: it.Action})
	}
	res, err := h.svc.PostBatch(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), items)
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

// IgnorePosting handles POST /api/v1/finance/posting/ignore: take one stuck
// document out of the blocking queue with a mandatory reason (FN-01).
func (h *Handler) IgnorePosting(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DocType string `json:"docType" validate:"required"`
		DocID   int64  `json:"docId" validate:"required"`
		Reason  string `json:"reason"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	item, err := h.svc.IgnoreSource(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), in.DocType, in.DocID, in.Reason)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Dokumen diabaikan dari antrian posting", item)
}

// RestorePosting handles POST /api/v1/finance/posting/restore.
func (h *Handler) RestorePosting(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DocType string `json:"docType" validate:"required"`
		DocID   int64  `json:"docId" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	if err := h.svc.RestoreSource(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), in.DocType, in.DocID); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Dokumen dikembalikan ke antrian posting", nil)
}

// DailyOverview handles GET /api/v1/finance/posting/overview?date=: the state of
// the daily-close screen (FN-02), derived from the books and the queue.
func (h *Handler) DailyOverview(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.DailyCloseOverview(r.Context(), authcontracts.BranchID(r), r.URL.Query().Get("date"))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Ikhtisar tutup hari", res)
}

// CloseDay handles POST /api/v1/finance/posting/close-day: post everything
// queued on or before the date, in rounds, and report what is still queued.
func (h *Handler) CloseDay(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Date string `json:"date" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	res, err := h.svc.CloseDay(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), in.Date)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	msg := fmt.Sprintf("Tutup harian: %d dokumen diposting", res.Posted)
	if res.Failed > 0 {
		msg += fmt.Sprintf(", %d gagal", res.Failed)
	}
	if res.Remaining > 0 {
		msg += fmt.Sprintf(", %d tersisa di antrean — jalankan lagi atau periksa yang gagal", res.Remaining)
	}
	response.OK(w, msg, res)
}

// Readiness handles GET /api/v1/finance/close/readiness?year=&month=.
func (h *Handler) Readiness(w http.ResponseWriter, r *http.Request) {
	year, month, appErr := yearMonth(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	res, err := h.svc.Readiness(r.Context(), authcontracts.BranchID(r), year, month)
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
	if err := h.svc.SafeClose(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), in.Year, in.Month); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Periode ditutup", nil)
}

// CreateTaxAdjustment handles POST /api/v1/finance/tax-adjustments.
func (h *Handler) CreateTaxAdjustment(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Date           string     `json:"date"`
		Memo           string     `json:"memo" validate:"required"`
		Lines          []legInput `json:"lines" validate:"required"`
		Classification string     `json:"classification"`
		Note           string     `json:"note"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	e, err := h.svc.CreateTaxAdjustmentWith(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), in.Date, in.Memo,
		toManualLines(in.Lines), application.TaxAdjustInput{Classification: in.Classification, Note: in.Note})
	if err != nil {
		response.FailErr(w, err)
		return
	}
	view := entryView(e, true)
	view["classification"] = in.Classification
	view["classLabel"] = application.TaxClassLabel(in.Classification)
	view["note"] = strings.TrimSpace(in.Note)
	response.Created(w, "Koreksi fiskal dicatat", view)
}

// ListTaxAdjustments handles GET /api/v1/finance/tax-adjustments.
func (h *Handler) ListTaxAdjustments(w http.ResponseWriter, r *http.Request) {
	entries, err := h.svc.ListTaxAdjustments(r.Context(), authcontracts.BranchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(entries))
	for _, e := range entries {
		view := entryView(e.Entry, true)
		view["classification"] = e.Classification
		view["classLabel"] = e.ClassLabel
		view["note"] = e.Note
		items = append(items, view)
	}
	response.OK(w, "Koreksi fiskal", items)
}

// AttachTaxAdjustment handles POST /api/v1/finance/tax-adjustments/{id}/attachments
// (multipart image): a supporting scan or photo of a correction (FN-07).
func (h *Handler) AttachTaxAdjustment(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Koreksi fiskal"))
		return
	}
	fh, header, appErr := httpx.MultipartFile(r, 6<<20)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	defer func() { _ = fh.Close() }()
	f, svcErr := h.svc.AttachTaxAdjustment(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id, mediacontracts.Upload{
		OriginalName: header.Filename, MIME: header.Header.Get("Content-Type"), Content: fh, Accept: []string{"image/"},
	})
	if svcErr != nil {
		response.FailErr(w, svcErr)
		return
	}
	response.Created(w, "Lampiran diunggah", f)
}

// TaxAdjustmentFiles handles GET /api/v1/finance/tax-adjustments/{id}/attachments.
func (h *Handler) TaxAdjustmentFiles(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Koreksi fiskal"))
		return
	}
	files, svcErr := h.svc.TaxAdjustmentFiles(r.Context(), authcontracts.BranchID(r), id)
	if svcErr != nil {
		response.FailErr(w, svcErr)
		return
	}
	response.OK(w, "Lampiran koreksi fiskal", files)
}

// FiscalSummary handles GET /api/v1/finance/reports/fiscal-summary?from=&to=.
func (h *Handler) FiscalSummary(w http.ResponseWriter, r *http.Request) {
	from, to, appErr := dateRange(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	res, err := h.svc.FiscalSummary(r.Context(), authcontracts.BranchID(r), from, to)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Rekonsiliasi fiskal", res)
}

// CashSummary handles GET /api/v1/finance/reports/cash-summary?year=&month=.
func (h *Handler) CashSummary(w http.ResponseWriter, r *http.Request) {
	ctx, scopeErr := h.branchScope(r)
	if scopeErr != nil {
		response.Fail(w, scopeErr)
		return
	}
	// A from-to range reports opening, receipts, payments and closing; the
	// year-month form keeps the original month-footing shape.
	if q := r.URL.Query(); q.Get("from") != "" && q.Get("to") != "" {
		rows, opening, closing, err := h.svc.CashFlow(ctx, authcontracts.BranchID(r), q.Get("from"), q.Get("to"))
		if err != nil {
			response.FailErr(w, err)
			return
		}
		response.OK(w, "Laporan kas", map[string]any{"rows": rows, "openingTotal": opening, "closingTotal": closing})
		return
	}
	year, month, appErr := yearMonth(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	rows, total, err := h.svc.CashSummary(ctx, authcontracts.BranchID(r), year, month)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Ringkas kas", map[string]any{"rows": rows, "total": total})
}

// TaxPackage handles
// GET /api/v1/finance/export/tax-package?year=&month=&variant= or
// ?from=&to=&variant= (free inclusive date range).
//
// variant selects WHICH working paper is produced (default: the actual
// books). The parameter is an application-level switch — the produced file
// names itself by its content, never by the switch.
func (h *Handler) TaxPackage(w http.ResponseWriter, r *http.Request) {
	variant := strings.TrimSpace(r.URL.Query().Get("variant"))
	if variant == "" {
		variant = application.PackageActual
	}
	// ?branch=all|<id> (finance.report.all_branches) packs several branches into
	// one working paper: tax is reported per taxpayer, not per branch. Only the
	// branches the user may read are ever included.
	ctx, scopeErr := h.branchScope(r)
	if scopeErr != nil {
		response.Fail(w, scopeErr)
		return
	}
	// A free date range (?from=&to=) or one month (?year=&month=).
	var raw []byte
	var name string
	var err error
	if q := r.URL.Query(); q.Get("from") != "" || q.Get("to") != "" {
		from, to, appErr := dateRange(r)
		if appErr != nil {
			response.Fail(w, appErr)
			return
		}
		raw, name, err = h.svc.TaxPackageRange(ctx, authcontracts.BranchID(r), from, to, variant)
	} else {
		year, month, appErr := yearMonth(r)
		if appErr != nil {
			response.Fail(w, appErr)
			return
		}
		raw, name, err = h.svc.TaxPackage(ctx, authcontracts.BranchID(r), year, month, variant)
	}
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
