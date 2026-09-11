package presentation

import (
	"net/http"
)

// Permission is injected by the composition root (built from the auth module).
type Permission func(perm string, next func(http.ResponseWriter, *http.Request)) http.Handler

// RegisterRoutes mounts party endpoints with explicit permissions.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm Permission) {
	mux.Handle("GET /api/v1/customers", perm("customers.view", h.ListCustomers))
	mux.Handle("POST /api/v1/customers", perm("customers.create", h.CreateCustomer))
	mux.Handle("GET /api/v1/customers/{id}", perm("customers.view", h.GetCustomer))
	mux.Handle("PUT /api/v1/customers/{id}", perm("customers.update", h.UpdateCustomer))
	mux.Handle("POST /api/v1/customers/{id}/archive", perm("customers.archive", h.ArchiveCustomer))
	mux.Handle("POST /api/v1/customers/{id}/restore", perm("customers.archive", h.RestoreCustomer))

	mux.Handle("GET /api/v1/suppliers", perm("suppliers.view", h.ListSuppliers))
	mux.Handle("POST /api/v1/suppliers", perm("suppliers.create", h.CreateSupplier))
	mux.Handle("GET /api/v1/suppliers/{id}", perm("suppliers.view", h.GetSupplier))
	mux.Handle("PUT /api/v1/suppliers/{id}", perm("suppliers.update", h.UpdateSupplier))
	mux.Handle("POST /api/v1/suppliers/{id}/archive", perm("suppliers.archive", h.ArchiveSupplier))
	mux.Handle("POST /api/v1/suppliers/{id}/restore", perm("suppliers.archive", h.RestoreSupplier))

	mux.Handle("GET /api/v1/customers/{id}/addresses", perm("customers.view", h.ListAddresses))
	mux.Handle("POST /api/v1/customers/{id}/addresses", perm("customers.update", h.CreateAddress))
	mux.Handle("PUT /api/v1/customers/{id}/addresses/{addrId}", perm("customers.update", h.UpdateAddress))
	mux.Handle("POST /api/v1/customers/{id}/addresses/{addrId}/archive", perm("customers.update", h.ArchiveAddress))

	mux.Handle("GET /api/v1/member-types", perm("member_types.manage", h.ListMemberTypes))
	mux.Handle("POST /api/v1/member-types", perm("member_types.manage", h.CreateMemberType))
	mux.Handle("PUT /api/v1/member-types/{id}", perm("member_types.manage", h.UpdateMemberType))
	mux.Handle("POST /api/v1/member-types/{id}/archive", perm("member_types.manage", h.ArchiveMemberType))
	mux.Handle("POST /api/v1/member-types/{id}/restore", perm("member_types.manage", h.RestoreMemberType))

	mux.Handle("POST /api/v1/pricing/quote", perm("pricing.quote", h.Quote))
}
