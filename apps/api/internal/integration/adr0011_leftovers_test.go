package integration

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"mini-erp/internal/bootstrap"
	companyapp "mini-erp/internal/modules/company/application"
	companycontracts "mini-erp/internal/modules/company/contracts"
	deliveryapp "mini-erp/internal/modules/delivery/application"
	financeapp "mini-erp/internal/modules/finance/application"
	purchasereturnapp "mini-erp/internal/modules/purchasereturn/application"
	salesreturnapp "mini-erp/internal/modules/salesreturn/application"
	stockapp "mini-erp/internal/modules/stock/application"
	userapp "mini-erp/internal/modules/user/application"
	usercontracts "mini-erp/internal/modules/user/contracts"
)

// blockAudit makes every trail insert for action fail (a CHECK constraint on
// audit_logs) until the returned release runs: the way to prove a flow audits
// INSIDE its transaction (ADR-0011) — if the trail write fails, the mutation
// must not survive.
func blockAudit(t *testing.T, action string) (release func()) {
	t.Helper()
	// Rows of that action written earlier (seeding) would violate the new
	// constraint's validation: clear them first (test database only).
	_, _ = testMods.db.Exec(`DELETE d FROM audit_log_details d JOIN audit_logs l ON l.id = d.log_id WHERE l.action_key = ?`, action)
	if _, err := testMods.db.Exec(`DELETE FROM audit_logs WHERE action_key = ?`, action); err != nil {
		t.Fatalf("clear audit %s: %v", action, err)
	}
	if _, err := testMods.db.Exec(`ALTER TABLE audit_logs ADD CONSTRAINT it_block_audit CHECK (action_key <> '` + action + `')`); err != nil {
		t.Fatalf("block audit %s: %v", action, err)
	}
	done := false
	release = func() {
		if done {
			return
		}
		done = true
		if _, err := testMods.db.Exec(`ALTER TABLE audit_logs DROP CHECK it_block_audit`); err != nil {
			t.Fatalf("unblock audit: %v", err)
		}
	}
	t.Cleanup(release)
	return release
}

func auditCount(t *testing.T, action string) int {
	t.Helper()
	var n int
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action_key = ?`, action).Scan(&n); err != nil {
		t.Fatalf("audit count: %v", err)
	}
	return n
}

func countRows(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := testMods.db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}

func lastDetail(t *testing.T, action string) (before, after, meta string) {
	t.Helper()
	var id int64
	if err := testMods.db.QueryRow(`SELECT MAX(id) FROM audit_logs WHERE action_key = ?`, action).Scan(&id); err != nil {
		t.Fatalf("last %s: %v", action, err)
	}
	b, a, m, found := detailRow(t, id)
	if !found {
		t.Fatalf("%s: no detail row", action)
	}
	return b, a, m
}

// TestFinanceAuditAtomic: manual journal, reverse, period close/reopen and
// expenses roll back when their trail row cannot be written, and write exactly
// one trail row when it can.
func TestFinanceAuditAtomic(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	lines := []financeapp.ManualLine{{AccountCode: "1000", Debit: 5000}, {AccountCode: "3000", Credit: 5000}}

	// Manual journal.
	release := blockAudit(t, "journals.manual")
	if _, err := fin.Manual(ctx(), 0, fx.branchID, "2027-03-01", "uji", lines); err == nil {
		t.Fatal("manual journal committed although its trail failed")
	}
	if n := countRows(t, `SELECT COUNT(*) FROM finance_journal_entries`); n != 0 {
		t.Fatalf("journal rows after failed audit = %d, want 0", n)
	}
	release()
	entry, err := fin.Manual(ctx(), 0, fx.branchID, "2027-03-01", "uji", lines)
	if err != nil {
		t.Fatalf("manual: %v", err)
	}
	if n := auditCount(t, "journals.manual"); n != 1 {
		t.Fatalf("manual audit rows = %d, want 1", n)
	}

	// Reverse: original must stay posted and no mirror exists.
	release = blockAudit(t, "journals.reverse")
	if _, err := fin.Reverse(ctx(), 0, fx.branchID, entry.ID, "salah"); err == nil {
		t.Fatal("reverse committed although its trail failed")
	}
	var status string
	if err := testMods.db.QueryRow(`SELECT status FROM finance_journal_entries WHERE id = ?`, entry.ID).Scan(&status); err != nil || status != "posted" {
		t.Fatalf("original status = %q (%v), want posted", status, err)
	}
	if n := countRows(t, `SELECT COUNT(*) FROM finance_journal_entries`); n != 1 {
		t.Fatalf("entries after failed reverse = %d, want 1", n)
	}
	release()
	if _, err := fin.Reverse(ctx(), 0, fx.branchID, entry.ID, "salah"); err != nil {
		t.Fatalf("reverse: %v", err)
	}
	if n := auditCount(t, "journals.reverse"); n != 1 {
		t.Fatalf("reverse audit rows = %d, want 1", n)
	}
	if _, err := fin.Reverse(ctx(), 0, fx.branchID, entry.ID, "lagi"); err == nil {
		t.Fatal("second reverse accepted")
	}
	if n := auditCount(t, "journals.reverse"); n != 1 {
		t.Fatalf("replayed reverse wrote another trail row: %d", n)
	}

	// Period close/reopen, with before/after status (F3b-8).
	release = blockAudit(t, "fiscal_periods.close")
	if err := fin.ClosePeriod(ctx(), 0, fx.branchID, 2027, 4); err == nil {
		t.Fatal("period closed although its trail failed")
	}
	if n := countRows(t, `SELECT COUNT(*) FROM finance_periods WHERE status = 'closed'`); n != 0 {
		t.Fatalf("closed periods after failed audit = %d", n)
	}
	release()
	if err := fin.ClosePeriod(ctx(), 0, fx.branchID, 2027, 4); err != nil {
		t.Fatalf("close: %v", err)
	}
	before, after, _ := lastDetail(t, "fiscal_periods.close")
	if !strings.Contains(before, `"open"`) || !strings.Contains(after, `"closed"`) {
		t.Fatalf("period close detail = %s / %s", before, after)
	}
	release = blockAudit(t, "fiscal_periods.reopen")
	if err := fin.ReopenPeriod(ctx(), 0, fx.branchID, 2027, 4, "koreksi audit"); err == nil {
		t.Fatal("period reopened although its trail failed")
	}
	if n := countRows(t, `SELECT COUNT(*) FROM finance_periods WHERE status = 'closed'`); n != 1 {
		t.Fatalf("period must stay closed, closed = %d", n)
	}
	release()

	// Expenses: create and cancel.
	accounts, _ := fin.ListAccounts(ctx(), "active")
	var cash, expense int64
	for _, a := range accounts {
		switch a.Code {
		case "1000":
			cash = a.ID
		case "6000":
			expense = a.ID
		}
	}
	in := financeapp.CreateExpenseInput{ExpenseAccountID: expense, PayAccountID: cash, Amount: 7000, Date: "2027-05-01", Notes: "listrik"}
	release = blockAudit(t, "expenses.create")
	if _, err := fin.CreateExpense(ctx(), 0, fx.branchID, in); err == nil {
		t.Fatal("expense created although its trail failed")
	}
	if n := countRows(t, `SELECT COUNT(*) FROM business_expenses`); n != 0 {
		t.Fatalf("expense rows after failed audit = %d", n)
	}
	release()
	exp, err := fin.CreateExpense(ctx(), 0, fx.branchID, in)
	if err != nil {
		t.Fatalf("expense: %v", err)
	}
	release = blockAudit(t, "expenses.cancel")
	if _, err := fin.CancelExpense(ctx(), 0, fx.branchID, exp.ID, "salah"); err == nil {
		t.Fatal("expense cancelled although its trail failed")
	}
	if n := countRows(t, `SELECT COUNT(*) FROM business_expenses WHERE status = 'posted'`); n != 1 {
		t.Fatalf("expense must stay posted, posted = %d", n)
	}
	release()
	// Replay-safe: the journal was reversed by the failed attempt; the retry
	// finishes the cancel and writes exactly one cancel row.
	if _, err := fin.CancelExpense(ctx(), 0, fx.branchID, exp.ID, "salah"); err != nil {
		t.Fatalf("retry cancel: %v", err)
	}
	if _, err := fin.CancelExpense(ctx(), 0, fx.branchID, exp.ID, "salah"); err == nil {
		t.Fatal("second cancel accepted")
	}
	if n := auditCount(t, "expenses.cancel"); n != 1 {
		t.Fatalf("cancel audit rows = %d, want 1", n)
	}
}

// TestDeliveryConfirmAuditAtomic: the confirm trail rides the status CAS — a
// failing trail leaves the note draft, the retry succeeds, and a replay writes
// no second row.
func TestDeliveryConfirmAuditAtomic(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	soID := sellFixture(t, ctx(), fx, 2)
	dn, err := m.delivery.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", deliveryapp.DocInput{},
		[]deliveryapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 2}})
	if err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	release := blockAudit(t, "deliveries.confirm")
	if _, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); err == nil {
		t.Fatal("delivery confirmed although its trail failed")
	}
	var status string
	if err := testMods.db.QueryRow(`SELECT status FROM delivery_notes WHERE id = ?`, dn.ID).Scan(&status); err != nil || status != "draft" {
		t.Fatalf("note status = %q (%v), want draft", status, err)
	}
	release()
	// The failed attempt already switched the COD order to net (idempotent
	// money step), so the retry carries no settlement; the second pass is a
	// replay of a finished confirm.
	for i := 0; i < 2; i++ {
		if _, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{}); err != nil {
			t.Fatalf("confirm #%d: %v", i+1, err)
		}
	}
	if n := auditCount(t, "deliveries.confirm"); n != 1 {
		t.Fatalf("confirm audit rows = %d, want 1 (replay must not add one)", n)
	}
	bal, err := m.stock.Stock().GetBalance(ctx(), fx.branchID, fx.productID, fx.variantID, fx.locationID)
	if err != nil || bal.Available() != 48 {
		t.Fatalf("stock after retry = %v (%v), want 48 (moved exactly once)", bal, err)
	}
}

// TestReturnAuditAtomic: sales-return confirm and settle audit in-transaction.
func TestReturnAuditAtomic(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	soID := sellFixture(t, ctx(), fx, 2)
	deliverAll(t, ctx(), fx, soID, 2)
	soTotal := orderTotal(t, soID)
	if _, err := m.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", soTotal, payAll(soID, soTotal)); err != nil {
		t.Fatalf("pay order: %v", err)
	}
	ret := createSalesReturn(t, fx, soID, 2)

	release := blockAudit(t, "sales_returns.confirm")
	if _, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err == nil {
		t.Fatal("return confirmed although its trail failed")
	}
	var status string
	if err := testMods.db.QueryRow(`SELECT status FROM sales_returns WHERE id = ?`, ret.ID).Scan(&status); err != nil || status != "draft" {
		t.Fatalf("return status = %q (%v), want draft", status, err)
	}
	release()
	for i := 0; i < 2; i++ {
		if _, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
			t.Fatalf("confirm #%d: %v", i+1, err)
		}
	}
	if n := auditCount(t, "sales_returns.confirm"); n != 1 {
		t.Fatalf("confirm audit rows = %d, want 1", n)
	}

	release = blockAudit(t, "sales_returns.settle")
	if _, err := m.salesreturn.Service().AddSettlement(ctx(), 0, fx.branchID, ret.ID,
		salesreturnapp.SettlementInput{Type: "refund", Amount: 1000}); err == nil {
		t.Fatal("settlement stored although its trail failed")
	}
	if n := countRows(t, `SELECT COUNT(*) FROM sales_return_settlements WHERE auto = 0`); n != 0 {
		t.Fatalf("settlement rows after failed audit = %d", n)
	}
	release()
	if _, err := m.salesreturn.Service().AddSettlement(ctx(), 0, fx.branchID, ret.ID,
		salesreturnapp.SettlementInput{Type: "refund", Amount: 1000}); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if n := auditCount(t, "sales_returns.settle"); n != 1 {
		t.Fatalf("settle audit rows = %d, want 1", n)
	}

	// Purchase return too.
	poID := purchaseFixture(t, ctx(), fx, 10000, 2)
	pret, err := m.purchasereturn.Service().Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "",
		[]purchasereturnapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: 1,
		}})
	if err != nil {
		t.Fatalf("create purchase return: %v", err)
	}
	release = blockAudit(t, "purchase_returns.confirm")
	if _, err := m.purchasereturn.Service().Confirm(ctx(), 0, fx.branchID, pret.ID); err == nil {
		t.Fatal("purchase return confirmed although its trail failed")
	}
	release()
	if _, err := m.purchasereturn.Service().Confirm(ctx(), 0, fx.branchID, pret.ID); err != nil {
		t.Fatalf("confirm purchase return: %v", err)
	}
	if n := auditCount(t, "purchase_returns.confirm"); n != 1 {
		t.Fatalf("purchase confirm audit rows = %d, want 1", n)
	}
}

// TestStockOpeningAuditAtomic: an opening chunk and its trail row commit
// together; a replay skips the moved positions and writes no trail.
func TestStockOpeningAuditAtomic(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	p, err := m.product.Service().GetByID(ctx(), fx.productID)
	if err != nil {
		t.Fatalf("get product: %v", err)
	}
	loc2, err := m.stock.Service().CreateLocation(ctx(), 0, fx.branchID, stockapp.LocationInput{Code: "OPN", Name: "Gudang Opname"})
	if err != nil {
		t.Fatalf("location: %v", err)
	}
	book := func() *bytes.Reader {
		return bytes.NewReader(openingBook(t, []string{p.Code, p.Variants[0].Code, "OPN", "7"}).Bytes())
	}
	release := blockAudit(t, "stock_openings.commit")
	if _, err := m.stock.Service().CommitOpening(ctx(), 0, fx.branchID, book()); err == nil {
		t.Fatal("opening committed although its trail failed")
	}
	if n := countRows(t, `SELECT COUNT(*) FROM stock_movements WHERE ref_type = 'opening' AND location_id = ?`, loc2.ID); n != 0 {
		t.Fatalf("opening movements after failed audit = %d, want 0", n)
	}
	release()
	res, err := m.stock.Service().CommitOpening(ctx(), 0, fx.branchID, book())
	if err != nil || res.Posted != 1 {
		t.Fatalf("commit: %+v %v", res, err)
	}
	if res, err = m.stock.Service().CommitOpening(ctx(), 0, fx.branchID, book()); err != nil || res.Posted != 0 {
		t.Fatalf("replay: %+v %v", res, err)
	}
	if n := auditCount(t, "stock_openings.commit"); n != 1 {
		t.Fatalf("opening audit rows = %d, want 1", n)
	}
}

// TestUserRoleCompanyAuditAtomicAndDetails: user/role/company writes audit in
// their transaction, with before/after that never carry secrets (F3b-8).
func TestUserRoleCompanyAuditAtomicAndDetails(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	if err := bootstrap.EnsureAuthz(ctx(), testMods.user.Service()); err != nil {
		t.Fatalf("seed authz: %v", err)
	}
	us := testMods.user.Service()

	role, err := us.CreateRole(ctx(), 0, userapp.CreateRoleInput{Code: "AUD-R", Name: "Audit Role"})
	if err != nil {
		t.Fatalf("role: %v", err)
	}
	u, err := us.CreateUser(ctx(), 0, userapp.CreateUserInput{
		Username: "audituser", Password: "rahasia-kuat-1", FullName: "Audit User", RoleIDs: []int64{role.ID},
		Branches: []usercontracts.BranchAccess{{BranchID: fx.branchID, IsDefault: true}},
	})
	if err != nil {
		t.Fatalf("user: %v", err)
	}
	var hash string
	if err := testMods.db.QueryRow(`SELECT password_hash FROM users WHERE id = ?`, u.ID).Scan(&hash); err != nil || hash == "" {
		t.Fatalf("hash: %v", err)
	}

	// Update with before/after.
	if _, err := us.UpdateUser(ctx(), 0, u.ID, userapp.UpdateUserInput{
		Username: "audituser", FullName: "Nama Baru", RoleIDs: []int64{role.ID},
		Branches: []usercontracts.BranchAccess{{BranchID: fx.branchID, IsDefault: true}},
	}); err != nil {
		t.Fatalf("update user: %v", err)
	}
	before, after, _ := lastDetail(t, "users.update")
	if !strings.Contains(before, "Audit User") || !strings.Contains(after, "Nama Baru") {
		t.Fatalf("user update detail = %s / %s", before, after)
	}
	for _, blob := range []string{before, after} {
		if strings.Contains(blob, hash) || strings.Contains(strings.ToLower(blob), "password") {
			t.Fatalf("user detail leaks a credential: %s", blob)
		}
	}

	// Password change: atomic and secret-free.
	release := blockAudit(t, "users.password")
	if err := us.ChangePassword(ctx(), 0, u.ID, "rahasia-baru-22"); err == nil {
		t.Fatal("password changed although its trail failed")
	}
	var hash2 string
	_ = testMods.db.QueryRow(`SELECT password_hash FROM users WHERE id = ?`, u.ID).Scan(&hash2)
	if hash2 != hash {
		t.Fatal("password hash changed despite failed audit")
	}
	release()
	if err := us.ChangePassword(ctx(), 0, u.ID, "rahasia-baru-22"); err != nil {
		t.Fatalf("change password: %v", err)
	}
	var detailCount int
	_ = testMods.db.QueryRow(`SELECT COUNT(*) FROM audit_log_details d JOIN audit_logs l ON l.id = d.log_id WHERE l.action_key = 'users.password'
		AND (d.before_json LIKE '%$2a$%' OR d.after_json LIKE '%$2a$%' OR d.metadata_json LIKE '%$2a$%')`).Scan(&detailCount)
	if detailCount != 0 {
		t.Fatal("password audit carries a hash")
	}

	// Role permissions: atomic, diff recorded.
	release = blockAudit(t, "roles.permissions")
	if err := us.SetRolePermissions(ctx(), 0, role.ID, []string{"company.view"}); err == nil {
		t.Fatal("permissions replaced although the trail failed")
	}
	if n := countRows(t, `SELECT COUNT(*) FROM role_permissions WHERE role_id = ?`, role.ID); n != 0 {
		t.Fatalf("permissions after failed audit = %d, want 0", n)
	}
	release()
	if err := us.SetRolePermissions(ctx(), 0, role.ID, []string{"company.view"}); err != nil {
		t.Fatalf("set permissions: %v", err)
	}
	_, _, meta := lastDetail(t, "roles.permissions")
	if !strings.Contains(meta, "company.view") || !strings.Contains(meta, "added") {
		t.Fatalf("permission diff = %s", meta)
	}
	if _, err := us.UpdateRole(ctx(), 0, role.ID, "Nama Role Baru", "d"); err != nil {
		t.Fatalf("update role: %v", err)
	}
	rb, ra, _ := lastDetail(t, "roles.update")
	if !strings.Contains(rb, "Audit Role") || !strings.Contains(ra, "Nama Role Baru") {
		t.Fatalf("role detail = %s / %s", rb, ra)
	}

	// Company settings: atomic, before/after, secrets masked.
	cs := testMods.company.Service()
	release = blockAudit(t, "company.settings")
	if err := cs.SaveSettings(ctx(), 0, map[string]string{"stock_approval_mode": "strict"}); err == nil {
		t.Fatal("settings saved although the trail failed")
	}
	if n := countRows(t, `SELECT COUNT(*) FROM company_settings WHERE setting_key = 'stock_approval_mode'`); n != 0 {
		t.Fatalf("setting rows after failed audit = %d", n)
	}
	release()
	if err := cs.SaveSettings(ctx(), 0, map[string]string{"stock_approval_mode": "strict", "webhook_token": "s3cr3t-value"}); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	sb, sa, _ := lastDetail(t, "company.settings")
	if !strings.Contains(sa, "strict") || !strings.Contains(sb, "stock_approval_mode") {
		t.Fatalf("settings detail = %s / %s", sb, sa)
	}
	if strings.Contains(sa, "s3cr3t-value") {
		t.Fatalf("settings detail leaks a secret: %s", sa)
	}
	if _, err := cs.SaveProfile(ctx(), 0, companycontracts.Profile{Name: "PT Uji", City: "Jakarta"}); err != nil {
		t.Fatalf("profile: %v", err)
	}
	if _, err := cs.SaveProfile(ctx(), 0, companycontracts.Profile{Name: "PT Uji Baru", City: "Jakarta"}); err != nil {
		t.Fatalf("profile 2: %v", err)
	}
	pb, pa, _ := lastDetail(t, "company.profile")
	if !strings.Contains(pb, "PT Uji") || !strings.Contains(pa, "PT Uji Baru") {
		t.Fatalf("profile detail = %s / %s", pb, pa)
	}
	_ = companyapp.SettingDefinitions
}

// TestPrintProfileAllowList (F3a-6): any authenticated user with a branch can
// read the print profile without company.view; only allow-listed settings come
// back; no token is 401; no branch is refused.
func TestPrintProfileAllowList(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	if err := bootstrap.EnsureAuthz(ctx(), testMods.user.Service()); err != nil {
		t.Fatalf("seed authz: %v", err)
	}
	cs := testMods.company.Service()
	if _, err := cs.SaveProfile(ctx(), 0, companycontracts.Profile{Name: "PT Cetak", Address: "Jl. A", City: "Jakarta", Phone: "021"}); err != nil {
		t.Fatalf("profile: %v", err)
	}
	if err := cs.SaveSettings(ctx(), 0, map[string]string{
		"document_profile":    `{"headerName":"Kop","bankAccounts":[{"bank":"BCA","holder":"PT","number":"123"}]}`,
		"stock_approval_mode": "strict",
		"rahasia_internal":    "jangan-bocor",
	}); err != nil {
		t.Fatalf("settings: %v", err)
	}
	h := routeServer(t).Handler()
	tokCashier := guardUser(t, fx.branchID, "pp-cashier", []string{"sales.view"}) // no company.view

	rec := doReq(h, http.MethodGet, "/api/v1/company/print-profile", tokCashier, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("print-profile = %d %s, want 200", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"PT Cetak", "Kop", "BCA"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
	for _, leak := range []string{"jangan-bocor", "rahasia_internal", "stock_approval_mode", "strict"} {
		if strings.Contains(body, leak) {
			t.Fatalf("print-profile leaks %q: %s", leak, body)
		}
	}
	// The wide endpoint stays guarded.
	if rec := doReq(h, http.MethodGet, "/api/v1/company/settings", tokCashier, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("company/settings for a cashier = %d, want 403", rec.Code)
	}
	if rec := doReq(h, http.MethodGet, "/api/v1/company/print-profile", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("print-profile without token = %d, want 401", rec.Code)
	}
}

// TestPaymentRefundNeedsReturnRefundPermission (F3b-2): a payment that
// allocates against a return is a refund leg and needs the return's refund
// permission on top of payment.create; ordinary allocations do not.
func TestPaymentRefundNeedsReturnRefundPermission(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	if err := bootstrap.EnsureAuthz(ctx(), testMods.user.Service()); err != nil {
		t.Fatalf("seed authz: %v", err)
	}
	h := routeServer(t).Handler()
	tokPay := guardUser(t, fx.branchID, "pay-only", []string{"payment.create", "payment.view"})
	tokSR := guardUser(t, fx.branchID, "pay-sr", []string{"payment.create", "salesreturn.refund"})

	body := func(orderType string) string {
		return `{"partyId":1,"amount":1000,"method":"cash","allocations":[{"orderType":"` + orderType + `","orderId":999999,"amount":1000}]}`
	}
	for _, rt := range []string{"sales_return", "purchase_return"} {
		rec := doReq(h, http.MethodPost, "/api/v1/payments", tokPay, body(rt))
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "refund") {
			t.Fatalf("%s payment without refund permission = %d %s, want 403 refund", rt, rec.Code, rec.Body.String())
		}
	}
	// Holding salesreturn.refund passes the permission gate for sales returns
	// (the bogus order then fails on its own merits, not 403) ...
	if rec := doReq(h, http.MethodPost, "/api/v1/payments", tokSR, body("sales_return")); rec.Code == http.StatusForbidden {
		t.Fatalf("sales_return payment with salesreturn.refund = 403: %s", rec.Body.String())
	}
	// ... but not for purchase returns.
	if rec := doReq(h, http.MethodPost, "/api/v1/payments", tokSR, body("purchase_return")); rec.Code != http.StatusForbidden {
		t.Fatalf("purchase_return payment with only salesreturn.refund = %d, want 403", rec.Code)
	}
	// Ordinary payments need only payment.create.
	if rec := doReq(h, http.MethodPost, "/api/v1/payments", tokPay, body("sales")); rec.Code == http.StatusForbidden {
		t.Fatalf("ordinary payment with payment.create = 403: %s", rec.Body.String())
	}
}
