package integration

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"mini-erp/internal/bootstrap"
	deliveryapp "mini-erp/internal/modules/delivery/application"
	financeapp "mini-erp/internal/modules/finance/application"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	paymentapp "mini-erp/internal/modules/payment/application"
	salesapp "mini-erp/internal/modules/sales/application"
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
	so, _ := testMods.sales.Service().GetByID(ctx(), soID)
	if so.ReceiptAccountID != 0 {
		t.Fatalf("rekening tujuan without rekening = %d", so.ReceiptAccountID)
	}
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

func TestRekeningOnBillsPaymentsAndJournals(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	bca := mkRekening(t, "1011", "BCA", "123", true)
	mdr := mkRekening(t, "1012", "Mandiri", "456", false)
	sales := testMods.sales.Service()
	pays := testMods.payment.Service()
	fin := testMods.finance.Service()

	// A new bill takes the default rekening; an explicit one wins; an archived
	// or unknown one is refused.
	soID := sellFixture(t, ctx(), fx, 4)
	so, _ := sales.GetByID(ctx(), soID)
	if so.ReceiptAccountID != bca.AccountID {
		t.Fatalf("default rekening tujuan = %d, want BCA %d", so.ReceiptAccountID, bca.AccountID)
	}
	in := salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none", ReceiptAccountID: mdr.AccountID,
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1}},
	}
	so2, err := sales.CreateOrder(ctx(), 0, fx.branchID, in)
	if err != nil || so2.ReceiptAccountID != mdr.AccountID {
		t.Fatalf("explicit rekening: %v %+v", err, so2)
	}
	in.ReceiptAccountID = 999999
	if _, err := sales.CreateOrder(ctx(), 0, fx.branchID, in); err == nil {
		t.Fatal("unknown rekening tujuan accepted")
	} else {
		wantFieldErr(t, err, "receiptAccountId")
	}

	// A transfer without an explicit rekening follows the bill (Mandiri on
	// so2), never the default; the journal debits that rekening's account.
	if _, err := sales.ConfirmOrder(ctx(), 0, fx.branchID, so2.ID); err != nil {
		t.Fatalf("confirm so2: %v", err)
	}
	t2 := orderTotal(t, so2.ID)
	p2, err := pays.Create(ctx(), 0, fx.branchID, fx.customerID, "transfer", "", "", t2, payAll(so2.ID, t2))
	if err != nil {
		t.Fatalf("pay so2: %v", err)
	}
	if p2.CashAccountID != mdr.AccountID {
		t.Fatalf("payment rekening = %d, want the bill's Mandiri %d", p2.CashAccountID, mdr.AccountID)
	}
	if _, err := fin.Post(ctx(), 0, fx.branchID, "payment", p2.ID); err != nil {
		t.Fatalf("post p2: %v", err)
	}
	if got := accountNet(t, fx.branchID, "1012"); got != t2 {
		t.Fatalf("1012 = %d, want %d", got, t2)
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
	// Explicit rekening on the payment wins over the bill's.
	p1, err := pays.Create(ctx(), 0, fx.branchID, fx.customerID, "transfer", "", "", total/2, payAll(soID, total/2),
		paymentapp.WithCashAccount(mdr.AccountID))
	if err != nil || p1.CashAccountID != mdr.AccountID {
		t.Fatalf("explicit rekening payment: %v %+v", err, p1)
	}
	if _, err := fin.Post(ctx(), 0, fx.branchID, "payment", p1.ID); err != nil {
		t.Fatalf("post p1: %v", err)
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
	if got := accountNet(t, fx.branchID, "1012"); got != t2 {
		t.Fatalf("1012 after correction = %d, want %d (p1 reversed)", got, t2)
	}
	if got := accountNet(t, fx.branchID, "1011"); got != total/2 {
		t.Fatalf("1011 after correction = %d, want %d", got, total/2)
	}

	// Archived rekening: no new payment may name it, and the bill's archived
	// rekening falls back to the default instead of failing the payment.
	if _, err := fin.SetBankAccountStatus(ctx(), 0, mdr.ID, false); err != nil {
		t.Fatalf("archive mandiri: %v", err)
	}
	so3, err := sales.CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none", ReceiptAccountID: bca.AccountID,
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1}},
	})
	if err != nil {
		t.Fatalf("so3: %v", err)
	}
	if _, err := testMods.db.Exec(`UPDATE sales_orders SET receipt_account_id = ? WHERE id = ?`, mdr.AccountID, so3.ID); err != nil {
		t.Fatalf("point so3 at the archived rekening: %v", err)
	}
	if _, err := sales.ConfirmOrder(ctx(), 0, fx.branchID, so3.ID); err != nil {
		t.Fatalf("confirm so3: %v", err)
	}
	t3 := orderTotal(t, so3.ID)
	if _, err := pays.Create(ctx(), 0, fx.branchID, fx.customerID, "transfer", "", "", t3, payAll(so3.ID, t3),
		paymentapp.WithCashAccount(mdr.AccountID)); err == nil {
		t.Fatal("payment to an archived rekening accepted")
	}
	p3, err := pays.Create(ctx(), 0, fx.branchID, fx.customerID, "transfer", "", "", t3, payAll(so3.ID, t3))
	if err != nil || p3.CashAccountID != bca.AccountID {
		t.Fatalf("fallback to default: %v %+v", err, p3)
	}
	// Editing an unrelated header field of a bill whose rekening was archived
	// keeps working (re-sending the current rekening is always accepted).
	got3, _ := sales.GetByID(ctx(), so3.ID)
	edit := salesapp.OrderInput{
		PartyID: got3.PartyID, Channel: got3.Channel, PaymentTerms: got3.PaymentTerms, TaxType: got3.TaxType,
		Notes: "catatan baru", ReceiptAccountID: got3.ReceiptAccountID, HeaderOnly: true,
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1}},
	}
	if _, err := sales.UpdateOrder(ctx(), 0, fx.branchID, so3.ID, edit); err != nil {
		t.Fatalf("header edit with archived rekening: %v", err)
	}
	// An update that does not send the field (older client) keeps the bill's rekening.
	edit.ReceiptAccountID = 0
	edit.Notes = "catatan lagi"
	if after, err := sales.UpdateOrder(ctx(), 0, fx.branchID, so3.ID, edit); err != nil || after.ReceiptAccountID != mdr.AccountID {
		t.Fatalf("update without rekening re-pointed the bill: %v %+v", err, after)
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

// Once rekening exist, a transfer must name one (or inherit the bill's / the
// default); with no default and no bill rekening it is refused.
func TestTransferRequiresRekeningOnceConfigured(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 2) // created before any rekening: none on the bill
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

// The rekening tujuan of a completed bill can still change (dedicated,
// audited endpoint); a cancelled one cannot.
func TestSetReceiptAccountOnCompletedBill(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	bca := mkRekening(t, "1011", "BCA", "123", true)
	mdr := mkRekening(t, "1012", "Mandiri", "456", false)
	soID := sellFixture(t, ctx(), fx, 2)
	deliverAll(t, ctx(), fx, soID, 2)
	so, _ := testMods.sales.Service().GetByID(ctx(), soID)
	if so.Status != "completed" || so.ReceiptAccountID != bca.AccountID {
		t.Fatalf("order = %s rekening %d", so.Status, so.ReceiptAccountID)
	}
	upd, err := testMods.sales.Service().SetReceiptAccount(ctx(), 0, fx.branchID, soID, mdr.AccountID)
	if err != nil || upd.ReceiptAccountID != mdr.AccountID {
		t.Fatalf("set rekening: %v %+v", err, upd)
	}
	var n int
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action_key = 'sales_orders.receipt_account_change' AND entity_id = ?`, soID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit rows = %d %v", n, err)
	}
	if _, err := testMods.sales.Service().SetReceiptAccount(ctx(), 0, fx.branchID, soID, 0); err == nil {
		t.Fatal("empty rekening accepted")
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
