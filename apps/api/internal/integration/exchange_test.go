package integration

import (
	"testing"

	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	paymentapp "mini-erp/internal/modules/payment/application"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	purchasereturnapp "mini-erp/internal/modules/purchasereturn/application"
	salesreturnapp "mini-erp/internal/modules/salesreturn/application"
)

// TestExchangeReplacementFlow locks the D4 happy path: replacement stock
// does NOT move at return creation, moves at replacement dispatch (confirm),
// and the replacement SJ is excluded from auto-journaling.
func TestExchangeReplacementFlow(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	soID := sellFixture(t, ctx(), fx, 2)
	deliverAll(t, ctx(), fx, soID, 2)
	// Available now 48 (50 opening − 2 delivered).

	ret, err := m.salesreturn.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", "exchange",
		[]salesreturnapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: 2,
		}},
		[]salesreturnapp.ReplacementInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: 1,
		}})
	if err != nil {
		t.Fatalf("create exchange: %v", err)
	}
	if ret.ReturnMode != "exchange" || ret.ReplacementDeliveryStatus != "pending" {
		t.Fatalf("mode=%q replStatus=%q, want exchange/pending", ret.ReturnMode, ret.ReplacementDeliveryStatus)
	}
	if len(ret.ReplacementItems) != 1 || ret.ReplacementItems[0].UnitPrice != 15000 {
		t.Fatalf("replacement not priced from catalog: %+v", ret.ReplacementItems)
	}
	avail := available(t, fx)
	if avail != 48 {
		t.Fatalf("stock moved at return creation: available=%v, want 48", avail)
	}

	if _, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm return: %v", err)
	}
	if avail := available(t, fx); avail != 50 {
		t.Fatalf("restock missing: available=%v, want 50", avail)
	}

	dn, err := m.salesreturn.Service().CreateReplacementDelivery(ctx(), 0, fx.branchID, ret.ID,
		salesreturnapp.ReplacementDispatchInput{})
	if err != nil {
		t.Fatalf("dispatch replacement: %v", err)
	}
	if dn.DocumentKind != deliverycontracts.DocumentKindReplacement || dn.SalesReturnID != ret.ID {
		t.Fatalf("replacement link broken: %+v", dn)
	}
	if dn.Status != "draft" {
		t.Fatalf("replacement status=%q, want draft", dn.Status)
	}
	if avail := available(t, fx); avail != 50 {
		t.Fatalf("draft replacement moved stock: available=%v, want 50", avail)
	}
	afterDispatch, err := m.salesreturn.Service().GetByID(ctx(), ret.ID)
	if err != nil {
		t.Fatalf("get return: %v", err)
	}
	if afterDispatch.ReplacementDeliveryStatus != "dispatched" {
		t.Fatalf("replStatus=%q, want dispatched", afterDispatch.ReplacementDeliveryStatus)
	}

	// Second dispatch must fail on status, not on unique key.
	if _, err := m.salesreturn.Service().CreateReplacementDelivery(ctx(), 0, fx.branchID, ret.ID,
		salesreturnapp.ReplacementDispatchInput{}); err == nil {
		t.Fatal("expected rejection for double dispatch, got nil")
	}

	confirmed, err := m.salesreturn.Service().ConfirmReplacementDelivery(ctx(), 0, fx.branchID, ret.ID, dn.ID,
		deliverycontracts.ReplacementConfirm{})
	if err != nil {
		t.Fatalf("confirm replacement: %v", err)
	}
	if confirmed.Status != "confirmed" {
		t.Fatalf("delivery status=%q, want confirmed", confirmed.Status)
	}
	if avail := available(t, fx); avail != 49 {
		t.Fatalf("replacement stock out missing: available=%v, want 49", avail)
	}
	done, err := m.salesreturn.Service().GetByID(ctx(), ret.ID)
	if err != nil {
		t.Fatalf("get return: %v", err)
	}
	if done.ReplacementDeliveryStatus != "confirmed" {
		t.Fatalf("replStatus=%q, want confirmed", done.ReplacementDeliveryStatus)
	}

	// F2b-2: the replacement goods are a NEW sale and are journalled: Dr
	// Piutang / Cr Penjualan (+ COGS from the shipment's own stock-out).
	entry, err := m.finance.Service().Post(ctx(), 0, fx.branchID, "delivery", dn.ID)
	if err != nil {
		t.Fatalf("post replacement delivery: %v", err)
	}
	var recv, rev int64
	for _, l := range entry.Lines {
		switch l.AccountCode {
		case "1100":
			recv += l.Debit
		case "4000":
			rev += l.Credit
		}
	}
	if recv != 15000 || rev != 15000 {
		t.Fatalf("replacement journal: receivable %d revenue %d; want 15000/15000", recv, rev)
	}
}

// TestPreviewMatchesCreate locks D5's no-drift rule for both return modules.
func TestPreviewMatchesCreate(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	soID := sellFixture(t, ctx(), fx, 4)
	deliverAll(t, ctx(), fx, soID, 4)
	lines := []salesreturnapp.LineInput{{
		ProductID: fx.productID, VariantID: fx.variantID,
		LocationID: fx.locationID, UOM: "pcs", Qty: 3,
	}}
	prev, err := m.salesreturn.Service().PreviewReturn(ctx(), fx.branchID, soID, "return_only", lines, nil)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	created := createSalesReturn(t, fx, soID, 3)
	if prev.Subtotal != created.Subtotal || prev.DiscountTotal != created.DiscountTotal ||
		prev.TaxTotal != created.TaxTotal || prev.Total != created.Total {
		t.Fatalf("preview %+v != created %+v", prev, created)
	}
	if len(prev.Items) != len(created.Items) || prev.Items[0].LineTotal != created.Items[0].LineTotal {
		t.Fatalf("preview lines drifted from created lines")
	}

	poID := purchaseFixture(t, ctx(), fx, 10000, 4)
	plines := []purchasereturnapp.LineInput{{
		ProductID: fx.productID, VariantID: fx.variantID,
		LocationID: fx.locationID, UOM: "pcs", Qty: 3,
	}}
	pprev, err := m.purchasereturn.Service().PreviewReturn(ctx(), fx.branchID, poID, plines)
	if err != nil {
		t.Fatalf("purchase preview: %v", err)
	}
	// Fixture PO: 10% discount, exclude-tax 11%.
	// 3 × 10000 = 30000 gross − 3000 = 27000 net + 2970 tax → 29970.
	if pprev.Subtotal != 27000 || pprev.TaxTotal != 2970 || pprev.Total != 29970 {
		t.Fatalf("purchase preview = %d/%d/%d, want 27000/2970/29970",
			pprev.Subtotal, pprev.TaxTotal, pprev.Total)
	}
	pcreated, err := m.purchasereturn.Service().Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "", plines)
	if err != nil {
		t.Fatalf("create purchase return: %v", err)
	}
	if pprev.Total != pcreated.Total || pprev.Subtotal != pcreated.Subtotal {
		t.Fatalf("purchase preview %+v != created %+v", pprev, pcreated)
	}
}

// TestReplacementDispatchShortStock locks the D4 failure path: a short line
// aborts the WHOLE dispatch and the return stays pending.
func TestReplacementDispatchShortStock(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	empty := costedProduct(t, fx, "TST-EMPTY", "Barang Kosong")
	soID := sellFixture(t, ctx(), fx, 2)
	deliverAll(t, ctx(), fx, soID, 2)

	ret, err := m.salesreturn.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", "exchange",
		[]salesreturnapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: 2,
		}},
		[]salesreturnapp.ReplacementInput{{
			ProductID: empty.productID, VariantID: empty.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: 1,
		}})
	if err != nil {
		t.Fatalf("create exchange: %v", err)
	}
	if _, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm return: %v", err)
	}
	if _, err := m.salesreturn.Service().CreateReplacementDelivery(ctx(), 0, fx.branchID, ret.ID,
		salesreturnapp.ReplacementDispatchInput{}); err == nil {
		t.Fatal("expected rejection for short stock, got nil")
	}
	after, err := m.salesreturn.Service().GetByID(ctx(), ret.ID)
	if err != nil {
		t.Fatalf("get return: %v", err)
	}
	if after.ReplacementDeliveryStatus != "pending" {
		t.Fatalf("replStatus=%q, want pending after failed dispatch", after.ReplacementDeliveryStatus)
	}
}

// TestSettlementCap locks D2's bound: settlements can never exceed the
// return total, on either module.
func TestSettlementCap(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	soID := sellFixture(t, ctx(), fx, 2)
	deliverAll(t, ctx(), fx, soID, 2)
	// Order fully paid: nothing open to offset, so the whole return stays
	// available for manual settlement.
	soTotal := orderTotal(t, soID)
	if _, err := m.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", soTotal, payAll(soID, soTotal)); err != nil {
		t.Fatalf("pay order: %v", err)
	}
	ret := createSalesReturn(t, fx, soID, 2)
	if _, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm return: %v", err)
	}
	if _, err := m.salesreturn.Service().AddSettlement(ctx(), 0, fx.branchID, ret.ID,
		salesreturnapp.SettlementInput{Type: "refund", Amount: ret.Total + 1}); err == nil {
		t.Fatal("expected rejection for over-settlement, got nil")
	}
	if _, err := m.salesreturn.Service().AddSettlement(ctx(), 0, fx.branchID, ret.ID,
		salesreturnapp.SettlementInput{Type: "bogus", Amount: 1000}); err == nil {
		t.Fatal("expected rejection for unknown type, got nil")
	}
	done, err := m.salesreturn.Service().AddSettlement(ctx(), 0, fx.branchID, ret.ID,
		salesreturnapp.SettlementInput{Type: "refund", Amount: ret.Total})
	if err != nil {
		t.Fatalf("add settlement: %v", err)
	}
	if len(done.Settlements) != 1 || done.Settlements[0].Amount != ret.Total {
		t.Fatalf("settlement not persisted: %+v", done.Settlements)
	}

	poID := purchaseFixture(t, ctx(), fx, 10000, 2)
	// K-04c: the return value first reduces what we owe; only the excess over
	// the open bill can be settled. Pay the bill so the whole return is excess.
	po, err := m.purchasing.Service().GetByID(ctx(), poID)
	if err != nil || po == nil {
		t.Fatalf("get PO: %v", err)
	}
	if _, err := m.payment.Service().Create(ctx(), 0, fx.branchID, po.PartyID, "cash", "", "", po.GrandTotal,
		[]paymentapp.AllocationInput{{OrderType: paymentcontracts.OrderPurchase, OrderID: poID, Amount: po.GrandTotal}}); err != nil {
		t.Fatalf("pay PO: %v", err)
	}
	pret, err := m.purchasereturn.Service().Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "",
		[]purchasereturnapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: 2,
		}})
	if err != nil {
		t.Fatalf("create purchase return: %v", err)
	}
	if _, err := m.purchasereturn.Service().Confirm(ctx(), 0, fx.branchID, pret.ID); err != nil {
		t.Fatalf("confirm purchase return: %v", err)
	}
	if _, err := m.purchasereturn.Service().AddSettlement(ctx(), 0, fx.branchID, pret.ID,
		purchasereturnapp.SettlementInput{Type: "refund", Amount: pret.Total + 1}); err == nil {
		t.Fatal("expected rejection for purchase over-settlement, got nil")
	}
	pdone, err := m.purchasereturn.Service().AddSettlement(ctx(), 0, fx.branchID, pret.ID,
		purchasereturnapp.SettlementInput{Type: "refund", Amount: pret.Total})
	if err != nil {
		t.Fatalf("add purchase settlement: %v", err)
	}
	if len(pdone.Settlements) != 1 {
		t.Fatalf("purchase settlement not persisted: %+v", pdone.Settlements)
	}
}

func available(t *testing.T, fx shopFixture) float64 {
	t.Helper()
	bal, err := testMods.stock.Stock().GetBalance(ctx(), fx.branchID, fx.productID, fx.variantID, fx.locationID)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	return bal.Available()
}

// exchangeWithSurcharge builds a confirmed exchange whose replacement costs
// MORE than the returned goods (1 returned, 2 replacement) with its replacement
// shipment dispatched (draft), ready for confirmation.
func exchangeWithSurcharge(t *testing.T, fx shopFixture, replQty float64) (retID, dnID int64) {
	t.Helper()
	m := testMods
	soID := sellFixture(t, ctx(), fx, 2)
	deliverAll(t, ctx(), fx, soID, 2)
	ret, err := m.salesreturn.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", "exchange",
		[]salesreturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 1}},
		[]salesreturnapp.ReplacementInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: replQty}})
	if err != nil {
		t.Fatalf("create exchange: %v", err)
	}
	if _, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm return: %v", err)
	}
	dn, err := m.salesreturn.Service().CreateReplacementDelivery(ctx(), 0, fx.branchID, ret.ID, salesreturnapp.ReplacementDispatchInput{})
	if err != nil {
		t.Fatalf("dispatch replacement: %v", err)
	}
	return ret.ID, dn.ID
}

// K-04e: the exchange surcharge is collected with the replacement hand-over.
// The payment is allocated to the return (charge kind), recorded atomically
// with the confirmation, capped at the open difference and refused for an
// exchange that does not cost more.
func TestExchangeSurchargePayNow(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	retID, dnID := exchangeWithSurcharge(t, fx, 2)
	ret, err := m.salesreturn.Service().GetByID(ctx(), retID)
	if err != nil {
		t.Fatalf("get return: %v", err)
	}
	if ret.DifferenceAmount <= 0 {
		t.Fatalf("difference = %d, want a surcharge", ret.DifferenceAmount)
	}
	if ret.ChargeRemaining != ret.DifferenceAmount || len(ret.AllowedSettlementTypes) != 1 || ret.AllowedSettlementTypes[0] != "collect_payment" {
		t.Fatalf("position = charge %d allowed %v, want %d + collect_payment", ret.ChargeRemaining, ret.AllowedSettlementTypes, ret.DifferenceAmount)
	}
	stockBefore := available(t, fx)

	// Refused BEFORE anything moves: over the surcharge, bad method.
	for name, st := range map[string]*deliverycontracts.ReplacementSettlement{
		"over the surcharge": {Amount: ret.DifferenceAmount + 1, Method: "cash"},
		"unknown method":     {Amount: ret.DifferenceAmount, Method: "bogus"},
	} {
		if _, err := m.salesreturn.Service().ConfirmReplacementDelivery(ctx(), 0, fx.branchID, retID, dnID,
			deliverycontracts.ReplacementConfirm{Settlement: st}); err == nil {
			t.Fatalf("%s: expected rejection, got nil", name)
		}
		if got := available(t, fx); got != stockBefore {
			t.Fatalf("%s: stock moved to %v although the settlement was refused (was %v)", name, got, stockBefore)
		}
	}

	// A payment that fails AFTER the stock moved (cash tendered short) must not
	// leave a confirmed shipment: the note stays draft, the retry finishes.
	if _, err := m.salesreturn.Service().ConfirmReplacementDelivery(ctx(), 0, fx.branchID, retID, dnID,
		deliverycontracts.ReplacementConfirm{Settlement: &deliverycontracts.ReplacementSettlement{
			Method: "cash", AmountTendered: ret.DifferenceAmount - 1}}); err == nil {
		t.Fatal("short cash tendered must fail the payment")
	}
	dn, err := m.delivery.Service().GetByID(ctx(), dnID)
	if err != nil || dn == nil || dn.Status != "draft" {
		t.Fatalf("after a failed payment the note must stay draft: %v %+v", err, dn)
	}
	if mid, _ := m.salesreturn.Service().GetByID(ctx(), retID); mid.ReplacementDeliveryStatus != "dispatched" {
		t.Fatalf("return status after failed payment = %q, want dispatched", mid.ReplacementDeliveryStatus)
	}
	var live int
	_ = m.db.QueryRow(`SELECT COUNT(*) FROM payment_allocations a JOIN payments p ON p.id=a.payment_id WHERE a.order_type='sales_return' AND a.order_id=? AND p.status='active'`, retID).Scan(&live)
	if live != 0 {
		t.Fatalf("payments after the failed attempt = %d, want 0", live)
	}

	confirmed, err := m.salesreturn.Service().ConfirmReplacementDelivery(ctx(), 0, fx.branchID, retID, dnID,
		deliverycontracts.ReplacementConfirm{Settlement: &deliverycontracts.ReplacementSettlement{Method: "cash", AmountTendered: ret.DifferenceAmount + 5000}})
	if err != nil || confirmed.Status != "confirmed" {
		t.Fatalf("retry: %v %+v", err, confirmed)
	}
	if got := available(t, fx); got != stockBefore-2 {
		t.Fatalf("stock after confirm = %v, want %v (moved exactly once)", got, stockBefore-2)
	}
	var amount int64
	var direction string
	if err := m.db.QueryRow(`SELECT a.amount, p.direction FROM payment_allocations a JOIN payments p ON p.id=a.payment_id WHERE a.order_type='sales_return' AND a.order_id=? AND p.status='active'`, retID).Scan(&amount, &direction); err != nil {
		t.Fatalf("surcharge payment: %v", err)
	}
	if amount != ret.DifferenceAmount || direction != "in" {
		t.Fatalf("surcharge payment = %d %s, want %d in", amount, direction, ret.DifferenceAmount)
	}
	done, _ := m.salesreturn.Service().GetByID(ctx(), retID)
	if done.ReplacementDeliveryStatus != "confirmed" || done.ChargeRemaining != 0 || len(done.AllowedSettlementTypes) != 0 {
		t.Fatalf("after pay_now: status %q charge remaining %d allowed %v", done.ReplacementDeliveryStatus, done.ChargeRemaining, done.AllowedSettlementTypes)
	}
	if n := len(done.Settlements); n != 1 || done.Settlements[0].Type != "collect_payment" || done.Settlements[0].Amount != ret.DifferenceAmount {
		t.Fatalf("settlement memo = %+v, want one collect_payment of %d", done.Settlements, ret.DifferenceAmount)
	}
	// The surcharge is journalled by the payment module (Dr Kas / Cr Piutang).
	if _, err := m.finance.Service().Post(ctx(), 0, fx.branchID, "delivery", dnID); err != nil {
		t.Fatalf("post replacement delivery: %v", err)
	}
	var payID int64
	_ = m.db.QueryRow(`SELECT p.id FROM payment_allocations a JOIN payments p ON p.id=a.payment_id WHERE a.order_type='sales_return' AND a.order_id=? AND p.status='active'`, retID).Scan(&payID)
	if _, err := m.finance.Service().Post(ctx(), 0, fx.branchID, "payment", payID); err != nil {
		t.Fatalf("post surcharge payment: %v", err)
	}
}

// K-04e: pay_now needs an exchange that costs MORE; a cheaper replacement is a
// refund-kind return that must never turn into money in.
func TestExchangePayNowRefusedWhenNotSurcharge(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	retID, dnID := exchangeWithSurcharge(t, fx, 1) // 1 returned, 1 replacement: even
	stockBefore := available(t, fx)
	if _, err := m.salesreturn.Service().ConfirmReplacementDelivery(ctx(), 0, fx.branchID, retID, dnID,
		deliverycontracts.ReplacementConfirm{Settlement: &deliverycontracts.ReplacementSettlement{Amount: 1000, Method: "cash"}}); err == nil {
		t.Fatal("pay_now on an exchange without a surcharge must be refused")
	}
	if got := available(t, fx); got != stockBefore {
		t.Fatalf("stock moved to %v although pay_now was refused (was %v)", got, stockBefore)
	}
	// Without a settlement the same confirmation still works.
	if _, err := m.salesreturn.Service().ConfirmReplacementDelivery(ctx(), 0, fx.branchID, retID, dnID,
		deliverycontracts.ReplacementConfirm{}); err != nil {
		t.Fatalf("plain confirm: %v", err)
	}
}
