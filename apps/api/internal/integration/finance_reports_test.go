package integration

import (
	"testing"

	branchapp "mini-erp/internal/modules/branch/application"
	deliveryapp "mini-erp/internal/modules/delivery/application"
	financeapp "mini-erp/internal/modules/finance/application"
	financeinfra "mini-erp/internal/modules/finance/infrastructure"
	salesapp "mini-erp/internal/modules/sales/application"
)

func manual(t *testing.T, branchID int64, date string, amount int64, cashIn bool) {
	t.Helper()
	lines := []financeapp.ManualLine{{AccountCode: "1000", Debit: amount}, {AccountCode: "2000", Credit: amount}}
	if !cashIn {
		lines = []financeapp.ManualLine{{AccountCode: "1000", Credit: amount}, {AccountCode: "2000", Debit: amount}}
	}
	if _, err := testMods.finance.Service().Manual(ctx(), 0, branchID, date, "uji laporan", lines); err != nil {
		t.Fatalf("manual %s: %v", date, err)
	}
}

// F2b-12a/b/f: the cumulative trial balance carries December into January, the
// cash report gives opening/in/out/closing, and the general ledger pages with a
// running balance that continues across windows.
func TestFinanceReportsCumulativeCashAndLedger(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	manual(t, fx.branchID, "2026-12-30", 1000000, true)
	manual(t, fx.branchID, "2027-01-05", 300000, false)
	manual(t, fx.branchID, "2027-01-10", 50000, true)

	rows, err := fin.TrialBalanceAsOf(ctx(), fx.branchID, "2027-01-31")
	if err != nil {
		t.Fatalf("trial balance: %v", err)
	}
	var debit, credit int64
	for _, r := range rows {
		debit += r.Debit
		credit += r.Credit
	}
	if debit != credit || debit != 1000000+300000+50000 {
		t.Fatalf("cumulative TB debit=%d credit=%d; want balanced at 1.350.000", debit, credit)
	}
	dec, _ := fin.TrialBalanceAsOf(ctx(), fx.branchID, "2026-12-31")
	var decDebit int64
	for _, r := range dec {
		decDebit += r.Debit
	}
	if decDebit != 1000000 {
		t.Fatalf("TB as of 31 Dec debit = %d, want 1.000.000", decDebit)
	}

	cash, opening, closing, err := fin.CashFlow(ctx(), fx.branchID, "2027-01-01", "2027-01-31")
	if err != nil {
		t.Fatalf("cash flow: %v", err)
	}
	if len(cash) != 1 || cash[0].Opening != 1000000 || cash[0].In != 50000 || cash[0].Out != 300000 || cash[0].Closing != 750000 {
		t.Fatalf("cash flow = %+v; want opening 1.000.000, in 50.000, out 300.000, closing 750.000", cash)
	}
	if opening != 1000000 || closing != 750000 {
		t.Fatalf("totals opening=%d closing=%d", opening, closing)
	}

	// General ledger of cash, 2 lines per window: the running balance continues.
	p1, err := fin.GeneralLedgerPage(ctx(), fx.branchID, "1000", "2026-12-01", "2027-01-31", 1, 2)
	if err != nil {
		t.Fatalf("ledger p1: %v", err)
	}
	p2, err := fin.GeneralLedgerPage(ctx(), fx.branchID, "1000", "2026-12-01", "2027-01-31", 2, 2)
	if err != nil {
		t.Fatalf("ledger p2: %v", err)
	}
	if p1.Total != 3 || len(p1.Lines) != 2 || len(p2.Lines) != 1 {
		t.Fatalf("ledger windows: total=%d p1=%d p2=%d; want 3, 2, 1", p1.Total, len(p1.Lines), len(p2.Lines))
	}
	if p1.Lines[0].Date != "2026-12-30" || p1.Lines[1].Balance != 700000 || p2.Lines[0].Balance != 750000 {
		t.Fatalf("running balances p1=%+v p2=%+v; want 1.000.000 → 700.000 → 750.000 in date order", p1.Lines, p2.Lines)
	}
}

// F2b-12d: all-branches reports merge the branches the user may read; the
// handler restricts the set, the service merges it.
func TestFinanceReportsAllBranches(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	second, err := testMods.branch.Service().CreateBranch(ctx(), 0, branchapp.CreateBranchInput{Code: "CB2", Name: "Cabang Dua"})
	if err != nil {
		t.Fatalf("branch: %v", err)
	}
	manual(t, fx.branchID, "2027-02-01", 100000, true)
	manual(t, second.ID, "2027-02-02", 40000, true)
	fin := testMods.finance.Service()
	one, _ := fin.TrialBalanceAsOf(ctx(), fx.branchID, "2027-02-28")
	both, err := fin.TrialBalanceAsOf(financeapp.WithBranchScope(ctx(), []int64{fx.branchID, second.ID}), fx.branchID, "2027-02-28")
	if err != nil {
		t.Fatalf("all branches: %v", err)
	}
	sum := func(rows []financeapp.TrialRow) (d int64) {
		for _, r := range rows {
			d += r.Debit
		}
		return
	}
	if sum(one) != 100000 || sum(both) != 140000 {
		t.Fatalf("debit one=%d all=%d; want 100.000 and 140.000", sum(one), sum(both))
	}
}

// F2b-12i: three shipments of one third each book the line exactly once — the
// completing shipment takes the rounding remainder (33.333 + 33.333 + 33.334).
func TestShipmentsOfOneLineSumExactly(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	so, err := testMods.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none",
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 3, DiscountNominal: 20000}},
	})
	if err != nil {
		t.Fatalf("order: %v", err)
	}
	if _, err := testMods.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
		t.Fatalf("confirm order: %v", err)
	}
	fin := testMods.finance.Service()
	var ids []int64
	for i := 0; i < 3; i++ {
		n, err := draftNote(t, fx, so.ID, 1)
		if err != nil {
			t.Fatalf("draft %d: %v", i, err)
		}
		in := deliveryapp.ConfirmInput{}
		if i == 2 {
			in.Settlement = codSettle
		}
		if _, err := testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, n.ID, in); err != nil {
			t.Fatalf("confirm %d: %v", i, err)
		}
		ids = append(ids, n.ID)
	}
	var want int64
	_ = testMods.db.QueryRow(`SELECT subtotal FROM sales_orders WHERE id = ?`, so.ID).Scan(&want)
	for _, id := range ids {
		if _, err := fin.Post(ctx(), 0, fx.branchID, "delivery", id); err != nil {
			t.Fatalf("post delivery %d: %v", id, err)
		}
	}
	var revenue int64
	if err := testMods.db.QueryRow(`SELECT COALESCE(SUM(l.credit - l.debit), 0)
		FROM finance_journal_lines l JOIN finance_journal_entries e ON e.id = l.entry_id
		JOIN finance_account_mappings m ON m.account_id = l.account_id AND m.mapping_key = 'revenue'
		WHERE e.source_doc_type = 'delivery'`).Scan(&revenue); err != nil {
		t.Fatalf("revenue: %v", err)
	}
	if revenue != want {
		t.Fatalf("revenue of three shipments = %d, want %d (the exact line)", revenue, want)
	}
}

// F2b-12g/h: an expense is one atomic unit (row + number + journal), the payer
// must be a cash/bank asset, cancelling needs a reason, and journal numbers run
// without gaps.
func TestExpenseAtomicAndCancelReason(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	accounts, _ := fin.ListAccounts(ctx(), "active")
	var cash, expense, payable int64
	for _, a := range accounts {
		switch a.Code {
		case "1000":
			cash = a.ID
		case "6000":
			expense = a.ID
		case "2000":
			payable = a.ID
		}
	}
	if cash == 0 || expense == 0 {
		t.Fatalf("chart missing cash(%d)/expense(%d)", cash, expense)
	}
	in := financeapp.CreateExpenseInput{ExpenseAccountID: expense, PayAccountID: payable, Amount: 50000, Date: "2027-03-01", Notes: "listrik"}
	if _, err := fin.CreateExpense(ctx(), 0, fx.branchID, in); err == nil {
		t.Fatal("expense paid from a non-cash account was accepted")
	}
	in.PayAccountID = cash
	e1, err := fin.CreateExpense(ctx(), 0, fx.branchID, in)
	if err != nil || e1.JournalID == 0 {
		t.Fatalf("expense: %+v, %v", e1, err)
	}
	e2, err := fin.CreateExpense(ctx(), 0, fx.branchID, in)
	if err != nil {
		t.Fatalf("second expense: %v", err)
	}
	var n1, n2 string
	_ = testMods.db.QueryRow(`SELECT number FROM finance_journal_entries WHERE id = ?`, e1.JournalID).Scan(&n1)
	_ = testMods.db.QueryRow(`SELECT number FROM finance_journal_entries WHERE id = ?`, e2.JournalID).Scan(&n2)
	if n1 == "" || n2 == "" || n1 == n2 {
		t.Fatalf("journal numbers %q / %q", n1, n2)
	}
	if _, err := fin.CancelExpense(ctx(), 0, fx.branchID, e1.ID, "  "); err == nil {
		t.Fatal("cancel without a reason accepted")
	}
	got, err := fin.CancelExpense(ctx(), 0, fx.branchID, e1.ID, "salah input")
	if err != nil || got.Status != "cancelled" || got.CancelReason != "salah input" {
		t.Fatalf("cancel = %+v, %v", got, err)
	}
	if rep, _ := fin.VerifyRollups(ctx(), fx.branchID); !rep.Consistent {
		t.Fatalf("rollup inconsistent after expense + cancel: %+v", rep)
	}
	list, err := fin.ListExpenses(ctx(), fx.branchID, infrastructureExpenseFilter("cancelled"), 1, 20)
	if err != nil || list.Total != 1 {
		t.Fatalf("filtered list total = %v, %v; want 1 cancelled", list, err)
	}
}

func infrastructureExpenseFilter(status string) financeinfra.ExpenseFilter {
	return financeinfra.ExpenseFilter{Status: status}
}
