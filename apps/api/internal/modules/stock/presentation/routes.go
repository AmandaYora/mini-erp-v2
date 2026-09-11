package presentation

import (
	"net/http"

	authcontracts "mini-erp/internal/modules/auth/contracts"
)

// Permission is injected by the composition root (built from the auth module).
type Permission func(perm string, next func(http.ResponseWriter, *http.Request)) http.Handler

// guarded adapts the branch guard into func-style middleware, preserving the
// order auth → permission → branch → handler.
func guarded(next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return authcontracts.RequireBranch(http.HandlerFunc(next)).ServeHTTP
}

// RegisterRoutes mounts stock endpoints with explicit permissions.
// RequireBranch guards every business route (active selected branch).
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm Permission) {
	mux.Handle("GET /api/v1/stock/locations", perm("stock.view", guarded(h.ListLocations)))
	mux.Handle("POST /api/v1/stock/locations", perm("stock.manage", guarded(h.CreateLocation)))
	mux.Handle("PUT /api/v1/stock/locations/{id}", perm("stock.manage", guarded(h.UpdateLocation)))
	mux.Handle("POST /api/v1/stock/locations/{id}/archive", perm("stock.manage", guarded(h.ArchiveLocation)))

	mux.Handle("GET /api/v1/stock/balances", perm("stock.view", guarded(h.Balances)))
	mux.Handle("GET /api/v1/stock/balances/{productId}", perm("stock.view", guarded(h.BalanceDetail)))
	mux.Handle("GET /api/v1/stock/movements", perm("stock.view", guarded(h.Movements)))
	mux.Handle("POST /api/v1/stock/scan-product", perm("stock.view", guarded(h.Scan)))
	mux.Handle("POST /api/v1/stock/allocations/suggest", perm("stock.view", guarded(h.Suggest)))

	mux.Handle("POST /api/v1/stock/reservations/hold", perm("stock.manage", guarded(h.Hold)))
	mux.Handle("POST /api/v1/stock/reservations/release", perm("stock.manage", guarded(h.Release)))

	mux.Handle("POST /api/v1/stock/adjustments", perm("stock.adjust", guarded(h.Adjust)))

	mux.Handle("GET /api/v1/stock/transfers", perm("stock.view", guarded(h.ListTransfers)))
	mux.Handle("POST /api/v1/stock/transfers", perm("stock.manage", guarded(h.CreateTransfer)))
	mux.Handle("POST /api/v1/stock/transfers/move-location", perm("stock.manage", guarded(h.MoveLocation)))
	mux.Handle("GET /api/v1/stock/transfers/{id}", perm("stock.view", guarded(h.GetTransfer)))
	mux.Handle("POST /api/v1/stock/transfers/{id}/dispatch", perm("stock.manage", guarded(h.DispatchTransfer)))
	mux.Handle("POST /api/v1/stock/transfers/{id}/receive", perm("stock.manage", guarded(h.ReceiveTransfer)))
	mux.Handle("POST /api/v1/stock/transfers/{id}/cancel", perm("stock.manage", guarded(h.CancelTransfer)))

	mux.Handle("GET /api/v1/stock/damaged", perm("stock.view", guarded(h.DamagedList)))
	mux.Handle("POST /api/v1/stock/damaged/move-in", perm("stock.manage", guarded(h.DamagedMoveIn)))
	mux.Handle("POST /api/v1/stock/damaged/restore", perm("stock.manage", guarded(h.DamagedRestore)))
	mux.Handle("POST /api/v1/stock/damaged/write-off", perm("stock.manage", guarded(h.DamagedWriteOff)))

	mux.Handle("GET /api/v1/stock/opening/template", perm("stock.manage", guarded(h.OpeningTemplate)))
	mux.Handle("POST /api/v1/stock/opening/preview", perm("stock.manage", guarded(h.OpeningPreview)))
	mux.Handle("POST /api/v1/stock/opening/commit", perm("stock.manage", guarded(h.OpeningCommit)))
}
