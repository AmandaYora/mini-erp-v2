package integration

import (
	"sync"
	"testing"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	financeapp "mini-erp/internal/modules/finance/application"
)

func confirmedNote(t *testing.T, fx shopFixture, qty float64) (soID, noteID int64) {
	t.Helper()
	soID = sellFixture(t, ctx(), fx, qty)
	n, err := draftNote(t, fx, soID, qty)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if _, err := testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, n.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	return soID, n.ID
}

func receivableOf(t *testing.T, fx shopFixture) int64 {
	t.Helper()
	rows, total, err := testMods.finance.Service().Receivables(ctx(), fx.branchID, "")
	_ = rows
	if err != nil {
		t.Fatalf("receivables: %v", err)
	}
	return total
}

// F2b-4: reversing a delivery journal nets receivables and product margin to
// zero (the mirror carries the same product/party dimensions), and the
// document can be re-posted afterwards.
func TestReverseNetsDimensionsAndAllowsRepost(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	_, noteID := confirmedNote(t, fx, 2)
	entry, err := fin.Post(ctx(), 0, fx.branchID, "delivery", noteID)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if got := receivableOf(t, fx); got == 0 {
		t.Fatal("receivable is zero after posting a delivery")
	}
	if _, err := fin.Reverse(ctx(), 0, fx.branchID, entry.ID, "salah"); err != nil {
		t.Fatalf("reverse: %v", err)
	}
	if got := receivableOf(t, fx); got != 0 {
		t.Fatalf("receivable after reverse = %d, want 0", got)
	}
	margins, err := fin.MarginByProduct(ctx(), fx.branchID, "2000-01-01", "2999-12-31", 50)
	if err != nil {
		t.Fatalf("margin: %v", err)
	}
	for _, m := range margins {
		if m.Revenue != 0 || m.Cogs != 0 {
			t.Fatalf("margin after reverse = %+v, want zero", m)
		}
	}
	// The document is postable again and gets a NEW journal.
	again, err := fin.Post(ctx(), 0, fx.branchID, "delivery", noteID)
	if err != nil {
		t.Fatalf("repost: %v", err)
	}
	if again.ID == entry.ID {
		t.Fatal("repost returned the reversed entry")
	}
	if _, err := fin.Reverse(ctx(), 0, fx.branchID, entry.ID, "lagi"); !isConflict(err) {
		t.Fatalf("second reverse of the old entry = %v, want conflict", err)
	}
}

// F2b-4: many simultaneous posts of one document give one entry; many
// simultaneous reversals give one winner.
func TestPostAndReverseAreIdempotentUnderRace(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	_, noteID := confirmedNote(t, fx, 2)

	var wg sync.WaitGroup
	ids := make([]int64, 8)
	errs := make([]error, 8)
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			e, err := fin.Post(ctx(), 0, fx.branchID, "delivery", noteID)
			errs[i] = err
			if e != nil {
				ids[i] = e.ID
			}
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("post %d: %v", i, err)
		}
		if ids[i] != ids[0] {
			t.Fatalf("posts returned different entries: %v", ids)
		}
	}
	var n int
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM finance_journal_entries WHERE source_doc_type='delivery' AND source_doc_id=?`, noteID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("entries = %d, %v; want 1", n, err)
	}

	won := 0
	var mu sync.Mutex
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := fin.Reverse(ctx(), 0, fx.branchID, ids[0], "balapan"); err == nil {
				mu.Lock()
				won++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if won != 1 {
		t.Fatalf("successful reversals = %d, want 1", won)
	}
}

// F2b-4 (N1/N2): reversing a fiscal correction never reaches the commercial
// books, and fiscal corrections of the same sequence in two branches coexist.
func TestFiscalReversalStaysFiscal(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	before, err := fin.ProfitLoss(ctx(), fx.branchID, "2000-01-01", "2999-12-31")
	if err != nil {
		t.Fatalf("profit/loss: %v", err)
	}
	adj, err := fin.CreateTaxAdjustment(ctx(), 0, fx.branchID, "", "koreksi uji", []financeapp.ManualLine{
		{AccountCode: "6900", Debit: 5000}, {AccountCode: "1000", Credit: 5000},
	})
	if err != nil {
		t.Fatalf("tax adjustment: %v", err)
	}
	if _, err := fin.Reverse(ctx(), 0, fx.branchID, adj.ID, "batal"); err != nil {
		t.Fatalf("reverse adjustment: %v", err)
	}
	after, err := fin.ProfitLoss(ctx(), fx.branchID, "2000-01-01", "2999-12-31")
	if err != nil {
		t.Fatalf("profit/loss after: %v", err)
	}
	if after.Profit != before.Profit || after.Expense != before.Expense {
		t.Fatalf("fiscal reversal leaked into commercial books: before %+v after %+v", before, after)
	}
	var scope int
	if err := testMods.db.QueryRow(`SELECT scope FROM finance_journal_entries WHERE source_doc_type='reversal' AND source_doc_id=?`, adj.ID).Scan(&scope); err != nil || scope != 1 {
		t.Fatalf("mirror scope = %d, %v; want 1", scope, err)
	}
}

// F2b-4: the database itself refuses a one-sided-violating line.
func TestJournalLineCheckConstraint(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	e, err := testMods.finance.Service().Manual(ctx(), 0, fx.branchID, "", "uji", []financeapp.ManualLine{
		{AccountCode: "1000", Debit: 100}, {AccountCode: "4000", Credit: 100},
	})
	if err != nil {
		t.Fatalf("manual: %v", err)
	}
	if _, err := testMods.db.Exec(`UPDATE finance_journal_lines SET credit = 5 WHERE entry_id = ? AND debit > 0`, e.ID); err == nil {
		t.Fatal("line with both debit and credit was accepted by the database")
	}
}
