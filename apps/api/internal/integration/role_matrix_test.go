package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"

	"mini-erp/internal/bootstrap"
	authcontracts "mini-erp/internal/modules/auth/contracts"
	userapp "mini-erp/internal/modules/user/application"
	usercontracts "mini-erp/internal/modules/user/contracts"
)

// T-01 (padanan 18-role-access-matrix legacy): matriks role seed x izin x rute.
//
// route_guard_test.go sudah membuktikan tiap rute punya guard izin dan menolak
// sesi tanpa izin. Tes ini menambah lapisan ROLE SEED: untuk tiap role sistem
// (owner, admin, staff, kasir) ia memeriksa
//  1. deklarasi matriks `roleMatrix` meliputi PERSIS katalog izin (izin baru
//     tanpa deklarasi => gagal; deklarasi usang => gagal),
//  2. izin nyata role di DB (lewat GET /auth/me) == deklarasi == DefaultGrants
//     (legacymap.go) — owner = seluruh katalog,
//  3. setiap rute terdaftar (dipindai dari sumber presentation): role TANPA
//     izin rute itu => 403 "tidak memiliki akses"; role DENGAN izin => GET
//     lolos guard (bukan 401/403-izin). Rute non-GET yang diizinkan tidak
//     dieksekusi (efek samping) — izin rute dibuktikan lewat guard-nya.
//
// Kode role: o=owner, a=admin, s=staff, k=kasir. Dump tabel baru dengan
// ROLE_MATRIX_DUMP=1 go test -run TestRoleSeedMatrix.
var roleMatrix = map[string]string{
	"assistant.manage":              "o",
	"assistant.simulate":            "o",
	"assistant.view":                "oa",
	"audit.view":                    "oa",
	"branches.manage":               "o",
	"branches.view":                 "oa",
	"company.manage":                "o",
	"company.view":                  "oa",
	"customers.archive":             "oa",
	"customers.create":              "oask",
	"customers.update":              "oask",
	"customers.view":                "oask",
	"dashboard.view":                "oask",
	"delivery.archive":              "oask",
	"delivery.create":               "oask",
	"delivery.create_direct":        "o", // ADR-0022 K5: hanya owner; pemilik memberikan per role
	"delivery.view":                 "oask",
	"finance.close.force":           "o",
	"finance.expense.cancel":        "o",
	"finance.expense.create":        "oa",
	"finance.expense.view":          "oa",
	"finance.journal.reverse":       "o",
	"finance.journal.view":          "oa",
	"finance.manage":                "o",
	"finance.posting.post":          "o",
	"finance.posting.view":          "oa",
	"finance.report.all_branches":   "o",
	"finance.report.view":           "oa",
	"finance.tax_adjustment.manage": "o",
	"finance.tax_adjustment.view":   "oa",
	"finance.tax_report.view":       "oa",
	"finance.view":                  "oa",
	"goodsreceipt.create":           "oask",
	"goodsreceipt.view":             "oask",
	"member_types.manage":           "oa",
	"member_types.view":             "oa",
	"payment.archive":               "oa",
	"payment.create":                "oask",
	"payment.view":                  "oask",
	"pricing.quote":                 "oask",
	"product_categories.manage":     "oa",
	"product_import.manage":         "oa",
	"products.archive":              "oa",
	"products.create":               "oa",
	"products.update":               "oa",
	"products.view":                 "oask",
	"purchasereturn.archive":        "oa",
	"purchasereturn.create":         "oa",
	"purchasereturn.refund":         "oa",
	"purchasereturn.view":           "oas",
	"purchasing.archive":            "oa",
	"purchasing.create":             "oask",
	"purchasing.export":             "oa",
	"purchasing.update":             "oask",
	"purchasing.view":               "oask",
	"reporting.view":                "oa",
	"roles.manage":                  "oa",
	"roles.view":                    "oa",
	"sales.approve":                 "oask",
	"sales.archive":                 "oa",
	"sales.create":                  "oask",
	"sales.export":                  "oa",
	"sales.update":                  "oask",
	"sales.view":                    "oask",
	"salesreturn.archive":           "oa",
	"salesreturn.create":            "oask",
	"salesreturn.refund":            "oa",
	"salesreturn.view":              "oask",
	"stock.adjust":                  "oask",
	"stock.approve":                 "oa",
	"stock.reserve":                 "oask",
	"stock.transfer":                "oa",
	"stock.update":                  "oa",
	"stock.view":                    "oask",
	"suppliers.archive":             "oa",
	"suppliers.create":              "oask",
	"suppliers.update":              "oask",
	"suppliers.view":                "oask",
	"users.archive":                 "o",
	"users.create":                  "o",
	"users.update":                  "o",
	"users.view":                    "oa",
}

var matrixRoles = []struct {
	code   string
	letter string
}{{"owner", "o"}, {"admin", "a"}, {"staff", "s"}, {"kasir", "k"}}

func TestRoleSeedMatrix(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.user.Service()
	if err := bootstrap.EnsureAuthz(ctx(), svc); err != nil {
		t.Fatalf("seed authz: %v", err)
	}
	srv := routeServer(t)
	h := srv.Handler()
	mux := srv.Mux()

	catalog := bootstrap.AllPermissionCodes()
	inCatalog := map[string]bool{}
	for _, c := range catalog {
		inCatalog[c] = true
	}

	// Token per role seed.
	tokens := map[string]string{}
	actual := map[string]map[string]bool{}
	for _, r := range matrixRoles {
		role, err := svc.GetRoleByCode(ctx(), r.code)
		if err != nil || role == nil {
			t.Fatalf("role %s tidak ada: %v", r.code, err)
		}
		name := "matrix-" + r.code
		if _, err := svc.CreateUser(ctx(), 0, userapp.CreateUserInput{
			Username: name, Password: "rahasia-kuat-1", RoleIDs: []int64{role.ID},
			Branches: []usercontracts.BranchAccess{{BranchID: fx.branchID, IsDefault: true}},
		}); err != nil {
			t.Fatalf("user %s: %v", name, err)
		}
		tokens[r.code] = login(t, name, "rahasia-kuat-1")
		rec := doReq(h, http.MethodGet, "/api/v1/auth/me", tokens[r.code], "")
		if rec.Code != http.StatusOK {
			t.Fatalf("auth/me %s = %d %s", r.code, rec.Code, rec.Body.String())
		}
		var env struct {
			Data struct {
				Permissions []string `json:"permissions"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		actual[r.code] = map[string]bool{}
		for _, p := range env.Data.Permissions {
			actual[r.code][p] = true
		}
	}

	if os.Getenv("ROLE_MATRIX_DUMP") != "" {
		codes := append([]string(nil), catalog...)
		sort.Strings(codes)
		var b strings.Builder
		for _, c := range codes {
			l := ""
			for _, r := range matrixRoles {
				if actual[r.code][c] {
					l += r.letter
				}
			}
			fmt.Fprintf(&b, "\t%q: %q,\n", c, l)
		}
		_ = os.WriteFile(os.Getenv("ROLE_MATRIX_DUMP"), []byte(b.String()), 0o644)
	}

	// 1. Deklarasi == katalog.
	for _, c := range catalog {
		if _, ok := roleMatrix[c]; !ok {
			t.Errorf("izin %q ada di katalog tetapi belum dideklarasikan di roleMatrix (tentukan role mana yang memegangnya)", c)
		}
	}
	for c := range roleMatrix {
		if !inCatalog[c] {
			t.Errorf("roleMatrix memuat %q yang tak ada di katalog (usang)", c)
		}
	}

	// 2. Izin nyata == deklarasi == DefaultGrants.
	grants := bootstrap.DefaultGrants()
	for _, r := range matrixRoles {
		for _, c := range catalog {
			want := strings.Contains(roleMatrix[c], r.letter)
			if got := actual[r.code][c]; got != want {
				t.Errorf("role %s izin %q: nyata=%v, deklarasi=%v", r.code, c, got, want)
			}
		}
		if r.code == "owner" {
			if len(actual[r.code]) != len(catalog) {
				t.Errorf("owner memegang %d izin, katalog %d", len(actual[r.code]), len(catalog))
			}
			continue
		}
		g := map[string]bool{}
		for _, c := range grants[r.code] {
			g[c] = true
		}
		for _, c := range catalog {
			if g[c] != strings.Contains(roleMatrix[c], r.letter) {
				t.Errorf("role %s izin %q: DefaultGrants(legacymap)=%v, deklarasi=%v", r.code, c, g[c], !g[c])
			}
		}
	}

	// 3. Rute x role.
	routes := scanRoutes(t)
	var checked, denied, allowedLive, allowedGuardOnly int
	for _, rt := range routes {
		pat := rt.pattern()
		if publicRoutes[pat] || authnOnlyRoutes[pat] {
			continue
		}
		path := concretePath(rt.path)
		req, _ := http.NewRequest(rt.method, path, nil)
		handler, matched := mux.Handler(req)
		if matched != pat {
			t.Errorf("%s: tidak terdaftar di tabel rute nyata", pat)
			continue
		}
		g, ok := handler.(authcontracts.PermissionGuarded)
		if !ok {
			t.Errorf("%s: tanpa guard izin", pat)
			continue
		}
		perm := g.RequiredPermission()
		holders, declared := roleMatrix[perm]
		if !declared {
			t.Errorf("%s menuntut izin %q yang belum ada di roleMatrix", pat, perm)
			continue
		}
		for _, r := range matrixRoles {
			checked++
			has := strings.Contains(holders, r.letter)
			if !has {
				denied++
				rec := doReq(h, rt.method, path, tokens[r.code], "")
				if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "tidak memiliki akses") {
					t.Errorf("%s sebagai %s (tanpa %s) = %d %s, want 403 tidak memiliki akses", pat, r.code, perm, rec.Code, rec.Body.String())
				}
				continue
			}
			if rt.method != http.MethodGet {
				allowedGuardOnly++
				continue
			}
			allowedLive++
			rec := doReq(h, rt.method, path, tokens[r.code], "")
			if rec.Code == http.StatusUnauthorized ||
				(rec.Code == http.StatusForbidden && strings.Contains(rec.Body.String(), "tidak memiliki akses")) {
				t.Errorf("%s sebagai %s (punya %s) ditolak: %d %s", pat, r.code, perm, rec.Code, rec.Body.String())
			}
		}
	}
	t.Logf("rute x role: %d pasangan (ditolak %d, diizinkan GET %d, diizinkan non-GET via guard %d)", checked, denied, allowedLive, allowedGuardOnly)
	if checked < 150*len(matrixRoles) {
		t.Fatalf("hanya %d pasangan diperiksa; pemindai rute rusak?", checked)
	}
}
