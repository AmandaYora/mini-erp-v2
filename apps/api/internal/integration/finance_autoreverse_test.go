package integration

import "testing"

func liveEntries(t *testing.T, docType string, docID int64) int {
	t.Helper()
	var n int
	if err := testMods.db.QueryRow(
		`SELECT COUNT(*) FROM finance_journal_entries WHERE source_doc_type = ? AND source_doc_id = ? AND status = 'posted'`,
		docType, docID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

// F2b-5: cancelling a payment after it was journalled reverses its journal and
// does not put it back in the posting queue.
func TestCancelledPaymentTakesItsJournal(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 20)
	deliverAll(t, ctx(), fx, soID, 10) // keeps the order open so the cancel guard allows it
	total := orderTotal(t, soID)
	pay, err := testMods.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", total/4, payAll(soID, total/4))
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	if _, err := testMods.finance.Service().Post(ctx(), 0, fx.branchID, "payment", pay.ID); err != nil {
		t.Fatalf("post: %v", err)
	}
	cashBefore, _, _ := testMods.finance.Service().AccountBalance(ctx(), fx.branchID, "1000")
	if cashBefore == 0 {
		t.Fatal("cash did not move on posting")
	}
	if _, err := testMods.payment.Service().Cancel(ctx(), 0, fx.branchID, pay.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if n := liveEntries(t, "payment", pay.ID); n != 0 {
		t.Fatalf("live journals after cancel = %d, want 0", n)
	}
	d, c, _ := testMods.finance.Service().AccountBalance(ctx(), fx.branchID, "1000")
	if d-c != 0 {
		t.Fatalf("cash after cancel = %d, want 0", d-c)
	}
	q, _ := testMods.finance.Service().UnpostedSources(ctx(), fx.branchID)
	for _, it := range q {
		if it.DocType == "payment" && it.DocID == pay.ID {
			t.Fatal("a cancelled payment came back into the posting queue")
		}
	}
}

// F2b-5: cancelling a confirmed return reverses its journal.
func TestCancelledReturnTakesItsJournal(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 10)
	deliverAll(t, ctx(), fx, soID, 10)
	ret := createSalesReturn(t, fx, soID, 3)
	if _, err := testMods.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	// Cancel is refused while the auto-offset exists; clear it first.
	var legID int64
	if err := testMods.db.QueryRow(`SELECT id FROM payments WHERE method='offset' AND status='active' LIMIT 1`).Scan(&legID); err != nil {
		t.Fatalf("offset leg: %v", err)
	}
	if _, err := testMods.finance.Service().Post(ctx(), 0, fx.branchID, "sales_return", ret.ID); err != nil {
		t.Fatalf("post return: %v", err)
	}
	if _, err := testMods.payment.Service().Cancel(ctx(), 0, fx.branchID, legID); err != nil {
		t.Fatalf("cancel offset pair: %v", err)
	}
	if _, err := testMods.salesreturn.Service().Cancel(ctx(), 0, fx.branchID, ret.ID, ""); err != nil {
		t.Fatalf("cancel return: %v", err)
	}
	if n := liveEntries(t, "sales_return", ret.ID); n != 0 {
		t.Fatalf("live journals after cancel = %d, want 0", n)
	}
}
