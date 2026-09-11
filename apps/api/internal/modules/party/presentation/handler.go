package presentation

import (
	"net/http"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/party/application"
	"mini-erp/internal/modules/party/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
)

// Handler serves the party module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires party endpoints.
func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func actorID(r *http.Request) int64 {
	if s := authcontracts.SessionFromContext(r.Context()); s != nil {
		return s.UserID
	}
	return 0
}

func partyView(p *contracts.Party) map[string]any {
	var email, member any
	if p.Email != "" {
		email = p.Email
	}
	if p.MemberTypeID != nil {
		member = map[string]any{"id": *p.MemberTypeID, "code": p.MemberCode}
	}
	return map[string]any{
		"id": p.ID, "code": p.Code, "type": p.Type, "name": p.Name,
		"phone": p.Phone, "email": email, "address": p.Address, "notes": p.Notes,
		"member": member, "status": p.Status,
	}
}

func addressView(a *contracts.Address) map[string]any {
	return map[string]any{
		"id": a.ID, "label": a.Label, "recipient": a.Recipient, "phone": a.Phone,
		"text": a.Text, "isPrimary": a.IsPrimary, "sortOrder": a.SortOrder, "status": a.Status,
	}
}

func memberView(m *contracts.MemberType) map[string]any {
	return map[string]any{
		"id": m.ID, "code": m.Code, "name": m.Name, "description": m.Description,
		"basis": m.PriceBasis, "direction": m.Direction, "type": m.AdjustmentType,
		"value": m.Adjustment, "roundingMode": m.RoundingMode, "roundingStep": m.RoundingStep,
		"status": m.Status,
	}
}

type partyPayload struct {
	Code         string `json:"code"`
	Name         string `json:"name" validate:"required"`
	Phone        string `json:"phone"`
	Email        string `json:"email" validate:"omitempty,email"`
	Address      string `json:"address"`
	Notes        string `json:"notes"`
	MemberTypeID *int64 `json:"memberTypeId"`
	ClearMember  bool   `json:"clearMember"`
}

func toInput(in partyPayload) application.PartyInput {
	return application.PartyInput{
		Code: in.Code, Name: in.Name, Phone: in.Phone, Email: in.Email,
		Address: in.Address, Notes: in.Notes,
		MemberTypeID: in.MemberTypeID, ClearMember: in.ClearMember,
	}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request, partyType, label string) {
	q := r.URL.Query()
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	res, err := h.svc.List(r.Context(), partyType, q.Get("search"), q.Get("status"), page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(res.Parties))
	for _, p := range res.Parties {
		items = append(items, partyView(p))
	}
	response.Paginated(w, label, items, response.Meta{
		Page: page, Limit: limit, Total: res.Total,
		TotalPages: pagination.TotalPages(res.Total, limit),
	})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request, partyType string) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound(partyLabel(partyType)))
		return
	}
	p, appErr := h.svc.GetByID(r.Context(), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	if p == nil || p.Type != partyType {
		response.Fail(w, apperror.NotFound(partyLabel(partyType)))
		return
	}
	addresses, appErr := h.svc.ListAddresses(r.Context(), id, partyType)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	items := make([]any, 0, len(addresses))
	for _, a := range addresses {
		items = append(items, addressView(a))
	}
	view := partyView(p)
	view["addresses"] = items
	response.OK(w, partyLabel(partyType), view)
}

func partyLabel(partyType string) string {
	if partyType == contracts.PartySupplier {
		return "Supplier"
	}
	return "Customer"
}

// ListCustomers handles GET /api/v1/customers.
func (h *Handler) ListCustomers(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, contracts.PartyCustomer, "Data customer")
}

// ListSuppliers handles GET /api/v1/suppliers.
func (h *Handler) ListSuppliers(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, contracts.PartySupplier, "Data supplier")
}

// GetCustomer handles GET /api/v1/customers/{id}.
func (h *Handler) GetCustomer(w http.ResponseWriter, r *http.Request) {
	h.get(w, r, contracts.PartyCustomer)
}

// GetSupplier handles GET /api/v1/suppliers/{id}.
func (h *Handler) GetSupplier(w http.ResponseWriter, r *http.Request) {
	h.get(w, r, contracts.PartySupplier)
}

// CreateCustomer handles POST /api/v1/customers.
func (h *Handler) CreateCustomer(w http.ResponseWriter, r *http.Request) {
	var in partyPayload
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	p, err := h.svc.CreateCustomer(r.Context(), actorID(r), toInput(in))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Customer dibuat", partyView(p))
}

// CreateSupplier handles POST /api/v1/suppliers.
func (h *Handler) CreateSupplier(w http.ResponseWriter, r *http.Request) {
	var in partyPayload
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	p, err := h.svc.CreateSupplier(r.Context(), actorID(r), toInput(in))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Supplier dibuat", partyView(p))
}

// UpdateCustomer handles PUT /api/v1/customers/{id}.
func (h *Handler) UpdateCustomer(w http.ResponseWriter, r *http.Request) {
	h.update(w, r, contracts.PartyCustomer)
}

// UpdateSupplier handles PUT /api/v1/suppliers/{id}.
func (h *Handler) UpdateSupplier(w http.ResponseWriter, r *http.Request) {
	h.update(w, r, contracts.PartySupplier)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request, partyType string) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound(partyLabel(partyType)))
		return
	}
	var in partyPayload
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	var p *contracts.Party
	if partyType == contracts.PartySupplier {
		p, err = h.svc.UpdateSupplier(r.Context(), actorID(r), id, toInput(in))
	} else {
		p, err = h.svc.UpdateCustomer(r.Context(), actorID(r), id, toInput(in))
	}
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, partyLabel(partyType)+" disimpan", partyView(p))
}

func (h *Handler) setStatus(w http.ResponseWriter, r *http.Request, partyType, status, doneMsg string) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound(partyLabel(partyType)))
		return
	}
	var appErr error
	if status == "archived" {
		appErr = h.svc.Archive(r.Context(), actorID(r), id, partyType)
	} else {
		appErr = h.svc.Restore(r.Context(), actorID(r), id, partyType)
	}
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, doneMsg, nil)
}

// ArchiveCustomer handles POST /api/v1/customers/{id}/archive.
func (h *Handler) ArchiveCustomer(w http.ResponseWriter, r *http.Request) {
	h.setStatus(w, r, contracts.PartyCustomer, "archived", "Customer diarsipkan")
}

// RestoreCustomer handles POST /api/v1/customers/{id}/restore.
func (h *Handler) RestoreCustomer(w http.ResponseWriter, r *http.Request) {
	h.setStatus(w, r, contracts.PartyCustomer, "active", "Customer dipulihkan")
}

// ArchiveSupplier handles POST /api/v1/suppliers/{id}/archive.
func (h *Handler) ArchiveSupplier(w http.ResponseWriter, r *http.Request) {
	h.setStatus(w, r, contracts.PartySupplier, "archived", "Supplier diarsipkan")
}

// RestoreSupplier handles POST /api/v1/suppliers/{id}/restore.
func (h *Handler) RestoreSupplier(w http.ResponseWriter, r *http.Request) {
	h.setStatus(w, r, contracts.PartySupplier, "active", "Supplier dipulihkan")
}

type addressPayload struct {
	Label     string `json:"label"`
	Recipient string `json:"recipient"`
	Phone     string `json:"phone"`
	Text      string `json:"text" validate:"required"`
	IsPrimary bool   `json:"isPrimary"`
	SortOrder int    `json:"sortOrder"`
}

func addressInput(in addressPayload) application.AddressInput {
	return application.AddressInput{
		Label: in.Label, Recipient: in.Recipient, Phone: in.Phone,
		Text: in.Text, IsPrimary: in.IsPrimary, SortOrder: in.SortOrder,
	}
}

// ListAddresses handles GET /api/v1/customers/{id}/addresses.
func (h *Handler) ListAddresses(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Customer"))
		return
	}
	addresses, appErr := h.svc.ListAddresses(r.Context(), id, contracts.PartyCustomer)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	items := make([]any, 0, len(addresses))
	for _, a := range addresses {
		items = append(items, addressView(a))
	}
	response.OK(w, "Buku alamat", items)
}

// CreateAddress handles POST /api/v1/customers/{id}/addresses.
func (h *Handler) CreateAddress(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Customer"))
		return
	}
	var in addressPayload
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	a, appErr := h.svc.CreateAddress(r.Context(), actorID(r), id, contracts.PartyCustomer, addressInput(in))
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.Created(w, "Alamat ditambahkan", addressView(a))
}

// UpdateAddress handles PUT /api/v1/customers/{id}/addresses/{addrId}.
func (h *Handler) UpdateAddress(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Customer"))
		return
	}
	addrID, err := httpx.PathInt(r, "addrId")
	if err != nil {
		response.Fail(w, apperror.NotFound("Alamat"))
		return
	}
	var in addressPayload
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	a, appErr := h.svc.UpdateAddress(r.Context(), actorID(r), id, contracts.PartyCustomer, addrID, addressInput(in))
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Alamat disimpan", addressView(a))
}

// ArchiveAddress handles POST /api/v1/customers/{id}/addresses/{addrId}/archive.
func (h *Handler) ArchiveAddress(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Customer"))
		return
	}
	addrID, err := httpx.PathInt(r, "addrId")
	if err != nil {
		response.Fail(w, apperror.NotFound("Alamat"))
		return
	}
	if appErr := h.svc.ArchiveAddress(r.Context(), actorID(r), id, contracts.PartyCustomer, addrID); appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Alamat diarsipkan", nil)
}

type memberPayload struct {
	Code        string  `json:"code" validate:"required"`
	Name        string  `json:"name" validate:"required"`
	Description string  `json:"description"`
	Basis       string  `json:"basis"`
	Direction   string  `json:"direction"`
	Type        string  `json:"type"`
	Value       float64 `json:"value"`
	Mode        string  `json:"mode"`
	Step        float64 `json:"step"`
	Confirmed   bool    `json:"confirmed"`
}

func memberInput(in memberPayload) application.MemberTypeInput {
	return application.MemberTypeInput{
		Code: in.Code, Name: in.Name, Description: in.Description,
		Basis: in.Basis, Direction: in.Direction, Type: in.Type,
		Value: in.Value, Mode: in.Mode, Step: in.Step, Confirmed: in.Confirmed,
	}
}

// ListMemberTypes handles GET /api/v1/member-types.
func (h *Handler) ListMemberTypes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	members, err := h.svc.ListMemberTypes(r.Context(), q.Get("search"), q.Get("status"))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(members))
	for _, m := range members {
		items = append(items, memberView(m))
	}
	response.OK(w, "Data tipe member", items)
}

// CreateMemberType handles POST /api/v1/member-types.
func (h *Handler) CreateMemberType(w http.ResponseWriter, r *http.Request) {
	var in memberPayload
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	m, err := h.svc.CreateMemberType(r.Context(), actorID(r), memberInput(in))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Tipe member dibuat", memberView(m))
}

// UpdateMemberType handles PUT /api/v1/member-types/{id}.
// Code is immutable after create and is not part of this payload.
func (h *Handler) UpdateMemberType(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Tipe member"))
		return
	}
	var in struct {
		Name        string  `json:"name" validate:"required"`
		Description string  `json:"description"`
		Basis       string  `json:"basis"`
		Direction   string  `json:"direction"`
		Type        string  `json:"type"`
		Value       float64 `json:"value"`
		Mode        string  `json:"mode"`
		Step        float64 `json:"step"`
		Confirmed   bool    `json:"confirmed"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	m, appErr := h.svc.UpdateMemberType(r.Context(), actorID(r), id, application.MemberTypeInput{
		Name: in.Name, Description: in.Description,
		Basis: in.Basis, Direction: in.Direction, Type: in.Type,
		Value: in.Value, Mode: in.Mode, Step: in.Step, Confirmed: in.Confirmed,
	})
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Tipe member disimpan", memberView(m))
}

// ArchiveMemberType handles POST /api/v1/member-types/{id}/archive.
func (h *Handler) ArchiveMemberType(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Tipe member"))
		return
	}
	if appErr := h.svc.ArchiveMemberType(r.Context(), actorID(r), id); appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Tipe member diarsipkan", nil)
}

// RestoreMemberType handles POST /api/v1/member-types/{id}/restore.
func (h *Handler) RestoreMemberType(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Tipe member"))
		return
	}
	if appErr := h.svc.RestoreMemberType(r.Context(), actorID(r), id); appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Tipe member dipulihkan", nil)
}

// Quote handles POST /api/v1/pricing/quote.
func (h *Handler) Quote(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CustomerID int64   `json:"customerId"`
		ProductIDs []int64 `json:"productIds" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	res, err := h.svc.Quote(r.Context(), in.CustomerID, in.ProductIDs)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Quote harga", res)
}
