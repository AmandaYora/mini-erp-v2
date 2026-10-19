package integration

import (
	"testing"

	"mini-erp/internal/shared/timeutil"
)

// F2b-13: the dashboard's sales figure comes from completed orders (tax
// included, net of returns) without any journal posting; outstanding is per
// order and never below zero; the best-seller list nets returned quantities.
func TestDashboardOperationsWithoutPosting(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 4)
	deliverAll(t, ctx(), fx, soID, 4)
	total := orderTotal(t, soID)
	dash := testMods.dashboard.Service()

	sum, err := dash.Summary(ctx(), fx.branchID)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if sum.Operations.SalesToday != total || sum.Operations.SalesOrdersToday != 1 {
		t.Fatalf("sales today = %d (%d orders), want %d from one completed order without posting",
			sum.Operations.SalesToday, sum.Operations.SalesOrdersToday, total)
	}
	if sum.Operations.OutstandingReceivable != total {
		t.Fatalf("outstanding receivable = %d, want %d (unpaid completed order)", sum.Operations.OutstandingReceivable, total)
	}
	if sum.Operations.OrdersToday != 1 || sum.Operations.TrackedCount < 1 {
		t.Fatalf("ops = %+v; want 1 order today and >=1 tracked product", sum.Operations)
	}

	// An overpaid order never offsets another order's debt.
	other := sellFixture(t, ctx(), fx, 1)
	otherTotal := orderTotal(t, other)
	if _, err := testMods.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", otherTotal, payAll(other, otherTotal)); err != nil {
		t.Fatalf("pay other: %v", err)
	}
	sum, _ = dash.Summary(ctx(), fx.branchID)
	if sum.Operations.OutstandingReceivable != total {
		t.Fatalf("outstanding = %d, want %d (the paid order owes nothing)", sum.Operations.OutstandingReceivable, total)
	}

	// A confirmed return-only return lowers the sales figure and the best sellers.
	// "Today" is the Jakarta day (not UTC), so the return is dated with Jakarta's clock:
	// UTC_TIMESTAMP() would fall on yesterday between 00:00 and 07:00 WIB.
	ret := createSalesReturn(t, fx, soID, 1)
	if _, err := testMods.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm return: %v", err)
	}
	if _, err := testMods.db.Exec(`UPDATE sales_returns SET return_date = ? WHERE id = ?`, timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02 15:04:05"), ret.ID); err != nil {
		t.Fatalf("date return: %v", err)
	}
	after, _ := dash.Summary(ctx(), fx.branchID)
	if after.Operations.SalesToday >= total {
		t.Fatalf("sales today after return = %d, want below %d", after.Operations.SalesToday, total)
	}
	top, err := dash.TopSellers(ctx(), fx.branchID, 5)
	if err != nil || len(top) != 1 || top[0].QtyBase != 3 {
		t.Fatalf("top sellers = %+v, %v; want one product at 3 units (4 sold − 1 returned)", top, err)
	}
}

// F2b-13: the order and stock reports page with exact totals.
func TestReportingOrdersAndStock(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	sellFixture(t, ctx(), fx, 2)
	sellFixture(t, ctx(), fx, 3)
	rep := testMods.reporting.Service()
	orders, err := rep.Orders(ctx(), fx.branchID, "sales", "", "", "", 1, 1)
	if err != nil || orders.Total != 2 || orders.GrandTotal <= 0 {
		t.Fatalf("orders report = %+v, %v; want total 2 with a grand total", orders, err)
	}
	stock, err := rep.Stock(ctx(), fx.branchID, false, 1, 10)
	if err != nil || stock.Total < 1 {
		t.Fatalf("stock report = %+v, %v", stock, err)
	}
	critical, _ := rep.Stock(ctx(), fx.branchID, true, 1, 10)
	if critical.Total != 0 {
		t.Fatalf("critical-only report = %d rows, want none for a well-stocked product", critical.Total)
	}
}

// F2b-12c: per-document aging lists each open order with its outstanding amount
// and age bucket, and drops paid ones.
func TestAgingPerDocument(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 2)
	paid := sellFixture(t, ctx(), fx, 1)
	paidTotal := orderTotal(t, paid)
	if _, err := testMods.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", paidTotal, payAll(paid, paidTotal)); err != nil {
		t.Fatalf("pay: %v", err)
	}
	// Age the unpaid order by 45 days.
	if _, err := testMods.db.Exec(`UPDATE sales_orders SET order_date = DATE_SUB(UTC_TIMESTAMP(), INTERVAL 45 DAY) WHERE id = ?`, soID); err != nil {
		t.Fatalf("age: %v", err)
	}
	rep, err := testMods.finance.Service().AgingDocuments(ctx(), fx.branchID, "receivable", "")
	if err != nil {
		t.Fatalf("aging: %v", err)
	}
	total := orderTotal(t, soID)
	if len(rep.Documents) != 1 || rep.Documents[0].OrderID != soID || rep.Documents[0].Outstanding != total {
		t.Fatalf("documents = %+v; want only the unpaid order owing %d", rep.Documents, total)
	}
	if rep.Buckets.D31_60 != total || rep.Total != total || rep.Documents[0].AgeDays < 44 {
		t.Fatalf("buckets %+v total %d age %d; want 31-60 bucket", rep.Buckets, rep.Total, rep.Documents[0].AgeDays)
	}
}
