package integration

import (
	"strings"
	"testing"
	"time"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	paymentapp "mini-erp/internal/modules/payment/application"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	purchasereturnapp "mini-erp/internal/modules/purchasereturn/application"
	salesapp "mini-erp/internal/modules/sales/application"
	salesreturnapp "mini-erp/internal/modules/salesreturn/application"
	"mini-erp/internal/shared/timeutil"
)

// A refund recorded on a return is a real payment: it journals to kas/bank
// (so the cash report shows it), the memo mirrors the payment number, a
// retried settlement never pays twice, and cancelling the payment releases the
// memo so the remainder can be settled again.

func cashOut(t *testing.T, branchID int64, code string) int64 {
	t.Helper()
	today := timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	rows, _, _, err := testMods.finance.Service().CashFlow(ctx(), branchID, today, today)
	if err != nil {
		t.Fatalf("cash flow: %v", err)
	}
	for _, r := range rows {
		if r.Code == code {
			return r.Out
		}
	}
	return 0
}

func paymentListByReturn(id int64) paymentapp.ListFilter {
	return paymentapp.ListFilter{OrderType: paymentcontracts.OrderSalesReturn, OrderID: id, Status: "active"}
}

func paymentListByPurchaseReturn(id int64) paymentapp.ListFilter {
	return paymentapp.ListFilter{OrderType: paymentcontracts.OrderPurchaseReturn, OrderID: id, Status: "active"}
}

func payPurchase(t *testing.T, fx shopFixture, partyID, poID, amount int64) {
	t.Helper()
	if _, err := testMods.payment.Service().Create(ctx(), 0, fx.branchID, partyID, "cash", "", "", amount,
		[]paymentapp.AllocationInput{{OrderType: paymentcontracts.OrderPurchase, OrderID: poID, Amount: amount}}); err != nil {
		t.Fatalf("pay PO %d: %v", poID, err)
	}
}

func TestSalesReturnRefundIsCashPayment(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	soID := sellFixture(t, ctx(), fx, 2)
	deliverAll(t, ctx(), fx, soID, 2)
	payOrder(t, fx, 0, soID, orderTotal(t, soID))
	ret := createSalesReturn(t, fx, soID, 1)
	if _, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm return: %v", err)
	}
	svc := m.salesreturn.Service()

	// A free-text method is refused: the money needs a real method.
	if _, err := svc.AddSettlement(ctx(), 0, fx.branchID, ret.ID,
		salesreturnapp.SettlementInput{Type: "refund", Amount: 1000, PaymentMethod: "Tunai"}); err == nil {
		t.Fatal("refund with a free-text method accepted")
	} else {
		wantFieldErr(t, err, "paymentMethod")
	}

	in := salesreturnapp.SettlementInput{Type: "refund", Amount: 1000, PaymentMethod: "cash", ClientRef: "form-1"}
	got, err := svc.AddSettlement(ctx(), 0, fx.branchID, ret.ID, in)
	if err != nil {
		t.Fatalf("refund: %v", err)
	}
	if len(got.Settlements) != 1 || !strings.HasPrefix(got.Settlements[0].ReferenceNumber, "PAY") || got.Settlements[0].PaymentMethod != "cash" {
		t.Fatalf("memo = %+v, want one cash memo carrying the payment number", got.Settlements)
	}
	// A retry of the same form (double click, timeout) neither pays nor memos twice.
	again, err := svc.AddSettlement(ctx(), 0, fx.branchID, ret.ID, in)
	if err != nil || len(again.Settlements) != 1 {
		t.Fatalf("replay = %v %+v", err, again)
	}

	page, err := m.payment.Service().ListFiltered(ctx(), fx.branchID, paymentListByReturn(ret.ID), 1, 10)
	if err != nil || len(page.Payments) != 1 {
		t.Fatalf("refund payments = %v %+v", err, page)
	}
	pay := page.Payments[0]
	if pay.Direction != "out" || pay.Amount != 1000 || pay.Number != got.Settlements[0].ReferenceNumber {
		t.Fatalf("refund payment = %+v", pay)
	}
	if _, err := m.finance.Service().Post(ctx(), 0, fx.branchID, "payment", pay.ID); err != nil {
		t.Fatalf("post refund: %v", err)
	}
	if out := cashOut(t, fx.branchID, financecontracts.CodeCash); out != 1000 {
		t.Fatalf("cash report out = %d, want the 1000 refund", out)
	}

	// Cancelling the payment releases the memo: the value is refundable again.
	if _, err := m.payment.Service().Cancel(ctx(), 0, fx.branchID, pay.ID); err != nil {
		t.Fatalf("cancel refund payment: %v", err)
	}
	after, err := svc.GetByID(ctx(), ret.ID)
	if err != nil {
		t.Fatalf("get return: %v", err)
	}
	if len(after.Settlements) != 0 || after.ExternalRemaining != got.ExternalRemaining+1000 {
		t.Fatalf("after cancel = %+v remaining %d, want no memo and %d", after.Settlements, after.ExternalRemaining, got.ExternalRemaining+1000)
	}

	// Without a caller key each new settlement still gets its own payment:
	// keys used by a cancelled or already-mirrored payment are skipped.
	for i := 0; i < 2; i++ {
		if _, err := svc.AddSettlement(ctx(), 0, fx.branchID, ret.ID,
			salesreturnapp.SettlementInput{Type: "refund", Amount: 500, PaymentMethod: "cash"}); err != nil {
			t.Fatalf("keyless refund #%d: %v", i+1, err)
		}
	}
	final, err := svc.GetByID(ctx(), ret.ID)
	if err != nil {
		t.Fatalf("get return: %v", err)
	}
	active, err := m.payment.Service().ListFiltered(ctx(), fx.branchID, paymentListByReturn(ret.ID), 1, 10)
	if err != nil || len(final.Settlements) != 2 || len(active.Payments) != 2 || final.Settlements[0].ReferenceNumber == final.Settlements[1].ReferenceNumber {
		t.Fatalf("keyless refunds = %v memos %+v payments %d, want two distinct", err, final.Settlements, len(active.Payments))
	}
}

// A walk-in POS sale can be refunded in cash too (no customer account).
func TestWalkInReturnRefund(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	so, err := m.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: 0, Channel: "pos", PaymentTerms: "cod", TaxType: "none",
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 2}},
	})
	if err != nil {
		t.Fatalf("create walk-in order: %v", err)
	}
	if _, err := m.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	dn, err := m.delivery.Service().Create(ctx(), 0, fx.branchID, so.ID, time.Now().Format("2006-01-02"), "", deliveryapp.DocInput{},
		[]deliveryapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 2}})
	if err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	if _, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{
		Settlement: &deliveryapp.Settlement{Mode: deliveryapp.SettlePayNow, Method: "cash"},
	}); err != nil {
		t.Fatalf("confirm delivery: %v", err)
	}
	ret := createSalesReturn(t, fx, so.ID, 1)
	confirmed, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID)
	if err != nil {
		t.Fatalf("confirm return: %v", err)
	}
	got, err := m.salesreturn.Service().AddSettlement(ctx(), 0, fx.branchID, ret.ID,
		salesreturnapp.SettlementInput{Type: "refund", Amount: confirmed.Total, PaymentMethod: "transfer", ReferenceNumber: "TRX-1"})
	if err != nil {
		t.Fatalf("walk-in refund: %v", err)
	}
	if got.ExternalRemaining != 0 || len(got.Settlements) != 1 || got.Settlements[0].PaymentMethod != "transfer" {
		t.Fatalf("walk-in refund = %+v", got)
	}
}

// The supplier's refund of a purchase return comes into the chosen kas.
func TestPurchaseReturnRefundIntoKas(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	kasBesar, err := m.finance.Service().CreateAccount(ctx(), 0, "1002", "Kas Besar", financecontracts.AccountAsset, true)
	if err != nil {
		t.Fatalf("create kas besar: %v", err)
	}
	poID := purchaseFixture(t, ctx(), fx, 10000, 2)
	po, err := m.purchasing.Service().GetByID(ctx(), poID)
	if err != nil || po == nil {
		t.Fatalf("get PO: %v", err)
	}
	payPurchase(t, fx, po.PartyID, poID, po.GrandTotal)
	pret, err := m.purchasereturn.Service().Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "",
		[]purchasereturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 1}})
	if err != nil {
		t.Fatalf("create purchase return: %v", err)
	}
	confirmed, err := m.purchasereturn.Service().Confirm(ctx(), 0, fx.branchID, pret.ID)
	if err != nil {
		t.Fatalf("confirm purchase return: %v", err)
	}
	got, err := m.purchasereturn.Service().AddSettlement(ctx(), 0, fx.branchID, pret.ID, purchasereturnapp.SettlementInput{
		Type: "refund", Amount: confirmed.Total, PaymentMethod: "cash", CashAccountID: kasBesar.ID,
	})
	if err != nil {
		t.Fatalf("purchase refund: %v", err)
	}
	if len(got.Settlements) != 1 || !strings.HasPrefix(got.Settlements[0].ReferenceNumber, "PAY") {
		t.Fatalf("purchase refund memo = %+v", got.Settlements)
	}
	page, err := m.payment.Service().ListFiltered(ctx(), fx.branchID, paymentListByPurchaseReturn(pret.ID), 1, 10)
	if err != nil || len(page.Payments) != 1 || page.Payments[0].Direction != "in" || page.Payments[0].CashAccountID != kasBesar.ID {
		t.Fatalf("purchase refund payment = %v %+v", err, page)
	}
	if _, err := m.finance.Service().Post(ctx(), 0, fx.branchID, "payment", page.Payments[0].ID); err != nil {
		t.Fatalf("post: %v", err)
	}
	if got := accountNet(t, fx.branchID, "1002"); got != confirmed.Total {
		t.Fatalf("Kas Besar = %d, want %d", got, confirmed.Total)
	}
}
