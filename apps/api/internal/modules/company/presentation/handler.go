package presentation

import (
	"net/http"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/company/application"
	"mini-erp/internal/modules/company/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/response"
)

// Handler serves the company module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires company endpoints.
func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func profileView(p *contracts.Profile) map[string]any {
	return map[string]any{
		"id": p.ID, "name": p.Name, "legalName": p.LegalName, "address": p.Address,
		"city": p.City, "phone": p.Phone, "email": p.Email, "taxId": p.TaxID,
	}
}

// GetProfile handles GET /api/v1/company/profile.
func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetProfile(r.Context())
	if err != nil {
		response.FailErr(w, err)
		return
	}
	if p == nil {
		response.OK(w, "Profil perusahaan", nil)
		return
	}
	response.OK(w, "Profil perusahaan", profileView(p))
}

// SaveProfile handles PUT /api/v1/company/profile.
func (h *Handler) SaveProfile(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name      string `json:"name" validate:"required"`
		LegalName string `json:"legalName"`
		Address   string `json:"address"`
		City      string `json:"city"`
		Phone     string `json:"phone"`
		Email     string `json:"email" validate:"omitempty,email"`
		TaxID     string `json:"taxId"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	p, appErr := h.svc.SaveProfile(r.Context(), authcontracts.ActorID(r), contracts.Profile{
		Name: in.Name, LegalName: in.LegalName, Address: in.Address, City: in.City,
		Phone: in.Phone, Email: in.Email, TaxID: in.TaxID,
	})
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Profil perusahaan disimpan", profileView(p))
}

// GetSettings handles GET /api/v1/company/settings.
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.svc.EffectiveSettings(r.Context())
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Pengaturan perusahaan", settings)
}

// SaveSettings handles PUT /api/v1/company/settings (merge, not replace).
func (h *Handler) SaveSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Settings map[string]string `json:"settings" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	if appErr := h.svc.SaveSettings(r.Context(), authcontracts.ActorID(r), in.Settings); appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	settings, err := h.svc.EffectiveSettings(r.Context())
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Pengaturan perusahaan disimpan", settings)
}

// SettingDefinitions handles GET /api/v1/company/settings/definitions: the
// registered settings (key, kind, default, allowed values) for the UI.
func (h *Handler) SettingDefinitions(w http.ResponseWriter, r *http.Request) {
	response.OK(w, "Definisi pengaturan", application.SettingDefinitions())
}

// PrintProfile handles GET /api/v1/company/print-profile: an allow-list for
// printing, readable by any authenticated user with a branch (no company.view).
func (h *Handler) PrintProfile(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.PrintProfile(r.Context())
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Profil cetak", map[string]any{
		"name": p.Name, "legalName": p.LegalName, "address": p.Address,
		"city": p.City, "phone": p.Phone,
		// raw JSON text of the document profile (letterhead, bank accounts,
		// paper calibration); "" when never saved.
		"documentProfile": p.Settings["document_profile"],
	})
}

// RegisterRoutes mounts company endpoints with explicit permissions.
//
// print-profile takes authentication plus the branch guard but no permission:
// every role that prints a document needs the letterhead (F3a-6).
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm authcontracts.Permission, authn func(func(http.ResponseWriter, *http.Request)) http.Handler) {
	mux.Handle("GET /api/v1/company/print-profile", authn(func(w http.ResponseWriter, r *http.Request) {
		authcontracts.RequireBranch(http.HandlerFunc(h.PrintProfile)).ServeHTTP(w, r)
	}))
	mux.Handle("GET /api/v1/company/profile", perm("company.view", h.GetProfile))
	mux.Handle("PUT /api/v1/company/profile", perm("company.manage", h.SaveProfile))
	mux.Handle("GET /api/v1/company/settings", perm("company.view", h.GetSettings))
	mux.Handle("PUT /api/v1/company/settings", perm("company.manage", h.SaveSettings))
	mux.Handle("GET /api/v1/company/settings/definitions", perm("company.view", h.SettingDefinitions))
}
