package integration

import (
	"testing"

	financeapp "mini-erp/internal/modules/finance/application"
)

// F4: aging "per tanggal" is a real cut-off. An order dated after the date and
// a payment made after it change nothing as of that date; the report carries
// the control account's balance as of the same date and the difference.
func TestAgingAsOfIgnoresLaterOrdersAndPayments(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	early := sellFixture(t, ctx(), fx, 2)
	late := sellFixture(t, ctx(), fx, 1)
	if _, err := testMods.db.Exec(`UPDATE sales_orders SET order_date = '2026-09-01 10:00:00' WHERE id = ?`, early); err != nil {
		t.Fatalf("date early: %v", err)
	}
	// Early morning WIB of the day after the cut-off: already "after".
	if _, err := testMods.db.Exec(`UPDATE sales_orders SET order_date = '2026-09-11 08:00:00' WHERE id = ?`, late); err != nil {
		t.Fatalf("date late: %v", err)
	}
	total := orderTotal(t, early)
	pay := testMods.payment.Service()
	before, after := total/4, total/2
	p1, err := pay.Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "2026-09-10", "", before, payAll(early, before))
	if err != nil {
		t.Fatalf("pay before: %v", err)
	}
	// The last moment of the cut-off day (paid_at is DATETIME(6), WIB) counts.
	if _, err := testMods.db.Exec(`UPDATE payments SET paid_at = '2026-09-10 23:59:59.500000' WHERE id = ?`, p1.ID); err != nil {
		t.Fatalf("time p1: %v", err)
	}
	if _, err := pay.Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "2026-09-11", "", after, payAll(early, after)); err != nil {
		t.Fatalf("pay after: %v", err)
	}
	// Opening receivable of the same customer before the date, and one after.
	fin := testMods.finance.Service()
	for _, j := range []struct {
		date   string
		amount int64
	}{{"2026-09-08", 7000}, {"2026-09-12", 3000}} {
		if _, err := fin.Manual(ctx(), 0, fx.branchID, j.date, "saldo piutang", []financeapp.ManualLine{
			{AccountCode: "1100", Debit: j.amount, PartyID: fx.customerID}, {AccountCode: "3000", Credit: j.amount},
		}); err != nil {
			t.Fatalf("manual %s: %v", j.date, err)
		}
	}

	rep, err := fin.AgingDocuments(ctx(), fx.branchID, "receivable", "2026-09-10")
	if err != nil {
		t.Fatalf("aging: %v", err)
	}
	if len(rep.Documents) != 1 || rep.Documents[0].OrderID != early {
		t.Fatalf("documents = %+v; want only the order dated before the cut-off", rep.Documents)
	}
	d := rep.Documents[0]
	if d.Paid != before || d.Outstanding != total-before || d.AgeDays != 9 {
		t.Fatalf("doc paid %d outstanding %d age %d; want %d, %d, 9 (later payment ignored)", d.Paid, d.Outstanding, d.AgeDays, before, total-before)
	}
	if rep.Total != total-before || rep.Buckets.Current != total-before {
		t.Fatalf("total %d buckets %+v; want %d", rep.Total, rep.Buckets, total-before)
	}
	if rep.AccountCode != "1100" || rep.GLBalance != 7000 || rep.Difference != 7000-(total-before) {
		t.Fatalf("reconciliation account %s gl %d diff %d; want 1100, 7000, %d", rep.AccountCode, rep.GLBalance, rep.Difference, 7000-(total-before))
	}

	// Later, both orders and both payments count.
	rep, err = fin.AgingDocuments(ctx(), fx.branchID, "receivable", "2026-09-30")
	if err != nil {
		t.Fatalf("aging later: %v", err)
	}
	want := total - before - after + orderTotal(t, late)
	if len(rep.Documents) != 2 || rep.Total != want || rep.GLBalance != 10000 {
		t.Fatalf("later aging docs %d total %d gl %d; want 2, %d, 10000", len(rep.Documents), rep.Total, rep.GLBalance, want)
	}
}
