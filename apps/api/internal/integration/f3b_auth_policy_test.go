package integration

import (
	"net/http"
	"strings"
	"testing"

	"mini-erp/internal/bootstrap"
	branchapp "mini-erp/internal/modules/branch/application"
	companyapp "mini-erp/internal/modules/company/application"
	deliveryapp "mini-erp/internal/modules/delivery/application"
	stockapp "mini-erp/internal/modules/stock/application"
	userapp "mini-erp/internal/modules/user/application"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/apperror"
)

// f3bToken makes a one-off user holding exactly codes and returns its token.
func f3bToken(t *testing.T, branchID int64, name string, codes ...string) string {
	t.Helper()
	return guardUser(t, branchID, name, codes)
}

// TestRefundNeedsRefundPermission (F3b-2): a refund settlement on a sales or
// purchase return needs *.refund on top of *.create; other settlement types
// do not. The check runs before the service, so a missing return id shows
// 403 vs 404 apart.
func TestRefundNeedsRefundPermission(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	f3bSeed(t)
	h := routeServer(t).Handler()
	for _, c := range []struct{ path, create, refund string }{
		{"/api/v1/sales-returns/999999999/settlements", "salesreturn.create", "salesreturn.refund"},
		{"/api/v1/purchase-returns/999999999/settlements", "purchasereturn.create", "purchasereturn.refund"},
	} {
		plain := f3bToken(t, fx.branchID, "plain-"+c.create, c.create)
		refunder := f3bToken(t, fx.branchID, "refund-"+c.create, c.create, c.refund)
		refundBody := `{"type":"refund","amount":1000}`
		otherBody := `{"type":"customer_credit","amount":1000}`

		rec := doReq(h, http.MethodPost, c.path, plain, refundBody)
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "izin memproses refund") {
			t.Errorf("%s refund tanpa izin = %d %s, want 403 izin refund", c.path, rec.Code, rec.Body.String())
		}
		if rec := doReq(h, http.MethodPost, c.path, plain, otherBody); rec.Code == http.StatusForbidden {
			t.Errorf("%s non-refund tak boleh 403: %s", c.path, rec.Body.String())
		}
		if rec := doReq(h, http.MethodPost, c.path, refunder, refundBody); rec.Code != http.StatusNotFound {
			t.Errorf("%s refund dengan izin = %d %s, want lolos guard (404 retur tak ada)", c.path, rec.Code, rec.Body.String())
		}
	}
}

// TestExportAndMemberTypePermissions (F3b-2): export has its own code and the
// member-type list is readable with .view alone.
func TestExportAndMemberTypePermissions(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	f3bSeed(t)
	h := routeServer(t).Handler()

	viewer := f3bToken(t, fx.branchID, "viewer", "sales.view", "purchasing.view", "member_types.manage")
	exporter := f3bToken(t, fx.branchID, "exporter", "sales.export", "purchasing.export", "member_types.view")

	for _, path := range []string{"/api/v1/sales-orders/export", "/api/v1/purchase-orders/export"} {
		if rec := doReq(h, http.MethodGet, path, viewer, ""); rec.Code != http.StatusForbidden {
			t.Errorf("%s dengan *.view = %d, want 403", path, rec.Code)
		}
		if rec := doReq(h, http.MethodGet, path, exporter, ""); rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
			t.Errorf("%s dengan *.export = %d %s, want lolos guard", path, rec.Code, rec.Body.String())
		}
	}
	if rec := doReq(h, http.MethodGet, "/api/v1/member-types", exporter, ""); rec.Code != http.StatusOK {
		t.Errorf("GET member-types dengan member_types.view = %d %s, want 200", rec.Code, rec.Body.String())
	}
	if rec := doReq(h, http.MethodPost, "/api/v1/member-types", exporter, `{}`); rec.Code != http.StatusForbidden {
		t.Errorf("POST member-types dengan .view saja = %d, want 403", rec.Code)
	}
	// manage no longer implies view at the route (migration 000053 grants both).
	if rec := doReq(h, http.MethodGet, "/api/v1/member-types", viewer, ""); rec.Code != http.StatusForbidden {
		t.Errorf("GET member-types dengan manage saja = %d, want 403", rec.Code)
	}
}

// TestStockAndFinanceGranularPermissions (F3b-2): the split codes guard what
// Lampiran C says; the old coarse codes are gone from the catalog.
func TestStockAndFinanceGranularPermissions(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	f3bSeed(t)
	h := routeServer(t).Handler()
	for _, gone := range []string{"stock.manage", "finance.post"} {
		for _, c := range catalogCodes() {
			if c == gone {
				t.Errorf("kode %s seharusnya sudah dihapus dari katalog", gone)
			}
		}
	}
	locOnly := f3bToken(t, fx.branchID, "loc-only", "stock.update", "stock.view")
	if rec := doReq(h, http.MethodPost, "/api/v1/stock/transfers", locOnly, `{}`); rec.Code != http.StatusForbidden {
		t.Errorf("transfer dengan stock.update = %d, want 403", rec.Code)
	}
	if rec := doReq(h, http.MethodPost, "/api/v1/stock/damaged/write-off", locOnly, `{}`); rec.Code != http.StatusForbidden {
		t.Errorf("write-off dengan stock.update = %d, want 403", rec.Code)
	}
	if rec := doReq(h, http.MethodPost, "/api/v1/stock/locations", locOnly, `{}`); rec.Code == http.StatusForbidden {
		t.Errorf("buat lokasi dengan stock.update tak boleh 403: %s", rec.Body.String())
	}
	reporter := f3bToken(t, fx.branchID, "reporter", "finance.report.view")
	if rec := doReq(h, http.MethodGet, "/api/v1/finance/reports/trial-balance", reporter, ""); rec.Code == http.StatusForbidden {
		t.Errorf("trial-balance dengan finance.report.view tak boleh 403: %s", rec.Body.String())
	}
	if rec := doReq(h, http.MethodGet, "/api/v1/finance/journals", reporter, ""); rec.Code != http.StatusForbidden {
		t.Errorf("journals dengan finance.report.view = %d, want 403 (butuh journal.view)", rec.Code)
	}
	if rec := doReq(h, http.MethodPost, "/api/v1/finance/periods/close", reporter, `{}`); rec.Code != http.StatusForbidden {
		t.Errorf("periods/close tanpa finance.close.force = %d, want 403", rec.Code)
	}
}

// TestReservedRolePolicy (F3b-2): is_reserved roles are invisible to and
// untouchable by sessions without a reserved role, and cannot be granted
// through the API.
func TestReservedRolePolicy(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	f3bSeed(t)
	svc := testMods.user.Service()

	resRole := f3bRole(t, "dev-reserved", "users.view")
	if _, err := testMods.db.Exec(`UPDATE roles SET is_reserved = 1, is_system = 1 WHERE id = ?`, resRole); err != nil {
		t.Fatal(err)
	}
	plainRole := f3bRole(t, "plain-admin", "users.view", "users.update", "roles.manage")
	// The API refuses to grant a reserved role, so holders are seeded in SQL.
	holder := func(name string) int64 {
		id := f3bUserWithRole(t, fx.branchID, name, plainRole)
		if _, err := testMods.db.Exec(`DELETE FROM user_roles WHERE user_id = ?`, id); err != nil {
			t.Fatal(err)
		}
		if _, err := testMods.db.Exec(`INSERT INTO user_roles (user_id, role_id) VALUES (?, ?)`, id, resRole); err != nil {
			t.Fatal(err)
		}
		return id
	}
	holderID := holder("pemegang")
	actorID := f3bUserWithRole(t, fx.branchID, "admin-biasa", plainRole)
	reservedActor := holder("dev-lain")

	roles, err := svc.ListRoles(ctx(), actorID)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range roles {
		if r.IsReserved {
			t.Errorf("role reserved %s terlihat oleh non-reserved", r.Code)
		}
	}
	if roles, _ := svc.ListRoles(ctx(), reservedActor); !hasRole(roles, "dev-reserved") {
		t.Error("pemegang role reserved harus melihat role reserved")
	}
	list, err := svc.ListUsers(ctx(), actorID, "", "", 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range list.Users {
		if u.ID == holderID || u.ID == reservedActor {
			t.Errorf("pemegang reserved %s terlihat di daftar user non-reserved", u.Username)
		}
	}
	if list, _ := svc.ListUsers(ctx(), reservedActor, "", "", 1, 50); !hasUser(list.Users, holderID) {
		t.Error("sesi reserved harus melihat pemegang reserved")
	}
	if _, err := svc.GetUser(ctx(), actorID, holderID); err == nil || err.(*apperror.AppError).Code != apperror.CodeNotFound {
		t.Errorf("GetUser pemegang oleh non-reserved = %v, want 404", err)
	}
	if err := svc.SetStatus(ctx(), actorID, holderID, "inactive"); err == nil || err.(*apperror.AppError).Code != apperror.CodeNotFound {
		t.Errorf("SetStatus pemegang oleh non-reserved = %v, want 404", err)
	}
	if err := svc.ChangePassword(ctx(), actorID, holderID, "sandi-baru-123"); err == nil || err.(*apperror.AppError).Code != apperror.CodeNotFound {
		t.Errorf("ChangePassword pemegang oleh non-reserved = %v, want 404", err)
	}
	if err := svc.SetRolePermissions(ctx(), actorID, resRole, []string{"users.view"}); err == nil || err.(*apperror.AppError).Code != apperror.CodeNotFound {
		t.Errorf("SetRolePermissions role reserved oleh non-reserved = %v, want 404", err)
	}
	if err := svc.DeleteRole(ctx(), actorID, resRole); err == nil || err.(*apperror.AppError).Code != apperror.CodeNotFound {
		t.Errorf("DeleteRole reserved oleh non-reserved = %v, want 404", err)
	}
	// Granting a reserved role through the API is refused for everybody.
	if _, err := svc.CreateUser(ctx(), reservedActor, userapp.CreateUserInput{
		Username: "calon", FullName: "Calon", Password: "rahasia-kuat-1", RoleIDs: []int64{resRole},
	}); err == nil {
		t.Error("CreateUser dengan role reserved harus ditolak")
	}
	if _, err := svc.UpdateUser(ctx(), reservedActor, actorID, userapp.UpdateUserInput{
		Username: "admin-biasa", FullName: "Admin", RoleIDs: []int64{plainRole, resRole},
	}); err == nil {
		t.Error("UpdateUser menambah role reserved harus ditolak")
	}
	// Bootstrap flags superadmin reserved.
	if err := bootstrapEnsure(t); err != nil {
		t.Fatal(err)
	}
	sa, err := svc.GetRoleByCode(ctx(), "superadmin")
	if err != nil || sa == nil || !sa.IsReserved || !sa.IsSystem {
		t.Fatalf("superadmin = %+v %v, want reserved+system", sa, err)
	}
}

// TestCompanySettingsRegistry (F3b-7): settings are validated per key, the
// effective view fills defaults, and the registry keys are the very keys the
// stock/delivery readers use.
func TestCompanySettingsRegistry(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	f3bSeed(t)
	if companyapp.SettingStockAlwaysApprove != stockapp.StrictAdjustSetting ||
		companyapp.SettingStockApprovalMode != stockapp.ApprovalModeSetting ||
		companyapp.SettingStockThreshold != stockapp.ApprovalThresholdSetting ||
		companyapp.SettingDeliveryEvidence != deliveryapp.EvidenceSetting {
		t.Fatal("kunci registri company tidak sama dengan konstanta pembacanya")
	}
	h := routeServer(t).Handler()
	admin := f3bToken(t, fx.branchID, "co-admin", "company.view", "company.manage")

	if rec := doReq(h, http.MethodPut, "/api/v1/company/settings", admin, `{"settings":{"stock_adjust_approval_mode":"paranoid"}}`); rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
		t.Errorf("mode tak valid = %d %s, want 400/422", rec.Code, rec.Body.String())
	}
	if rec := doReq(h, http.MethodPut, "/api/v1/company/settings", admin, `{"settings":{"document_profile":"bukan json"}}`); rec.Code == http.StatusOK {
		t.Errorf("document_profile tak valid harus ditolak")
	}
	rec := doReq(h, http.MethodPut, "/api/v1/company/settings", admin,
		`{"settings":{"stock_adjust_approval_mode":"STRICT","stock_adjust_always_approve":"false","stock_adjust_approval_threshold":"250"}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("simpan setting valid = %d %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, `"stock_adjust_approval_mode":"strict"`) {
		t.Errorf("tampilan efektif = %s, want mode strict (dinormalkan)", body)
	}
	if _, err := testMods.db.Exec(`DELETE FROM company_settings WHERE setting_key = 'stock_adjust_approval_threshold'`); err != nil {
		t.Fatal(err)
	}
	if rec := doReq(h, http.MethodGet, "/api/v1/company/settings", admin, ""); !strings.Contains(rec.Body.String(), `"stock_adjust_approval_threshold":"100"`) {
		t.Errorf("default ambang tidak terisi: %s", rec.Body.String())
	}
	// What the stock reader sees is the stored, normalized value.
	var mode string
	if err := testMods.db.QueryRow(`SELECT setting_value FROM company_settings WHERE setting_key = 'stock_adjust_approval_mode'`).Scan(&mode); err != nil || mode != "strict" {
		t.Errorf("tersimpan = %q (%v), want strict", mode, err)
	}
	if rec := doReq(h, http.MethodGet, "/api/v1/company/settings/definitions", admin, ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "stock_adjust_approval_mode") {
		t.Errorf("definitions = %d %s", rec.Code, rec.Body.String())
	}
}

// TestBranchColumnWidths (F3b-6): 50-char codes and the legacy widths fit,
// longer values are a validation error (not a 500), document-number columns
// are wide enough for a long code, and the default-location label names (and
// renames) the branch's system location.
func TestBranchColumnWidths(t *testing.T) {
	freshDB(t)
	svc := testMods.branch.Service()
	code := strings.Repeat("A", 50)
	br, err := svc.CreateBranch(ctx(), 0, branchapp.CreateBranchInput{
		Code: code, Name: strings.Repeat("N", 255), City: strings.Repeat("C", 120),
		Address: strings.Repeat("alamat ", 100), DefaultLocationLabel: "Toko Pusat",
	})
	if err != nil {
		t.Fatalf("create 50-char code: %v", err)
	}
	if br.Code != code || br.DefaultLocationLabel != "Toko Pusat" {
		t.Fatalf("branch = %+v", br)
	}
	for _, bad := range []branchapp.CreateBranchInput{
		{Code: strings.Repeat("B", 51), Name: "x"},
		{Code: "OK", Name: strings.Repeat("N", 256)},
		{Code: "OK2", Name: "x", City: strings.Repeat("C", 121)},
	} {
		if _, err := svc.CreateBranch(ctx(), 0, bad); err == nil {
			t.Errorf("input %q harus ditolak", bad.Code)
		} else if e, ok := err.(*apperror.AppError); !ok || e.Code != apperror.CodeValidation {
			t.Errorf("input %q: err=%v, want validation", bad.Code, err)
		}
	}

	// Default location follows the label, including a later edit.
	locName := func() string {
		var name string
		if err := testMods.db.QueryRow(`SELECT name FROM stock_locations WHERE branch_id = ? AND code = 'GDG'`, br.ID).Scan(&name); err != nil {
			t.Fatalf("lokasi default: %v", err)
		}
		return name
	}
	if _, err := testMods.stock.Service().CreateLocation(ctx(), 0, br.ID, stockapp.LocationInput{Code: "RAK1", Name: "Rak 1"}); err != nil {
		t.Fatalf("create location: %v", err)
	}
	if got := locName(); got != "Toko Pusat" {
		t.Errorf("nama lokasi default = %q, want Toko Pusat", got)
	}
	if _, err := svc.UpdateBranch(ctx(), 0, br.ID, branchapp.UpdateBranchInput{
		Code: code, Name: "Cabang", Status: "active", DefaultLocationLabel: "Gudang Belakang",
	}); err != nil {
		t.Fatalf("update branch: %v", err)
	}
	if _, err := testMods.stock.Service().CreateLocation(ctx(), 0, br.ID, stockapp.LocationInput{Code: "RAK2", Name: "Rak 2"}); err != nil {
		t.Fatalf("create location 2: %v", err)
	}
	if got := locName(); got != "Gudang Belakang" {
		t.Errorf("nama lokasi default setelah edit = %q, want Gudang Belakang", got)
	}

	// Document numbers embed the code: 50-char code => number > 60 chars.
	for _, tbl := range []string{"sales_orders", "purchase_orders", "delivery_notes", "payments", "sales_returns",
		"purchase_returns", "stock_transfers", "business_expenses", "goods_receipts", "finance_journal_entries"} {
		var width int
		if err := testMods.db.QueryRow(`SELECT CHARACTER_MAXIMUM_LENGTH FROM information_schema.columns
			WHERE table_schema = DATABASE() AND table_name = ? AND column_name = 'number'`, tbl).Scan(&width); err != nil {
			t.Fatalf("%s.number: %v", tbl, err)
		}
		if width < 100 {
			t.Errorf("%s.number lebar %d, want >= 100", tbl, width)
		}
	}
}

func f3bUserWithRole(t *testing.T, branchID int64, name string, roleID int64) int64 {
	t.Helper()
	return f3bUser(t, branchID, userapp.CreateUserInput{Username: name, FullName: name, RoleIDs: []int64{roleID}})
}

func hasRole(roles []*usercontracts.Role, code string) bool {
	for _, r := range roles {
		if r.Code == code {
			return true
		}
	}
	return false
}

func hasUser(users []*usercontracts.User, id int64) bool {
	for _, u := range users {
		if u.ID == id {
			return true
		}
	}
	return false
}

func bootstrapEnsure(t *testing.T) error {
	t.Helper()
	return bootstrap.EnsureAuthz(ctx(), testMods.user.Service())
}
