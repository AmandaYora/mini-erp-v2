package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"mini-erp/internal/bootstrap"
	productapp "mini-erp/internal/modules/product/application"
	stockapp "mini-erp/internal/modules/stock/application"
)

// Pemilih produk berhalaman: search-options dengan `page` menjangkau SETIAP
// produk yang cocok (dulu maks. 20–30 baris dari offset 0, sehingga produk
// sesudahnya tidak pernah bisa dipilih). Halaman tidak tumpang tindih dan
// tanpa `page` perilaku lama tetap.
func TestSearchOptionsPagesReachEveryProduct(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx()) // 1 produk "TST-001" berstok
	const n = 45
	for i := 1; i <= n; i++ {
		f3bProduct(t, productapp.ProductInput{Code: fmt.Sprintf("PG-%03d", i), Name: fmt.Sprintf("Barang Paging %03d", i)})
	}
	svc := testMods.product.Service()

	collect := func(f productapp.ListFilter, o productapp.SearchOptionsOpts, limit int) []string {
		t.Helper()
		var codes []string
		seen := map[string]bool{}
		for page := 1; page <= 10; page++ {
			o.Page = page
			rows, err := svc.SearchOptionsFor(ctx(), f, limit, o)
			if err != nil {
				t.Fatalf("page %d: %v", page, err)
			}
			for _, p := range rows {
				if seen[p.Code] {
					t.Fatalf("page %d repeats %s", page, p.Code)
				}
				seen[p.Code] = true
				codes = append(codes, p.Code)
				if len(p.Variants) == 0 {
					t.Fatalf("%s without variants", p.Code)
				}
			}
			if len(rows) < limit {
				return codes
			}
		}
		t.Fatal("paging did not end")
		return nil
	}

	// Tanpa kata kunci: seluruh produk aktif (45 + produk fixture) terjangkau.
	all := collect(productapp.ListFilter{}, productapp.SearchOptionsOpts{}, 20)
	if len(all) != n+1 {
		t.Fatalf("browse reached %d products, want %d", len(all), n+1)
	}
	// Dengan kata kunci: 45 kecocokan, lebih dari satu halaman, urut relevansi.
	hits := collect(productapp.ListFilter{Search: "PG-"}, productapp.SearchOptionsOpts{}, 20)
	if len(hits) != n || hits[0] != "PG-001" || hits[n-1] != "PG-045" {
		t.Fatalf("search reached %d (%v … %v), want %d in order", len(hits), hits[0], hits[len(hits)-1], n)
	}
	// preferStock berhalaman: tetap lengkap dan tidak berulang (urutan stok per halaman).
	if err := testMods.stock.Service().Adjust(ctx(), 0, fx.branchID, stockapp.AdjustInput{
		ProductID: fx.productID, VariantID: fx.variantID, Location: fx.locationID, Mode: "in", QtyDelta: 1, Reason: "uji paging",
	}); err != nil {
		t.Fatalf("adjust: %v", err)
	}
	pref := collect(productapp.ListFilter{}, productapp.SearchOptionsOpts{PreferStock: true, BranchID: fx.branchID}, 20)
	if len(pref) != n+1 {
		t.Fatalf("preferStock paging reached %d, want %d", len(pref), n+1)
	}
	// Tanpa page: hanya baris pertama, seperti sebelumnya.
	first, err := svc.SearchOptionsFor(ctx(), productapp.ListFilter{Search: "PG-"}, 20, productapp.SearchOptionsOpts{})
	if err != nil || len(first) != 20 || first[0].Code != "PG-001" {
		t.Fatalf("unpaged = %d rows %v", len(first), err)
	}

	// Lewat HTTP: parameter page dibaca handler.
	if err := bootstrap.EnsureAuthz(ctx(), testMods.user.Service()); err != nil {
		t.Fatalf("seed authz: %v", err)
	}
	h := routeServer(t).Handler()
	tok := guardUser(t, fx.branchID, "pg-viewer", []string{"products.view"})
	page := func(p int) []string {
		rec := doReq(h, http.MethodGet, fmt.Sprintf("/api/v1/products/search-options?q=PG-&limit=20&page=%d", p), tok, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("page %d = %d %s", p, rec.Code, rec.Body.String())
		}
		var body struct {
			Data []struct {
				Code string `json:"code"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		out := make([]string, 0, len(body.Data))
		for _, r := range body.Data {
			out = append(out, r.Code)
		}
		return out
	}
	p3 := page(3)
	if len(p3) != 5 || p3[0] != "PG-041" || p3[4] != "PG-045" {
		t.Fatalf("http page 3 = %v, want PG-041..PG-045", p3)
	}
	if p1 := page(1); len(p1) != 20 || p1[0] != "PG-001" {
		t.Fatalf("http page 1 = %v", p1)
	}
}
