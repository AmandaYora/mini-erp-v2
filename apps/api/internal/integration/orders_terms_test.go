package integration

import (
	"testing"

	goodsreceiptapp "mini-erp/internal/modules/goodsreceipt/application"
	partyapp "mini-erp/internal/modules/party/application"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	purchasingapp "mini-erp/internal/modules/purchasing/application"
	salesapp "mini-erp/internal/modules/sales/application"
	salescontracts "mini-erp/internal/modules/sales/contracts"
)

func termPO(t *testing.T, fx shopFixture, terms string, qty float64) (poID int64, total int64) {
	t.Helper()
	sup, err := testMods.party.Service().CreateSupplier(ctx(), 0, partyapp.PartyInput{Name: "Supplier " + terms})
	if err != nil {
		t.Fatalf("supplier: %v", err)
	}
	in := purchasingapp.OrderInput{
		PartyID: sup.ID, PaymentTerms: terms, TaxType: "none",
		Items: []purchasingapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: qty, UnitPrice: 10000}},
	}
	if terms == "net" {
		in.DueDate = "2026-12-31"
	}
	po, err := testMods.purchasing.Service().CreateOrder(ctx(), 0, fx.branchID, in)
	if err != nil {
		t.Fatalf("create PO: %v", err)
	}
	if _, err := testMods.purchasing.Service().ConfirmOrder(ctx(), 0, fx.branchID, po.ID); err != nil {
		t.Fatalf("confirm PO: %v", err)
	}
	return po.ID, po.GrandTotal
}

func purchasePaid(t *testing.T, poID int64) int64 {
	t.Helper()
	m, err := testMods.payment.Payments().PaidForOrders(ctx(), paymentcontracts.OrderPurchase, []int64{poID})
	if err != nil {
		t.Fatalf("paid: %v", err)
	}
	return m[poID]
}

// F2a-10b: a COD purchase order settles on the receipt that completes it —
// earlier receipts need nothing, the last one is refused without a settlement
// (and records nothing), and pay_now pays exactly once even when repeated.
func TestPurchaseCODSettlesOnLastReceipt(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	poID, total := termPO(t, fx, "cod", 10)
	svc := testMods.goodsreceipt.Service()
	if _, err := svc.Create(ctx(), 0, fx.branchID, poID, "", "", receiptLines(fx, 4)); err != nil {
		t.Fatalf("partial receipt needs no settlement: %v", err)
	}
	if _, err := svc.Create(ctx(), 0, fx.branchID, poID, "", "", receiptLines(fx, 6)); err == nil {
		t.Fatal("completing receipt of an unpaid COD order was accepted without settlement")
	}
	var n int
	_ = testMods.db.QueryRow(`SELECT COUNT(*) FROM goods_receipts WHERE purchase_order_id = ?`, poID).Scan(&n)
	if n != 1 {
		t.Fatalf("receipts after refusal = %d, want 1 (nothing recorded)", n)
	}
	pay := goodsreceiptapp.WithSettlement(&goodsreceiptapp.Settlement{Mode: goodsreceiptapp.SettlePayNow, Method: "cash"})
	if _, err := svc.Create(ctx(), 0, fx.branchID, poID, "", "", receiptLines(fx, 6), pay, goodsreceiptapp.WithClientRef("gr-cod")); err != nil {
		t.Fatalf("completing receipt with pay_now: %v", err)
	}
	if got := purchasePaid(t, poID); got != total {
		t.Fatalf("paid = %d, want %d", got, total)
	}
	if poStatus(t, poID) != "completed" {
		t.Fatalf("PO status = %s, want completed", poStatus(t, poID))
	}
	// Same client reference again: replay, no second payment.
	if _, err := svc.Create(ctx(), 0, fx.branchID, poID, "", "", receiptLines(fx, 6), pay, goodsreceiptapp.WithClientRef("gr-cod")); err != nil {
		t.Fatalf("replay: %v", err)
	}
	if got := purchasePaid(t, poID); got != total {
		t.Fatalf("paid after replay = %d, want %d", got, total)
	}
}

// F2a-10b: switch_to_net converts the COD order to credit terms instead of paying.
func TestPurchaseCODSwitchToNet(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	poID, _ := termPO(t, fx, "cod", 5)
	sw := goodsreceiptapp.WithSettlement(&goodsreceiptapp.Settlement{Mode: goodsreceiptapp.SettleSwitchToNet, DueDate: "2027-01-31"})
	if _, err := testMods.goodsreceipt.Service().Create(ctx(), 0, fx.branchID, poID, "", "", receiptLines(fx, 5), sw); err != nil {
		t.Fatalf("switch_to_net: %v", err)
	}
	var terms string
	_ = testMods.db.QueryRow(`SELECT payment_terms FROM purchase_orders WHERE id = ?`, poID).Scan(&terms)
	if terms != "net" {
		t.Fatalf("terms = %s, want net", terms)
	}
	if got := purchasePaid(t, poID); got != 0 {
		t.Fatalf("paid = %d, want 0", got)
	}
}

// F2a-10b: a prepaid purchase order takes no goods before it is paid, and a
// settlement is meaningless on it.
func TestPurchasePrepaidNeedsPaymentFirst(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	poID, total := termPO(t, fx, "prepaid", 3)
	svc := testMods.goodsreceipt.Service()
	if _, err := svc.Create(ctx(), 0, fx.branchID, poID, "", "", receiptLines(fx, 3)); err == nil {
		t.Fatal("unpaid prepaid order accepted goods")
	}
	if _, _, err := testMods.payment.Payments().RecordForOrder(ctx(), paymentcontracts.RecordInput{
		BranchID: fx.branchID, OrderType: paymentcontracts.OrderPurchase, OrderID: poID, Method: "cash", ClientRef: "prepay-1",
	}); err != nil {
		t.Fatalf("prepay: %v", err)
	}
	if got := purchasePaid(t, poID); got != total {
		t.Fatalf("paid = %d, want %d", got, total)
	}
	if _, err := svc.Create(ctx(), 0, fx.branchID, poID, "", "", receiptLines(fx, 3)); err != nil {
		t.Fatalf("paid prepaid order: %v", err)
	}
}

// F2a-10c: a confirmed order may be edited — freely while nothing hangs off it,
// header-only once a shipment exists.
func TestUpdateConfirmedOrder(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	sales := testMods.sales.Service()
	o, err := sales.CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none",
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 2}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := sales.ConfirmOrder(ctx(), 0, fx.branchID, o.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	edit := func(qty float64, notes string) error {
		_, err := sales.UpdateOrder(ctx(), 0, fx.branchID, o.ID, salesapp.OrderInput{
			PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none", Notes: notes,
			Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: qty}},
		})
		return err
	}
	if err := edit(5, "ubah qty"); err != nil {
		t.Fatalf("economic edit of a clean confirmed order: %v", err)
	}
	cur, _ := sales.GetByID(ctx(), o.ID)
	if cur.Items[0].Qty != 5 || cur.Status != "confirmed" {
		t.Fatalf("after edit qty=%v status=%s", cur.Items[0].Qty, cur.Status)
	}
	if _, err := draftNote(t, fx, o.ID, 1); err != nil {
		t.Fatalf("draft shipment: %v", err)
	}
	if err := edit(6, "ubah lagi"); err == nil {
		t.Fatal("economic edit accepted while a shipment exists")
	}
	if err := edit(5, "hanya catatan"); err != nil {
		t.Fatalf("notes-only edit with a shipment: %v", err)
	}
	cur, _ = sales.GetByID(ctx(), o.ID)
	if cur.Notes != "hanya catatan" || cur.Items[0].Qty != 5 {
		t.Fatalf("after header edit notes=%q qty=%v", cur.Notes, cur.Items[0].Qty)
	}
}

// F2a-10d: list/summary expose paid, outstanding and status from one grouped read.
func TestPaymentSummaries(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 2)
	total := orderTotal(t, soID)
	so, _ := testMods.sales.Service().GetByID(ctx(), soID)
	sum := func() salesapp.PaymentSummary {
		t.Helper()
		m, err := testMods.sales.Service().PaymentSummaries(ctx(), []*salescontracts.SalesOrder{so})
		if err != nil {
			t.Fatalf("summaries: %v", err)
		}
		return m[soID]
	}
	if s := sum(); s.PaymentStatus != "unpaid" || s.OutstandingAmount != total {
		t.Fatalf("unpaid summary = %+v", s)
	}
	if _, err := testMods.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", total/2, payAll(soID, total/2)); err != nil {
		t.Fatalf("pay: %v", err)
	}
	if s := sum(); s.PaymentStatus != "partial" || s.PaidAmount != total/2 {
		t.Fatalf("partial summary = %+v", s)
	}
	payInFull2 := total - total/2
	if _, err := testMods.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", payInFull2, payAll(soID, payInFull2)); err != nil {
		t.Fatalf("pay rest: %v", err)
	}
	if s := sum(); s.PaymentStatus != "paid" || s.OutstandingAmount != 0 {
		t.Fatalf("paid summary = %+v", s)
	}
}

// F2a-10b: a net PO derives its due date from supplier invoice date + credit days.
func TestPurchaseDueDateFromTermDays(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	sup, err := testMods.party.Service().CreateSupplier(ctx(), 0, partyapp.PartyInput{Name: "Supplier Hari"})
	if err != nil {
		t.Fatalf("supplier: %v", err)
	}
	po, err := testMods.purchasing.Service().CreateOrder(ctx(), 0, fx.branchID, purchasingapp.OrderInput{
		PartyID: sup.ID, PaymentTerms: "net", TaxType: "none", PaymentTermDays: 30,
		SupplierInvoiceNumber: "INV-1", SupplierInvoiceDate: "2026-10-01",
		Items: []purchasingapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1, UnitPrice: 1000}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(po.DueDate) < 10 || po.DueDate[:10] != "2026-10-31" || po.PaymentTermDays != 30 {
		t.Fatalf("due=%q days=%d; want 2026-10-31 and 30", po.DueDate, po.PaymentTermDays)
	}
}
