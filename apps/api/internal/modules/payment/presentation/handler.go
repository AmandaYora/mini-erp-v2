package presentation

import (
	"net/http"
	"strconv"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	mediacontracts "mini-erp/internal/modules/media/contracts"
	"mini-erp/internal/modules/payment/application"
	"mini-erp/internal/modules/payment/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
)

// Handler serves the payment module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires payment endpoints.
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

func paymentView(p *contracts.Payment) map[string]any {
	allocs := make([]any, 0, len(p.Allocations))
	for _, a := range p.Allocations {
		allocs = append(allocs, map[string]any{
			"orderType": a.OrderType, "orderId": a.OrderID,
			"orderNumber": a.OrderNumber, "amount": a.Amount,
		})
	}
	return map[string]any{
		"id": p.ID, "number": p.Number, "branchId": p.BranchID,
		"partyId": p.PartyID, "partyName": p.PartyName, "partyType": p.PartyType,
		"direction": p.Direction, "amount": p.Amount, "method": p.Method,
		"paidAt": p.PaidAt, "notes": p.Notes, "status": p.Status,
		"allocations": allocs,
	}
}

// List handles GET /api/v1/payments.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	var partyID int64
	if raw := q.Get("partyId"); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			partyID = id
		}
	}
	res, err := h.svc.List(r.Context(), branchID(r), partyID, q.Get("status"), page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(res.Payments))
	for _, p := range res.Payments {
		items = append(items, paymentView(p))
	}
	response.Paginated(w, "Data pembayaran", items, response.Meta{
		Page: page, Limit: limit, Total: res.Total,
		TotalPages: pagination.TotalPages(res.Total, limit),
	})
}

// Get handles GET /api/v1/payments/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Pembayaran"))
		return
	}
	p, appErr := h.svc.GetByID(r.Context(), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	if p == nil || p.BranchID != branchID(r) {
		response.Fail(w, apperror.NotFound("Pembayaran"))
		return
	}
	response.OK(w, "Data pembayaran", paymentView(p))
}

// Create handles POST /api/v1/payments. Empty allocations auto-fill FIFO.
// partyId 0 = walk-in cash (POS): allocations must be explicit.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PartyID     int64  `json:"partyId"`
		Amount      int64  `json:"amount" validate:"required"`
		Method      string `json:"method"`
		PaidAt      string `json:"paidAt"`
		Notes       string `json:"notes"`
		Allocations []struct {
			OrderType string `json:"orderType"`
			OrderID   int64  `json:"orderId" validate:"required"`
			Amount    int64  `json:"amount" validate:"required"`
		} `json:"allocations"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	method := in.Method
	if method == "" {
		method = "cash"
	}
	allocs := make([]application.AllocationInput, 0, len(in.Allocations))
	for _, a := range in.Allocations {
		allocs = append(allocs, application.AllocationInput{
			OrderType: a.OrderType, OrderID: a.OrderID, Amount: a.Amount,
		})
	}
	p, err := h.svc.Create(r.Context(), actorID(r), branchID(r), in.PartyID, method, in.PaidAt, in.Notes, in.Amount, allocs)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Pembayaran dicatat", paymentView(p))
}

// Cancel handles POST /api/v1/payments/{id}/cancel.
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Pembayaran"))
		return
	}
	p, appErr := h.svc.Cancel(r.Context(), actorID(r), branchID(r), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Pembayaran dibatalkan", paymentView(p))
}

// SettleCredit handles POST /api/v1/payments/settle-credit. It offsets a
// confirmed return against an open bill with no cash moving (two contra
// legs, method "offset").
func (h *Handler) SettleCredit(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PartyID    int64  `json:"partyId" validate:"required"`
		ReturnType string `json:"returnType" validate:"required"`
		ReturnID   int64  `json:"returnId" validate:"required"`
		OrderType  string `json:"orderType" validate:"required"`
		OrderID    int64  `json:"orderId" validate:"required"`
		Amount     int64  `json:"amount" validate:"required"`
		Notes      string `json:"notes"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	p, err := h.svc.SettleCredit(r.Context(), actorID(r), branchID(r), in.PartyID,
		in.ReturnType, in.ReturnID, in.OrderType, in.OrderID, in.Amount, in.Notes)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Kredit di-offset", paymentView(p))
}

// Balance handles GET /api/v1/payments/party-balances?partyId=.
func (h *Handler) Balance(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("partyId")
	partyID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || partyID <= 0 {
		response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "partyId", Message: "wajib diisi"}}))
		return
	}
	bal, appErr := h.svc.GetPartyBalance(r.Context(), branchID(r), partyID)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Saldo pihak", bal)
}

// Ledger handles GET /api/v1/payments/party-ledger?partyId=.
func (h *Handler) Ledger(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	partyID, err := strconv.ParseInt(q.Get("partyId"), 10, 64)
	if err != nil || partyID <= 0 {
		response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "partyId", Message: "wajib diisi"}}))
		return
	}
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	payments, total, svcErr := h.svc.Ledger(r.Context(), branchID(r), partyID, page, limit)
	if svcErr != nil {
		response.FailErr(w, svcErr)
		return
	}
	items := make([]any, 0, len(payments))
	for _, p := range payments {
		items = append(items, paymentView(p))
	}
	response.Paginated(w, "Buku pembayaran pihak", items, response.Meta{
		Page: page, Limit: limit, Total: total,
		TotalPages: pagination.TotalPages(total, limit),
	})
}

// ProofUpload handles POST /api/v1/payments/{id}/proof (multipart image).
func (h *Handler) ProofUpload(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Pembayaran"))
		return
	}
	fh, header, appErr := httpx.MultipartFile(r, 6<<20)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	defer func() { _ = fh.Close() }()
	url, svcErr := h.svc.Proof(r.Context(), actorID(r), branchID(r), id, mediacontracts.Upload{
		OriginalName: header.Filename, MIME: header.Header.Get("Content-Type"),
		Content: fh, Accept: []string{"image/"},
	})
	if svcErr != nil {
		response.FailErr(w, svcErr)
		return
	}
	response.Created(w, "Bukti bayar diunggah", map[string]any{"url": url})
}

// Proofs handles GET /api/v1/payments/{id}/proofs.
func (h *Handler) Proofs(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Pembayaran"))
		return
	}
	proofs, appErr := h.svc.Proofs(r.Context(), branchID(r), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	items := make([]any, 0, len(proofs))
	for _, p := range proofs {
		items = append(items, p)
	}
	response.OK(w, "Bukti bayar", items)
}

// Permission is injected by the composition root (built from the auth module).
type Permission func(perm string, next func(http.ResponseWriter, *http.Request)) http.Handler

// guarded adapts the branch guard into func-style middleware.
func guarded(next func(http.ResponseWriter, *http.Request), branchGuard func(http.Handler) http.Handler) func(http.ResponseWriter, *http.Request) {
	return branchGuard(http.HandlerFunc(next)).ServeHTTP
}

// RegisterRoutes mounts payment endpoints with explicit permissions.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/payments", perm("payment.view", guarded(h.List, branchGuard)))
	mux.Handle("POST /api/v1/payments", perm("payment.create", guarded(h.Create, branchGuard)))
	mux.Handle("GET /api/v1/payments/{id}", perm("payment.view", guarded(h.Get, branchGuard)))
	mux.Handle("POST /api/v1/payments/{id}/cancel", perm("payment.archive", guarded(h.Cancel, branchGuard)))
	mux.Handle("POST /api/v1/payments/settle-credit", perm("payment.create", guarded(h.SettleCredit, branchGuard)))
	mux.Handle("GET /api/v1/payments/party-balances", perm("payment.view", guarded(h.Balance, branchGuard)))
	mux.Handle("GET /api/v1/payments/party-ledger", perm("payment.view", guarded(h.Ledger, branchGuard)))
	mux.Handle("POST /api/v1/payments/{id}/proof", perm("payment.create", guarded(h.ProofUpload, branchGuard)))
	mux.Handle("GET /api/v1/payments/{id}/proofs", perm("payment.view", guarded(h.Proofs, branchGuard)))
}
