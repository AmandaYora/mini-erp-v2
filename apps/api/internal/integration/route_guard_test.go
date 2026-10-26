package integration

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"mini-erp/internal/bootstrap"
	"mini-erp/internal/config"
	authcontracts "mini-erp/internal/modules/auth/contracts"
	userapp "mini-erp/internal/modules/user/application"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/server"
)

// F3b-2 route registry. Every route the modules register must (a) exist in
// the real route table, (b) be public / authn-only on an explicit allowlist or
// else carry the auth+permission guard built by authMod.Permission (which
// records the permission it demands), (c) carry the branch guard unless it is
// on the explicit "works without a branch" list. Dispatch is real HTTP.

var routeDeclRe = regexp.MustCompile(`mux\.Handle(?:Func)?\(\s*"([A-Z]+) (/[^"]*)"`)
var pathParamRe = regexp.MustCompile(`\{[^}]*\}`)

type routeDecl struct{ method, path string }

func (r routeDecl) pattern() string { return r.method + " " + r.path }

// scanRoutes reads the declared patterns from the presentation sources (the
// ServeMux cannot be enumerated). Test files are skipped.
func scanRoutes(t *testing.T) []routeDecl {
	t.Helper()
	var files []string
	for _, glob := range []string{
		"../modules/*/presentation/*.go",
		"../bootstrap/ops.go",
	} {
		m, err := filepath.Glob(glob)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	seen := map[string]bool{}
	var out []routeDecl
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range routeDeclRe.FindAllStringSubmatch(string(raw), -1) {
			d := routeDecl{method: m[1], path: m[2]}
			if !seen[d.pattern()] {
				seen[d.pattern()] = true
				out = append(out, d)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].pattern() < out[j].pattern() })
	return out
}

// publicRoutes need no session at all.
var publicRoutes = map[string]bool{
	"POST /api/v1/auth/login":   true,
	"POST /api/v1/auth/refresh": true,
}

// authnOnlyRoutes need a session but no permission and no branch (selection
// and identity endpoints that a branchless session must reach).
var authnOnlyRoutes = map[string]bool{
	"POST /api/v1/auth/logout":        true,
	"GET /api/v1/auth/me":             true,
	"POST /api/v1/auth/switch-branch": true,
	"POST /api/v1/auth/switch-role":   true,
	"GET /api/v1/branches/my-access":  true,
	// F3a-6: printing needs the letterhead for every role; the handler adds
	// RequireBranch itself and returns only an allow-list (covered by
	// TestPrintProfileAllowList).
	"GET /api/v1/company/print-profile": true,
	// ADR-0026: every role that bills or records a transfer picks a company
	// rekening; the handler adds RequireBranch itself (TestBankAccountOptions).
	"GET /api/v1/finance/bank-accounts/options": true,
}

// noBranchPrefixes are permission-guarded areas that deliberately work
// without a selected branch (identity, master data, settings). Everything
// else MUST be behind RequireBranch.
var noBranchPrefixes = []string{
	"/api/v1/users", "/api/v1/roles", "/api/v1/permissions", "/api/v1/branches",
	"/api/v1/company", "/api/v1/products", "/api/v1/product-", "/api/v1/media",
	"/api/v1/customers", "/api/v1/suppliers", "/api/v1/member-types",
	"/api/v1/pricing",
}

func allowsNoBranch(path string) bool {
	for _, p := range noBranchPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// expectedPerms pins the permission of every route F3b-2 (Lampiran C)
// changed. Other routes are checked only for "has a catalogued permission".
var expectedPerms = map[string]string{
	"GET /api/v1/member-types":                              "member_types.view",
	"POST /api/v1/member-types":                             "member_types.manage",
	"GET /api/v1/sales-orders/export":                       "sales.export",
	"GET /api/v1/purchase-orders/export":                    "purchasing.export",
	"POST /api/v1/sales-returns/{id}/settlements":           "salesreturn.create",
	"POST /api/v1/purchase-returns/{id}/settlements":        "purchasereturn.create",
	"POST /api/v1/deliveries/{id}/proof":                    "delivery.create",
	"POST /api/v1/assistant/simulate":                       "assistant.simulate",
	"POST /api/v1/stock/locations":                          "stock.update",
	"PUT /api/v1/stock/locations/{id}":                      "stock.update",
	"POST /api/v1/stock/locations/{id}/archive":             "stock.update",
	"POST /api/v1/stock/reservations/hold":                  "stock.reserve",
	"POST /api/v1/stock/reservations/release":               "stock.reserve",
	"POST /api/v1/stock/transfers":                          "stock.transfer",
	"POST /api/v1/stock/transfers/move-location":            "stock.transfer",
	"POST /api/v1/stock/transfers/{id}/dispatch":            "stock.transfer",
	"POST /api/v1/stock/transfers/{id}/receive":             "stock.transfer",
	"POST /api/v1/stock/transfers/{id}/cancel":              "stock.transfer",
	"POST /api/v1/stock/damaged/move-in":                    "stock.adjust",
	"POST /api/v1/stock/damaged/restore":                    "stock.adjust",
	"POST /api/v1/stock/damaged/write-off":                  "stock.adjust",
	"GET /api/v1/stock/opening/template":                    "stock.adjust",
	"POST /api/v1/stock/opening/preview":                    "stock.adjust",
	"POST /api/v1/stock/opening/commit":                     "stock.adjust",
	"POST /api/v1/finance/periods/close":                    "finance.close.force",
	"POST /api/v1/finance/periods/reopen":                   "finance.manage",
	"POST /api/v1/finance/tax-periods/close":                "finance.manage",
	"POST /api/v1/finance/tax-periods/reopen":               "finance.manage",
	"POST /api/v1/finance/opening/post":                     "finance.manage",
	"GET /api/v1/finance/opening/inventory":                 "finance.view",
	"POST /api/v1/finance/opening/complete":                 "finance.manage",
	"POST /api/v1/finance/cost-restatement/preview":         "finance.manage",
	"POST /api/v1/finance/cost-restatement/commit":          "finance.close.force",
	"POST /api/v1/finance/inventory-revaluations/preview":   "finance.manage",
	"POST /api/v1/finance/inventory-revaluations/commit":    "finance.close.force",
	"POST /api/v1/finance/accounts":                         "finance.manage",
	"POST /api/v1/finance/journals/manual":                  "finance.manage",
	"POST /api/v1/finance/close/safe-close":                 "finance.manage",
	"GET /api/v1/finance/posting/preview":                   "finance.posting.view",
	"GET /api/v1/finance/posting-sources":                   "finance.posting.view",
	"POST /api/v1/finance/posting/post":                     "finance.posting.post",
	"POST /api/v1/finance/posting/post-batch":               "finance.posting.post",
	"POST /api/v1/finance/posting/close-day":                "finance.posting.post",
	"GET /api/v1/finance/posting/overview":                  "finance.posting.view",
	"POST /api/v1/finance/posting/ignore":                   "finance.posting.post",
	"POST /api/v1/finance/posting/restore":                  "finance.posting.post",
	"GET /api/v1/finance/tax-adjustments/{id}/attachments":  "finance.tax_adjustment.view",
	"POST /api/v1/finance/tax-adjustments/{id}/attachments": "finance.tax_adjustment.manage",
	"POST /api/v1/finance/costing/sync":                     "finance.posting.post",
	"GET /api/v1/finance/journals":                          "finance.journal.view",
	"GET /api/v1/finance/journals/{id}":                     "finance.journal.view",
	"POST /api/v1/finance/journals/{id}/reverse":            "finance.journal.reverse",
	"GET /api/v1/finance/expenses":                          "finance.expense.view",
	"GET /api/v1/finance/expenses/{id}":                     "finance.expense.view",
	"POST /api/v1/finance/expenses":                         "finance.expense.create",
	"POST /api/v1/finance/expenses/{id}/cancel":             "finance.expense.cancel",
	"GET /api/v1/finance/reports/trial-balance":             "finance.report.view",
	"GET /api/v1/finance/reports/profit-loss":               "finance.report.view",
	"GET /api/v1/finance/reports/balance-sheet":             "finance.report.view",
	"GET /api/v1/finance/reports/general-ledger":            "finance.report.view",
	"GET /api/v1/finance/reports/inventory-value":           "finance.report.view",
	"GET /api/v1/finance/reports/margin":                    "finance.report.view",
	"GET /api/v1/finance/reports/margin-by-product":         "finance.report.view",
	"GET /api/v1/finance/reports/receivables":               "finance.report.view",
	"GET /api/v1/finance/reports/payables":                  "finance.report.view",
	"GET /api/v1/finance/reports/cash-summary":              "finance.report.view",
	"GET /api/v1/finance/reports/fiscal-summary":            "finance.report.view",
	"GET /api/v1/finance/export/tax-package":                "finance.report.view",
	"GET /api/v1/finance/reports/tax-summary":               "finance.tax_report.view",
	"GET /api/v1/finance/reports/tax-detail":                "finance.tax_report.view",
	"GET /api/v1/finance/tax-periods":                       "finance.tax_report.view",
	"GET /api/v1/finance/tax-adjustments":                   "finance.tax_adjustment.view",
	"POST /api/v1/finance/tax-adjustments":                  "finance.tax_adjustment.manage",
	"PUT /api/v1/company/settings":                          "company.manage",
	"GET /api/v1/company/settings/definitions":              "company.view",
}

// routeServer builds the real route table over the test database.
func routeServer(t *testing.T) *server.Server {
	t.Helper()
	m := testMods
	mods := &bootstrap.Modules{
		Audit: m.audit, User: m.user, Branch: m.branch, Company: m.company, Auth: m.auth,
		Media: m.media, Product: m.product, Party: m.party, Stock: m.stock,
		Purchasing: m.purchasing, Sales: m.sales, GoodsReceipt: m.goodsreceipt,
		Delivery: m.delivery, SalesReturn: m.salesreturn, PurchaseReturn: m.purchasereturn,
		Payment: m.payment, Finance: m.finance, Dashboard: m.dashboard, Reporting: m.reporting,
		Assistant: m.assistant,
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := server.New(&config.Config{}, log)
	bootstrap.RegisterRoutes(mods, srv.Mux(), log)
	return srv
}

func concretePath(p string) string { return pathParamRe.ReplaceAllString(p, "999999999") }

func doReq(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.RemoteAddr = "203.0.113.9:5000"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// guardUser creates a role holding exactly codes, one user on branchID, and
// returns a live access token.
func guardUser(t *testing.T, branchID int64, name string, codes []string) string {
	t.Helper()
	role, err := testMods.user.Service().CreateRole(ctx(), 0, userapp.CreateRoleInput{Code: "G-" + name, Name: "Guard " + name})
	if err != nil {
		t.Fatalf("role %s: %v", name, err)
	}
	if len(codes) > 0 {
		if err := testMods.user.Service().SyncRolePermissions(ctx(), 0, role.ID, codes); err != nil {
			t.Fatalf("perms %s: %v", name, err)
		}
	}
	if _, err := testMods.user.Service().CreateUser(ctx(), 0, userapp.CreateUserInput{
		Username: name, Password: "rahasia-kuat-1", RoleIDs: []int64{role.ID},
		Branches: []usercontracts.BranchAccess{{BranchID: branchID, IsDefault: true}},
	}); err != nil {
		t.Fatalf("user %s: %v", name, err)
	}
	return login(t, name, "rahasia-kuat-1")
}

func catalogCodes() []string { return bootstrap.AllPermissionCodes() }

func TestRouteGuards(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	if err := bootstrap.EnsureAuthz(ctx(), testMods.user.Service()); err != nil {
		t.Fatalf("seed authz: %v", err)
	}
	srv := routeServer(t)
	h := srv.Handler()
	mux := srv.Mux()

	routes := scanRoutes(t)
	if len(routes) < 150 {
		t.Fatalf("hanya %d rute terbaca dari sumber; pemindai rusak?", len(routes))
	}

	tokEmpty := guardUser(t, fx.branchID, "guard-empty", nil)
	tokAll := guardUser(t, fx.branchID, "guard-all", catalogCodes())
	// A session with every permission but NO active branch.
	var allSessionID int64
	if err := testMods.db.QueryRow(`SELECT MAX(id) FROM user_sessions`).Scan(&allSessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := testMods.db.Exec(`UPDATE user_sessions SET id_active_branch = NULL WHERE id = ?`, allSessionID); err != nil {
		t.Fatal(err)
	}
	testMods.auth.Invalidator().InvalidateAll()

	known := map[string]bool{}
	for _, c := range catalogCodes() {
		known[c] = true
	}

	for _, r := range routes {
		pat := r.pattern()
		path := concretePath(r.path)
		req := httptest.NewRequest(r.method, path, nil)
		handler, matched := mux.Handler(req)
		if matched != pat {
			t.Errorf("%s: tidak terdaftar di tabel rute nyata (cocok dengan %q)", pat, matched)
			continue
		}
		if publicRoutes[pat] {
			continue
		}
		// 1. No token => 401, always.
		if rec := doReq(h, r.method, path, "", ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: tanpa token = %d, want 401", pat, rec.Code)
		}
		if authnOnlyRoutes[pat] {
			continue
		}
		g, ok := handler.(authcontracts.PermissionGuarded)
		if !ok {
			t.Errorf("%s: rute tanpa guard izin (bukan hasil authMod.Permission) dan tidak di allowlist", pat)
			continue
		}
		perm := g.RequiredPermission()
		if !known[perm] {
			t.Errorf("%s: izin %q tak ada di katalog", pat, perm)
		}
		if want, pinned := expectedPerms[pat]; pinned && want != perm {
			t.Errorf("%s: izin = %q, want %q (Lampiran C)", pat, perm, want)
		}
		// 2. Authenticated but without the permission => 403.
		if rec := doReq(h, r.method, path, tokEmpty, ""); rec.Code != http.StatusForbidden ||
			!strings.Contains(rec.Body.String(), "tidak memiliki akses") {
			t.Errorf("%s: tanpa izin = %d %s, want 403 tidak memiliki akses", pat, rec.Code, rec.Body.String())
		}
		// 3. Every permission but no branch: business routes stop at RequireBranch.
		if allowsNoBranch(r.path) {
			continue
		}
		rec := doReq(h, r.method, path, tokAll, "")
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "Pilih cabang aktif") {
			t.Errorf("%s: tanpa cabang = %d %s, want 403 Pilih cabang aktif (guard cabang hilang?)", pat, rec.Code, rec.Body.String())
		}
	}
	t.Logf("rute diperiksa: %d", len(routes))
}

// TestRouteTableMatchesMain fails when cmd/server/main.go mounts a module that
// bootstrap.RegisterRoutes (and so TestRouteGuards) does not know about, or the
// other way round.
func TestRouteTableMatchesMain(t *testing.T) {
	mainSrc, err := os.ReadFile("../../cmd/server/main.go")
	if err != nil {
		t.Fatal(err)
	}
	routesSrc, err := os.ReadFile("../bootstrap/routes.go")
	if err != nil {
		t.Fatal(err)
	}
	mainRe := regexp.MustCompile(`(\w+)Mod\.RegisterRoutes\(|mods\.(RegisterOps)\(`)
	routesRe := regexp.MustCompile(`m\.(\w+)\.RegisterRoutes\(|m\.(RegisterOps)\(`)
	names := func(re *regexp.Regexp, src []byte) []string {
		var out []string
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			n := m[1]
			if n == "" {
				n = m[2]
			}
			out = append(out, strings.ToLower(n))
		}
		sort.Strings(out)
		return out
	}
	a, b := names(mainRe, mainSrc), names(routesRe, routesSrc)
	if strings.Join(a, ",") != strings.Join(b, ",") {
		t.Fatalf("cmd/server/main.go dan bootstrap/routes.go berbeda:\n main  = %v\n routes = %v\nTambahkan modul di KEDUANYA.", a, b)
	}
}
