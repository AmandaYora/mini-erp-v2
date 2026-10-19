package integration

import (
	"errors"
	"strings"
	"testing"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	goodsreceiptapp "mini-erp/internal/modules/goodsreceipt/application"
	partyapp "mini-erp/internal/modules/party/application"
	purchasereturnapp "mini-erp/internal/modules/purchasereturn/application"
	purchasingapp "mini-erp/internal/modules/purchasing/application"
	salesreturnapp "mini-erp/internal/modules/salesreturn/application"
	userapp "mini-erp/internal/modules/user/application"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/apperror"
)

func floatp(f float64) *float64 { return &f }

// PO-03: tax invoice, person in charge and a per-line tax rate are stored and
// read back; an unknown person in charge is refused.
func TestPurchaseOrderTaxInvoiceAssigneeLineRate(t *testing.T) {
	freshDB(t)
	fx := purchaseProduct(t, newShop(t, ctx()))
	m := testMods
	sup, err := m.party.Service().CreateSupplier(ctx(), 0, partyapp.PartyInput{Name: "Pemasok Pajak"})
	if err != nil {
		t.Fatalf("supplier: %v", err)
	}
	role, err := m.user.Service().CreateRole(ctx(), 0, userapp.CreateRoleInput{Code: "R-PO", Name: "Peran PO"})
	if err != nil {
		t.Fatalf("role: %v", err)
	}
	u, err := m.user.Service().CreateUser(ctx(), 0, userapp.CreateUserInput{
		Username: "pj-po", Password: "rahasia-kuat-1", FullName: "Penanggung Jawab", RoleIDs: []int64{role.ID},
		Branches: []usercontracts.BranchAccess{{BranchID: fx.branchID, IsDefault: true}},
	})
	if err != nil {
		t.Fatalf("user: %v", err)
	}
	in := purchasingapp.OrderInput{
		PartyID: sup.ID, PaymentTerms: "net", DueDate: "2026-12-10", TaxType: "exclude", TaxRate: 11,
		SupplierInvoiceNumber: "INV-1", SupplierInvoiceDate: "2026-09-01",
		TaxInvoiceNumber: "010.000-26.00000001", TaxInvoiceDate: "2026-09-02", AssignedUserID: u.ID,
		Items: []purchasingapp.LineInput{
			{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1, UnitPrice: 10000, TaxRate: floatp(5)},
		},
	}
	po, err := m.purchasing.Service().CreateOrder(ctx(), u.ID, fx.branchID, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if po.TaxInvoiceNumber != in.TaxInvoiceNumber || !strings.HasPrefix(po.TaxInvoiceDate, "2026-09-02") ||
		po.AssignedUserID != u.ID || po.AssignedUserName != "Penanggung Jawab" || po.CreatedBy != u.ID || po.CreatedByName != "Penanggung Jawab" {
		t.Fatalf("header = %+v", po)
	}
	it := po.Items[0]
	if it.TaxRate != 5 || it.TaxAmount != 500 || po.TaxTotal != 500 || po.GrandTotal != 10500 {
		t.Fatalf("line rate %v tax %d total tax %d grand %d, want 5/500/500/10500", it.TaxRate, it.TaxAmount, po.TaxTotal, po.GrandTotal)
	}
	in.AssignedUserID = 987654
	if _, err := m.purchasing.Service().CreateOrder(ctx(), 0, fx.branchID, in); err == nil {
		t.Fatal("unknown person in charge must be refused")
	}
}

// PO-01: the list carries the return state (none/partial/full), the creator
// name, and finds an order by supplier phone / email / invoice number.
func TestPurchaseOrderListReturnSummaryAndSearch(t *testing.T) {
	freshDB(t)
	fx := purchaseProduct(t, newShop(t, ctx()))
	m := testMods
	sup, err := m.party.Service().CreateSupplier(ctx(), 0, partyapp.PartyInput{Name: "Pemasok Cari", Phone: "0215550199", Email: "cari@pemasok.test"})
	if err != nil {
		t.Fatalf("supplier: %v", err)
	}
	po, err := m.purchasing.Service().CreateOrder(ctx(), 0, fx.branchID, purchasingapp.OrderInput{
		PartyID: sup.ID, PaymentTerms: "net", DueDate: "2026-12-10", TaxType: "none", SupplierInvoiceNumber: "FKT-777",
		Items: []purchasingapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 4, UnitPrice: 10000}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := m.purchasing.Service().ConfirmOrder(ctx(), 0, fx.branchID, po.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if _, err := m.goodsreceipt.Service().Create(ctx(), 0, fx.branchID, po.ID, "2026-09-10", "",
		[]goodsreceiptapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 4}}); err != nil {
		t.Fatalf("receive: %v", err)
	}
	state := func() string {
		t.Helper()
		res, err := m.purchasing.Service().List(ctx(), fx.branchID, purchasingapp.ListFilter{}, 1, 20)
		if err != nil || len(res.Orders) != 1 {
			t.Fatalf("list: %v %v", err, res)
		}
		rs, err := m.purchasing.Service().ReturnSummaries(ctx(), res.Orders)
		if err != nil {
			t.Fatalf("summaries: %v", err)
		}
		return rs[po.ID]
	}
	if got := state(); got != "none" {
		t.Fatalf("state = %q, want none", got)
	}
	ret := func(qty float64) {
		t.Helper()
		r, err := m.purchasereturn.Service().Create(ctx(), 0, fx.branchID, po.ID, "2026-09-10", "",
			[]purchasereturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: qty}},
			purchasereturnapp.WithReason("uji"))
		if err != nil {
			t.Fatalf("return: %v", err)
		}
		if _, err := m.purchasereturn.Service().Confirm(ctx(), 0, fx.branchID, r.ID); err != nil {
			t.Fatalf("confirm return: %v", err)
		}
	}
	ret(1)
	if got := state(); got != "partial" {
		t.Fatalf("state = %q, want partial", got)
	}
	ret(3)
	if got := state(); got != "full" {
		t.Fatalf("state = %q, want full", got)
	}
	for _, term := range []string{"0215550199", "cari@pemasok", "FKT-777", "Pemasok Cari"} {
		res, err := m.purchasing.Service().List(ctx(), fx.branchID, purchasingapp.ListFilter{Search: term}, 1, 20)
		if err != nil || len(res.Orders) != 1 {
			t.Fatalf("search %q: %v %v", term, err, res)
		}
	}
}

// GR-01 + GR-02: the receipt list names the order, the line count and the
// receiver; a line without a location lands in the default location; pay_now
// carries notes and hands back the payment so a slip can be attached.
func TestGoodsReceiptListAndDefaults(t *testing.T) {
	freshDB(t)
	fx := purchaseProduct(t, newShop(t, ctx()))
	m := testMods
	sup, _ := m.party.Service().CreateSupplier(ctx(), 0, partyapp.PartyInput{Name: "Pemasok GR"})
	po, err := m.purchasing.Service().CreateOrder(ctx(), 0, fx.branchID, purchasingapp.OrderInput{
		PartyID: sup.ID, PaymentTerms: "cod", TaxType: "none",
		Items: []purchasingapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 2, UnitPrice: 10000}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := m.purchasing.Service().ConfirmOrder(ctx(), 0, fx.branchID, po.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	role, err := m.user.Service().CreateRole(ctx(), 0, userapp.CreateRoleInput{Code: "R-GR", Name: "Peran GR"})
	if err != nil {
		t.Fatalf("role: %v", err)
	}
	u, err := m.user.Service().CreateUser(ctx(), 0, userapp.CreateUserInput{
		Username: "penerima", Password: "rahasia-kuat-1", FullName: "Petugas Terima", RoleIDs: []int64{role.ID},
		Branches: []usercontracts.BranchAccess{{BranchID: fx.branchID, IsDefault: true}},
	})
	if err != nil {
		t.Fatalf("user: %v", err)
	}
	rec, err := m.goodsreceipt.Service().Create(ctx(), u.ID, fx.branchID, po.ID, "2026-09-10", "", // LocationID 0 on purpose
		[]goodsreceiptapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 2}},
		goodsreceiptapp.WithSettlement(&goodsreceiptapp.Settlement{Mode: goodsreceiptapp.SettlePayNow, Method: "transfer", Reference: "TRX-1", Notes: "lunas di tempat"}))
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if rec.Payment == nil || rec.Payment.ID == 0 {
		t.Fatalf("pay_now must return the payment, got %+v", rec.Payment)
	}
	pay, err := m.payment.Service().GetByID(ctx(), rec.Payment.ID)
	if err != nil || !strings.Contains(pay.Notes, "lunas di tempat") {
		t.Fatalf("payment notes = %v err %v", pay, err)
	}
	var loc int64
	if err := m.db.QueryRow(`SELECT i.location_id FROM goods_receipt_items i WHERE i.receipt_id = ?`, rec.ID).Scan(&loc); err != nil || loc == 0 {
		t.Fatalf("location = %d err %v, want the default location", loc, err)
	}
	if loc == fx.locationID {
		t.Fatal("expected the branch default location, not the test location")
	}

	res, err := m.goodsreceipt.Service().List(ctx(), fx.branchID, 0, 1, 20)
	if err != nil || len(res.Receipts) != 1 {
		t.Fatalf("list: %v %v", err, res)
	}
	r := res.Receipts[0]
	if r.OrderNumber != po.Number || r.LineCount != 1 || r.ReceivedByName != "Petugas Terima" {
		t.Fatalf("row = order %q lines %d by %q", r.OrderNumber, r.LineCount, r.ReceivedByName)
	}
}

// RT-04: purchase return defaults the source location to the latest receipt
// location and refuses a date before the goods were received.
func TestPurchaseReturnLocationDefaultAndDate(t *testing.T) {
	freshDB(t)
	fx := purchaseProduct(t, newShop(t, ctx()))
	m := testMods
	poID := receivePO(t, fx, 10000, 2) // received 2026-09-10 into fx.locationID
	ctxMap, err := m.purchasereturn.Service().ReturnContext(ctx(), fx.branchID, poID)
	if err != nil {
		t.Fatalf("context: %v", err)
	}
	lines := ctxMap["lines"].([]any)
	if got := lines[0].(map[string]any)["defaultLocationId"]; got != fx.locationID {
		t.Fatalf("defaultLocationId = %v, want %d", got, fx.locationID)
	}
	if !strings.HasPrefix(ctxMap["firstReceivedAt"].(string), "2026-09-10") {
		t.Fatalf("firstReceivedAt = %v", ctxMap["firstReceivedAt"])
	}
	_, err = m.purchasereturn.Service().Create(ctx(), 0, fx.branchID, poID, "2026-09-09", "",
		[]purchasereturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 1}},
		purchasereturnapp.WithReason("uji"))
	var ae *apperror.AppError
	if !errors.As(err, &ae) || len(ae.Fields) == 0 || ae.Fields[0].Field != "returnDate" {
		t.Fatalf("return before receipt = %v, want a returnDate refusal", err)
	}
}

// RT-01: both return lists show the order number and search by order number,
// party name and reason.
func TestReturnListsOrderColumnAndSearch(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	soID := sellFixture(t, ctx(), fx, 2)
	deliverAll(t, ctx(), fx, soID, 2)
	so, _ := m.sales.Service().GetByID(ctx(), soID)
	if _, err := m.salesreturn.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", "return_only",
		[]salesreturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 1}}, nil,
		salesreturnapp.WithReason("kemasan penyok")); err != nil {
		t.Fatalf("return: %v", err)
	}
	for _, term := range []string{so.Number, "Customer Uji", "penyok"} {
		res, err := m.salesreturn.Service().ListFiltered(ctx(), fx.branchID, salesreturnapp.ListFilter{Search: term}, 1, 20)
		if err != nil || len(res.Returns) != 1 {
			t.Fatalf("sales search %q: %v %v", term, err, res)
		}
		if res.Returns[0].OrderNumber != so.Number {
			t.Fatalf("order column = %q, want %q", res.Returns[0].OrderNumber, so.Number)
		}
	}
	if res, _ := m.salesreturn.Service().ListFiltered(ctx(), fx.branchID, salesreturnapp.ListFilter{Search: "tidak-ada-zzz"}, 1, 20); len(res.Returns) != 0 {
		t.Fatal("unmatched search must be empty")
	}

	pfx := purchaseProduct(t, fx)
	poID := receivePO(t, pfx, 10000, 2)
	po, _ := m.purchasing.Service().GetByID(ctx(), poID)
	if _, err := m.purchasereturn.Service().Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "",
		[]purchasereturnapp.LineInput{{ProductID: pfx.productID, VariantID: pfx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 1}},
		purchasereturnapp.WithReason("barang cacat")); err != nil {
		t.Fatalf("purchase return: %v", err)
	}
	for _, term := range []string{po.Number, "Supplier Uji", "cacat"} {
		res, err := m.purchasereturn.Service().ListFiltered(ctx(), fx.branchID, purchasereturnapp.ListFilter{Search: term}, 1, 20)
		if err != nil || len(res.Returns) != 1 || res.Returns[0].OrderNumber != po.Number {
			t.Fatalf("purchase search %q: %v %v", term, err, res)
		}
	}
}

// RT-03: a replacement shipment needs driver and warehouse staff, unless the
// company switched the evidence rule off.
func TestReplacementShipmentRequiresCrew(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	if _, err := m.db.Exec(`UPDATE company_settings SET setting_value='true' WHERE setting_key='delivery_require_evidence'`); err != nil {
		t.Fatalf("setting: %v", err)
	}
	soID := sellFixture(t, ctx(), fx, 2)
	deliverAllWithCrew(t, fx, soID, 2)
	ret, err := m.salesreturn.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", "exchange",
		[]salesreturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 1}},
		[]salesreturnapp.ReplacementInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 1}})
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if _, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if _, err := m.salesreturn.Service().CreateReplacementDelivery(ctx(), 0, fx.branchID, ret.ID, salesreturnapp.ReplacementDispatchInput{}); err == nil {
		t.Fatal("a replacement shipment without crew must be refused")
	}
	if _, err := m.salesreturn.Service().CreateReplacementDelivery(ctx(), 0, fx.branchID, ret.ID,
		salesreturnapp.ReplacementDispatchInput{DriverName: "Budi", WarehouseStaffName: "Sari"}); err != nil {
		t.Fatalf("with crew: %v", err)
	}
}

// deliverAllWithCrew ships an order while evidence is required: crew and a
// recipient are given, the signed-document photo is not demanded here because
// the media client of the test harness has no files; the rule is exercised by
// the delivery tests themselves.
func deliverAllWithCrew(t *testing.T, fx shopFixture, soID int64, qty float64) {
	t.Helper()
	m := testMods
	if _, err := m.db.Exec(`UPDATE company_settings SET setting_value='false' WHERE setting_key='delivery_require_evidence'`); err != nil {
		t.Fatalf("setting off: %v", err)
	}
	deliverAll(t, ctx(), fx, soID, qty)
	if _, err := m.db.Exec(`UPDATE company_settings SET setting_value='true' WHERE setting_key='delivery_require_evidence'`); err != nil {
		t.Fatalf("setting on: %v", err)
	}
}

// RT-02: an exchange on a POS order hands the replacement over in the same
// step as the return: stock out right away, replacement closed, and the
// journal of the replacement (a new sale) is postable; a regular order keeps
// its pending replacement.
func TestPosExchangeHandsOverAtCounter(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	o, _, err := m.sales.Service().CreateOrderIdempotent(ctx(), 0, fx.branchID, posOrderInput(fx, "xchg-1", 2))
	if err != nil {
		t.Fatalf("pos order: %v", err)
	}
	if _, err := m.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, o.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	dn, err := m.delivery.Service().Create(ctx(), 0, fx.branchID, o.ID, "2026-09-10", "", deliveryapp.DocInput{},
		[]deliveryapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 2}})
	if err != nil {
		t.Fatalf("delivery: %v", err)
	}
	total := orderTotal(t, o.ID)
	if _, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{
		Settlement: &deliveryapp.Settlement{Mode: deliveryapp.SettlePayNow, Method: "cash", AmountTendered: total},
	}); err != nil {
		t.Fatalf("confirm delivery: %v", err)
	}
	if avail := available(t, fx); avail != 48 {
		t.Fatalf("available after sale = %v, want 48", avail)
	}
	ret, err := m.salesreturn.Service().Create(ctx(), 0, fx.branchID, o.ID, "2026-09-10", "", "exchange",
		[]salesreturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 2}},
		[]salesreturnapp.ReplacementInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 1}})
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if avail := available(t, fx); avail != 48 {
		t.Fatalf("stock moved before confirm: %v", avail)
	}
	done, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID)
	if err != nil {
		t.Fatalf("confirm exchange: %v", err)
	}
	if done.ReplacementDeliveryStatus != "confirmed" {
		t.Fatalf("replacement status = %q, want confirmed in one step", done.ReplacementDeliveryStatus)
	}
	// +2 returned, -1 replacement.
	if avail := available(t, fx); avail != 49 {
		t.Fatalf("available after counter exchange = %v, want 49", avail)
	}
	// Confirming again replays without moving stock a second time.
	if _, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("replay: %v", err)
	}
	if avail := available(t, fx); avail != 49 {
		t.Fatalf("replay moved stock: %v", avail)
	}
	var kind string
	if err := m.db.QueryRow(`SELECT document_kind FROM delivery_notes WHERE sales_return_id = ? AND status='confirmed'`, ret.ID).Scan(&kind); err != nil || kind != "replacement" {
		t.Fatalf("replacement note = %q err %v", kind, err)
	}
	// No manual shipment: exactly one note belongs to the return, auto-confirmed.
	var notes int
	if err := m.db.QueryRow(`SELECT COUNT(*) FROM delivery_notes WHERE sales_return_id = ?`, ret.ID).Scan(&notes); err != nil || notes != 1 {
		t.Fatalf("notes of the return = %d err %v, want exactly the auto note", notes, err)
	}
	retEntry, err := m.finance.Service().Post(ctx(), 0, fx.branchID, "sales_return", ret.ID)
	if err != nil {
		t.Fatalf("post return: %v", err)
	}
	var dnID int64
	if err := m.db.QueryRow(`SELECT id FROM delivery_notes WHERE sales_return_id = ? AND status='confirmed'`, ret.ID).Scan(&dnID); err != nil {
		t.Fatalf("note id: %v", err)
	}
	replEntry, err := m.finance.Service().Post(ctx(), 0, fx.branchID, "delivery", dnID)
	if err != nil {
		t.Fatalf("post replacement journal: %v", err)
	}
	for name, e := range map[string]*financecontracts.JournalEntry{"return": retEntry, "replacement": replEntry} {
		var dr, cr int64
		for _, l := range e.Lines {
			dr += l.Debit
			cr += l.Credit
		}
		if dr == 0 || dr != cr {
			t.Fatalf("%s journal unbalanced: debit %d credit %d", name, dr, cr)
		}
	}
	// The replacement is a NEW sale: revenue credited, cost of goods booked.
	if len(replEntry.Lines) < 4 {
		t.Fatalf("replacement journal has %d lines, want receivable+revenue+cogs+inventory", len(replEntry.Lines))
	}
	if q := liveEntries(t, "delivery", dnID); q != 1 {
		t.Fatalf("replacement journals = %d, want 1", q)
	}
}
