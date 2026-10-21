package integration

import (
	"strings"
	"testing"

	"mini-erp/internal/modules/finance/infrastructure"
	goodsreceiptapp "mini-erp/internal/modules/goodsreceipt/application"
	partyapp "mini-erp/internal/modules/party/application"
	paymentapp "mini-erp/internal/modules/payment/application"
	purchasereturnapp "mini-erp/internal/modules/purchasereturn/application"
	purchasingapp "mini-erp/internal/modules/purchasing/application"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	"mini-erp/internal/shared/timeutil"
)

// ADR-0023: correcting human input on purchase orders without breaking what
// the books, the stock ledger and the documents already hold.

// correctionPO creates a confirmed PO of qty units of fx's product at unit
// (tax exclude 11%, no discount) and receives `received` of them (posted).
func correctionPO(t *testing.T, fx shopFixture, unit int64, qty, received float64) (poID, supplierID int64) {
	t.Helper()
	m := testMods
	sup, err := m.party.Service().CreateSupplier(ctx(), 0, partyapp.PartyInput{Name: "Supplier Koreksi"})
	if err != nil {
		t.Fatalf("supplier: %v", err)
	}
	po, err := m.purchasing.Service().CreateOrder(ctx(), 0, fx.branchID, purchasingapp.OrderInput{
		PartyID: sup.ID, PaymentTerms: "net", DueDate: "2026-12-31", TaxType: "exclude", TaxRate: 11,
		Items: []purchasingapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: qty, UnitPrice: unit}},
	})
	if err != nil {
		t.Fatalf("create PO: %v", err)
	}
	if _, err := m.purchasing.Service().ConfirmOrder(ctx(), 0, fx.branchID, po.ID); err != nil {
		t.Fatalf("confirm PO: %v", err)
	}
	if received > 0 {
		if _, err := m.goodsreceipt.Service().Create(ctx(), 0, fx.branchID, po.ID, "2026-09-10", "",
			[]goodsreceiptapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: received}}); err != nil {
			t.Fatalf("receive: %v", err)
		}
	}
	return po.ID, sup.ID
}

func receiptIDsOf(t *testing.T, poID int64) []int64 {
	t.Helper()
	rows, err := testMods.db.Query(`SELECT id FROM goods_receipts WHERE purchase_order_id = ? AND status = 'posted' ORDER BY id`, poID)
	if err != nil {
		t.Fatalf("receipts: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

// booksBalanced asserts every journal line of the branch foots (Σ debit = Σ credit).
func booksBalanced(t *testing.T, branchID int64) {
	t.Helper()
	var d, c int64
	if err := testMods.db.QueryRow(`SELECT COALESCE(SUM(l.debit),0), COALESCE(SUM(l.credit),0)
		FROM finance_journal_lines l JOIN finance_journal_entries e ON e.id = l.entry_id WHERE e.branch_id = ?`, branchID).Scan(&d, &c); err != nil {
		t.Fatalf("trial balance: %v", err)
	}
	if d != c {
		t.Fatalf("books do not balance: debit %d, credit %d", d, c)
	}
}

func reconExact(t *testing.T, branchID int64) {
	t.Helper()
	rec, err := testMods.finance.Service().ReconcileInventory(ctx(), branchID)
	if err != nil {
		t.Fatalf("recon: %v", err)
	}
	// The shop fixture's own opening adjustment waits in the queue; whatever
	// else differs between book and ledger must be explained (zero).
	if !rec.Consistent || rec.Unexplained != 0 {
		t.Fatalf("recon = %+v, want book = ledger", rec)
	}
}

// Task 1: a confirmed order with a receipt keeps its economics. A price or qty
// change in a full update is refused by name — never saved half with the
// price silently dropped — while the header still saves.
func TestConfirmedPOEditRefusesEconomicChangeUnderReceipt(t *testing.T) {
	freshDB(t)
	fx := costedProduct(t, newShop(t, ctx()), "KRS-1", "Barang Koreksi")
	poID, supID := correctionPO(t, fx, 10000, 10, 4)
	svc := testMods.purchasing.Service()
	in := purchasingapp.OrderInput{
		PartyID: supID, PaymentTerms: "net", DueDate: "2026-12-31", TaxType: "exclude", TaxRate: 11, Notes: "catatan baru",
		Items: []purchasingapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 10, UnitPrice: 12000}},
	}
	_, err := svc.UpdateOrder(ctx(), 0, fx.branchID, poID, in)
	if !isConflict(err) || !strings.Contains(err.Error(), "harga/diskon") {
		t.Fatalf("price change under a receipt = %v, want conflict naming harga/diskon", err)
	}
	po, _ := svc.GetByID(ctx(), poID)
	if po.Items[0].UnitPrice != 10000 || po.Notes == "catatan baru" {
		t.Fatalf("refused update wrote something: price %d, notes %q", po.Items[0].UnitPrice, po.Notes)
	}
	in.Items[0].UnitPrice, in.Items[0].Qty = 10000, 12
	if _, err := svc.UpdateOrder(ctx(), 0, fx.branchID, poID, in); !isConflict(err) || !strings.Contains(err.Error(), "qty") {
		t.Fatalf("qty change under a receipt = %v, want conflict naming qty", err)
	}
	// Same economics, new header: saved.
	in.Items[0].Qty, in.Reason = 10, "faktur datang"
	in.SupplierInvoiceNumber = "INV-77"
	got, err := svc.UpdateOrder(ctx(), 0, fx.branchID, poID, in)
	if err != nil || got.Notes != "catatan baru" || got.SupplierInvoiceNumber != "INV-77" {
		t.Fatalf("header-only update = %+v, %v", got, err)
	}
	// The dedicated header edit saves the header alone.
	if _, err := svc.UpdateHeader(ctx(), 0, fx.branchID, poID, purchasingapp.HeaderInput{
		DueDate: "2027-01-15", TaxInvoiceNumber: "010.000-26.00000001", Reason: "jatuh tempo diperpanjang",
	}); err != nil {
		t.Fatalf("header edit: %v", err)
	}
	po, _ = svc.GetByID(ctx(), poID)
	if !strings.HasPrefix(po.DueDate, "2027-01-15") || po.TaxInvoiceNumber != "010.000-26.00000001" || po.GrandTotal != 111000 {
		t.Fatalf("after header edit: due %s, tax inv %q, total %d", po.DueDate, po.TaxInvoiceNumber, po.GrandTotal)
	}
	if n := auditCount(t, "purchase_orders.update"); n < 2 {
		t.Fatalf("update audit rows = %d", n)
	}
}

// Task 3: cancel needs a reason, keeps it, and an order with a POSTED receipt
// is refused with a pointer to the purchase return / short close.
func TestCancelPurchaseOrderWithReason(t *testing.T) {
	freshDB(t)
	fx := costedProduct(t, newShop(t, ctx()), "KRS-2", "Barang Batal")
	svc := testMods.purchasing.Service()
	draftID, _ := correctionPO(t, fx, 5000, 3, 0)
	if _, err := svc.CancelOrder(ctx(), 0, fx.branchID, draftID, "  "); !isValidation(err) {
		t.Fatalf("cancel without reason = %v, want validation", err)
	}
	po, err := svc.CancelOrder(ctx(), 7, fx.branchID, draftID, "dobel input")
	if err != nil || po.Status != purchasingcontracts.StatusCancelled || po.CancelReason != "dobel input" || po.CancelledAt == "" || po.CancelledBy != 7 {
		t.Fatalf("cancel = %+v, %v", po, err)
	}
	if again, err := svc.CancelOrder(ctx(), 7, fx.branchID, draftID, "lagi"); err != nil || again.CancelReason != "dobel input" {
		t.Fatalf("replay = %+v, %v", again, err)
	}
	received, _ := correctionPO(t, fx, 5000, 3, 1)
	_, err = svc.CancelOrder(ctx(), 0, fx.branchID, received, "batal")
	if !isConflict(err) || !strings.Contains(err.Error(), "retur pembelian") || !strings.Contains(err.Error(), "Tutup sisa order") {
		t.Fatalf("cancel with a posted receipt = %v, want the return / short close pointer", err)
	}
}

// Task 4: closing the rest of a partly received order cuts every line to what
// arrived, moves no journal, and leaves the supplier owing exactly the
// receipts' payable minus what was paid.
func TestShortCloseMatchesReceiptPayable(t *testing.T) {
	freshDB(t)
	fx := costedProduct(t, newShop(t, ctx()), "KRS-3", "Barang Sisa")
	poID, supID := correctionPO(t, fx, 10001, 3, 0)
	m := testMods
	svc := m.purchasing.Service()
	if _, _, err := svc.ShortClose(ctx(), 0, fx.branchID, poID, "pemasok kehabisan"); !isConflict(err) {
		t.Fatalf("short close with nothing received = %v, want conflict", err)
	}
	if _, err := m.goodsreceipt.Service().Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "",
		[]goodsreceiptapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 1}}); err != nil {
		t.Fatalf("receive: %v", err)
	}
	pending := insertPendingReceipt(t, fx, poID, 1, "GR-SC-PENDING")
	if _, _, err := svc.ShortClose(ctx(), 0, fx.branchID, poID, "pemasok kehabisan"); !isConflict(err) {
		t.Fatalf("short close with a pending receipt = %v, want conflict", err)
	}
	if _, err := m.goodsreceipt.Service().Discard(ctx(), 0, fx.branchID, pending, ""); err != nil {
		t.Fatalf("discard: %v", err)
	}
	var payable int64
	for _, id := range receiptIDsOf(t, poID) {
		e, err := m.finance.Service().Post(ctx(), 0, fx.branchID, "goods_receipt", id)
		if err != nil {
			t.Fatalf("post receipt: %v", err)
		}
		payable += journalSums(e)["2000"][1]
	}
	if _, err := m.payment.Service().Create(ctx(), 0, fx.branchID, supID, "cash", "", "", 5000,
		[]paymentapp.AllocationInput{{OrderType: "purchase", OrderID: poID, Amount: 5000}}); err != nil {
		t.Fatalf("pay: %v", err)
	}
	entriesBefore := countRows(t, `SELECT COUNT(*) FROM finance_journal_entries`)
	pv, err := svc.PreviewShortClose(ctx(), fx.branchID, poID)
	if err != nil || len(pv.Lines) != 1 || pv.Lines[0].NewQty != 1 {
		t.Fatalf("preview = %+v, %v", pv, err)
	}
	po, done, err := svc.ShortClose(ctx(), 0, fx.branchID, poID, "pemasok kehabisan")
	if err != nil {
		t.Fatalf("short close: %v", err)
	}
	if po.Status != purchasingcontracts.StatusCompleted || po.ShortCloseReason != "pemasok kehabisan" || po.Items[0].Qty != 1 {
		t.Fatalf("closed order = status %s, reason %q, qty %v", po.Status, po.ShortCloseReason, po.Items[0].Qty)
	}
	if po.GrandTotal != payable || done.NewGrandTotal != payable || done.Outstanding != payable-5000 {
		t.Fatalf("order total %d / preview %+v, want the receipts' payable %d", po.GrandTotal, done, payable)
	}
	if n := countRows(t, `SELECT COUNT(*) FROM finance_journal_entries`); n != entriesBefore {
		t.Fatalf("journals %d -> %d: a short close must not journal", entriesBefore, n)
	}
	bal, err := m.payment.Service().GetPartyBalance(ctx(), fx.branchID, supID)
	if err != nil || bal.Outstanding != payable-5000 {
		t.Fatalf("supplier balance = %+v, %v; want %d", bal, err, payable-5000)
	}
	// The receipt journal rebuilt today still books what it booked.
	ids := receiptIDsOf(t, poID)
	e, err := m.finance.Service().Preview(ctx(), fx.branchID, "goods_receipt", ids[0])
	if err != nil || journalSums(e)["2000"][1] != payable {
		t.Fatalf("receipt rebuilt after the close = %+v, %v; want payable %d", e, err, payable)
	}
	if _, _, err := svc.ShortClose(ctx(), 0, fx.branchID, poID, "lagi"); !isConflict(err) {
		t.Fatalf("second short close = %v, want conflict", err)
	}
	if n := auditCount(t, "purchase_orders.short_close"); n != 1 {
		t.Fatalf("short close audit rows = %d", n)
	}
}

// Task 5: a wrong purchase price found after receipt and after part of the
// goods were sold. The order takes the new price, finance books the received
// part's difference: the share still on hand to Persediaan with a matching
// value-only ledger row, the sold share to HPP, PPN Masukan and Hutang on the
// supplier. Books balance, book = ledger, replays are idempotent, and the
// receipt journal rebuilt later still books the old price.
func TestPurchasePriceCorrectionAfterReceipt(t *testing.T) {
	freshDB(t)
	fx := costedProduct(t, newShop(t, ctx()), "KRS-4", "Barang Harga Salah")
	m := testMods
	fin := m.finance.Service()
	svc := m.purchasing.Service()
	poID, supID := correctionPO(t, fx, 10000, 10, 10)
	recs := receiptIDsOf(t, poID)
	if _, err := fin.Post(ctx(), 0, fx.branchID, "goods_receipt", recs[0]); err != nil {
		t.Fatalf("post receipt: %v", err)
	}
	dn := sellAndDeliver(t, fx, fx.productID, fx.variantID, 4)
	if _, err := fin.Post(ctx(), 0, fx.branchID, "delivery", dn); err != nil {
		t.Fatalf("post delivery: %v", err)
	}
	reconExact(t, fx.branchID)
	po, _ := svc.GetByID(ctx(), poID)
	lines := []purchasingapp.PriceLineInput{{LineID: po.Items[0].ID, UnitPrice: 12000}}

	pv, err := svc.PreviewPriceCorrection(ctx(), fx.branchID, poID, lines)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if pv.ValueDelta != 20000 || pv.TaxDelta != 2200 || pv.StockDelta != 12000 || pv.CogsDelta != 8000 ||
		pv.OldGrandTotal != 111000 || pv.NewGrandTotal != 133200 {
		t.Fatalf("preview = %+v", pv)
	}
	if _, _, err := svc.CorrectPrices(ctx(), 0, fx.branchID, poID, lines, ""); !isValidation(err) {
		t.Fatalf("correction without reason = %v, want validation", err)
	}
	po, done, err := svc.CorrectPrices(ctx(), 5, fx.branchID, poID, lines, "harga di faktur 12.000")
	if err != nil {
		t.Fatalf("correct: %v", err)
	}
	if po.GrandTotal != 133200 || po.Items[0].UnitPrice != 12000 || po.Items[0].TaxBase != 120000 || done.CorrectionID == 0 {
		t.Fatalf("corrected order = total %d, line %+v, correction %d", po.GrandTotal, po.Items[0], done.CorrectionID)
	}
	if !queueHas(t, fx.branchID, "purchase_price_correction", done.CorrectionID) {
		t.Fatal("the correction is not in the posting queue")
	}
	entry, err := fin.Post(ctx(), 0, fx.branchID, "purchase_price_correction", done.CorrectionID)
	if err != nil {
		t.Fatalf("post correction: %v", err)
	}
	legs := journalSums(entry)
	if legs["1200"] != [2]int64{12000, 0} || legs["5000"] != [2]int64{8000, 0} ||
		legs["1300"] != [2]int64{2200, 0} || legs["2000"] != [2]int64{0, 22200} {
		t.Fatalf("correction legs = %v", legs)
	}
	for _, l := range entry.Lines {
		if l.AccountCode == "2000" && l.PartyID != supID {
			t.Fatalf("Hutang leg party = %d, want supplier %d", l.PartyID, supID)
		}
	}
	booksBalanced(t, fx.branchID)
	reconExact(t, fx.branchID)
	if _, total := positionOf(t, fx.branchID, fx.productID); total != 72000 {
		t.Fatalf("position value = %v, want 6 x 12.000", total)
	}
	// Idempotent replay: the same journal, no second ledger row.
	again, err := fin.Post(ctx(), 0, fx.branchID, "purchase_price_correction", done.CorrectionID)
	if err != nil || again.ID != entry.ID {
		t.Fatalf("replay = %+v, %v", again, err)
	}
	if rows := refRows(t, fx.branchID, fx.productID, infrastructure.RefPurchasePriceCorrection); len(rows) != 1 || rows[0].total != 12000 {
		t.Fatalf("value rows = %+v, want one +12000", rows)
	}
	if queueHas(t, fx.branchID, "purchase_price_correction", done.CorrectionID) {
		t.Fatal("a posted correction is still queued")
	}
	// The supplier owes the corrected total.
	bal, err := m.payment.Service().GetPartyBalance(ctx(), fx.branchID, supID)
	if err != nil || bal.TotalBilled != 133200 || bal.Outstanding != 133200 {
		t.Fatalf("supplier balance = %+v, %v", bal, err)
	}
	// The receipt journal rebuilt after the correction still books the old price.
	rebuilt, err := fin.Preview(ctx(), fx.branchID, "goods_receipt", recs[0])
	if err != nil || journalSums(rebuilt)["1200"][0] != 100000 {
		t.Fatalf("receipt rebuilt = %+v, %v; want Persediaan 100000", rebuilt, err)
	}
	// Reversing the correction journal (it re-queues) takes its ledger rows
	// back; re-posting books them once again — book = ledger throughout.
	if _, err := fin.Reverse(ctx(), 0, fx.branchID, entry.ID, "salah posting"); err != nil {
		t.Fatalf("reverse: %v", err)
	}
	reconExact(t, fx.branchID)
	if _, err := fin.Post(ctx(), 0, fx.branchID, "purchase_price_correction", done.CorrectionID); err != nil {
		t.Fatalf("re-post: %v", err)
	}
	booksBalanced(t, fx.branchID)
	reconExact(t, fx.branchID)
	if _, total := positionOf(t, fx.branchID, fx.productID); total != 72000 {
		t.Fatalf("position value after re-post = %v, want 72000", total)
	}
	if n := auditCount(t, "purchase_orders.price_correct"); n != 1 {
		t.Fatalf("price correction audit rows = %d", n)
	}
}

// A second receipt after the correction is valued at the new price and the
// order's receipts plus the correction add up to the corrected order.
func TestPriceCorrectionThenFurtherReceipt(t *testing.T) {
	freshDB(t)
	fx := costedProduct(t, newShop(t, ctx()), "KRS-5", "Barang Lanjut")
	m := testMods
	fin := m.finance.Service()
	poID, _ := correctionPO(t, fx, 1000, 3, 1)
	po, _ := m.purchasing.Service().GetByID(ctx(), poID)
	_, done, err := m.purchasing.Service().CorrectPrices(ctx(), 0, fx.branchID, poID,
		[]purchasingapp.PriceLineInput{{LineID: po.Items[0].ID, UnitPrice: 1001}}, "pembulatan pemasok")
	if err != nil {
		t.Fatalf("correct: %v", err)
	}
	if _, err := m.goodsreceipt.Service().Create(ctx(), 0, fx.branchID, poID, "2026-09-11", "",
		[]goodsreceiptapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 2}}); err != nil {
		t.Fatalf("second receipt: %v", err)
	}
	var payable int64
	for _, id := range receiptIDsOf(t, poID) {
		e, err := fin.Post(ctx(), 0, fx.branchID, "goods_receipt", id)
		if err != nil {
			t.Fatalf("post receipt: %v", err)
		}
		payable += journalSums(e)["2000"][1]
	}
	e, err := fin.Post(ctx(), 0, fx.branchID, "purchase_price_correction", done.CorrectionID)
	if err != nil {
		t.Fatalf("post correction: %v", err)
	}
	payable += journalSums(e)["2000"][1] - journalSums(e)["2000"][0]
	po, _ = m.purchasing.Service().GetByID(ctx(), poID)
	if po.Status != purchasingcontracts.StatusCompleted || payable != po.GrandTotal {
		t.Fatalf("receipts + correction payable %d vs order total %d (status %s)", payable, po.GrandTotal, po.Status)
	}
	booksBalanced(t, fx.branchID)
	reconExact(t, fx.branchID)
}

// The correction is refused while a purchase return holds the old price, on a
// cancelled order, and when today's period is closed.
func TestPriceCorrectionRefusals(t *testing.T) {
	freshDB(t)
	fx := costedProduct(t, newShop(t, ctx()), "KRS-6", "Barang Tolak")
	m := testMods
	svc := m.purchasing.Service()
	poID, _ := correctionPO(t, fx, 10000, 5, 5)
	po, _ := svc.GetByID(ctx(), poID)
	lines := []purchasingapp.PriceLineInput{{LineID: po.Items[0].ID, UnitPrice: 9000}}
	ret, err := m.purchasereturn.Service().Create(ctx(), 0, fx.branchID, poID, "2026-09-12", "",
		[]purchasereturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 1}})
	if err != nil {
		t.Fatalf("return: %v", err)
	}
	if _, _, err := svc.CorrectPrices(ctx(), 0, fx.branchID, poID, lines, "salah"); !isConflict(err) || !strings.Contains(err.Error(), "retur") {
		t.Fatalf("correction under a return = %v, want conflict", err)
	}
	if _, err := m.purchasereturn.Service().Cancel(ctx(), 0, fx.branchID, ret.ID, ""); err != nil {
		t.Fatalf("cancel return: %v", err)
	}
	today := timeutil.TodayWIB()
	if _, err := m.db.Exec(`INSERT INTO finance_periods (branch_id, year, month, status) VALUES (?, ?, ?, 'closed')`,
		fx.branchID, today[:4], today[5:7]); err != nil {
		t.Fatalf("close period: %v", err)
	}
	if _, _, err := svc.CorrectPrices(ctx(), 0, fx.branchID, poID, lines, "salah"); !isConflict(err) || !strings.Contains(err.Error(), "Periode") {
		t.Fatalf("correction in a closed period = %v, want conflict", err)
	}
	if _, err := m.db.Exec(`DELETE FROM finance_periods WHERE branch_id = ?`, fx.branchID); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	cancelled, _ := correctionPO(t, fx, 10000, 2, 0)
	if _, err := svc.CancelOrder(ctx(), 0, fx.branchID, cancelled, "batal"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	c, _ := svc.GetByID(ctx(), cancelled)
	if _, _, err := svc.CorrectPrices(ctx(), 0, fx.branchID, cancelled,
		[]purchasingapp.PriceLineInput{{LineID: c.Items[0].ID, UnitPrice: 1}}, "x"); !isConflict(err) {
		t.Fatalf("correction of a cancelled order = %v, want conflict", err)
	}
	if _, _, err := svc.CorrectPrices(ctx(), 0, fx.branchID, poID, lines, "harga turun"); err != nil {
		t.Fatalf("correction once clear: %v", err)
	}
}
