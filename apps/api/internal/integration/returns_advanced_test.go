package integration

import (
	"testing"

	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	purchasereturnapp "mini-erp/internal/modules/purchasereturn/application"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	salesapp "mini-erp/internal/modules/sales/application"
	salesreturnapp "mini-erp/internal/modules/salesreturn/application"
	salesreturncontracts "mini-erp/internal/modules/salesreturn/contracts"
)

func returnWith(t *testing.T, fx shopFixture, soID int64, cond string, qty float64) *salesreturncontracts.SalesReturn {
	t.Helper()
	ret, err := testMods.salesreturn.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", "return_only",
		[]salesreturnapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: qty, Condition: cond,
		}}, nil, salesreturnapp.WithReason("uji"))
	if err != nil {
		t.Fatalf("create %s return: %v", cond, err)
	}
	return ret
}

// F2a-8 #1: damaged goods land in the damaged location (not sellable stock);
// goods the customer keeps move no stock but still return their value.
func TestReturnConditions(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 10)
	deliverAll(t, ctx(), fx, soID, 10) // stock 40
	svc := testMods.salesreturn.Service()
	stock := testMods.stock.Service()

	dmgRet := returnWith(t, fx, soID, "damaged", 3)
	if dmgRet.Items[0].Condition != "damaged" {
		t.Fatalf("condition = %q", dmgRet.Items[0].Condition)
	}
	if _, err := svc.Confirm(ctx(), 0, fx.branchID, dmgRet.ID); err != nil {
		t.Fatalf("confirm damaged: %v", err)
	}
	dmg, err := stock.DamagedLocation(ctx(), fx.branchID)
	if err != nil {
		t.Fatalf("damaged location: %v", err)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, dmg.ID); got != 3 {
		t.Fatalf("damaged stock = %v, want 3", got)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 40 {
		t.Fatalf("sellable stock = %v, want 40 (damaged goods are not sellable)", got)
	}

	keptRet := returnWith(t, fx, soID, "not_returned", 2)
	if _, err := svc.Confirm(ctx(), 0, fx.branchID, keptRet.ID); err != nil {
		t.Fatalf("confirm kept: %v", err)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 40 {
		t.Fatalf("stock moved for goods the customer kept: %v", got)
	}
	if keptRet.Total <= 0 {
		t.Fatalf("kept-goods return carries no value: %d", keptRet.Total)
	}
}

// F2a-8 #3: the return is offset automatically against the order's open bill,
// up to the smaller of the two; a repeated confirm never offsets twice.
func TestReturnAutoOffset(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 10)
	deliverAll(t, ctx(), fx, soID, 10) // order is now net, nothing paid
	total := orderTotal(t, soID)
	svc := testMods.salesreturn.Service()
	pay := testMods.payment.Service()

	ret := returnWith(t, fx, soID, "normal", 4)
	if _, err := svc.Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	done, _ := svc.GetByID(ctx(), ret.ID)
	if done.ReceivableReduction != ret.Total {
		t.Fatalf("reduction = %d, want the whole return %d", done.ReceivableReduction, ret.Total)
	}
	auto := 0
	for _, m := range done.Settlements {
		if m.Auto {
			auto++
		}
	}
	if auto != 1 {
		t.Fatalf("auto settlement memos = %d, want 1", auto)
	}
	// Repeat: replay, still one offset.
	if _, err := svc.Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("repeat confirm: %v", err)
	}
	var legs int
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM payments WHERE method='offset' AND status='active'`).Scan(&legs); err != nil || legs != 2 {
		t.Fatalf("offset legs = %d, %v; want 2 (one pair)", legs, err)
	}
	paid, _ := pay.PaidForOrders(ctx(), paymentcontracts.OrderSales, []int64{soID})
	if paid[soID] != ret.Total {
		t.Fatalf("order credited %d, want %d", paid[soID], ret.Total)
	}
	bal, _ := pay.GetPartyBalance(ctx(), fx.branchID, fx.customerID)
	if bal.Outstanding != total-ret.Total {
		t.Fatalf("outstanding = %d, want %d", bal.Outstanding, total-ret.Total)
	}

	// A return larger than what is still open: only the open part is offset,
	// the rest stays refundable.
	big := returnWith(t, fx, soID, "normal", 6)
	if _, err := svc.Confirm(ctx(), 0, fx.branchID, big.ID); err != nil {
		t.Fatalf("confirm big: %v", err)
	}
	bigDone, _ := svc.GetByID(ctx(), big.ID)
	if bigDone.ReceivableReduction != total-ret.Total {
		t.Fatalf("second reduction = %d, want the remaining bill %d", bigDone.ReceivableReduction, total-ret.Total)
	}
}

// F2a-8 #2: an exchange is priced with the order's tax; the signed difference
// decides refund vs charge for the payment module.
func TestExchangeDifferenceAndKind(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	so, err := m.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "exclude", TaxRate: 10,
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 2}},
	})
	if err != nil {
		t.Fatalf("create SO: %v", err)
	}
	if _, err := m.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
		t.Fatalf("confirm SO: %v", err)
	}
	deliverAll(t, ctx(), fx, so.ID, 2)

	exchange := func(replQty float64) *salesreturncontracts.SalesReturn {
		ret, err := m.salesreturn.Service().Create(ctx(), 0, fx.branchID, so.ID, "2026-09-10", "", "exchange",
			[]salesreturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 1}},
			[]salesreturnapp.ReplacementInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: replQty}},
			salesreturnapp.WithReason("tukar"))
		if err != nil {
			t.Fatalf("exchange: %v", err)
		}
		return ret
	}
	// Return 1 pc (15000 + 10% = 16500); replace with 2 pcs (33000): customer owes 16500.
	dearer := exchange(2)
	if dearer.ReplacementTaxTotal != 3000 || dearer.ReplacementTotal != 33000 || dearer.DifferenceAmount != 16500 {
		t.Fatalf("dearer exchange = tax %d total %d diff %d", dearer.ReplacementTaxTotal, dearer.ReplacementTotal, dearer.DifferenceAmount)
	}
	proj, err := m.salesreturn.Service().GetReturn(ctx(), dearer.ID)
	if err != nil || proj.Kind != salesreturncontracts.KindCharge || proj.Total != 16500 {
		t.Fatalf("projection = %+v, %v; want charge 16500", proj, err)
	}
	if _, err := m.salesreturn.Service().Cancel(ctx(), 0, fx.branchID, dearer.ID, ""); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	// Replace with nothing dearer than a half... use the same qty: difference 0.
	equal := exchange(1)
	if equal.DifferenceAmount != 0 {
		t.Fatalf("equal exchange diff = %d, want 0", equal.DifferenceAmount)
	}
	// An even exchange, once confirmed, owes the customer nothing: the party
	// balance must not list its goods value as money owed (it would invite a
	// refund the customer was never due).
	if _, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, equal.ID); err != nil {
		t.Fatalf("confirm equal exchange: %v", err)
	}
	bal, err := m.payment.Service().GetPartyBalance(ctx(), fx.branchID, fx.customerID)
	if err != nil {
		t.Fatalf("party balance: %v", err)
	}
	if len(bal.Returns) != 0 || bal.TotalReturned != 0 {
		t.Fatalf("even exchange counted as owed: returns=%+v totalReturned=%d", bal.Returns, bal.TotalReturned)
	}
}

// F2a-8 #5 (purchase side): future-dated return rejected; a confirmed return
// is offset against the PO bill.
func TestPurchaseReturnDateAndOffset(t *testing.T) {
	freshDB(t)
	fx := purchaseProduct(t, newShop(t, ctx()))
	poID := receivePO(t, fx, 10000, 10)
	svc := testMods.purchasereturn.Service()
	line := []purchasereturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 4}}
	if _, err := svc.Create(ctx(), 0, fx.branchID, poID, "2999-01-01", "", line); err == nil {
		t.Fatal("future-dated purchase return accepted")
	}
	ret, err := svc.Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "", line, purchasereturnapp.WithReason("cacat"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	done, _ := svc.GetByID(ctx(), ret.ID)
	if done.PayableReduction != ret.Total || ret.Total != 40000 {
		t.Fatalf("payable reduction = %d (return %d), want 40000", done.PayableReduction, ret.Total)
	}
	paid, _ := testMods.payment.Service().PaidForOrders(ctx(), paymentcontracts.OrderPurchase, []int64{poID})
	if paid[poID] != 40000 {
		t.Fatalf("PO credited %d, want 40000", paid[poID])
	}
	// Audit P5: the PO shows no money paid — the return cleared part of the
	// bill (returnOffset), exactly as the sales side splits it (SL-03).
	po, err := testMods.purchasing.Service().GetByID(ctx(), poID)
	if err != nil || po == nil {
		t.Fatalf("get PO: %v", err)
	}
	sums, err := testMods.purchasing.Service().PaymentSummaries(ctx(), []*purchasingcontracts.PurchaseOrder{po})
	if err != nil {
		t.Fatalf("summaries: %v", err)
	}
	if got := sums[poID]; got.PaidAmount != 0 || got.ReturnOffset != 40000 || got.OutstandingAmount != po.GrandTotal-40000 || got.PaymentStatus != "unpaid" {
		t.Fatalf("PO summary = %+v, want paid 0, offset 40000, outstanding %d, unpaid", got, po.GrandTotal-40000)
	}
}

// Audit S6: cancelling the automatic offset pair (the step before cancelling
// a confirmed return) releases the return's mirror of it — the reduction and
// the auto memo — so the order balance, the refundable remainder and the books
// agree. A repeated confirm then does not mirror the cancelled pair again.
func TestCancelledAutoOffsetReleasesReturn(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 10)
	deliverAll(t, ctx(), fx, soID, 10)
	svc := testMods.salesreturn.Service()
	pay := testMods.payment.Service()
	ret := returnWith(t, fx, soID, "normal", 4)
	if _, err := svc.Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	var refundLeg int64 // cancel through the leg WITHOUT the client ref
	if err := testMods.db.QueryRow(`SELECT id FROM payments WHERE method='offset' AND client_ref IS NULL`).Scan(&refundLeg); err != nil {
		t.Fatalf("refund leg: %v", err)
	}
	if _, err := pay.Cancel(ctx(), 0, fx.branchID, refundLeg); err != nil {
		t.Fatalf("cancel offset: %v", err)
	}
	check := func(when string) {
		t.Helper()
		got, err := svc.GetByID(ctx(), ret.ID)
		if err != nil {
			t.Fatalf("%s: get return: %v", when, err)
		}
		if got.ReceivableReduction != 0 {
			t.Fatalf("%s: reduction = %d, want 0", when, got.ReceivableReduction)
		}
		for _, m := range got.Settlements {
			if m.Auto {
				t.Fatalf("%s: auto memo kept: %+v", when, m)
			}
		}
		paid, _ := pay.PaidForOrders(ctx(), paymentcontracts.OrderSales, []int64{soID})
		if paid[soID] != 0 {
			t.Fatalf("%s: order still credited %d", when, paid[soID])
		}
	}
	check("after cancel")
	if _, err := svc.Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("repeat confirm: %v", err)
	}
	check("after repeat confirm")
	if _, err := svc.Cancel(ctx(), 0, fx.branchID, ret.ID, ""); err != nil {
		t.Fatalf("cancel return once its offset is gone: %v", err)
	}
}
