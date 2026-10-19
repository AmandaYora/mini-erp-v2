package integration

import (
	"fmt"
	"sync"
	"testing"

	financeapp "mini-erp/internal/modules/finance/application"
	paymentapp "mini-erp/internal/modules/payment/application"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
)

// insertConfirmedNotes writes n confirmed shipment headers straight into the
// table (the queue only needs the headers) and returns the first and last id.
func insertConfirmedNotes(t *testing.T, branchID int64, n int, date string) (first, last int64) {
	t.Helper()
	for i := 0; i < n; i++ {
		res, err := testMods.db.Exec(
			`INSERT INTO delivery_notes (number, branch_id, sales_order_id, delivery_date, status)
			 VALUES (?, ?, 999999, ?, 'confirmed')`, fmt.Sprintf("SJ-BULK-%s-%d-%d", date, branchID, i), branchID, date)
		if err != nil {
			t.Fatalf("insert note: %v", err)
		}
		id, _ := res.LastInsertId()
		if first == 0 {
			first = id
		}
		last = id
	}
	return
}

func queueCount(t *testing.T, branchID int64, docType string) int {
	t.Helper()
	q, err := testMods.finance.Service().UnpostedSources(ctx(), branchID)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	n := 0
	for _, it := range q {
		if it.DocType == docType {
			n++
		}
	}
	return n
}

// F2b-9: the queue is exact — 1.200 unposted shipments are all seen (the old
// scan stopped at the newest 200), and the close gate counts every one.
func TestQueueIsExactBeyondTwoHundred(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	insertConfirmedNotes(t, fx.branchID, 1200, "2026-09-10")
	if got := queueCount(t, fx.branchID, "delivery"); got != 1200 {
		t.Fatalf("queued shipments = %d, want 1200", got)
	}
	check, err := testMods.finance.Service().Readiness(ctx(), fx.branchID, 2026, 9)
	if err != nil {
		t.Fatalf("readiness: %v", err)
	}
	if check.Ready || check.Unposted < 1200 {
		t.Fatalf("readiness ready=%v unposted=%d; want blocked with >= 1200", check.Ready, check.Unposted)
	}
}

// F2b-10: documents at or below the cutover boundary never enter the queue,
// and a boundary may not claim documents this system already journalled.
func TestCutoverBoundary(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	first, last := insertConfirmedNotes(t, fx.branchID, 10, "2026-09-10")
	fin := testMods.finance.Service()
	mid := first + 4
	if err := fin.SetCutover(ctx(), 0, fx.branchID, "delivery", mid); err != nil {
		t.Fatalf("set cutover: %v", err)
	}
	if got := queueCount(t, fx.branchID, "delivery"); got != int(last-mid) {
		t.Fatalf("queued = %d, want %d (ids above %d)", got, last-mid, mid)
	}
	// Idempotent.
	if err := fin.SetCutover(ctx(), 0, fx.branchID, "delivery", mid); err != nil {
		t.Fatalf("set cutover again: %v", err)
	}
	if err := fin.SetCutover(ctx(), 0, fx.branchID, "bogus", 1); err == nil {
		t.Fatal("unknown doc type accepted")
	}
}

// F2b-10: a document of an already-closed month stays visible as a warning but
// neither blocks another month nor breaks batch posting.
func TestClosedPeriodDocsDoNotBlockOtherMonths(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	// Close August first, then a shipment dated in August appears.
	if err := fin.ClosePeriod(ctx(), 0, fx.branchID, 2026, 8); err != nil {
		t.Fatalf("close: %v", err)
	}
	insertConfirmedNotes(t, fx.branchID, 1, "2026-08-15")
	q, err := fin.UnpostedSources(ctx(), fx.branchID)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	flagged := 0
	for _, it := range q {
		if it.DocType == "delivery" && it.PeriodClosed {
			flagged++
		}
	}
	if flagged != 1 {
		t.Fatalf("period-closed items = %d, want 1", flagged)
	}
	// Clear the fixture's own stock movement so September is clean.
	for _, id := range queuedStockMovements(t, fx.branchID) {
		postedLegs(t, "stock_movement", id)
	}
	check, err := fin.Readiness(ctx(), fx.branchID, 2026, 9)
	if err != nil {
		t.Fatalf("readiness: %v", err)
	}
	if !check.Ready || check.LatePeriodDocs != 1 {
		t.Fatalf("september readiness ready=%v late=%d blockers=%v; want ready with 1 late doc", check.Ready, check.LatePeriodDocs, check.Blockers)
	}
	// CloseDay skips the closed-month document instead of failing on it.
	res, err := fin.CloseDay(ctx(), 0, fx.branchID, "2026-12-31")
	if err != nil {
		t.Fatalf("close day: %v", err)
	}
	for _, r := range res.Results {
		if r.DocType == "delivery" {
			t.Fatalf("close-day tried to post a closed-month document: %+v", r)
		}
	}
}

// F2b-5: when the automatic reversal did not happen, the orphan check finds
// the journal of the cancelled payment, blocks the close, and a reverse batch
// repairs it.
func TestOrphanedJournalDetectedAndRepaired(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 20)
	deliverAll(t, ctx(), fx, soID, 10)
	total := orderTotal(t, soID)
	pay, err := testMods.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", total/4,
		[]paymentapp.AllocationInput{{OrderType: paymentcontracts.OrderSales, OrderID: soID, Amount: total / 4}})
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	fin := testMods.finance.Service()
	if _, err := fin.Post(ctx(), 0, fx.branchID, "payment", pay.ID); err != nil {
		t.Fatalf("post: %v", err)
	}
	testMods.payment.SetReverser(nil) // the automatic reversal "fails"
	defer testMods.payment.SetReverser(testMods.finance.Reverser())
	if _, err := testMods.payment.Service().Cancel(ctx(), 0, fx.branchID, pay.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	orphans, err := fin.Orphans(ctx(), fx.branchID)
	if err != nil || len(orphans) != 1 || orphans[0].DocID != pay.ID {
		t.Fatalf("orphans = %+v, %v; want the cancelled payment", orphans, err)
	}
	if check, _ := fin.Readiness(ctx(), fx.branchID, 2026, 10); check.Ready {
		t.Fatal("readiness passed with an orphaned journal")
	}
	res, err := fin.PostBatch(ctx(), 0, fx.branchID, []financeapp.BatchItem{{DocType: "payment", DocID: pay.ID, Action: "reverse"}})
	if err != nil || res[0].Error != "" {
		t.Fatalf("reverse batch = %+v, %v", res, err)
	}
	if orphans, _ := fin.Orphans(ctx(), fx.branchID); len(orphans) != 0 {
		t.Fatalf("orphans after repair = %+v", orphans)
	}
}

// F2b-10: a posting racing the safe-close either lands before the gate (and
// blocks it) or is refused after it — never inside the sealed month.
func TestSafeCloseVersusPostRace(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	_, noteID := confirmedNote(t, fx, 1) // dated today
	for _, id := range queuedStockMovements(t, fx.branchID) {
		postedLegs(t, "stock_movement", id)
	}
	year, month := currentYearMonth(t, noteID)
	var wg sync.WaitGroup
	var closeErr, postErr error
	wg.Add(2)
	go func() { defer wg.Done(); closeErr = fin.SafeClose(ctx(), 0, fx.branchID, year, month) }()
	go func() { defer wg.Done(); _, postErr = fin.Post(ctx(), 0, fx.branchID, "delivery", noteID) }()
	wg.Wait()
	var n int
	if err := testMods.db.QueryRow(
		`SELECT COUNT(*) FROM finance_journal_entries WHERE source_doc_type='delivery' AND source_doc_id=? AND status='posted'`, noteID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	p, _ := fin.ListPeriods(ctx(), fx.branchID)
	closed := false
	for _, pe := range p {
		if pe.Year == year && pe.Month == month && pe.Status == "closed" {
			closed = true
		}
	}
	switch {
	case closed && n == 1 && closeErr == nil:
		// The post won the race, the gate then saw it and the month sealed.
	case closed && n == 0 && closeErr == nil:
		t.Fatalf("month closed with an unposted shipment (post err: %v)", postErr)
	case !closed && closeErr != nil && n == 1:
		// The post won, the gate failed on something else — acceptable only if no journal is lost.
	case !closed && n == 0 && postErr == nil:
		t.Fatal("inconsistent: neither closed nor posted")
	}
	if closed && n == 0 {
		t.Fatal("a posting slipped past the sealed gate")
	}
}

func currentYearMonth(t *testing.T, noteID int64) (int, int) {
	t.Helper()
	var y, m int
	if err := testMods.db.QueryRow(`SELECT YEAR(delivery_date), MONTH(delivery_date) FROM delivery_notes WHERE id = ?`, noteID).Scan(&y, &m); err != nil {
		t.Fatalf("note date: %v", err)
	}
	return y, m
}

// A-4: a document of an already-closed month is journalled "Susulan" into the
// open period — the closed month stays untouched and the books complete.
func TestCatchUpPostingForClosedMonth(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	_, noteID := confirmedNote(t, fx, 1)
	for _, id := range queuedStockMovements(t, fx.branchID) {
		postedLegs(t, "stock_movement", id)
	}
	if _, err := testMods.db.Exec(`UPDATE delivery_notes SET delivery_date = '2026-08-15' WHERE id = ?`, noteID); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	if err := fin.ClosePeriod(ctx(), 0, fx.branchID, 2026, 8); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := fin.Post(ctx(), 0, fx.branchID, "delivery", noteID); err == nil {
		t.Fatal("normal post into a closed month succeeded")
	}
	e, err := fin.PostCatchUp(ctx(), 0, fx.branchID, "delivery", noteID)
	if err != nil {
		t.Fatalf("catch-up: %v", err)
	}
	if len(e.Memo) < 8 || e.Memo[:8] != "Susulan:" || e.Date[:7] == "2026-08" {
		t.Fatalf("catch-up entry memo=%q date=%q; want a Susulan memo outside August", e.Memo, e.Date)
	}
	again, err := fin.PostCatchUp(ctx(), 0, fx.branchID, "delivery", noteID)
	if err != nil || again.ID != e.ID {
		t.Fatalf("catch-up replay = %v, %v; want the same entry", again, err)
	}
}

// N-52: close-day no longer stops silently after MaxBatch. 130 eligible shipments
// (more than two rounds of 50) are all ATTEMPTED once; the headers point at a
// missing order so each posting fails, which must surface as Failed + Remaining
// (nothing posted counts as done), not loop forever. Documents dated after the
// day are neither attempted nor counted as remaining. The success path at volume
// (150 real documents drained by one call) is covered by e2e E13.
func TestCloseDayRunsRoundsAndReportsRemaining(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	insertConfirmedNotes(t, fx.branchID, 130, "2026-09-10")
	insertConfirmedNotes(t, fx.branchID, 5, "2026-10-20")

	res, err := fin.CloseDay(ctx(), 0, fx.branchID, "2026-09-30")
	if err != nil {
		t.Fatalf("close day: %v", err)
	}
	if len(res.Results) != 130 || res.Posted+res.Failed != 130 {
		t.Fatalf("attempted %d (posted %d, failed %d), want 130 attempted across several rounds", len(res.Results), res.Posted, res.Failed)
	}
	seen := map[string]bool{}
	for _, r := range res.Results {
		k := fmt.Sprintf("%s:%d", r.DocType, r.DocID)
		if seen[k] {
			t.Fatalf("document %s attempted twice in one close", k)
		}
		seen[k] = true
	}
	if res.Remaining != 130-res.Posted {
		t.Fatalf("remaining = %d, want %d (everything not posted stays reported)", res.Remaining, 130-res.Posted)
	}
	// A repeat attempts the same failures again but never the future-dated ones.
	again, err := fin.CloseDay(ctx(), 0, fx.branchID, "2026-09-30")
	if err != nil {
		t.Fatalf("close day again: %v", err)
	}
	if len(again.Results) != res.Remaining {
		t.Fatalf("second close attempted %d, want %d", len(again.Results), res.Remaining)
	}
}
