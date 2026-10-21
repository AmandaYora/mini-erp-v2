package presentation

import (
	"net/http"
	"strconv"
	"strings"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/finance/application"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
)

// Cost corrections on the web: the cost restatement (OP-04, also the
// cmd/costrebuild console tool) and the inventory revaluation. Previews need
// finance.manage; commits rewrite valuation history and need
// finance.close.force. The branch is always the session's.

// RestatePreview handles POST /api/v1/finance/cost-restatement/preview: the
// dry run (nothing is written; gates are reported).
func (h *Handler) RestatePreview(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Date string `json:"date"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	rep, err := h.svc.RestateCost(r.Context(), application.RestateCostOptions{
		BranchID: authcontracts.BranchID(r), ActorID: authcontracts.ActorID(r), Date: in.Date})
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Pratinjau hitung ulang HPP", rep)
}

// RestateCommit handles POST /api/v1/finance/cost-restatement/commit: the
// restatement behind its gates, with the reason recorded in the audit entry.
func (h *Handler) RestateCommit(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Date   string `json:"date"`
		Reason string `json:"reason"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" || len([]rune(reason)) > 500 {
		msg := "alasan wajib diisi"
		if reason != "" {
			msg = "maksimal 500 karakter"
		}
		response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "reason", Message: msg}}))
		return
	}
	rep, err := h.svc.RestateCost(r.Context(), application.RestateCostOptions{
		BranchID: authcontracts.BranchID(r), ActorID: authcontracts.ActorID(r), Commit: true, Date: in.Date, Reason: reason})
	if err != nil {
		response.FailErr(w, err)
		return
	}
	msg := "HPP dihitung ulang"
	if !rep.Applied {
		msg = "Tidak ada biaya keluar yang perlu dihitung ulang"
	}
	response.OK(w, msg, rep)
}

type revaluationBody struct {
	Date      string `json:"date"`
	Reason    string `json:"reason"`
	ClientRef string `json:"clientRef"`
	// One position at the top level, or several in items (one journal).
	ProductID   int64  `json:"productId"`
	VariantID   int64  `json:"variantId"`
	NewUnitCost *int64 `json:"newUnitCost"`
	Items       []struct {
		ProductID   int64  `json:"productId"`
		VariantID   int64  `json:"variantId"`
		NewUnitCost *int64 `json:"newUnitCost"`
	} `json:"items"`
}

func (h *Handler) revaluationInput(w http.ResponseWriter, r *http.Request) (application.RevaluationInput, bool) {
	var in revaluationBody
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return application.RevaluationInput{}, false
	}
	out := application.RevaluationInput{BranchID: authcontracts.BranchID(r), ActorID: authcontracts.ActorID(r),
		Date: in.Date, Reason: in.Reason, ClientRef: in.ClientRef}
	var fails []apperror.FieldError
	add := func(productID, variantID int64, cost *int64, field string) {
		if cost == nil {
			fails = append(fails, apperror.FieldError{Field: field, Message: "harga modal baru wajib diisi"})
			return
		}
		out.Items = append(out.Items, application.RevaluationItem{ProductID: productID, VariantID: variantID, NewUnitCost: *cost})
	}
	if in.ProductID > 0 {
		if len(in.Items) > 0 {
			fails = append(fails, apperror.FieldError{Field: "items", Message: "pakai productId atau items, bukan keduanya"})
		}
		add(in.ProductID, in.VariantID, in.NewUnitCost, "newUnitCost")
	}
	for i, it := range in.Items {
		add(it.ProductID, it.VariantID, it.NewUnitCost, "items."+strconv.Itoa(i)+".newUnitCost")
	}
	if len(fails) > 0 {
		response.Fail(w, apperror.Validation("", fails))
		return application.RevaluationInput{}, false
	}
	return out, true
}

// RevaluationPreview handles POST /api/v1/finance/inventory-revaluations/preview.
func (h *Handler) RevaluationPreview(w http.ResponseWriter, r *http.Request) {
	in, ok := h.revaluationInput(w, r)
	if !ok {
		return
	}
	res, err := h.svc.PreviewRevaluation(r.Context(), in)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Pratinjau revaluasi harga modal", res)
}

// RevaluationCommit handles POST /api/v1/finance/inventory-revaluations/commit.
func (h *Handler) RevaluationCommit(w http.ResponseWriter, r *http.Request) {
	in, ok := h.revaluationInput(w, r)
	if !ok {
		return
	}
	res, err := h.svc.CommitRevaluation(r.Context(), in)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	if res.Replayed {
		response.OK(w, "Revaluasi ini sudah tercatat", res)
		return
	}
	response.Created(w, "Harga modal direvaluasi", res)
}

// RevaluationPositions handles GET
// /api/v1/finance/inventory-revaluations/positions?productId=: the product's
// positions (per variant) with reference prices.
func (h *Handler) RevaluationPositions(w http.ResponseWriter, r *http.Request) {
	productID, err := strconv.ParseInt(r.URL.Query().Get("productId"), 10, 64)
	if err != nil || productID <= 0 {
		response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "productId", Message: "wajib diisi"}}))
		return
	}
	items, appErr := h.svc.RevaluationPositions(r.Context(), authcontracts.BranchID(r), productID)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Posisi persediaan produk", items)
}

// ListRevaluations handles GET /api/v1/finance/inventory-revaluations.
func (h *Handler) ListRevaluations(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	items, total, err := h.svc.ListRevaluations(r.Context(), authcontracts.BranchID(r), page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Paginated(w, "Riwayat revaluasi harga modal", items, response.Meta{
		Page: page, Limit: limit, Total: total, TotalPages: pagination.TotalPages(total, limit),
	})
}
