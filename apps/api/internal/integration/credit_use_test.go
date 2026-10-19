package integration

import (
	"testing"

	paymentapp "mini-erp/internal/modules/payment/application"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	purchasereturnapp "mini-erp/internal/modules/purchasereturn/application"
	purchasingapp "mini-erp/internal/modules/purchasing/application"
	salesreturnapp "mini-erp/internal/modules/salesreturn/application"
)

// N-41 sales side: the saldo a return left as customer balance is spent on a
// new bill. The bill leg is journalled Dr Uang Muka Pelanggan / Cr Piutang, the
// bill's open amount falls, the use can never exceed the saldo, and cancelling
// the payment reverses the journal and gives the saldo back.
func TestUseCreditSales(t *testing.T) {
	freshDB(t)
	fx, _, retID, retTotal := creditSalesFixture(t)
	m := testMods
	fin := m.finance.Service()
	done, err := m.salesreturn.Service().AddSettlement(ctx(), 0, fx.branchID, retID,
		salesreturnapp.SettlementInput{Type: "customer_credit", Amount: retTotal})
	if err != nil {
		t.Fatalf("credit: %v", err)
	}
	if _, err := fin.Post(ctx(), 0, fx.branchID, "sales_return_settlement", done.Settlements[0].ID); err != nil {
		t.Fatalf("post settlement: %v", err)
	}
	if adv := netOf(t, fx.branchID, "2200"); adv != -retTotal {
		t.Fatalf("advance before use = %d, want %d", adv, -retTotal)
	}

	so2 := sellFixture(t, ctx(), fx, 1)
	deliverAll(t, ctx(), fx, so2, 1)
	bill := orderTotal(t, so2)
	use := bill
	if use > retTotal {
		use = retTotal
	}
	pay := m.payment.Service()
	// Over the saldo / over the bill / wrong pair are refused.
	if _, err := pay.UseCredit(ctx(), 0, fx.branchID, fx.customerID, paymentcontracts.OrderSalesReturn, retID,
		paymentcontracts.OrderSales, so2, retTotal+1, ""); err == nil {
		t.Fatal("use over the saldo must be refused")
	}
	if _, err := pay.UseCredit(ctx(), 0, fx.branchID, fx.customerID, paymentcontracts.OrderPurchaseReturn, retID,
		paymentcontracts.OrderSales, so2, 1, ""); err == nil {
		t.Fatal("a mismatched return/bill pair must be refused")
	}
	if _, err := pay.UseCredit(ctx(), 0, fx.branchID, fx.customerID, paymentcontracts.OrderSalesReturn, retID,
		paymentcontracts.OrderSales, so2, bill+1, ""); err == nil && bill+1 <= retTotal {
		t.Fatal("use over the bill must be refused")
	}

	p, err := pay.UseCredit(ctx(), 0, fx.branchID, fx.customerID, paymentcontracts.OrderSalesReturn, retID,
		paymentcontracts.OrderSales, so2, use, "uji")
	if err != nil {
		t.Fatalf("use credit: %v", err)
	}
	if p.Method != paymentcontracts.MethodAdvance || p.CreditSourceID != retID || p.PairID == 0 {
		t.Fatalf("bill leg = %+v", p)
	}
	// Trail + remaining saldo.
	bals, err := pay.CreditBalances(ctx(), fx.branchID, fx.customerID)
	if err != nil || len(bals) != 1 || bals[0].Used != use || bals[0].Remaining != retTotal-use {
		t.Fatalf("credit balances = %+v err %v, want used %d remaining %d", bals, err, use, retTotal-use)
	}
	// More than what is left is refused, and so is a refund of the saldo.
	if retTotal-use > 0 {
		so3 := sellFixture(t, ctx(), fx, 1)
		if _, err := pay.UseCredit(ctx(), 0, fx.branchID, fx.customerID, paymentcontracts.OrderSalesReturn, retID,
			paymentcontracts.OrderSales, so3, retTotal-use+1, ""); err == nil {
			t.Fatal("use over the remaining saldo must be refused")
		}
	}
	if _, err := pay.Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", 1,
		refundAlloc(paymentcontracts.OrderSalesReturn, retID, 1)); err == nil {
		t.Fatal("the saldo must not be refundable in cash")
	}
	// The bill is settled by the advance; the return cannot be cancelled while
	// its saldo is spent.
	if paid := paidOf(t, paymentcontracts.OrderSales, so2); paid != use {
		t.Fatalf("bill paid = %d, want %d", paid, use)
	}
	if _, err := m.salesreturn.Service().Cancel(ctx(), 0, fx.branchID, retID); err == nil {
		t.Fatal("a return whose saldo is spent must not be cancellable")
	}

	// Journal: Dr 2200 / Cr 1100 of the used amount, on the customer.
	if !queueHas(t, fx.branchID, "payment", p.ID) {
		t.Fatal("saldo use missing from the posting queue")
	}
	entry, err := fin.Post(ctx(), 0, fx.branchID, "payment", p.ID)
	if err != nil {
		t.Fatalf("post use: %v", err)
	}
	legs := journalSums(entry)
	if legs["2200"] != [2]int64{use, 0} || legs["1100"] != [2]int64{0, use} {
		t.Fatalf("legs = %v, want Dr 2200 / Cr 1100 of %d", legs, use)
	}
	for _, l := range entry.Lines {
		if l.PartyID != fx.customerID {
			t.Fatalf("line %s party = %d, want %d", l.AccountCode, l.PartyID, fx.customerID)
		}
	}
	// The return leg is sub-ledger only and never queued.
	if queueHas(t, fx.branchID, "payment", p.PairID) {
		t.Fatal("the return leg must never be queued")
	}
	if adv := netOf(t, fx.branchID, "2200"); adv != -(retTotal - use) {
		t.Fatalf("advance after use = %d, want %d", adv, -(retTotal - use))
	}

	// Cancelling gives everything back.
	if _, err := pay.Cancel(ctx(), 0, fx.branchID, p.ID); err != nil {
		t.Fatalf("cancel use: %v", err)
	}
	if n := liveEntries(t, "payment", p.ID); n != 0 {
		t.Fatalf("live journals after cancel = %d, want 0", n)
	}
	if adv := netOf(t, fx.branchID, "2200"); adv != -retTotal {
		t.Fatalf("advance after cancel = %d, want %d", adv, -retTotal)
	}
	bals, _ = pay.CreditBalances(ctx(), fx.branchID, fx.customerID)
	if len(bals) != 1 || bals[0].Remaining != retTotal {
		t.Fatalf("saldo after cancel = %+v, want %d", bals, retTotal)
	}
	if paid := paidOf(t, paymentcontracts.OrderSales, so2); paid != 0 {
		t.Fatalf("bill paid after cancel = %d, want 0", paid)
	}
}

func purchaseAlloc(poID, amount int64) []paymentapp.AllocationInput {
	return []paymentapp.AllocationInput{{OrderType: paymentcontracts.OrderPurchase, OrderID: poID, Amount: amount}}
}

func paidOf(t *testing.T, orderType string, id int64) int64 {
	t.Helper()
	m, err := testMods.payment.Service().PaidForOrders(ctx(), orderType, []int64{id})
	if err != nil {
		t.Fatalf("paid: %v", err)
	}
	return m[id]
}

// N-41 purchase side: Dr Hutang / Cr Uang Muka Supplier.
func TestUseCreditPurchase(t *testing.T) {
	freshDB(t)
	fx := purchaseProduct(t, newShop(t, ctx()))
	m := testMods
	fin := m.finance.Service()
	poID := purchaseFixture(t, ctx(), fx, 10000, 2)
	po, _ := m.purchasing.Service().GetByID(ctx(), poID)
	pay, err := m.payment.Service().Create(ctx(), 0, fx.branchID, po.PartyID, "cash", "", "", po.GrandTotal,
		purchaseAlloc(poID, po.GrandTotal))
	if err != nil {
		t.Fatalf("pay PO: %v", err)
	}
	ret, err := m.purchasereturn.Service().Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "",
		[]purchasereturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 2}})
	if err != nil {
		t.Fatalf("return: %v", err)
	}
	if _, err := m.purchasereturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	done, err := m.purchasereturn.Service().AddSettlement(ctx(), 0, fx.branchID, ret.ID,
		purchasereturnapp.SettlementInput{Type: "supplier_credit", Amount: ret.Total})
	if err != nil {
		t.Fatalf("credit: %v", err)
	}
	var grID int64
	if err := m.db.QueryRow(`SELECT id FROM goods_receipts WHERE purchase_order_id = ? LIMIT 1`, poID).Scan(&grID); err != nil {
		t.Fatalf("receipt: %v", err)
	}
	for _, p := range []struct {
		typ string
		id  int64
	}{{"goods_receipt", grID}, {"payment", pay.ID}, {"purchase_return", ret.ID}, {"purchase_return_settlement", done.Settlements[0].ID}} {
		if _, err := fin.Post(ctx(), 0, fx.branchID, p.typ, p.id); err != nil {
			t.Fatalf("post %s: %v", p.typ, err)
		}
	}
	adv0 := netOf(t, fx.branchID, "2300")
	if adv0 != ret.Total {
		t.Fatalf("supplier advance = %d, want %d", adv0, ret.Total)
	}

	po2, err := m.purchasing.Service().CreateOrder(ctx(), 0, fx.branchID, purchasingapp.OrderInput{
		PartyID: po.PartyID, PaymentTerms: "net", DueDate: "2026-12-10", TaxType: "none",
		Items: []purchasingapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1, UnitPrice: 5000}},
	})
	if err != nil {
		t.Fatalf("PO2: %v", err)
	}
	if _, err := m.purchasing.Service().ConfirmOrder(ctx(), 0, fx.branchID, po2.ID); err != nil {
		t.Fatalf("confirm PO2: %v", err)
	}
	use := int64(3000)
	if _, err := m.payment.Service().UseCredit(ctx(), 0, fx.branchID, po.PartyID, paymentcontracts.OrderPurchaseReturn, ret.ID,
		paymentcontracts.OrderPurchase, po2.ID, ret.Total+1, ""); err == nil {
		t.Fatal("use over the saldo must be refused")
	}
	p, err := m.payment.Service().UseCredit(ctx(), 0, fx.branchID, po.PartyID, paymentcontracts.OrderPurchaseReturn, ret.ID,
		paymentcontracts.OrderPurchase, po2.ID, use, "")
	if err != nil {
		t.Fatalf("use: %v", err)
	}
	entry, err := fin.Post(ctx(), 0, fx.branchID, "payment", p.ID)
	if err != nil {
		t.Fatalf("post use: %v", err)
	}
	legs := journalSums(entry)
	if legs["2000"] != [2]int64{use, 0} || legs["2300"] != [2]int64{0, use} {
		t.Fatalf("legs = %v, want Dr 2000 / Cr 2300 of %d", legs, use)
	}
	if adv := netOf(t, fx.branchID, "2300"); adv != ret.Total-use {
		t.Fatalf("supplier advance after use = %d, want %d", adv, ret.Total-use)
	}
	if _, err := m.payment.Service().Cancel(ctx(), 0, fx.branchID, p.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if adv := netOf(t, fx.branchID, "2300"); adv != ret.Total {
		t.Fatalf("supplier advance after cancel = %d, want %d", adv, ret.Total)
	}
}
