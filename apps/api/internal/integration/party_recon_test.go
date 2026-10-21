package integration

import (
	"testing"

	financeapp "mini-erp/internal/modules/finance/application"
	"mini-erp/internal/shared/timeutil"
)

// ADR-0023: the party reconciliation shows, per party, where the books'
// control accounts and the documents disagree. A settled customer agrees; a
// manual journal on the customer's receivable (an input error) is reported as
// an unexplained difference of exactly its amount.
func TestPartyReconciliationFindsBookOnlyEntries(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 2)
	deliverAll(t, ctx(), fx, soID, 2)
	if _, err := testMods.finance.Service().CloseDay(ctx(), 0, fx.branchID, timeutil.TodayWIB()); err != nil {
		t.Fatalf("post the day: %v", err)
	}
	rep := testMods.reporting.Service()
	clean, err := rep.PartyReconciliation(ctx(), fx.branchID, "receivable")
	if err != nil {
		t.Fatalf("reconciliation: %v", err)
	}
	if clean.PartiesChecked == 0 || clean.Unexplained != 0 {
		t.Fatalf("clean books = %+v, want the customer checked and no unexplained row", clean)
	}

	if _, err := testMods.finance.Service().Manual(ctx(), 0, fx.branchID, "2026-10-01", "salah input",
		[]financeapp.ManualLine{{AccountCode: "1100", Debit: 7000, PartyID: fx.customerID}, {AccountCode: "3000", Credit: 7000}}); err != nil {
		t.Fatalf("manual: %v", err)
	}
	got, err := rep.PartyReconciliation(ctx(), fx.branchID, "receivable")
	if err != nil {
		t.Fatalf("reconciliation: %v", err)
	}
	if len(got.Rows) != 1 || got.Rows[0].PartyID != fx.customerID || got.Rows[0].Difference != 7000 {
		t.Fatalf("rows = %+v, want the customer with a 7000 book-only difference", got.Rows)
	}
	if got.Rows[0].Timing != (got.Rows[0].OpenOrders > 0) || got.TotalDifference != 7000 {
		t.Fatalf("report = %+v", got)
	}
}
