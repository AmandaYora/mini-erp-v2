package integration

import (
	"errors"
	"strings"
	"testing"

	financeapp "mini-erp/internal/modules/finance/application"
	paymentapp "mini-erp/internal/modules/payment/application"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	purchasereturnapp "mini-erp/internal/modules/purchasereturn/application"
	salesreturnapp "mini-erp/internal/modules/salesreturn/application"
	"mini-erp/internal/shared/apperror"
)

// netOf is debit - credit of an account code in the branch books.
func netOf(t *testing.T, branchID int64, code string) int64 {
	t.Helper()
	d, c, err := testMods.finance.Service().AccountBalance(ctx(), branchID, code)
	if err != nil {
		t.Fatalf("balance %s: %v", code, err)
	}
	return d - c
}

func queueHas(t *testing.T, branchID int64, docType string, docID int64) bool {
	t.Helper()
	q, err := testMods.finance.Service().UnpostedSources(ctx(), branchID)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	for _, it := range q {
		if it.DocType == docType && it.DocID == docID {
			return true
		}
	}
	return false
}

func orphanHas(t *testing.T, branchID int64, docType string, docID int64) bool {
	t.Helper()
	o, err := testMods.finance.Service().Orphans(ctx(), branchID)
	if err != nil {
		t.Fatalf("orphans: %v", err)
	}
	for _, it := range o {
		if it.DocType == docType && it.DocID == docID {
			return true
		}
	}
	return false
}

// creditSalesFixture sells and delivers 2 units, takes the full payment and
// returns both: the whole return value is excess over the (settled) bill.
// Everything up to the return is journalled, so only the settlement is left.
func creditSalesFixture(t *testing.T) (fx shopFixture, soID int64, retID, retTotal int64) {
	t.Helper()
	fx = newShop(t, ctx())
	soID = sellFixture(t, ctx(), fx, 2)
	deliverAll(t, ctx(), fx, soID, 2)
	total := orderTotal(t, soID)
	pay, err := testMods.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", total, payAll(soID, total))
	if err != nil {
		t.Fatalf("pay order: %v", err)
	}
	ret := createSalesReturn(t, fx, soID, 2)
	if _, err := testMods.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm return: %v", err)
	}
	fin := testMods.finance.Service()
	var dnID int64
	if err := testMods.db.QueryRow(`SELECT id FROM delivery_notes WHERE sales_order_id = ? AND status='confirmed' LIMIT 1`, soID).Scan(&dnID); err != nil {
		t.Fatalf("delivery note: %v", err)
	}
	for _, p := range []struct {
		typ string
		id  int64
	}{{"delivery", dnID}, {"payment", pay.ID}, {"sales_return", ret.ID}} {
		if _, err := fin.Post(ctx(), 0, fx.branchID, p.typ, p.id); err != nil {
			t.Fatalf("post %s: %v", p.typ, err)
		}
	}
	return fx, soID, ret.ID, ret.Total
}

// K-04a: a customer_credit settlement is no memo any more. It enters the
// posting queue, books Dr Piutang / Cr Uang Muka Pelanggan on the customer, the
// advance shows on the balance sheet as its own liability (not a negative
// receivable), and cancelling the return takes the journal with it.
func TestCreditSettlementJournalSales(t *testing.T) {
	freshDB(t)
	fx, _, retID, retTotal := creditSalesFixture(t)
	fin := testMods.finance.Service()

	// Before the settlement the excess hides as a NEGATIVE receivable.
	if ar := netOf(t, fx.branchID, "1100"); ar != -retTotal {
		t.Fatalf("receivable before = %d, want %d (negative: return credited it)", ar, -retTotal)
	}
	view, err := testMods.salesreturn.Service().GetByID(ctx(), retID)
	if err != nil {
		t.Fatalf("get return: %v", err)
	}
	if view.ExternalRemaining != retTotal || !contains(view.AllowedSettlementTypes, "customer_credit") {
		t.Fatalf("position = remaining %d allowed %v, want %d + customer_credit", view.ExternalRemaining, view.AllowedSettlementTypes, retTotal)
	}

	done, err := testMods.salesreturn.Service().AddSettlement(ctx(), 0, fx.branchID, retID,
		salesreturnapp.SettlementInput{Type: "customer_credit", Amount: retTotal})
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	sid := done.Settlements[0].ID
	if done.ExternalRemaining != 0 || len(done.AllowedSettlementTypes) != 0 {
		t.Fatalf("after settlement remaining %d allowed %v, want none", done.ExternalRemaining, done.AllowedSettlementTypes)
	}
	if !queueHas(t, fx.branchID, "sales_return_settlement", sid) {
		t.Fatal("credit settlement missing from the posting queue")
	}
	prev, err := fin.Preview(ctx(), fx.branchID, "sales_return_settlement", sid)
	if err != nil || len(prev.Lines) != 2 {
		t.Fatalf("preview: %v %+v", err, prev)
	}

	entry, err := fin.Post(ctx(), 0, fx.branchID, "sales_return_settlement", sid)
	if err != nil {
		t.Fatalf("post settlement: %v", err)
	}
	legs := journalSums(entry)
	if legs["1100"] != [2]int64{retTotal, 0} || legs["2200"] != [2]int64{0, retTotal} {
		t.Fatalf("legs = %v, want Dr 1100 / Cr 2200 of %d", legs, retTotal)
	}
	for _, l := range entry.Lines {
		if l.PartyID != fx.customerID {
			t.Fatalf("line %s party = %d, want the customer %d", l.AccountCode, l.PartyID, fx.customerID)
		}
	}
	again, err := fin.Post(ctx(), 0, fx.branchID, "sales_return_settlement", sid)
	if err != nil || again.ID != entry.ID {
		t.Fatalf("re-post must replay: %v %v", err, again)
	}
	if queueHas(t, fx.branchID, "sales_return_settlement", sid) {
		t.Fatal("posted settlement still queued")
	}

	// Receivable back to zero, the credit sits in Uang Muka Pelanggan.
	if ar := netOf(t, fx.branchID, "1100"); ar != 0 {
		t.Fatalf("receivable after = %d, want 0", ar)
	}
	if adv := netOf(t, fx.branchID, "2200"); adv != -retTotal {
		t.Fatalf("advance net = %d, want credit %d", adv, retTotal)
	}
	bs, err := fin.BalanceSheet(ctx(), fx.branchID, "2099-12-31")
	if err != nil {
		t.Fatalf("balance sheet: %v", err)
	}
	if !bs.Balanced {
		t.Fatalf("balance sheet unbalanced: assets %d vs %d", bs.TotalAssets, bs.TotalLiaEq)
	}
	var seen int64
	for _, r := range bs.Liabilities {
		if r.Code == "2200" {
			seen = r.Debit // TrialRow carries the signed balance in Debit
		}
	}
	if seen != retTotal {
		t.Fatalf("balance sheet liability 2200 = %d, want %d", seen, retTotal)
	}
	if rep, err := fin.VerifyRollups(ctx(), fx.branchID); err != nil || !rep.Consistent {
		t.Fatalf("rollup integrity: %v %+v", err, rep)
	}
	if orphanHas(t, fx.branchID, "sales_return_settlement", sid) {
		t.Fatal("live settlement reported as orphan")
	}

	// Cancelling the return reverses BOTH journals.
	if _, err := testMods.salesreturn.Service().Cancel(ctx(), 0, fx.branchID, retID); err != nil {
		t.Fatalf("cancel return: %v", err)
	}
	if n := liveEntries(t, "sales_return_settlement", sid); n != 0 {
		t.Fatalf("live settlement journals after cancel = %d, want 0", n)
	}
	if adv := netOf(t, fx.branchID, "2200"); adv != 0 {
		t.Fatalf("advance after cancel = %d, want 0", adv)
	}
	if ar := netOf(t, fx.branchID, "1100"); ar != 0 {
		t.Fatalf("receivable after cancel = %d, want 0", ar)
	}
	if queueHas(t, fx.branchID, "sales_return_settlement", sid) || orphanHas(t, fx.branchID, "sales_return_settlement", sid) {
		t.Fatal("cancelled return's settlement is queued or orphaned")
	}
	if _, err := fin.Post(ctx(), 0, fx.branchID, "sales_return_settlement", sid); err == nil {
		t.Fatal("a cancelled return's settlement must not post")
	}
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// K-04a safety net: when the automatic reversal did not happen, the journal of
// a settlement whose return is cancelled shows up as an orphan.
func TestCreditSettlementOrphanAfterCancel(t *testing.T) {
	freshDB(t)
	fx, _, retID, retTotal := creditSalesFixture(t)
	fin := testMods.finance.Service()
	done, err := testMods.salesreturn.Service().AddSettlement(ctx(), 0, fx.branchID, retID,
		salesreturnapp.SettlementInput{Type: "customer_credit", Amount: retTotal})
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	sid := done.Settlements[0].ID
	if _, err := fin.Post(ctx(), 0, fx.branchID, "sales_return_settlement", sid); err != nil {
		t.Fatalf("post: %v", err)
	}
	// Simulate a failed auto-reverse: the return flips to cancelled behind
	// finance's back.
	if _, err := testMods.db.Exec(`UPDATE sales_returns SET status='cancelled' WHERE id = ?`, retID); err != nil {
		t.Fatalf("flip status: %v", err)
	}
	if !orphanHas(t, fx.branchID, "sales_return_settlement", sid) {
		t.Fatal("orphan not reported for the settlement of a cancelled return")
	}
	// The accountant clears it from the batch screen ("reverse" action).
	res, err := fin.PostBatch(ctx(), 0, fx.branchID, []financeapp.BatchItem{{DocType: "sales_return_settlement", DocID: sid, Action: "reverse"}})
	if err != nil || res[0].Error != "" {
		t.Fatalf("reverse orphan: %v %+v", err, res)
	}
	if orphanHas(t, fx.branchID, "sales_return_settlement", sid) {
		t.Fatal("orphan still listed after reversal")
	}
	if adv := netOf(t, fx.branchID, "2200"); adv != 0 {
		t.Fatalf("advance after orphan reversal = %d, want 0", adv)
	}
}

// K-04c through the service: collect_payment on a plain return, refund of a
// part the open bill already absorbed, and a manual reduce_receivable are all
// refused with a clear Indonesian message.
func TestSalesSettlementRulesRejected(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 2)
	deliverAll(t, ctx(), fx, soID, 2) // unpaid: the return offsets the open bill
	ret := createSalesReturn(t, fx, soID, 2)
	if _, err := testMods.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	svc := testMods.salesreturn.Service()
	for _, c := range []struct{ typ, want string }{
		{"collect_payment", "Tambah bayar hanya untuk retur tukar"},
		{"refund", "sepenuhnya mengurangi sisa piutang"},
		{"customer_credit", "sepenuhnya mengurangi sisa piutang"},
		{"reduce_receivable", "dilakukan otomatis"},
	} {
		_, err := svc.AddSettlement(ctx(), 0, fx.branchID, ret.ID, salesreturnapp.SettlementInput{Type: c.typ, Amount: 1000})
		var ae *apperror.AppError
		if !errors.As(err, &ae) || !strings.Contains(ae.Message, c.want) {
			t.Fatalf("%s: err = %v, want message containing %q", c.typ, err, c.want)
		}
	}
	if _, err := svc.AddSettlement(ctx(), 0, fx.branchID, ret.ID, salesreturnapp.SettlementInput{Type: "bogus", Amount: 1000}); err == nil {
		t.Fatal("unknown type must be refused")
	}
}

// K-04a + K-04b purchase side: supplier_credit books Dr Uang Muka Supplier /
// Cr Hutang on the supplier; the sales vocabulary is gone; cancel reverses.
func TestCreditSettlementJournalPurchase(t *testing.T) {
	freshDB(t)
	fx := purchaseProduct(t, newShop(t, ctx()))
	m := testMods
	fin := m.finance.Service()

	poID := purchaseFixture(t, ctx(), fx, 10000, 2)
	po, err := m.purchasing.Service().GetByID(ctx(), poID)
	if err != nil || po == nil {
		t.Fatalf("get PO: %v", err)
	}
	// Pay the bill in full: the whole return is excess over what we owe.
	pay, err := m.payment.Service().Create(ctx(), 0, fx.branchID, po.PartyID, "cash", "", "", po.GrandTotal,
		[]paymentapp.AllocationInput{{OrderType: paymentcontracts.OrderPurchase, OrderID: poID, Amount: po.GrandTotal}})
	if err != nil {
		t.Fatalf("pay PO: %v", err)
	}
	ret, err := m.purchasereturn.Service().Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "",
		[]purchasereturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 2}})
	if err != nil {
		t.Fatalf("create return: %v", err)
	}
	if _, err := m.purchasereturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm return: %v", err)
	}
	var grID int64
	if err := m.db.QueryRow(`SELECT id FROM goods_receipts WHERE purchase_order_id = ? LIMIT 1`, poID).Scan(&grID); err != nil {
		t.Fatalf("receipt: %v", err)
	}
	for _, p := range []struct {
		typ string
		id  int64
	}{{"goods_receipt", grID}, {"payment", pay.ID}, {"purchase_return", ret.ID}} {
		if _, err := fin.Post(ctx(), 0, fx.branchID, p.typ, p.id); err != nil {
			t.Fatalf("post %s: %v", p.typ, err)
		}
	}
	if ap := netOf(t, fx.branchID, "2000"); ap != ret.Total {
		t.Fatalf("payable before = %d, want debit %d (return relieved a paid bill)", ap, ret.Total)
	}

	svc := m.purchasereturn.Service()
	// K-04b/c: sales types and the automatic reduce are refused.
	for _, typ := range []string{"customer_credit", "reduce_receivable", "collect_payment", "reduce_payable"} {
		if _, err := svc.AddSettlement(ctx(), 0, fx.branchID, ret.ID, purchasereturnapp.SettlementInput{Type: typ, Amount: 1000}); err == nil {
			t.Fatalf("%s must be refused on a purchase return", typ)
		}
	}
	if _, err := svc.AddSettlement(ctx(), 0, fx.branchID, ret.ID, purchasereturnapp.SettlementInput{Type: "supplier_credit", Amount: ret.Total + 1}); err == nil {
		t.Fatal("over-settlement must be refused")
	}
	done, err := svc.AddSettlement(ctx(), 0, fx.branchID, ret.ID, purchasereturnapp.SettlementInput{Type: "supplier_credit", Amount: ret.Total})
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	sid := done.Settlements[0].ID
	if !queueHas(t, fx.branchID, "purchase_return_settlement", sid) {
		t.Fatal("supplier credit missing from the posting queue")
	}
	entry, err := fin.Post(ctx(), 0, fx.branchID, "purchase_return_settlement", sid)
	if err != nil {
		t.Fatalf("post settlement: %v", err)
	}
	legs := journalSums(entry)
	if legs["2300"] != [2]int64{ret.Total, 0} || legs["2000"] != [2]int64{0, ret.Total} {
		t.Fatalf("legs = %v, want Dr 2300 / Cr 2000 of %d", legs, ret.Total)
	}
	for _, l := range entry.Lines {
		if l.PartyID != po.PartyID {
			t.Fatalf("line %s party = %d, want supplier %d", l.AccountCode, l.PartyID, po.PartyID)
		}
	}
	if ap := netOf(t, fx.branchID, "2000"); ap != 0 {
		t.Fatalf("payable after = %d, want 0", ap)
	}
	if adv := netOf(t, fx.branchID, "2300"); adv != ret.Total {
		t.Fatalf("supplier advance = %d, want debit %d", adv, ret.Total)
	}
	bs, err := fin.BalanceSheet(ctx(), fx.branchID, "2099-12-31")
	if err != nil || !bs.Balanced {
		t.Fatalf("balance sheet: %v balanced=%v", err, bs != nil && bs.Balanced)
	}
	var seen int64
	for _, r := range bs.Assets {
		if r.Code == "2300" {
			seen = r.Debit
		}
	}
	if seen != ret.Total {
		t.Fatalf("balance sheet asset 2300 = %d, want %d", seen, ret.Total)
	}

	if _, err := svc.Cancel(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("cancel return: %v", err)
	}
	if n := liveEntries(t, "purchase_return_settlement", sid); n != 0 {
		t.Fatalf("live settlement journals after cancel = %d, want 0", n)
	}
	if adv := netOf(t, fx.branchID, "2300"); adv != 0 {
		t.Fatalf("advance after cancel = %d, want 0", adv)
	}
	if queueHas(t, fx.branchID, "purchase_return_settlement", sid) || orphanHas(t, fx.branchID, "purchase_return_settlement", sid) {
		t.Fatal("cancelled return's settlement is queued or orphaned")
	}
}

// K-04a: the cutover accepts the new source types and refuses to cover a
// settlement this system already journalled.
func TestCreditSettlementCutover(t *testing.T) {
	freshDB(t)
	fx, _, retID, retTotal := creditSalesFixture(t)
	fin := testMods.finance.Service()
	done, err := testMods.salesreturn.Service().AddSettlement(ctx(), 0, fx.branchID, retID,
		salesreturnapp.SettlementInput{Type: "customer_credit", Amount: retTotal})
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	sid := done.Settlements[0].ID
	if err := fin.SetCutover(ctx(), 0, fx.branchID, "purchase_return_settlement", 0); err != nil {
		t.Fatalf("cutover purchase kind: %v", err)
	}
	if err := fin.SetCutover(ctx(), 0, fx.branchID, "sales_return_settlement", sid); err != nil {
		t.Fatalf("cutover covering the unposted settlement: %v", err)
	}
	if queueHas(t, fx.branchID, "sales_return_settlement", sid) {
		t.Fatal("a covered settlement must leave the queue (ETL: already journalled in legacy)")
	}
	if err := fin.SetCutover(ctx(), 0, fx.branchID, "sales_return_settlement", 0); err != nil {
		t.Fatalf("reset cutover: %v", err)
	}
	if _, err := fin.Post(ctx(), 0, fx.branchID, "sales_return_settlement", sid); err != nil {
		t.Fatalf("post: %v", err)
	}
	if err := fin.SetCutover(ctx(), 0, fx.branchID, "sales_return_settlement", sid); err == nil {
		t.Fatal("cutover must refuse to cover a settlement already journalled here")
	}
}
