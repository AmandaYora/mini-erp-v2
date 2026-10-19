package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"mini-erp/internal/bootstrap"
)

// TestProductScanLookupEndpoint (PR-10): GET /products/scan resolves a typed or
// scanned code from the catalog alone — it needs products.view, never
// stock.view — and unknown / empty codes are answered cleanly.
func TestProductScanLookupEndpoint(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	if err := bootstrap.EnsureAuthz(ctx(), testMods.user.Service()); err != nil {
		t.Fatalf("seed authz: %v", err)
	}
	h := routeServer(t).Handler()

	viewer := guardUser(t, fx.branchID, "scan-viewer", []string{"products.view"})
	nobody := guardUser(t, fx.branchID, "scan-nobody", []string{"sales.view"})

	rec := doReq(h, http.MethodGet, "/api/v1/products/scan?code=TST-001", viewer, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("known code with products.view only: %d %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Success bool `json:"success"`
		Data    struct {
			ID   int64  `json:"id"`
			Code string `json:"code"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !env.Success || env.Data.ID != fx.productID || env.Data.Code != "TST-001" {
		t.Fatalf("scan data = %+v, want product %d TST-001", env.Data, fx.productID)
	}

	if rec := doReq(h, http.MethodGet, "/api/v1/products/scan?code=TIDAK-ADA-999", viewer, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown code: %d, want 404", rec.Code)
	}
	if rec := doReq(h, http.MethodGet, "/api/v1/products/scan?code=", viewer, ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty code: %d, want 400", rec.Code)
	}
	if rec := doReq(h, http.MethodGet, "/api/v1/products/scan?code=TST-001", nobody, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("without products.view: %d, want 403", rec.Code)
	}
	if rec := doReq(h, http.MethodGet, "/api/v1/products/scan?code=TST-001", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: %d, want 401", rec.Code)
	}
}
