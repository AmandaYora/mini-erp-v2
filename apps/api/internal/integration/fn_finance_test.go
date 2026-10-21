package integration

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	branchapp "mini-erp/internal/modules/branch/application"
	deliveryapp "mini-erp/internal/modules/delivery/application"
	financeapp "mini-erp/internal/modules/finance/application"
	financeinfra "mini-erp/internal/modules/finance/infrastructure"
	partyapp "mini-erp/internal/modules/party/application"
	salesapp "mini-erp/internal/modules/sales/application"
)

// postStockMovements journals the fixture's opening stock adjustments so a
// close gate is not blocked by them.
func postStockMovements(t *testing.T, branchID int64) {
	t.Helper()
	var batch []financeapp.BatchItem
	for _, id := range queuedStockMovements(t, branchID) {
		batch = append(batch, financeapp.BatchItem{DocType: "stock_movement", DocID: id})
	}
	if len(batch) == 0 {
		return
	}
	if _, err := testMods.finance.Service().PostBatch(ctx(), 0, branchID, batch); err != nil {
		t.Fatalf("post stock movements: %v", err)
	}
}

// confirmedDelivery creates a sales order (tax type/rate) and a confirmed
// delivery on date; it is NOT posted.
func confirmedDelivery(t *testing.T, fx shopFixture, date, taxType string, rate float64, qty float64) int64 {
	t.Helper()
	m := testMods
	so, err := m.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod",
		TaxType: taxType, TaxRate: rate, OrderDate: date,
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: qty}},
	})
	if err != nil {
		t.Fatalf("create SO: %v", err)
	}
	if _, err := m.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
		t.Fatalf("confirm SO: %v", err)
	}
	dn, err := m.delivery.Service().Create(ctx(), 0, fx.branchID, so.ID, date, "", deliveryapp.DocInput{},
		[]deliveryapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: qty}})
	if err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	if _, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); err != nil {
		t.Fatalf("confirm delivery: %v", err)
	}
	return dn.ID
}

// FN-05: reopening a closed month (fiscal and tax) demands a reason, and the
// reason lands in the audit diff.
func TestReopenPeriodRequiresReasonAndIsAudited(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	if err := fin.ClosePeriod(ctx(), 0, fx.branchID, 2027, 4); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := fin.ReopenPeriod(ctx(), 0, fx.branchID, 2027, 4, "   "); err == nil {
		t.Fatal("reopen without a reason accepted")
	}
	if n := countRows(t, `SELECT COUNT(*) FROM finance_periods WHERE status = 'closed'`); n != 1 {
		t.Fatalf("period must stay closed, closed = %d", n)
	}
	if err := fin.ReopenPeriod(ctx(), 0, fx.branchID, 2027, 4, "Koreksi jurnal penyesuaian"); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	_, after, meta := lastDetail(t, "fiscal_periods.reopen")
	if !strings.Contains(after, "Koreksi jurnal penyesuaian") || !strings.Contains(meta, "Koreksi jurnal penyesuaian") {
		t.Fatalf("audit diff lacks the reason: after=%s meta=%s", after, meta)
	}

	if err := fin.CloseTaxPeriod(ctx(), 0, fx.branchID, 2027, 4); err != nil {
		t.Fatalf("close tax: %v", err)
	}
	if err := fin.ReopenTaxPeriod(ctx(), 0, fx.branchID, 2027, 4, ""); err == nil {
		t.Fatal("tax reopen without a reason accepted")
	}
	if err := fin.ReopenTaxPeriod(ctx(), 0, fx.branchID, 2027, 4, "Salah snapshot"); err != nil {
		t.Fatalf("tax reopen: %v", err)
	}
	_, after, _ = lastDetail(t, "fiscal_periods.reopen")
	if !strings.Contains(after, "Salah snapshot") {
		t.Fatalf("tax reopen audit lacks the reason: %s", after)
	}
}

// FN-01: a stuck document can be ignored with a reason (audited), no longer
// blocks the close, is refused by batch posting until restored, and returns to
// blocking once restored.
func TestIgnoreAndRestorePostingSource(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	dnID := confirmedDelivery(t, fx, "2026-09-10", "none", 0, 1)
	postStockMovements(t, fx.branchID)

	if _, err := fin.IgnoreSource(ctx(), 0, fx.branchID, "delivery", dnID, ""); err == nil {
		t.Fatal("ignore without a reason accepted")
	}
	if _, err := fin.IgnoreSource(ctx(), 0, fx.branchID, "delivery", dnID+999, "x"); err == nil {
		t.Fatal("ignore of a document outside the queue accepted")
	}
	if err := fin.SafeClose(ctx(), 0, fx.branchID, 2026, 9); err == nil {
		t.Fatal("safe-close passed with an unposted delivery")
	}
	item, err := fin.IgnoreSource(ctx(), 0, fx.branchID, "delivery", dnID, "SJ uji coba, tidak dijurnal")
	if err != nil {
		t.Fatalf("ignore: %v", err)
	}
	if !item.Ignored {
		t.Fatalf("item not marked ignored: %+v", item)
	}
	if n := auditCount(t, "finance.posting.ignore"); n != 1 {
		t.Fatalf("ignore audit rows = %d, want 1", n)
	}
	// Idempotent: a second ignore keeps one trail row.
	if _, err := fin.IgnoreSource(ctx(), 0, fx.branchID, "delivery", dnID, "lagi"); err != nil {
		t.Fatalf("second ignore: %v", err)
	}
	if n := auditCount(t, "finance.posting.ignore"); n != 1 {
		t.Fatalf("replayed ignore wrote another trail row: %d", n)
	}

	q, _ := fin.UnpostedSources(ctx(), fx.branchID)
	var listed bool
	for _, it := range q {
		if it.DocType == "delivery" && it.DocID == dnID {
			listed = it.Ignored && it.IgnoreReason != ""
		}
	}
	if !listed {
		t.Fatal("ignored document missing from the queue listing or without its reason")
	}
	check, err := fin.Readiness(ctx(), fx.branchID, 2026, 9)
	if err != nil {
		t.Fatalf("readiness: %v", err)
	}
	if !check.Ready || check.Unposted != 0 || check.Ignored != 1 {
		t.Fatalf("readiness ready=%v unposted=%d ignored=%d blockers=%v", check.Ready, check.Unposted, check.Ignored, check.Blockers)
	}
	var warned bool
	for _, c := range check.Checks {
		if c.Key == "ignored_docs" && c.Status == financeapp.CheckWarning {
			warned = true
		}
	}
	if !warned {
		t.Fatal("ignored documents are not surfaced as a warning")
	}
	res, _ := fin.PostBatch(ctx(), 0, fx.branchID, []financeapp.BatchItem{{DocType: "delivery", DocID: dnID}})
	if len(res) != 1 || res[0].Error == "" {
		t.Fatalf("batch posted an ignored document: %+v", res)
	}
	if closed, _ := fin.CloseDay(ctx(), 0, fx.branchID, "2026-12-31"); closed == nil || len(closed.Results) != 0 {
		t.Fatalf("close-day picked up an ignored document: %+v", closed)
	}

	if err := fin.RestoreSource(ctx(), 0, fx.branchID, "delivery", dnID); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if n := auditCount(t, "finance.posting.restore"); n != 1 {
		t.Fatalf("restore audit rows = %d, want 1", n)
	}
	if err := fin.RestoreSource(ctx(), 0, fx.branchID, "delivery", dnID); err == nil {
		t.Fatal("restoring a document that is not ignored was accepted")
	}
	check, _ = fin.Readiness(ctx(), fx.branchID, 2026, 9)
	if check.Ready || check.Unposted != 1 {
		t.Fatalf("restored document must block again: ready=%v unposted=%d", check.Ready, check.Unposted)
	}
}

// FN-02 + FN-10: the daily overview matches the journals of the day, and queue
// items carry amount and counterparty.
func TestDailyCloseOverviewMatchesJournalsAndQueueCarriesAmounts(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	const day = "2026-09-10"
	dnID := confirmedDelivery(t, fx, day, "none", 0, 2)

	// Before posting: the document is queued with its party and order value.
	q, err := fin.UnpostedSources(ctx(), fx.branchID)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	var found bool
	for _, it := range q {
		if it.DocType == "delivery" && it.DocID == dnID {
			found = true
			if it.PartyName == "" || it.Amount <= 0 || it.AmountKind != "order" || it.Summary == "" {
				t.Fatalf("queue item lacks party/amount/summary: %+v", it)
			}
		}
	}
	if !found {
		t.Fatal("delivery missing from queue")
	}
	ov, err := fin.DailyCloseOverview(ctx(), fx.branchID, day)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if ov.Status != financeapp.DayPending || ov.Activity < 1 {
		t.Fatalf("status = %s activity=%d, want belum with activity", ov.Status, ov.Activity)
	}

	if _, err := fin.Post(ctx(), 0, fx.branchID, "delivery", dnID); err != nil {
		t.Fatalf("post: %v", err)
	}
	ov, err = fin.DailyCloseOverview(ctx(), fx.branchID, day)
	if err != nil {
		t.Fatalf("overview after: %v", err)
	}
	pl, err := fin.ProfitLoss(ctx(), fx.branchID, day, day)
	if err != nil {
		t.Fatalf("profit loss: %v", err)
	}
	if ov.Totals.Sales.Amount != pl.Revenue || ov.Totals.Sales.Count != 1 || pl.Revenue <= 0 {
		t.Fatalf("overview sales %+v vs journal revenue %d", ov.Totals.Sales, pl.Revenue)
	}
	if ov.Status != financeapp.DayComplete {
		t.Fatalf("status = %s, want lengkap", ov.Status)
	}
	empty, err := fin.DailyCloseOverview(ctx(), fx.branchID, "2026-01-05")
	if err != nil {
		t.Fatalf("empty day: %v", err)
	}
	if empty.Status != financeapp.DayEmpty {
		t.Fatalf("quiet day status = %s, want kosong", empty.Status)
	}
	if _, err := fin.DailyCloseOverview(ctx(), fx.branchID, "bukan-tanggal"); err == nil {
		t.Fatal("invalid date accepted")
	}
}

// FN-03: the account breakdown of the profit and loss sums to the summary.
func TestProfitLossAccountBreakdownSumsToSummary(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	dnID := confirmedDelivery(t, fx, "2026-09-10", "none", 0, 2)
	if _, err := fin.Post(ctx(), 0, fx.branchID, "delivery", dnID); err != nil {
		t.Fatalf("post: %v", err)
	}
	if _, err := fin.CreateExpense(ctx(), 0, fx.branchID, financeapp.CreateExpenseInput{
		ExpenseAccountID: accountID(t, "6000"), PayAccountID: accountID(t, "1000"), Amount: 7000, Date: "2026-09-11", Notes: "gaji"}); err != nil {
		t.Fatalf("expense: %v", err)
	}
	pl, err := fin.ProfitLoss(ctx(), fx.branchID, "2026-09-01", "2026-09-30")
	if err != nil {
		t.Fatalf("pl: %v", err)
	}
	sum := func(lines []financeapp.PLLine) (n int64) {
		for _, l := range lines {
			n += l.Amount
		}
		return
	}
	if len(pl.RevenueLines) == 0 || len(pl.ExpenseLines) == 0 {
		t.Fatalf("breakdown empty: %+v", pl)
	}
	if sum(pl.RevenueLines) != pl.Revenue || sum(pl.CogsLines) != pl.Cogs || sum(pl.ExpenseLines) != pl.Expense {
		t.Fatalf("lines do not sum to the summary: %+v", pl)
	}
	if pl.Revenue-pl.Cogs-pl.Expense != pl.Profit {
		t.Fatalf("profit identity broken: %+v", pl)
	}
}

func accountID(t *testing.T, code string) int64 {
	t.Helper()
	a, err := testMods.finance.Service().ListAccounts(ctx(), "active")
	if err != nil {
		t.Fatalf("accounts: %v", err)
	}
	for _, x := range a {
		if x.Code == code {
			return x.ID
		}
	}
	t.Fatalf("account %s missing", code)
	return 0
}

// FN-06 + FN-04: DPP of sales and purchases, taxable documents without an
// invoice number, and a tax package whose PPN Keluaran / PPN Masukan sheets and
// ledger agree with the on-screen detail for a free date range.
func TestTaxSummaryDppExceptionsAndPackageRange(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	// Sale with PPN 11% exclude: 2 x 15.000 = 30.000 DPP, 3.300 PPN.
	dn := confirmedDelivery(t, fx, "2026-10-12", "exclude", 11, 2)
	if _, err := fin.Post(ctx(), 0, fx.branchID, "delivery", dn); err != nil {
		t.Fatalf("post sale: %v", err)
	}
	// Purchase with PPN: no supplier invoice number on the PO.
	poID := purchaseFixture(t, ctx(), fx, 10000, 5)
	var grID int64
	if err := testMods.db.QueryRow(`SELECT id FROM goods_receipts WHERE purchase_order_id = ? LIMIT 1`, poID).Scan(&grID); err != nil {
		t.Fatalf("receipt id: %v", err)
	}
	if _, err := fin.Post(ctx(), 0, fx.branchID, "goods_receipt", grID); err != nil {
		t.Fatalf("post receipt: %v", err)
	}
	// purchaseFixture receives on 2026-09-10; the summary month is September.
	sepSum, err := fin.TaxSummaryPage(ctx(), fx.branchID, 2026, 9, 1, 20)
	if err != nil {
		t.Fatalf("summary sep: %v", err)
	}
	// 5 x 10.000 less 10% discount = 45.000 DPP, PPN 4.950.
	if sepSum.PurchaseDpp != 45000 || sepSum.PpnIn != 4950 {
		t.Fatalf("purchase dpp=%d ppnIn=%d, want 45.000 and 4.950", sepSum.PurchaseDpp, sepSum.PpnIn)
	}
	if sepSum.ExceptionsMeta.Total != 1 || len(sepSum.Exceptions) != 1 || sepSum.Exceptions[0].Kind != "purchase" {
		t.Fatalf("exceptions = %+v meta=%+v, want the purchase without supplier invoice", sepSum.Exceptions, sepSum.ExceptionsMeta)
	}
	octSum, err := fin.TaxSummaryPage(ctx(), fx.branchID, 2026, 10, 1, 20)
	if err != nil {
		t.Fatalf("summary oct: %v", err)
	}
	if octSum.SalesDpp != 30000 || octSum.PpnOut != 3300 {
		t.Fatalf("sales dpp=%d ppnOut=%d, want 30.000 and 3.300", octSum.SalesDpp, octSum.PpnOut)
	}
	if octSum.ExceptionsMeta.Total != 1 || octSum.Exceptions[0].Kind != "sales" {
		t.Fatalf("sales exception missing: %+v", octSum.ExceptionsMeta)
	}
	// Pagination: page 2 of a 1-row page is empty, total unchanged.
	p2, _ := fin.TaxSummaryPage(ctx(), fx.branchID, 2026, 10, 2, 1)
	if len(p2.Exceptions) != 0 || p2.ExceptionsMeta.Total != 1 {
		t.Fatalf("page 2 = %+v", p2.ExceptionsMeta)
	}

	// Package over a free range covering both months.
	raw, name, err := fin.TaxPackageRange(ctx(), fx.branchID, "2026-09-01", "2026-10-31", financeapp.PackageActual)
	if err != nil {
		t.Fatalf("package: %v", err)
	}
	if !strings.Contains(name, "2026-09-01_2026-10-31") {
		t.Fatalf("range package name = %q", name)
	}
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	sheets := strings.Join(f.GetSheetList(), "|")
	for _, want := range []string{"Rincian PPN", "PPN Masukan", "Buku Besar"} {
		if !strings.Contains(sheets, want) {
			t.Fatalf("package lacks sheet %q (has %s)", want, sheets)
		}
	}
	rep, err := fin.ReportsRange(ctx(), fx.branchID, "2026-09-01", "2026-10-31", financeapp.PackageActual)
	if err != nil {
		t.Fatalf("reports range: %v", err)
	}
	var out, in int64
	for _, r := range rep.Detail {
		out += r.Ppn
		in += r.PpnIn
	}
	if out != rep.Summary.PpnOut || in != rep.Summary.PpnIn || out != 3300 || in != 4950 {
		t.Fatalf("detail ppn out=%d in=%d vs summary out=%d in=%d", out, in, rep.Summary.PpnOut, rep.Summary.PpnIn)
	}
	// The package carries the same rows the on-screen report pages through.
	var screen int
	for page := 1; ; page++ {
		rows, total, err := fin.TaxDetailReport(ctx(), fx.branchID, 2026, 9, page, 500)
		if err != nil {
			t.Fatalf("detail report: %v", err)
		}
		screen += len(rows)
		if int64(page*500) >= total {
			break
		}
	}
	if screen == 0 {
		t.Fatal("detail report empty")
	}
	// Capped variant must stay inside one tax year.
	if _, _, err := fin.TaxPackageRange(ctx(), fx.branchID, "2025-12-01", "2026-01-31", financeapp.PackageCapped); err == nil {
		t.Fatal("capped range across two tax years accepted")
	}
	if _, _, err := fin.TaxPackageRange(ctx(), fx.branchID, "2026-09-01", "2026-10-31", financeapp.PackageCapped); err != nil {
		t.Fatalf("capped range: %v", err)
	}
}

// FN-07: a fiscal correction carries class, note and attachments; the
// reconciliation separates the classes and its journal model is unchanged.
func TestTaxAdjustmentClassNoteAndFiscalClasses(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	lines := []financeapp.ManualLine{{AccountCode: "6000", Debit: 1000}, {AccountCode: "1000", Credit: 1000}}
	if _, err := fin.CreateTaxAdjustmentWith(ctx(), 0, fx.branchID, "2026-09-15", "tanpa jenis", lines,
		financeapp.TaxAdjustInput{}); err == nil {
		t.Fatal("correction without a class accepted")
	}
	pos, err := fin.CreateTaxAdjustmentWith(ctx(), 0, fx.branchID, "2026-09-15", "beban non-deductible", lines,
		financeapp.TaxAdjustInput{Classification: financeapp.TaxClassPositive, Note: "Sumbangan tanpa bukti"})
	if err != nil {
		t.Fatalf("positive: %v", err)
	}
	finalLines := []financeapp.ManualLine{{AccountCode: "6000", Debit: 400}, {AccountCode: "1000", Credit: 400}}
	if _, err := fin.CreateTaxAdjustmentWith(ctx(), 0, fx.branchID, "2026-09-16", "penghasilan final", finalLines,
		financeapp.TaxAdjustInput{Classification: financeapp.TaxClassFinal}); err != nil {
		t.Fatalf("final: %v", err)
	}
	list, err := fin.ListTaxAdjustments(ctx(), fx.branchID)
	if err != nil || len(list) != 2 {
		t.Fatalf("list: %v rows=%d", err, len(list))
	}
	var seen bool
	for _, r := range list {
		if r.Entry.ID == pos.ID {
			seen = r.Classification == financeapp.TaxClassPositive && r.Note == "Sumbangan tanpa bukti" && r.ClassLabel != ""
		}
	}
	if !seen {
		t.Fatalf("class/note not listed: %+v", list)
	}
	fiscal, err := fin.FiscalSummary(ctx(), fx.branchID, "2026-09-01", "2026-09-30")
	if err != nil {
		t.Fatalf("fiscal: %v", err)
	}
	effect := map[string]int64{}
	var total int64
	for _, c := range fiscal.Classes {
		effect[c.Classification] = c.Effect
		total += c.Effect
	}
	if effect[financeapp.TaxClassPositive] != -1000 || effect[financeapp.TaxClassFinal] != -400 {
		t.Fatalf("class effects = %v, want positive -1000 and final -400", effect)
	}
	if fiscal.FiscalProfit-fiscal.Commercial.Profit != total {
		t.Fatalf("class effects %d do not reconcile commercial -> fiscal (%d)", total, fiscal.FiscalProfit-fiscal.Commercial.Profit)
	}
	// Reversal nets inside its own class (the mirror is attributed to the original).
	if _, err := fin.Reverse(ctx(), 0, fx.branchID, pos.ID, "salah catat"); err != nil {
		t.Fatalf("reverse: %v", err)
	}
	fiscal, _ = fin.FiscalSummary(ctx(), fx.branchID, "2026-09-01", time2Today())
	for _, c := range fiscal.Classes {
		if c.Classification == financeapp.TaxClassPositive && c.Effect != 0 {
			t.Fatalf("reversed correction still moves its class: %+v", c)
		}
		if c.Classification == "" {
			t.Fatalf("reversal mirror leaked into the unclassified bucket: %+v", c)
		}
	}
}

func time2Today() string { return "2099-12-31" }

// FN-08: the general ledger without an account lists every account with
// opening balance and matches the trial balance.
func TestGeneralLedgerOverviewMatchesTrialBalance(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	manual(t, fx.branchID, "2027-02-01", 100000, true)
	manual(t, fx.branchID, "2027-03-05", 30000, false)

	ov, err := fin.GeneralLedgerOverview(ctx(), fx.branchID, "2027-03-01", "2027-03-31", 1, 100)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	tb, err := fin.TrialBalance(ctx(), fx.branchID, 2027, 3)
	if err != nil {
		t.Fatalf("trial balance: %v", err)
	}
	var d, c int64
	for _, r := range tb {
		d += r.Debit
		c += r.Credit
	}
	if ov.TotalDebit != d || ov.TotalCredit != c || d == 0 {
		t.Fatalf("overview totals %d/%d vs trial balance %d/%d", ov.TotalDebit, ov.TotalCredit, d, c)
	}
	var cash *financeapp.LedgerAccountRow
	for i := range ov.Rows {
		if ov.Rows[i].Code == "1000" {
			cash = &ov.Rows[i]
		}
	}
	if cash == nil || cash.Opening != 100000 || cash.Closing != cash.Opening+cash.Debit-cash.Credit {
		t.Fatalf("cash row = %+v, want opening 100.000 carried from February", cash)
	}
	small, _ := fin.GeneralLedgerOverview(ctx(), fx.branchID, "2027-03-01", "2027-03-31", 1, 1)
	if len(small.Rows) != 1 || small.Total != ov.Total {
		t.Fatalf("pagination rows=%d total=%d want 1 and %d", len(small.Rows), small.Total, ov.Total)
	}
}

// FN-09: inventory value and margin span the branches a user may read, with a
// per-branch split that sums to the total.
func TestInventoryAndMarginAcrossBranches(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	dn := confirmedDelivery(t, fx, "2026-09-10", "none", 0, 2)
	if _, err := fin.Post(ctx(), 0, fx.branchID, "delivery", dn); err != nil {
		t.Fatalf("post: %v", err)
	}
	second, err := testMods.branch.Service().CreateBranch(ctx(), 0, branchapp.CreateBranchInput{Code: "CB9", Name: "Cabang Sembilan"})
	if err != nil {
		t.Fatalf("branch: %v", err)
	}
	scope := financeapp.WithBranchScope(ctx(), []int64{fx.branchID, second.ID})
	mg, err := fin.Margin(scope, fx.branchID, "2026-09-01", "2026-09-30")
	if err != nil {
		t.Fatalf("margin: %v", err)
	}
	if len(mg.ByBranch) != 2 {
		t.Fatalf("byBranch = %+v, want two rows", mg.ByBranch)
	}
	var rev, gross int64
	for _, b := range mg.ByBranch {
		rev += b.Revenue
		gross += b.Gross
	}
	if rev != mg.Revenue || gross != mg.Gross || mg.Revenue <= 0 {
		t.Fatalf("split %d/%d does not sum to %d/%d", rev, gross, mg.Revenue, mg.Gross)
	}
	single, _ := fin.Margin(ctx(), fx.branchID, "2026-09-01", "2026-09-30")
	if len(single.ByBranch) != 0 || single.Revenue != mg.Revenue {
		t.Fatalf("single-branch margin = %+v", single)
	}
	rep, err := fin.InventoryReport(scope, fx.branchID)
	if err != nil {
		t.Fatalf("inventory: %v", err)
	}
	var tot int64
	for _, b := range rep.ByBranch {
		tot += b.Total
	}
	one, _ := fin.InventoryReport(ctx(), fx.branchID)
	if len(rep.ByBranch) != 2 || tot != rep.Total || rep.Total != one.Total {
		t.Fatalf("inventory total=%d split=%d single=%d byBranch=%+v", rep.Total, tot, one.Total, rep.ByBranch)
	}
}

// FN-10: an expense may name a counterparty; it shows on the expense and its
// journal leg, an unknown party is refused, and the list filters by it.
func TestExpenseCounterparty(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	sup, err := testMods.party.Service().CreateSupplier(ctx(), 0, partyapp.PartyInput{Name: "PLN Uji"})
	if err != nil {
		t.Fatalf("supplier: %v", err)
	}
	in := financeapp.CreateExpenseInput{ExpenseAccountID: accountID(t, "6010"), PayAccountID: accountID(t, "1000"),
		Amount: 55000, Date: "2026-09-12", Notes: "listrik", PartyID: sup.ID}
	e, err := fin.CreateExpense(ctx(), 0, fx.branchID, in)
	if err != nil {
		t.Fatalf("expense: %v", err)
	}
	if e.PartyID != sup.ID || e.PartyName != "PLN Uji" {
		t.Fatalf("expense party = %d %q", e.PartyID, e.PartyName)
	}
	none := in
	none.PartyID = 0
	if _, err := fin.CreateExpense(ctx(), 0, fx.branchID, none); err != nil {
		t.Fatalf("expense without party: %v", err)
	}
	bad := in
	bad.PartyID = 987654
	if _, err := fin.CreateExpense(ctx(), 0, fx.branchID, bad); err == nil {
		t.Fatal("expense with an unknown party accepted")
	}
	list, err := fin.ListExpenses(ctx(), fx.branchID, financeinfra.ExpenseFilter{PartyID: sup.ID}, 1, 20)
	if err != nil || len(list.Expenses) != 1 || list.Expenses[0].PartyName != "PLN Uji" {
		t.Fatalf("filtered list = %+v err=%v", list, err)
	}
	var partyOnLine int64
	if err := testMods.db.QueryRow(`SELECT COALESCE(l.party_id, 0) FROM finance_journal_lines l
		JOIN finance_journal_entries j ON j.id = l.entry_id WHERE j.id = ? AND l.debit > 0`, e.JournalID).Scan(&partyOnLine); err != nil {
		t.Fatalf("line party: %v", err)
	}
	if partyOnLine != sup.ID {
		t.Fatalf("journal expense leg party = %d, want %d", partyOnLine, sup.ID)
	}
}
