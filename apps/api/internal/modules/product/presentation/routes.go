package presentation

import (
	"net/http"

	mediacontracts "mini-erp/internal/modules/media/contracts"
)

// importMaxBytes caps import workbooks at 10 MB.
const importMaxBytes = 10 << 20

// Permission is injected by the composition root (built from the auth module).
type Permission func(perm string, next func(http.ResponseWriter, *http.Request)) http.Handler

// RegisterRoutes mounts product endpoints with explicit permissions.
// Media endpoints close over the media contract client — bytes never cross
// module internals.
func RegisterRoutes(mux *http.ServeMux, h *Handler, media mediacontracts.MediaClient, perm Permission) {
	mux.Handle("GET /api/v1/products", perm("products.view", h.List))
	mux.Handle("POST /api/v1/products", perm("products.create", h.Create))
	mux.Handle("GET /api/v1/products/search-options", perm("products.view", h.SearchOptions))
	mux.Handle("GET /api/v1/products/qr-codes/export", perm("products.view", h.QRExport))
	mux.Handle("GET /api/v1/products/{id}", perm("products.view", h.Get))
	mux.Handle("GET /api/v1/products/{id}/qr", perm("products.view", h.QR))
	mux.Handle("PUT /api/v1/products/{id}", perm("products.update", h.Update))
	mux.Handle("POST /api/v1/products/{id}/archive", perm("products.archive", h.Archive))

	mux.Handle("GET /api/v1/product-categories", perm("products.view", h.ListCategories))
	mux.Handle("POST /api/v1/product-categories", perm("product_categories.manage", h.CreateCategory))
	mux.Handle("PUT /api/v1/product-categories/{id}", perm("product_categories.manage", h.UpdateCategory))
	mux.Handle("POST /api/v1/product-categories/{id}/archive", perm("product_categories.manage", h.ArchiveCategory))

	mux.Handle("GET /api/v1/products/{id}/media", perm("products.view",
		func(w http.ResponseWriter, r *http.Request) { h.MediaList(w, r, media) }))
	mux.Handle("POST /api/v1/products/{id}/media", perm("products.update",
		func(w http.ResponseWriter, r *http.Request) { h.MediaUpload(w, r, media) }))
	mux.Handle("POST /api/v1/products/{id}/media/{mediaId}/primary", perm("products.update",
		func(w http.ResponseWriter, r *http.Request) { h.MediaSetPrimary(w, r, media) }))
	mux.Handle("DELETE /api/v1/products/{id}/media/{mediaId}", perm("products.update",
		func(w http.ResponseWriter, r *http.Request) { h.MediaArchive(w, r, media) }))

	mux.Handle("GET /api/v1/product-imports/template", perm("product_import.manage", h.Template))
	mux.Handle("POST /api/v1/product-imports/preview", perm("product_import.manage",
		func(w http.ResponseWriter, r *http.Request) { h.Preview(w, r, importMaxBytes) }))
	mux.Handle("POST /api/v1/product-imports/commit", perm("product_import.manage",
		func(w http.ResponseWriter, r *http.Request) { h.Commit(w, r, importMaxBytes) }))
}
