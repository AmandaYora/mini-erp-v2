package presentation

import (
	"net/http"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/finance/application"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/response"
)

// Rekening perusahaan (ADR-0026).

type bankAccountPayload struct {
	AccountID       int64  `json:"accountId"`
	AccountCode     string `json:"accountCode"`
	AccountName     string `json:"accountName"`
	BankName        string `json:"bankName"`
	AccountNumber   string `json:"accountNumber"`
	AccountHolder   string `json:"accountHolder"`
	ShowOnDocuments bool   `json:"showOnDocuments"`
	IsDefault       bool   `json:"isDefault"`
	SortOrder       int    `json:"sortOrder"`
}

func (p bankAccountPayload) input() application.BankAccountInput {
	return application.BankAccountInput{
		AccountID: p.AccountID, AccountCode: p.AccountCode, AccountName: p.AccountName,
		BankName: p.BankName, AccountNumber: p.AccountNumber, AccountHolder: p.AccountHolder,
		ShowOnDocuments: p.ShowOnDocuments, IsDefault: p.IsDefault, SortOrder: p.SortOrder,
	}
}

// ListBankAccounts handles GET /api/v1/finance/bank-accounts (finance.view)
// and GET /api/v1/finance/bank-accounts/options (any signed-in user with a
// branch: cashiers pick the rekening, printing shows it). Both return every
// rekening, archived ones included so old documents keep their label;
// pickers offer only status=active.
func (h *Handler) ListBankAccounts(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListBankAccounts(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Data rekening", items)
}

// CreateBankAccount handles POST /api/v1/finance/bank-accounts.
func (h *Handler) CreateBankAccount(w http.ResponseWriter, r *http.Request) {
	var in bankAccountPayload
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	b, err := h.svc.CreateBankAccount(r.Context(), authcontracts.ActorID(r), in.input())
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Rekening ditambahkan", b)
}

// UpdateBankAccount handles PUT /api/v1/finance/bank-accounts/{id}.
func (h *Handler) UpdateBankAccount(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Rekening"))
		return
	}
	var in bankAccountPayload
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	b, appErr := h.svc.UpdateBankAccount(r.Context(), authcontracts.ActorID(r), id, in.input())
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Rekening disimpan", b)
}

// ArchiveBankAccount handles POST /api/v1/finance/bank-accounts/{id}/archive.
func (h *Handler) ArchiveBankAccount(w http.ResponseWriter, r *http.Request) {
	h.setBankAccountStatus(w, r, false, "Rekening diarsipkan")
}

// RestoreBankAccount handles POST /api/v1/finance/bank-accounts/{id}/restore.
func (h *Handler) RestoreBankAccount(w http.ResponseWriter, r *http.Request) {
	h.setBankAccountStatus(w, r, true, "Rekening diaktifkan")
}

func (h *Handler) setBankAccountStatus(w http.ResponseWriter, r *http.Request, active bool, msg string) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Rekening"))
		return
	}
	b, appErr := h.svc.SetBankAccountStatus(r.Context(), authcontracts.ActorID(r), id, active)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, msg, b)
}
