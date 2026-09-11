package presentation

import (
	"net/http"

	"mini-erp/internal/modules/assistant/application"
	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/response"
)

// Handler serves the assistant module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires assistant endpoints.
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

func answerView(branchID int64, branch string, runID int64, intent, mode, text string) map[string]any {
	return map[string]any{
		"runId": runID, "branchId": branchID, "branchName": branch,
		"intent": intent, "mode": mode, "answer": text,
	}
}

// Chat handles POST /api/v1/assistant/chat (in-app console, no channel).
func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Message  string `json:"message" validate:"required"`
		BranchID int64  `json:"branchId"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	ans, err := h.svc.Ask(r.Context(), actorID(r), in.BranchID, in.Message)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Jawaban asisten", answerView(ans.BranchID, ans.BranchName, ans.RunID, ans.Intent, ans.Mode, ans.Text))
}

// Simulate handles POST /api/v1/assistant/simulate (operator test path,
// channel required, nothing sent).
func (h *Handler) Simulate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone    string `json:"phone" validate:"required"`
		Message  string `json:"message" validate:"required"`
		BranchID int64  `json:"branchId"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	ans, err := h.svc.Simulate(r.Context(), actorID(r), in.BranchID, in.Phone, in.Message)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Hasil simulasi", answerView(ans.BranchID, ans.BranchName, ans.RunID, ans.Intent, ans.Mode, ans.Text))
}

// RunStats handles GET /api/v1/assistant/runs/stats.
func (h *Handler) RunStats(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.RunStats(r.Context())
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(rows))
	for _, s := range rows {
		items = append(items, map[string]any{
			"day": s.Day, "intent": s.Intent, "mode": s.Mode,
			"total": s.Total, "success": s.Success, "avgDurationMs": s.AvgDurationMs,
		})
	}
	response.OK(w, "Statistik run", items)
}

// ChannelStatus handles GET /api/v1/assistant/channel.
func (h *Handler) ChannelStatus(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.ChannelStatus(r.Context())
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Status kanal", map[string]any{
		"connected": st.Connected, "phone": st.Phone, "qrDataUrl": st.QRDataURL,
		"lastError": st.LastError, "updatedAt": st.UpdatedAt,
	})
}

// ChannelConnect handles POST /api/v1/assistant/channel/connect.
func (h *Handler) ChannelConnect(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Connect(r.Context()); err != nil {
		response.FailErr(w, err)
		return
	}
	h.ChannelStatus(w, r)
}

// ChannelDisconnect handles POST /api/v1/assistant/channel/disconnect.
func (h *Handler) ChannelDisconnect(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Disconnect(r.Context()); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Kanal diputus", nil)
}

// ChannelReset handles POST /api/v1/assistant/channel/reset.
func (h *Handler) ChannelReset(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.ResetSession(r.Context()); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Sesi dihapus, pindai ulang untuk menautkan", nil)
}

// ListAuthorizations handles GET /api/v1/assistant/authorizations.
func (h *Handler) ListAuthorizations(w http.ResponseWriter, r *http.Request) {
	revoked := r.URL.Query().Get("includeRevoked") == "true"
	rows, err := h.svc.ListAuthorizations(r.Context(), revoked)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(rows))
	for _, a := range rows {
		items = append(items, authRow(a.ID, a.Phone, a.Name, a.AccessLevel, a.Status, a.IsPrimaryOwner, a.LastSeenAt))
	}
	response.OK(w, "Nomor terotorisasi", items)
}

func authRow(id int64, phone, name, level, status string, primary bool, seen string) map[string]any {
	return map[string]any{
		"id": id, "phone": phone, "name": name, "accessLevel": level,
		"status": status, "isPrimaryOwner": primary, "lastSeenAt": seen,
	}
}

// CreateAuthorization handles POST /api/v1/assistant/authorizations.
func (h *Handler) CreateAuthorization(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone          string `json:"phone" validate:"required"`
		Name           string `json:"name" validate:"required"`
		AccessLevel    string `json:"accessLevel"`
		IsPrimaryOwner bool   `json:"isPrimaryOwner"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	a, err := h.svc.CreateAuthorization(r.Context(), actorID(r), application.AuthorizationInput{
		Phone: in.Phone, Name: in.Name, AccessLevel: in.AccessLevel, IsPrimaryOwner: in.IsPrimaryOwner,
	})
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Nomor terotorisasi", authRow(a.ID, a.Phone, a.Name, a.AccessLevel, a.Status, a.IsPrimaryOwner, a.LastSeenAt))
}

// UpdateAuthorization handles PUT /api/v1/assistant/authorizations/{id}.
func (h *Handler) UpdateAuthorization(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Nomor WhatsApp"))
		return
	}
	var in struct {
		Phone          string `json:"phone" validate:"required"`
		Name           string `json:"name" validate:"required"`
		AccessLevel    string `json:"accessLevel"`
		IsPrimaryOwner bool   `json:"isPrimaryOwner"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	a, appErr := h.svc.UpdateAuthorization(r.Context(), actorID(r), id, application.AuthorizationInput{
		Phone: in.Phone, Name: in.Name, AccessLevel: in.AccessLevel, IsPrimaryOwner: in.IsPrimaryOwner,
	})
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Otorisasi diperbarui", authRow(a.ID, a.Phone, a.Name, a.AccessLevel, a.Status, a.IsPrimaryOwner, a.LastSeenAt))
}

// RevokeAuthorization handles POST /api/v1/assistant/authorizations/{id}/revoke.
func (h *Handler) RevokeAuthorization(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Nomor WhatsApp"))
		return
	}
	if appErr := h.svc.RevokeAuthorization(r.Context(), actorID(r), id); appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Otorisasi dicabut", nil)
}

// GetConfig handles GET /api/v1/assistant/config.
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.svc.GetConfig(r.Context())
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Konfigurasi asisten", map[string]any{
		"mode": cfg.Mode, "effectiveMode": cfg.EffectiveMode,
		"rateLimitPerMinute": cfg.RateLimitPerMin, "updatedAt": cfg.UpdatedAt,
	})
}

// UpdateConfig handles PUT /api/v1/assistant/config.
func (h *Handler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Mode            string `json:"mode"`
		RateLimitPerMin int    `json:"rateLimitPerMinute"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	cfg, err := h.svc.UpdateConfig(r.Context(), in.Mode, in.RateLimitPerMin)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Konfigurasi disimpan", map[string]any{
		"mode": cfg.Mode, "effectiveMode": cfg.EffectiveMode,
		"rateLimitPerMinute": cfg.RateLimitPerMin, "updatedAt": cfg.UpdatedAt,
	})
}

// Permission is injected by the composition root (built from the auth module).
type Permission func(perm string, next func(http.ResponseWriter, *http.Request)) http.Handler

func guarded(next func(http.ResponseWriter, *http.Request), branchGuard func(http.Handler) http.Handler) func(http.ResponseWriter, *http.Request) {
	return branchGuard(http.HandlerFunc(next)).ServeHTTP
}

// RegisterRoutes mounts assistant endpoints with explicit permissions.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("POST /api/v1/assistant/chat", perm("assistant.view", guarded(h.Chat, branchGuard)))
	mux.Handle("POST /api/v1/assistant/simulate", perm("assistant.manage", guarded(h.Simulate, branchGuard)))
	mux.Handle("GET /api/v1/assistant/runs/stats", perm("assistant.view", guarded(h.RunStats, branchGuard)))
	mux.Handle("GET /api/v1/assistant/channel", perm("assistant.view", guarded(h.ChannelStatus, branchGuard)))
	mux.Handle("POST /api/v1/assistant/channel/connect", perm("assistant.manage", guarded(h.ChannelConnect, branchGuard)))
	mux.Handle("POST /api/v1/assistant/channel/disconnect", perm("assistant.manage", guarded(h.ChannelDisconnect, branchGuard)))
	mux.Handle("POST /api/v1/assistant/channel/reset", perm("assistant.manage", guarded(h.ChannelReset, branchGuard)))
	mux.Handle("GET /api/v1/assistant/authorizations", perm("assistant.view", guarded(h.ListAuthorizations, branchGuard)))
	mux.Handle("POST /api/v1/assistant/authorizations", perm("assistant.manage", guarded(h.CreateAuthorization, branchGuard)))
	mux.Handle("PUT /api/v1/assistant/authorizations/{id}", perm("assistant.manage", guarded(h.UpdateAuthorization, branchGuard)))
	mux.Handle("POST /api/v1/assistant/authorizations/{id}/revoke", perm("assistant.manage", guarded(h.RevokeAuthorization, branchGuard)))
	mux.Handle("GET /api/v1/assistant/config", perm("assistant.view", guarded(h.GetConfig, branchGuard)))
	mux.Handle("PUT /api/v1/assistant/config", perm("assistant.manage", guarded(h.UpdateConfig, branchGuard)))
}
