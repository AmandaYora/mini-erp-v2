package presentation

import (
	"net/http"

	"mini-erp/internal/modules/branch/application"
	"mini-erp/internal/modules/branch/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"

	authcontracts "mini-erp/internal/modules/auth/contracts"
)

// Handler serves the branch module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires branch endpoints.
func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func actorID(r *http.Request) int64 {
	if s := authcontracts.SessionFromContext(r.Context()); s != nil {
		return s.UserID
	}
	return 0
}

// MyAccess handles GET /api/v1/branches/my-access: the caller's accessible
// ACTIVE branches with the default mark (legacy F-05). Login suffices —
// deliberately no branches.view gate, so a cashier without it can still
// switch branches. Never paginated: it is the user's own short list.
func (h *Handler) MyAccess(w http.ResponseWriter, r *http.Request) {
	rows, appErr := h.svc.MyAccess(r.Context(), actorID(r))
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	items := make([]any, 0, len(rows))
	for _, row := range rows {
		v := branchView(row.Branch)
		v["isDefault"] = row.IsDefault
		items = append(items, v)
	}
	response.OK(w, "Cabang yang dapat diakses", items)
}

func branchView(b *contracts.Branch) map[string]any {
	return map[string]any{
		"id": b.ID, "code": b.Code, "name": b.Name, "address": b.Address,
		"city": b.City, "phone": b.Phone, "status": b.Status, "isHead": b.IsHead,
	}
}

// List handles GET /api/v1/branches.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	res, err := h.svc.List(r.Context(), q.Get("search"), q.Get("status"), page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(res.Branches))
	for _, b := range res.Branches {
		items = append(items, branchView(b))
	}
	response.Paginated(w, "Data cabang", items, response.Meta{
		Page: page, Limit: limit, Total: res.Total,
		TotalPages: pagination.TotalPages(res.Total, limit),
	})
}

// Get handles GET /api/v1/branches/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Cabang"))
		return
	}
	b, appErr := h.svc.GetByID(r.Context(), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	if b == nil {
		response.Fail(w, apperror.NotFound("Cabang"))
		return
	}
	response.OK(w, "Data cabang", branchView(b))
}

// Create handles POST /api/v1/branches.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code    string `json:"code" validate:"required"`
		Name    string `json:"name" validate:"required"`
		Address string `json:"address"`
		City    string `json:"city"`
		Phone   string `json:"phone"`
		IsHead  bool   `json:"isHead"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	b, appErr := h.svc.CreateBranch(r.Context(), actorID(r), application.CreateBranchInput{
		Code: in.Code, Name: in.Name, Address: in.Address,
		City: in.City, Phone: in.Phone, IsHead: in.IsHead,
	})
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.Created(w, "Cabang dibuat", branchView(b))
}

// Update handles PUT /api/v1/branches/{id}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Cabang"))
		return
	}
	var in struct {
		Code    string `json:"code" validate:"required"`
		Name    string `json:"name" validate:"required"`
		Address string `json:"address"`
		City    string `json:"city"`
		Phone   string `json:"phone"`
		Status  string `json:"status" validate:"required"`
		IsHead  bool   `json:"isHead"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	b, appErr := h.svc.UpdateBranch(r.Context(), actorID(r), id, application.UpdateBranchInput{
		Code: in.Code, Name: in.Name, Address: in.Address, City: in.City,
		Phone: in.Phone, Status: in.Status, IsHead: in.IsHead,
	})
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Cabang disimpan", branchView(b))
}

// Permission is injected by the composition root (built from the auth module).
type Permission func(perm string, next func(http.ResponseWriter, *http.Request)) http.Handler

// RegisterRoutes mounts branch endpoints with explicit permissions.
// my-access takes the bare authn wrapper (login suffices, F-05.4) and is
// registered before the {id} pattern for readability — the Go 1.22 mux
// prefers the literal over the wildcard regardless of order.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm Permission, authn func(func(http.ResponseWriter, *http.Request)) http.Handler) {
	mux.Handle("GET /api/v1/branches/my-access", authn(h.MyAccess))
	mux.Handle("GET /api/v1/branches", perm("branches.view", h.List))
	mux.Handle("POST /api/v1/branches", perm("branches.manage", h.Create))
	mux.Handle("GET /api/v1/branches/{id}", perm("branches.view", h.Get))
	mux.Handle("PUT /api/v1/branches/{id}", perm("branches.manage", h.Update))
}
