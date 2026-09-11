package presentation

import (
	"net/http"

	"mini-erp/internal/modules/user/application"
	"mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
)

// Handler serves the user module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires user endpoints.
func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

type branchPayload struct {
	BranchID  int64 `json:"branchId"`
	IsDefault bool  `json:"isDefault"`
}

type userPayload struct {
	Username string          `json:"username" validate:"required"`
	Email    *string         `json:"email,omitempty" validate:"omitempty,email"`
	FullName string          `json:"fullName" validate:"required"`
	Password string          `json:"password,omitempty"`
	RoleIDs  []int64         `json:"roleIds"`
	Branches []branchPayload `json:"branches"`
}

func toAccess(in []branchPayload) []contracts.BranchAccess {
	out := make([]contracts.BranchAccess, 0, len(in))
	for _, b := range in {
		out = append(out, contracts.BranchAccess{BranchID: b.BranchID, IsDefault: b.IsDefault})
	}
	return out
}

func userView(u *contracts.User) map[string]any {
	var email any
	if u.Email != nil {
		email = *u.Email
	}
	return map[string]any{
		"id": u.ID, "username": u.Username, "email": email,
		"fullName": u.FullName, "status": u.Status,
	}
}

// ListUsers handles GET /api/v1/users.
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	res, err := h.svc.ListUsers(r.Context(), q.Get("search"), q.Get("status"), page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(res.Users))
	for _, u := range res.Users {
		items = append(items, userView(u))
	}
	response.Paginated(w, "Data pengguna", items, response.Meta{
		Page: page, Limit: limit, Total: res.Total,
		TotalPages: pagination.TotalPages(res.Total, limit),
	})
}

// GetUser handles GET /api/v1/users/{id} (includes role/branch assignments
// so the edit form can prefill — update is replace-all).
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Pengguna"))
		return
	}
	u, appErr := h.svc.GetByID(r.Context(), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	if u == nil {
		response.Fail(w, apperror.NotFound("Pengguna"))
		return
	}
	view := userView(u)
	roleIDs, appErr := h.svc.GetRoleIDs(r.Context(), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	access, appErr := h.svc.GetBranchAccess(r.Context(), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	branches := make([]any, 0, len(access))
	for _, a := range access {
		branches = append(branches, map[string]any{"branchId": a.BranchID, "isDefault": a.IsDefault})
	}
	view["roleIds"] = roleIDs
	view["branches"] = branches
	response.OK(w, "Data pengguna", view)
}

// CreateUser handles POST /api/v1/users.
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var in userPayload
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	if in.Password == "" {
		response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "password", Message: "wajib diisi"}}))
		return
	}
	u, appErr := h.svc.CreateUser(r.Context(), actorID(r), application.CreateUserInput{
		Username: in.Username, Email: in.Email, FullName: in.FullName,
		Password: in.Password, RoleIDs: in.RoleIDs, Branches: toAccess(in.Branches),
	})
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.Created(w, "Pengguna dibuat", userView(u))
}

// UpdateUser handles PUT /api/v1/users/{id}.
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Pengguna"))
		return
	}
	var in userPayload
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	u, appErr := h.svc.UpdateUser(r.Context(), actorID(r), id, application.UpdateUserInput{
		Username: in.Username, Email: in.Email, FullName: in.FullName,
		RoleIDs: in.RoleIDs, Branches: toAccess(in.Branches),
	})
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Pengguna disimpan", userView(u))
}

// SetStatus handles PATCH /api/v1/users/{id}/status.
func (h *Handler) SetStatus(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Pengguna"))
		return
	}
	var in struct {
		Status string `json:"status" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	if appErr := h.svc.SetStatus(r.Context(), actorID(r), id, in.Status); appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Status pengguna disimpan", nil)
}

// ChangePassword handles POST /api/v1/users/{id}/password.
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Pengguna"))
		return
	}
	var in struct {
		Password string `json:"password" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	if appErr := h.svc.ChangePassword(r.Context(), actorID(r), id, in.Password); appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Password disimpan", nil)
}

// ListRoles handles GET /api/v1/roles.
func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	roles, appErr := h.svc.List(r.Context())
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	items := make([]any, 0, len(roles))
	for _, role := range roles {
		items = append(items, map[string]any{
			"id": role.ID, "code": role.Code, "name": role.Name,
			"description": role.Description, "isSystem": role.IsSystem,
		})
	}
	response.OK(w, "Data role", items)
}

// ListPermissions handles GET /api/v1/permissions (full catalog for the
// role-permissions UI).
func (h *Handler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	perms, appErr := h.svc.ListPermissions(r.Context())
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	items := make([]any, 0, len(perms))
	for _, p := range perms {
		items = append(items, map[string]any{"code": p.Code, "name": p.Name})
	}
	response.OK(w, "Katalog permission", items)
}

// CreateRole handles POST /api/v1/roles.
func (h *Handler) CreateRole(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code        string `json:"code" validate:"required"`
		Name        string `json:"name" validate:"required"`
		Description string `json:"description"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	role, appErr := h.svc.CreateRole(r.Context(), actorID(r), application.CreateRoleInput{
		Code: in.Code, Name: in.Name, Description: in.Description,
	})
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.Created(w, "Role dibuat", map[string]any{
		"id": role.ID, "code": role.Code, "name": role.Name, "description": role.Description,
	})
}

// UpdateRole handles PUT /api/v1/roles/{id}.
func (h *Handler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Role"))
		return
	}
	var in struct {
		Name        string `json:"name" validate:"required"`
		Description string `json:"description"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	role, appErr := h.svc.UpdateRole(r.Context(), actorID(r), id, in.Name, in.Description)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Role disimpan", map[string]any{
		"id": role.ID, "code": role.Code, "name": role.Name, "description": role.Description,
	})
}

// DeleteRole handles DELETE /api/v1/roles/{id}.
func (h *Handler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Role"))
		return
	}
	if appErr := h.svc.DeleteRole(r.Context(), actorID(r), id); appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Role dihapus", nil)
}

// SetRolePermissions handles PUT /api/v1/roles/{id}/permissions.
func (h *Handler) SetRolePermissions(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Role"))
		return
	}
	var in struct {
		Permissions []string `json:"permissions"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	if appErr := h.svc.SetRolePermissions(r.Context(), actorID(r), id, in.Permissions); appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Izin role disimpan", nil)
}
