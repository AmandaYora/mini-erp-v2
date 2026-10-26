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
	"mini-erp/internal/shared/listquery"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
	"mini-erp/internal/shared/timeutil"
)

// Handler serves the payment module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires payment endpoints.
func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
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
		"partyPhone": p.PartyPhone, "partyAddress": p.PartyAddress,
		"direction": p.Direction, "amount": p.Amount, "method": p.Method,
		"paidAt": p.PaidAt, "notes": p.Notes, "status": p.Status,
		"clientRef": p.ClientRef, "reference": p.Reference, "cashAccountId": p.CashAccountID,
		"amountTendered": p.AmountTendered, "change": change(p),
		"pairId":      p.PairID,
		"allocations": allocs,
		// N-41: the return whose saldo a use-credit leg spends (absent otherwise).
		"creditSourceType": p.CreditSourceType, "creditSourceId": p.CreditSourceID,
		// Correction links (numbers filled on the detail read) and the cancel trail.
		"correctsPaymentId": p.CorrectsPaymentID, "correctsPaymentNumber": p.CorrectsPaymentNumber,
		"correctedById": p.CorrectedByID, "correctedByNumber": p.CorrectedByNumber,
		"cancelReason": p.CancelReason, "cancelledAt": timeutil.FormatWIB(p.CancelledAt),
		"cancelledBy": p.CancelledBy,
	}
}

// change is the cash returned to the payer, derived on display from the
// amount handed over (never stored).
func change(p *contracts.Payment) int64 {
	if p.AmountTendered > p.Amount {
		return p.AmountTendered - p.Amount
	}
	return 0
}

// List handles GET /api/v1/payments.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	c, err := listquery.ParseCommon(q, nil)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	page, limit := c.Page, c.Limit
	var orderID int64
	if raw := q.Get("orderId"); raw != "" {
		id, perr := strconv.ParseInt(raw, 10, 64)
		if perr != nil || id <= 0 {
			response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "orderId", Message: "harus angka positif"}}))
			return
		}
		orderID = id
	}
	var cashAccountID int64
	if raw := q.Get("cashAccountId"); raw != "" {
		id, perr := strconv.ParseInt(raw, 10, 64)
		if perr != nil || id <= 0 {
			response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "cashAccountId", Message: "harus angka positif"}}))
			return
		}
		cashAccountID = id
	}
	res, err := h.svc.ListFiltered(r.Context(), authcontracts.BranchID(r), application.ListFilter{
		Search: c.Search, PartyID: c.PartyID, Status: q.Get("status"), Method: q.Get("method"),
		Direction: q.Get("direction"), From: c.From, To: c.To,
		OrderType: q.Get("orderType"), OrderID: orderID, CashAccountID: cashAccountID,
	}, page, limit)
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
	if p == nil || p.BranchID != authcontracts.BranchID(r) {
		response.Fail(w, apperror.NotFound("Pembayaran"))
		return
	}
	response.OK(w, "Data pembayaran", paymentView(p))
}

// refundPermission maps a return order type to the permission its refunds
// require; "" for ordinary orders.
func refundPermission(orderType string) string {
	switch orderType {
	case contracts.OrderSalesReturn:
		return "salesreturn.refund"
	case contracts.OrderPurchaseReturn:
		return "purchasereturn.refund"
	}
	return ""
}

// Create handles POST /api/v1/payments. Empty allocations auto-fill FIFO.
// partyId 0 = walk-in cash (POS): allocations must be explicit.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PartyID        int64  `json:"partyId"`
		Amount         int64  `json:"amount" validate:"required"`
		Method         string `json:"method"`
		PaidAt         string `json:"paidAt"`
		Notes          string `json:"notes"`
		ClientRef      string `json:"clientRef"`
		AmountTendered int64  `json:"amountTendered"`
		Reference      string `json:"reference"`
		CashAccountID  int64  `json:"cashAccountId"`
		Allocations    []struct {
			OrderType string `json:"orderType"`
			OrderID   int64  `json:"orderId" validate:"required"`
			Amount    int64  `json:"amount" validate:"required"`
		} `json:"allocations"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	// A refund leg (an allocation against a return) moves money out: like the
	// return settlement endpoints it demands the return's own refund
	// permission on top of payment.create (F3b-2). Other payments need only
	// payment.create, which the route already enforced.
	for _, a := range in.Allocations {
		need := refundPermission(a.OrderType)
		if need == "" {
			continue
		}
		if sess := authcontracts.SessionFromContext(r.Context()); sess == nil || !sess.HasPermission(need) {
			response.Fail(w, &apperror.AppError{Code: apperror.CodeForbidden, Message: "Anda tidak memiliki izin memproses refund"})
			return
		}
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
	p, err := h.svc.Create(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), in.PartyID, method, in.PaidAt, in.Notes, in.Amount, allocs,
		application.WithClientRef(in.ClientRef),
		application.WithAmountTendered(in.AmountTendered),
		application.WithReference(in.Reference),
		application.WithCashAccount(in.CashAccountID))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Pembayaran dicatat", paymentView(p))
}

// Cancel handles POST /api/v1/payments/{id}/cancel with an optional
// {reason}. The web always sends one; an empty body stays accepted for older
// clients and scripts.
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Pembayaran"))
		return
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0 {
		if fields := httpx.DecodeJSON(r, &in); fields != nil {
			response.Fail(w, apperror.Validation("", fields))
			return
		}
	}
	p, appErr := h.svc.CancelWithReason(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id, in.Reason)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Pembayaran dibatalkan", paymentView(p))
}

// Correct handles POST /api/v1/payments/{id}/correct: a replacement payment
// with the corrected data takes over the original's orders and the original is
// cancelled with the reason. Needs payment.create (route) AND payment.archive
// (it cancels), plus the refund permission when the payment is a refund.
func (h *Handler) Correct(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Pembayaran"))
		return
	}
	var in struct {
		Method         string `json:"method" validate:"required"`
		Amount         int64  `json:"amount" validate:"required"`
		PaidAt         string `json:"paidAt"`
		Reference      string `json:"reference"`
		AmountTendered int64  `json:"amountTendered"`
		Notes          string `json:"notes"`
		Reason         string `json:"reason" validate:"required"`
		CashAccountID  int64  `json:"cashAccountId"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	sess := authcontracts.SessionFromContext(r.Context())
	if sess == nil || !sess.HasPermission("payment.archive") {
		response.Fail(w, &apperror.AppError{Code: apperror.CodeForbidden, Message: "Anda tidak memiliki izin membatalkan pembayaran"})
		return
	}
	branchID := authcontracts.BranchID(r)
	cur, appErr := h.svc.GetByID(r.Context(), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	if cur == nil || cur.BranchID != branchID {
		response.Fail(w, apperror.NotFound("Pembayaran"))
		return
	}
	for _, a := range cur.Allocations {
		if need := refundPermission(a.OrderType); need != "" && !sess.HasPermission(need) {
			response.Fail(w, &apperror.AppError{Code: apperror.CodeForbidden, Message: "Anda tidak memiliki izin memproses refund"})
			return
		}
	}
	p, replayed, appErr := h.svc.Correct(r.Context(), application.CorrectInput{
		ActorID: authcontracts.ActorID(r), BranchID: branchID, PaymentID: id,
		Method: in.Method, Amount: in.Amount, PaidAt: in.PaidAt, Reference: in.Reference,
		AmountTendered: in.AmountTendered, Notes: in.Notes, Reason: in.Reason,
		CashAccountID: in.CashAccountID,
	})
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	if replayed {
		response.OK(w, "Pembayaran sudah dikoreksi", paymentView(p))
		return
	}
	response.Created(w, "Pembayaran dikoreksi", paymentView(p))
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
	p, err := h.svc.SettleCredit(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), in.PartyID,
		in.ReturnType, in.ReturnID, in.OrderType, in.OrderID, in.Amount, in.Notes)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Kredit di-offset", paymentView(p))
}

// UseCredit handles POST /api/v1/payments/use-credit (N-41): spends part of the
// saldo (uang muka) a confirmed return left as the party's balance on an open
// bill of the same party. No cash moves; the bill leg is journalled
// Dr Uang Muka Pelanggan / Cr Piutang (sales) or Dr Hutang / Cr Uang Muka
// Supplier (purchase).
func (h *Handler) UseCredit(w http.ResponseWriter, r *http.Request) {
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
	p, err := h.svc.UseCredit(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), in.PartyID,
		in.ReturnType, in.ReturnID, in.OrderType, in.OrderID, in.Amount, in.Notes)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Saldo uang muka dipakai", paymentView(p))
}

// CreditBalances handles GET /api/v1/payments/credit-balances?partyId=: the
// party's returns that hold saldo, with what was used and what remains.
func (h *Handler) CreditBalances(w http.ResponseWriter, r *http.Request) {
	partyID, err := strconv.ParseInt(r.URL.Query().Get("partyId"), 10, 64)
	if err != nil || partyID <= 0 {
		response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "partyId", Message: "wajib diisi"}}))
		return
	}
	rows, appErr := h.svc.CreditBalances(r.Context(), authcontracts.BranchID(r), partyID)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	var remaining int64
	for _, row := range rows {
		remaining += row.Remaining
	}
	response.OK(w, "Saldo uang muka pihak", map[string]any{"items": rows, "totalRemaining": remaining})
}

// Balance handles GET /api/v1/payments/party-balances?partyId=.
func (h *Handler) Balance(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("partyId")
	partyID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || partyID <= 0 {
		response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "partyId", Message: "wajib diisi"}}))
		return
	}
	bal, appErr := h.svc.GetPartyBalance(r.Context(), authcontracts.BranchID(r), partyID)
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
	payments, total, svcErr := h.svc.Ledger(r.Context(), authcontracts.BranchID(r), partyID, page, limit)
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

// OpenParties handles GET /api/v1/payments/open-parties?side=&search=&page=&limit=
// (PY-01): every party with an unpaid balance on one side, oldest first.
func (h *Handler) OpenParties(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	side := q.Get("side")
	if side == "" {
		side = application.SideReceivable
	}
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	res, err := h.svc.OpenParties(r.Context(), authcontracts.BranchID(r), side, q.Get("search"), page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(res.Items))
	for _, it := range res.Items {
		items = append(items, it)
	}
	// data carries the figures of the WHOLE filtered set next to the page, so the
	// header can show the total owed without a second request; meta stays standard.
	totalPages := pagination.TotalPages(res.Total, limit)
	response.Paginated(w, "Daftar pihak bersaldo", map[string]any{
		"items": items, "total": res.Total, "totalOutstanding": res.TotalOutstanding,
		"page": page, "limit": limit, "totalPages": totalPages,
	}, response.Meta{Page: page, Limit: limit, Total: res.Total, TotalPages: totalPages})
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
	url, svcErr := h.svc.Proof(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id, mediacontracts.Upload{
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
	proofs, appErr := h.svc.Proofs(r.Context(), authcontracts.BranchID(r), id)
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

// RegisterRoutes mounts payment endpoints with explicit permissions.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm authcontracts.Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/payments", perm("payment.view", authcontracts.Guard(h.List, branchGuard)))
	mux.Handle("POST /api/v1/payments", perm("payment.create", authcontracts.Guard(h.Create, branchGuard)))
	mux.Handle("GET /api/v1/payments/{id}", perm("payment.view", authcontracts.Guard(h.Get, branchGuard)))
	mux.Handle("POST /api/v1/payments/{id}/cancel", perm("payment.archive", authcontracts.Guard(h.Cancel, branchGuard)))
	// Also checks payment.archive in the handler (a correction cancels the original).
	mux.Handle("POST /api/v1/payments/{id}/correct", perm("payment.create", authcontracts.Guard(h.Correct, branchGuard)))
	mux.Handle("POST /api/v1/payments/settle-credit", perm("payment.create", authcontracts.Guard(h.SettleCredit, branchGuard)))
	mux.Handle("POST /api/v1/payments/use-credit", perm("payment.create", authcontracts.Guard(h.UseCredit, branchGuard)))
	mux.Handle("GET /api/v1/payments/credit-balances", perm("payment.view", authcontracts.Guard(h.CreditBalances, branchGuard)))
	mux.Handle("GET /api/v1/payments/party-balances", perm("payment.view", authcontracts.Guard(h.Balance, branchGuard)))
	mux.Handle("GET /api/v1/payments/open-parties", perm("payment.view", authcontracts.Guard(h.OpenParties, branchGuard)))
	mux.Handle("GET /api/v1/payments/party-ledger", perm("payment.view", authcontracts.Guard(h.Ledger, branchGuard)))
	mux.Handle("POST /api/v1/payments/{id}/proof", perm("payment.create", authcontracts.Guard(h.ProofUpload, branchGuard)))
	mux.Handle("GET /api/v1/payments/{id}/proofs", perm("payment.view", authcontracts.Guard(h.Proofs, branchGuard)))
}
