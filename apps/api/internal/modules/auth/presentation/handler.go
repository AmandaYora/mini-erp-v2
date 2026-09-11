package presentation

import (
	"log/slog"
	"net/http"

	"mini-erp/internal/modules/auth/application"
	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/response"
)

// Handler serves the auth module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires auth endpoints.
func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	host := r.RemoteAddr
	for i := len(host) - 1; i >= 0; i-- {
		if host[i] == ':' {
			return host[:i]
		}
	}
	return host
}

// Login handles POST /api/v1/auth/login (public, rate-limited).
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username" validate:"required"`
		Password string `json:"password" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	res, err := h.svc.Login(r.Context(), in.Username, in.Password, clientIP(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Login berhasil", res)
}

// Refresh handles POST /api/v1/auth/refresh (public, rotating).
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RefreshToken string `json:"refreshToken" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	res, err := h.svc.Refresh(r.Context(), in.RefreshToken)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Token diperbarui", res)
}

// Logout handles POST /api/v1/auth/logout.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	s := authcontracts.SessionFromContext(r.Context())
	if err := h.svc.Logout(r.Context(), s); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Logout berhasil", nil)
}

// Me handles GET /api/v1/auth/me.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	s := authcontracts.SessionFromContext(r.Context())
	res, err := h.svc.Me(r.Context(), s)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Data sesi", res)
}

// SwitchBranch handles POST /api/v1/auth/switch-branch.
func (h *Handler) SwitchBranch(w http.ResponseWriter, r *http.Request) {
	var in struct {
		BranchID int64 `json:"branchId" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	s := authcontracts.SessionFromContext(r.Context())
	token, err := h.svc.SwitchBranch(r.Context(), s, in.BranchID)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Cabang aktif diganti", map[string]any{"accessToken": token})
}

// SwitchRole handles POST /api/v1/auth/switch-role.
func (h *Handler) SwitchRole(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RoleID int64 `json:"roleId" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	s := authcontracts.SessionFromContext(r.Context())
	token, err := h.svc.SwitchRole(r.Context(), s, in.RoleID)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Role aktif diganti", map[string]any{"accessToken": token})
}

// RegisterRoutes mounts auth endpoints. Login/refresh stay public; the rest
// require a session. No branch guard here — selection endpoints must work
// before a branch exists.
func RegisterRoutes(mux *http.ServeMux, h *Handler, log *slog.Logger) {
	mux.HandleFunc("POST /api/v1/auth/login", h.Login)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.Refresh)
	mux.Handle("POST /api/v1/auth/logout",
		authcontracts.Authenticate(log, h.svc, http.HandlerFunc(h.Logout)))
	mux.Handle("GET /api/v1/auth/me",
		authcontracts.Authenticate(log, h.svc, http.HandlerFunc(h.Me)))
	mux.Handle("POST /api/v1/auth/switch-branch",
		authcontracts.Authenticate(log, h.svc, http.HandlerFunc(h.SwitchBranch)))
	mux.Handle("POST /api/v1/auth/switch-role",
		authcontracts.Authenticate(log, h.svc, http.HandlerFunc(h.SwitchRole)))
}
