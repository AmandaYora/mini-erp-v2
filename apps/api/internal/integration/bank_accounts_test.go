package integration

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"mini-erp/internal/bootstrap"
	companycontracts "mini-erp/internal/modules/company/contracts"
	deliveryapp "mini-erp/internal/modules/delivery/application"
	financeapp "mini-erp/internal/modules/finance/application"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	paymentapp "mini-erp/internal/modules/payment/application"
	"mini-erp/internal/shared/apperror"
)

// Rekening perusahaan (ADR-0026): a rekening is bank identity on top of its
// own cash/bank GL account; bills carry a rekening tujuan, transfers/checks
// carry the rekening they moved through, and the payment journal posts there.

func mkRekening(t *testing.T, code, bank, number string, isDefault bool) *financecontracts.BankAccount {
	t.Helper()
	b, err := testMods.finance.Service().CreateBankAccount(ctx(), 0, financeapp.BankAccountInput{
		AccountCode: code, BankName: bank, AccountNumber: number, AccountHolder: "PT Uji",
		ShowOnDocuments: true, IsDefault: isDefault,
	})
	if err != nil {
		t.Fatalf("create rekening %s: %v", bank, err)
	}
	return b
}

func accountNet(t *testing.T, branchID int64, code string) int64 {
	t.Helper()
	d, c, err := testMods.finance.Service().AccountBalance(ctx(), branchID, code)
	if err != nil {
		t.Fatalf("balance %s: %v", code, err)
	}
	return d - c
}

func wantFieldErr(t *testing.T, err error, field string) {
	t.Helper()
	var ae *apperror.AppError
	ok := errors.As(err, &ae)
	if !ok || ae.Code != apperror.CodeValidation {
		t.Fatalf("want validation error on %s, got %v", field, err)
	}
	for _, f := range ae.Fields {
		if f.Field == field {
			return
		}
	}
	t.Fatalf("want field %s in %+v", field, ae.Fields)
}

func TestBankAccountMaster(t *testing.T) {
	freshDB(t)
	_ = newShop(t, ctx())
	fin := testMods.finance.Service()

	bca := mkRekening(t, "1011", "BCA", "123", true)
	if bca.AccountCode != "1011" || !bca.IsDefault || bca.Status != "active" {
		t.Fatalf("rekening = %+v", bca)
	}
	acc, _ := fin.ListAccounts(ctx(), "")
	var gl *financecontracts.Account
	for _, a := range acc {
		if a.Code == "1011" {
			gl = a
		}
	}
	if gl == nil || !gl.IsCash || gl.Type != financecontracts.AccountAsset {
		t.Fatalf("GL account of the rekening = %+v", gl)
	}
	// Same bank + number twice: refused, and no orphan GL account is created.
	if _, err := fin.CreateBankAccount(ctx(), 0, financeapp.BankAccountInput{
		AccountCode: "1019", BankName: "bca", AccountNumber: "123", AccountHolder: "PT Uji",
	}); err == nil {
		t.Fatal("duplicate rekening accepted")
	}
	if a, _ := fin.ListAccounts(ctx(), ""); len(a) != len(acc) {
		t.Fatal("refused rekening left a GL account behind")
	}
	// One GL account, one rekening; Kas Tunai is no rekening.
	if _, err := fin.CreateBankAccount(ctx(), 0, financeapp.BankAccountInput{
		AccountID: bca.AccountID, BankName: "BRI", AccountNumber: "9", AccountHolder: "PT Uji",
	}); err == nil {
		t.Fatal("second rekening on one GL account accepted")
	}
	var cashID int64
	for _, a := range acc {
		if a.Code == "1000" {
			cashID = a.ID
		}
	}
	if _, err := fin.CreateBankAccount(ctx(), 0, financeapp.BankAccountInput{
		AccountID: cashID, BankName: "BRI", AccountNumber: "9", AccountHolder: "PT Uji",
	}); err == nil {
		t.Fatal("rekening on Kas Tunai accepted")
	}
	// A second default takes the flag over; at most one default exists.
	mdr := mkRekening(t, "1012", "Mandiri", "456", true)
	list, _ := fin.ListBankAccounts(ctx(), "")
	defaults := 0
	for _, b := range list {
		if b.IsDefault {
			defaults++
			if b.ID != mdr.ID {
				t.Fatalf("default = %s, want Mandiri", b.BankName)
			}
		}
	}
	if defaults != 1 {
		t.Fatalf("defaults = %d, want 1", defaults)
	}
	// The GL account under a rekening stays a cash asset and cannot be archived.
	if _, err := fin.UpdateAccount(ctx(), 0, bca.AccountID, "Bank BCA", financecontracts.AccountAsset, false); err == nil {
		t.Fatal("rekening account lost its cash flag")
	}
	if err := fin.ArchiveAccount(ctx(), 0, bca.AccountID); err == nil {
		t.Fatal("rekening account archived")
	}
	// Archiving drops the default flag; policy follows.
	if _, err := fin.SetBankAccountStatus(ctx(), 0, mdr.ID, false); err != nil {
		t.Fatalf("archive: %v", err)
	}
	pol, _ := fin.BankPolicy(ctx())
	if !pol.Required || pol.DefaultAccountID != 0 {
		t.Fatalf("policy after archiving the default = %+v", pol)
	}
	if err := fin.CheckBankAccount(ctx(), mdr.AccountID, "x"); err == nil {
		t.Fatal("archived rekening accepted")
	}
	if _, err := fin.SetBankAccountStatus(ctx(), 0, mdr.ID, true); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if err := fin.CheckBankAccount(ctx(), mdr.AccountID, "x"); err != nil {
		t.Fatalf("restored rekening refused: %v", err)
	}
}

// Without any rekening nothing changes: transfers post to the bank mapping.
func TestTransferWithoutRekeningKeepsBankMapping(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 2)
	total := orderTotal(t, soID)
	pay, err := testMods.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "transfer", "", "", total, payAll(soID, total))
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	if pay.CashAccountID != 0 {
		t.Fatalf("cash account = %d, want 0", pay.CashAccountID)
	}
	if _, err := testMods.finance.Service().Post(ctx(), 0, fx.branchID, "payment", pay.ID); err != nil {
		t.Fatalf("post: %v", err)
	}
	if got := accountNet(t, fx.branchID, financecontracts.CodeBank); got != total {
		t.Fatalf("1010 = %d, want %d", got, total)
	}
}

// The rekening is chosen when the payment is recorded (ADR-0026 revisi,
// analysis/005): a transfer names one or takes the company default, and the
// journal debits that rekening's account.
func TestRekeningOnPaymentsAndJournals(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	bca := mkRekening(t, "1011", "BCA", "123", true)
	mdr := mkRekening(t, "1012", "Mandiri", "456", false)
	pays := testMods.payment.Service()
	fin := testMods.finance.Service()

	// A transfer without an explicit rekening takes the default (BCA).
	soID := sellFixture(t, ctx(), fx, 4)
	so2 := sellFixture(t, ctx(), fx, 1)
	t2 := orderTotal(t, so2)
	p2, err := pays.Create(ctx(), 0, fx.branchID, fx.customerID, "transfer", "", "", t2, payAll(so2, t2))
	if err != nil {
		t.Fatalf("pay so2: %v", err)
	}
	if p2.CashAccountID != bca.AccountID {
		t.Fatalf("payment rekening = %d, want the default BCA %d", p2.CashAccountID, bca.AccountID)
	}
	if _, err := fin.Post(ctx(), 0, fx.branchID, "payment", p2.ID); err != nil {
		t.Fatalf("post p2: %v", err)
	}
	if got := accountNet(t, fx.branchID, "1011"); got != t2 {
		t.Fatalf("1011 = %d, want %d", got, t2)
	}
	if got := accountNet(t, fx.branchID, financecontracts.CodeBank); got != 0 {
		t.Fatalf("1010 moved by a rekening payment: %d", got)
	}

	// Cash never carries a rekening; a cash payment with one is refused.
	total := orderTotal(t, soID)
	if _, err := pays.Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", 1000, payAll(soID, 1000),
		paymentapp.WithCashAccount(bca.AccountID)); err == nil {
		t.Fatal("cash payment with a rekening accepted")
	} else {
		wantFieldErr(t, err, "cashAccountId")
	}
	// An explicit rekening on the payment wins over the default.
	p1, err := pays.Create(ctx(), 0, fx.branchID, fx.customerID, "transfer", "", "", total/2, payAll(soID, total/2),
		paymentapp.WithCashAccount(mdr.AccountID))
	if err != nil || p1.CashAccountID != mdr.AccountID {
		t.Fatalf("explicit rekening payment: %v %+v", err, p1)
	}
	if _, err := fin.Post(ctx(), 0, fx.branchID, "payment", p1.ID); err != nil {
		t.Fatalf("post p1: %v", err)
	}
	if got := accountNet(t, fx.branchID, "1012"); got != total/2 {
		t.Fatalf("1012 = %d, want %d", got, total/2)
	}

	// Wrong rekening fixed through a correction: the old journal is reversed,
	// the replacement posts to BCA.
	fixed, _, err := pays.Correct(ctx(), paymentapp.CorrectInput{
		BranchID: fx.branchID, PaymentID: p1.ID, Method: "transfer", Amount: total / 2,
		CashAccountID: bca.AccountID, Reason: "salah pilih rekening",
	})
	if err != nil {
		t.Fatalf("correct: %v", err)
	}
	if fixed.CashAccountID != bca.AccountID {
		t.Fatalf("corrected rekening = %d, want BCA", fixed.CashAccountID)
	}
	if _, err := fin.Post(ctx(), 0, fx.branchID, "payment", fixed.ID); err != nil {
		t.Fatalf("post corrected: %v", err)
	}
	if got := accountNet(t, fx.branchID, "1012"); got != 0 {
		t.Fatalf("1012 after correction = %d, want 0 (p1 reversed)", got)
	}
	if got := accountNet(t, fx.branchID, "1011"); got != t2+total/2 {
		t.Fatalf("1011 after correction = %d, want %d", got, t2+total/2)
	}

	// Archived rekening: no new payment may name it; the default still applies.
	if _, err := fin.SetBankAccountStatus(ctx(), 0, mdr.ID, false); err != nil {
		t.Fatalf("archive mandiri: %v", err)
	}
	so3 := sellFixture(t, ctx(), fx, 1)
	t3 := orderTotal(t, so3)
	if _, err := pays.Create(ctx(), 0, fx.branchID, fx.customerID, "transfer", "", "", t3, payAll(so3, t3),
		paymentapp.WithCashAccount(mdr.AccountID)); err == nil {
		t.Fatal("payment to an archived rekening accepted")
	}
	p3, err := pays.Create(ctx(), 0, fx.branchID, fx.customerID, "transfer", "", "", t3, payAll(so3, t3))
	if err != nil || p3.CashAccountID != bca.AccountID {
		t.Fatalf("default rekening: %v %+v", err, p3)
	}

	// "Kas & Bank" counts every cash/bank account, rekening included.
	cash, err := fin.CashOnHand(ctx(), fx.branchID, "")
	if err != nil {
		t.Fatalf("cash on hand: %v", err)
	}
	want := accountNet(t, fx.branchID, "1000") + accountNet(t, fx.branchID, "1010") +
		accountNet(t, fx.branchID, "1011") + accountNet(t, fx.branchID, "1012")
	if cash != want || cash == 0 {
		t.Fatalf("cash on hand = %d, want %d", cash, want)
	}
}

// Once rekening exist, a transfer must name one (or take the default); with
// no default it is refused.
func TestTransferRequiresRekeningOnceConfigured(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 2)
	mkRekening(t, "1011", "BCA", "123", false)
	total := orderTotal(t, soID)
	_, err := testMods.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "transfer", "", "", total, payAll(soID, total))
	if err == nil {
		t.Fatal("transfer without rekening accepted while rekening exist")
	}
	wantFieldErr(t, err, "cashAccountId")
	// Cash still needs none.
	if _, err := testMods.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", total, payAll(soID, total)); err != nil {
		t.Fatalf("cash payment: %v", err)
	}
}

// Orders carry no rekening any more (ADR-0026 revisi): the dedicated endpoint
// is gone, an order payload from the previous web build that still sends
// receiptAccountId is accepted and ignored, and the COD settlement of a
// shipment carries the rekening chosen on the spot.
func TestOrdersCarryNoRekening(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	if err := bootstrap.EnsureAuthz(ctx(), testMods.user.Service()); err != nil {
		t.Fatalf("seed authz: %v", err)
	}
	mkRekening(t, "1011", "BCA", "123", true)
	mdr := mkRekening(t, "1012", "Mandiri", "456", false)
	h := routeServer(t).Handler()
	tok := guardUser(t, fx.branchID, "rk-sales", []string{"sales.view", "sales.create", "sales.update"})
	body := fmt.Sprintf(`{"partyId":%d,"channel":"regular","paymentTerms":"cod","taxType":"none","receiptAccountId":%d,
		"items":[{"productId":%d,"variantId":%d,"uom":"pcs","qty":1}]}`, fx.customerID, mdr.AccountID, fx.productID, fx.variantID)
	rec := doReq(h, http.MethodPost, "/api/v1/sales-orders", tok, body)
	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("create with legacy receiptAccountId = %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "receiptAccountId") {
		t.Fatalf("order view still exposes receiptAccountId: %s", rec.Body.String())
	}
	var created struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.Data.ID == 0 {
		t.Fatalf("decode created order: %v %s", err, rec.Body.String())
	}
	if rec := doReq(h, http.MethodPut, fmt.Sprintf("/api/v1/sales-orders/%d/receipt-account", created.Data.ID), tok,
		fmt.Sprintf(`{"receiptAccountId":%d}`, mdr.AccountID)); rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("receipt-account route = %d, want gone", rec.Code)
	}

	// The COD settlement of a shipment carries the rekening chosen on the spot.
	so2 := sellFixture(t, ctx(), fx, 1)
	dn, err := testMods.delivery.Service().Create(ctx(), 0, fx.branchID, so2, "2026-09-10", "", deliveryapp.DocInput{},
		[]deliveryapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 1}})
	if err != nil {
		t.Fatalf("delivery: %v", err)
	}
	if _, err := testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{
		RecipientName: "Budi", RecipientSignatureStatus: "signed",
		Settlement: &deliveryapp.Settlement{Mode: deliveryapp.SettlePayNow, Method: "transfer", CashAccountID: mdr.AccountID},
	}); err != nil {
		t.Fatalf("confirm with COD transfer: %v", err)
	}
	var acct int64
	if err := testMods.db.QueryRow(`SELECT COALESCE(cash_account_id, 0) FROM payments WHERE client_ref = ?`, "sj:"+itoa(dn.ID)).Scan(&acct); err != nil || acct != mdr.AccountID {
		t.Fatalf("COD payment rekening = %d %v, want %d", acct, err, mdr.AccountID)
	}
}

// Cashiers read the rekening without finance.view; managing them needs it.
func TestBankAccountOptions(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	if err := bootstrap.EnsureAuthz(ctx(), testMods.user.Service()); err != nil {
		t.Fatalf("seed authz: %v", err)
	}
	mkRekening(t, "1011", "BCA", "1234567", true)
	h := routeServer(t).Handler()
	tok := guardUser(t, fx.branchID, "ba-cashier", []string{"sales.view", "payment.create"})
	rec := doReq(h, http.MethodGet, "/api/v1/finance/bank-accounts/options", tok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("options = %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data []financecontracts.BankAccount `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || len(body.Data) != 1 || body.Data[0].AccountNumber != "1234567" || !body.Data[0].IsDefault {
		t.Fatalf("options body = %s (%v)", rec.Body.String(), err)
	}
	if rec := doReq(h, http.MethodGet, "/api/v1/finance/bank-accounts/options", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("options without token = %d, want 401", rec.Code)
	}
	if rec := doReq(h, http.MethodGet, "/api/v1/finance/bank-accounts", tok, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("admin list for a cashier = %d, want 403", rec.Code)
	}
	if rec := doReq(h, http.MethodPost, "/api/v1/finance/bank-accounts", tok,
		`{"accountCode":"1013","bankName":"BRI","accountNumber":"9","accountHolder":"PT"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("create by a cashier = %d, want 403", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "\"accountId\":0") {
		t.Fatalf("option without GL account: %s", rec.Body.String())
	}
}

// Rekening satu sumber (analysis/004): the text rekening of the document
// profile move into the master once at start — first one default on the bank
// mapping account 1010, the others on new cash accounts — and never again
// once the master holds a rekening. The profile text stays untouched.
func TestImportLegacyBankAccountsOnce(t *testing.T) {
	freshDB(t)
	_ = newShop(t, ctx())
	fin := testMods.finance.Service()
	cs := testMods.company.Service()
	if _, err := cs.SaveProfile(ctx(), 0, companycontracts.Profile{Name: "Toko Lama"}); err != nil {
		t.Fatalf("profile: %v", err)
	}
	profile := `{"headerName":"TOKO","bankAccounts":[
		{"bank":"BCA","holder":"PT Uji","number":"123-456"},
		{"bank":"BCA","holder":"PT Uji","number":"123 456"},
		{"bank":"","holder":"","number":""},
		{"bank":"BNI","holder":"","number":"999"}]}`
	if err := cs.SaveSettings(ctx(), 0, map[string]string{"document_profile": profile}); err != nil {
		t.Fatalf("settings: %v", err)
	}

	res, err := testMods.wired.ImportLegacyBankAccounts(ctx())
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(res.Created) != 2 || len(res.Skipped) != 1 || !strings.Contains(res.Skipped[0], "nomor ganda") {
		t.Fatalf("import = %d dibuat, dilewati %v", len(res.Created), res.Skipped)
	}
	bca, bni := res.Created[0], res.Created[1]
	if bca.AccountCode != "1010" || !bca.IsDefault || !bca.ShowOnDocuments || bca.SortOrder != 0 || bca.AccountNumber != "123-456" {
		t.Fatalf("BCA = %+v, want default on 1010", bca)
	}
	if bni.AccountCode != "1011" || bni.IsDefault || bni.AccountHolder != "Toko Lama" || bni.SortOrder != 1 {
		t.Fatalf("BNI = %+v, want 1011, holder from company name", bni)
	}
	pol, _ := fin.BankPolicy(ctx())
	if !pol.Required || pol.DefaultAccountID != bca.AccountID {
		t.Fatalf("policy = %+v", pol)
	}

	// Second start: the master holds rekening, nothing happens (even after an archive).
	if _, err := fin.SetBankAccountStatus(ctx(), 0, bni.ID, false); err != nil {
		t.Fatalf("archive: %v", err)
	}
	again, err := testMods.wired.ImportLegacyBankAccounts(ctx())
	if err != nil || len(again.Created) != 0 {
		t.Fatalf("second import = %+v, %v", again, err)
	}
	if list, _ := fin.ListBankAccounts(ctx(), ""); len(list) != 2 {
		t.Fatalf("rekening = %d, want 2", len(list))
	}
	settings, _ := cs.GetSettings(ctx())
	if settings["document_profile"] != profile {
		t.Fatal("document profile text changed")
	}
}

// No text rekening, or an empty profile: the master stays empty and the
// pre-ADR-0026 behaviour (transfer to 1010, no picker) is kept.
func TestImportLegacyBankAccountsNothingToMove(t *testing.T) {
	freshDB(t)
	_ = newShop(t, ctx())
	for _, raw := range []string{"", `{"headerName":"TOKO"}`, `{"bankAccounts":[]}`} {
		if raw != "" {
			if err := testMods.company.Service().SaveSettings(ctx(), 0, map[string]string{"document_profile": raw}); err != nil {
				t.Fatalf("settings: %v", err)
			}
		}
		res, err := testMods.wired.ImportLegacyBankAccounts(ctx())
		if err != nil || len(res.Created) != 0 {
			t.Fatalf("%q: import = %+v, %v", raw, res, err)
		}
	}
	if list, _ := testMods.finance.Service().ListBankAccounts(ctx(), ""); len(list) != 0 {
		t.Fatalf("rekening = %d, want 0", len(list))
	}
}
