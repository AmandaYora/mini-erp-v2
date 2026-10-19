package integration

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"mini-erp/internal/bootstrap"
	branchapp "mini-erp/internal/modules/branch/application"
	financeapp "mini-erp/internal/modules/finance/application"
	userapp "mini-erp/internal/modules/user/application"
	usercontracts "mini-erp/internal/modules/user/contracts"
)

// twoBranchUser creates a role with exactly codes and a user who may read both
// branches, returning a live token.
func twoBranchUser(t *testing.T, name string, codes []string, branches ...int64) string {
	t.Helper()
	role, err := testMods.user.Service().CreateRole(ctx(), 0, userapp.CreateRoleInput{Code: "TB-" + name, Name: "TB " + name})
	if err != nil {
		t.Fatalf("role: %v", err)
	}
	if err := testMods.user.Service().SyncRolePermissions(ctx(), 0, role.ID, codes); err != nil {
		t.Fatalf("perms: %v", err)
	}
	var access []usercontracts.BranchAccess
	for i, b := range branches {
		access = append(access, usercontracts.BranchAccess{BranchID: b, IsDefault: i == 0})
	}
	if _, err := testMods.user.Service().CreateUser(ctx(), 0, userapp.CreateUserInput{
		Username: name, Password: "rahasia-kuat-1", RoleIDs: []int64{role.ID}, Branches: access,
	}); err != nil {
		t.Fatalf("user: %v", err)
	}
	return login(t, name, "rahasia-kuat-1")
}

func sheetCell(t *testing.T, raw []byte, sheet, cell string) string {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	v, err := f.GetCellValue(sheet, cell)
	if err != nil {
		t.Fatalf("cell %s!%s: %v", sheet, cell, err)
	}
	return v
}

func sheetText(t *testing.T, raw []byte, sheet string) string {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("rows %s: %v", sheet, err)
	}
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(strings.Join(r, "|"))
		b.WriteString("\n")
	}
	return b.String()
}

// The tax package spans branches (tax is reported per taxpayer): branch=all is
// the sum of both branches, needs finance.report.all_branches (403 otherwise),
// names the branch on the rows, and leaves the single-branch package unchanged.
func TestTaxPackageAllBranches(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	if err := bootstrap.EnsureAuthz(ctx(), testMods.user.Service()); err != nil {
		t.Fatalf("seed authz: %v", err)
	}
	fin := testMods.finance.Service()
	second, err := testMods.branch.Service().CreateBranch(ctx(), 0, branchapp.CreateBranchInput{Code: "CB2", Name: "Cabang Dua"})
	if err != nil {
		t.Fatalf("branch: %v", err)
	}
	// Branch 1: a sale with PPN; branch 2: a manual cash journal.
	dn := confirmedDelivery(t, fx, "2026-10-12", "exclude", 11, 2)
	if _, err := fin.Post(ctx(), 0, fx.branchID, "delivery", dn); err != nil {
		t.Fatalf("post: %v", err)
	}
	manual(t, second.ID, "2026-10-14", 40000, true)

	// Service level: the 'all' package equals the sum of both branches.
	scope := financeapp.WithBranchScope(ctx(), []int64{fx.branchID, second.ID})
	both, err := fin.ReportsRange(scope, fx.branchID, "2026-10-01", "2026-10-31", financeapp.PackageActual)
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	one, err := fin.ReportsRange(ctx(), fx.branchID, "2026-10-01", "2026-10-31", financeapp.PackageActual)
	if err != nil {
		t.Fatalf("one: %v", err)
	}
	only2, err := fin.ReportsRange(financeapp.WithBranchScope(ctx(), []int64{second.ID}), second.ID, "2026-10-01", "2026-10-31", financeapp.PackageActual)
	if err != nil {
		t.Fatalf("two: %v", err)
	}
	debit := func(r *financeapp.PackageReports) (n int64) {
		for _, x := range r.Trial {
			n += x.Debit
		}
		return
	}
	if debit(both) != debit(one)+debit(only2) || debit(only2) != 40000 {
		t.Fatalf("trial debit all=%d one=%d two=%d, want all = one + two", debit(both), debit(one), debit(only2))
	}
	if both.Summary.PpnOut != one.Summary.PpnOut || both.Summary.PpnOut != 3300 {
		t.Fatalf("ppn out all=%d one=%d", both.Summary.PpnOut, one.Summary.PpnOut)
	}
	if !both.Multi || one.Multi {
		t.Fatalf("multi flags: all=%v one=%v", both.Multi, one.Multi)
	}
	var detailBranch bool
	for _, r := range both.Detail {
		if r.BranchID == fx.branchID && r.BranchName != "" {
			detailBranch = true
		}
	}
	if !detailBranch {
		t.Fatalf("detail rows do not name their branch: %+v", both.Detail)
	}

	// HTTP: permissions and isolation.
	srv := routeServer(t)
	h := srv.Handler()
	base := []string{"finance.view", "finance.report.view"}
	tokNo := twoBranchUser(t, "pkg-no", base, fx.branchID, second.ID)
	tokAll := twoBranchUser(t, "pkg-all", append(append([]string{}, base...), "finance.report.all_branches"), fx.branchID, second.ID)
	tokOne := twoBranchUser(t, "pkg-one", append(append([]string{}, base...), "finance.report.all_branches"), fx.branchID)
	path := "/api/v1/finance/export/tax-package?from=2026-10-01&to=2026-10-31&variant=actual"

	if rec := doReq(h, "GET", path+"&branch=all", tokNo, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("branch=all without the permission = %d, want 403", rec.Code)
	}
	recAll := doReq(h, "GET", path+"&branch=all", tokAll, "")
	if recAll.Code != http.StatusOK {
		t.Fatalf("branch=all = %d: %s", recAll.Code, recAll.Body.String())
	}
	if got := sheetCell(t, recAll.Body.Bytes(), "Rincian PPN", "B1"); got != "Cabang" {
		t.Fatalf("all-branch VAT sheet B1 = %q, want the Cabang column", got)
	}
	gl := sheetText(t, recAll.Body.Bytes(), "Buku Besar")
	if !strings.Contains(gl, "Cabang Dua") {
		t.Fatalf("ledger sheet does not name the second branch:\n%s", gl)
	}
	cover := sheetText(t, recAll.Body.Bytes(), "Ringkasan")
	if !strings.Contains(cover, "Cabang Dua") {
		t.Fatalf("cover does not list the branches:\n%s", cover)
	}

	// A user who may read only one branch cannot reach 'all' beyond it, nor the other branch.
	recOne := doReq(h, "GET", path+"&branch=all", tokOne, "")
	if recOne.Code != http.StatusOK {
		t.Fatalf("single-access all = %d", recOne.Code)
	}
	if strings.Contains(sheetText(t, recOne.Body.Bytes(), "Buku Besar"), "Cabang Dua") {
		t.Fatal("a user without access to branch 2 received its ledger")
	}
	if rec := doReq(h, "GET", path+"&branch=999999", tokOne, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("foreign branch id = %d, want 403", rec.Code)
	}

	// Own branch: unchanged shape (no Cabang column).
	recSelf := doReq(h, "GET", path, tokAll, "")
	if recSelf.Code != http.StatusOK {
		t.Fatalf("own branch = %d", recSelf.Code)
	}
	if got := sheetCell(t, recSelf.Body.Bytes(), "Rincian PPN", "B1"); got != "Nomor" {
		t.Fatalf("own-branch VAT sheet B1 = %q, want Nomor (no branch column)", got)
	}
	// The month form takes the same parameter.
	if rec := doReq(h, "GET", "/api/v1/finance/export/tax-package?year=2026&month=10&branch=all&variant=capped", tokAll, ""); rec.Code != http.StatusOK {
		t.Fatalf("capped month all = %d: %s", rec.Code, rec.Body.String())
	}
	if rec := doReq(h, "GET", "/api/v1/finance/export/tax-package?year=2026&month=10&branch=all", tokNo, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("month form without permission = %d, want 403", rec.Code)
	}
}
