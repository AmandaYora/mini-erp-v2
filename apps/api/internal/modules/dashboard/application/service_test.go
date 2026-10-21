package application

import (
	"context"
	"fmt"
	dashboardcontracts "mini-erp/internal/modules/dashboard/contracts"
	"testing"

	financecontracts "mini-erp/internal/modules/finance/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/timeutil"
)

// --- fakes (in-memory contract doubles with call counters) ------------------

type fakeFinance struct {
	netProfitCalls     int
	dailyCalls         int
	balanceCalls       int
	balancesCalls      int
	inventoryTotalCall int
	inventoryValueCall int
	marginCalls        int
	marginFrom         string
	marginTo           string
	marginLimit        int
	// trendToday, when set, is echoed as the last point of DailyTrend so a
	// test can assert which day Summary reads as "today".
	trendToday string
	// balancesAsOf records the asOf day BalancesByKey was asked for.
	balancesAsOf string
}

func (f *fakeFinance) OpeningPosted(_ context.Context, _ int64) (bool, error) { return false, nil }

func (f *fakeFinance) UncostedCount(_ context.Context, _ int64) (int64, error) { return 0, nil }

func (f *fakeFinance) AverageUnitCost(_ context.Context, _, _, _ int64) (int64, error) { return 0, nil }

func (f *fakeFinance) AccountBalance(_ context.Context, _ int64, code string) (int64, int64, error) {
	f.balanceCalls++
	switch code {
	case financecontracts.CodeCash:
		return 1000, 0, nil
	case financecontracts.CodeReceivable:
		return 500, 0, nil
	case financecontracts.CodePayable:
		return 0, 300, nil
	case financecontracts.CodeInventory:
		return 700, 0, nil
	}
	return 0, 0, nil
}

func (f *fakeFinance) BalancesByKey(_ context.Context, _ int64, keys []string, asOf string) (map[string]int64, error) {
	f.balancesCalls++
	f.balancesAsOf = asOf
	net := map[string]int64{"cash": 1000, "receivable": 500, "payable": -300, "inventory": 700}
	out := map[string]int64{}
	for _, k := range keys {
		out[k] = net[k]
	}
	return out, nil
}

func (f *fakeFinance) AccountBalances(_ context.Context, _ int64, codes []string) (map[string]int64, error) {
	f.balancesCalls++
	net := map[string]int64{
		financecontracts.CodeCash:       1000,
		financecontracts.CodeReceivable: 500,
		financecontracts.CodePayable:    -300,
		financecontracts.CodeInventory:  700,
	}
	out := map[string]int64{}
	for _, c := range codes {
		out[c] = net[c]
	}
	return out, nil
}

func (f *fakeFinance) NetProfit(_ context.Context, _ int64, _, _ string) (int64, int64, int64, error) {
	f.netProfitCalls++
	return 100, 40, 60, nil
}

func (f *fakeFinance) InventoryValue(_ context.Context, _ int64) ([]financecontracts.InventoryPosition, int64, error) {
	f.inventoryValueCall++
	return []financecontracts.InventoryPosition{}, 700, nil
}

func (f *fakeFinance) InventoryTotal(_ context.Context, _ int64) (int64, error) {
	f.inventoryTotalCall++
	return 700, nil
}

func (f *fakeFinance) DailyTrend(_ context.Context, _ int64, _, to string) ([]financecontracts.DailyPoint, error) {
	f.dailyCalls++
	if f.trendToday == "" {
		return []financecontracts.DailyPoint{}, nil
	}
	// Two days: one earlier, one for `to` (which Summary treats as today),
	// so month-to-date and today are distinguishable.
	return []financecontracts.DailyPoint{
		{Date: "1970-01-01", Revenue: 20, Expense: 5, Profit: 15},
		{Date: to, Revenue: 100, Expense: 40, Profit: 60},
	}, nil
}

func (f *fakeFinance) MarginByProduct(_ context.Context, _ int64, from, to string, limit int) ([]financecontracts.ProductMargin, error) {
	f.marginCalls++
	f.marginFrom, f.marginTo, f.marginLimit = from, to, limit
	return []financecontracts.ProductMargin{{ProductID: 1, Gross: 900}}, nil
}

type fakeStock struct {
	criticalCalls int
	criticalLimit int
	items         []*stockcontracts.CriticalItem
}

func (f *fakeStock) CriticalStock(_ context.Context, _ int64, limit int) ([]*stockcontracts.CriticalItem, error) {
	f.criticalCalls++
	f.criticalLimit = limit
	return f.items, nil
}

func (f *fakeStock) GetBalance(_ context.Context, branchID, productID, variantID, locationID int64) (*stockcontracts.Balance, error) {
	return &stockcontracts.Balance{}, nil
}

func (f *fakeStock) Hold(_ context.Context, _, _, _, _, _ int64, _ float64, _ string, _ int) (int64, error) {
	return 0, nil
}

func (f *fakeStock) Allocate(_ context.Context, _, _, _ int64, _ float64) ([]stockcontracts.Suggestion, float64, error) {
	return nil, 0, nil
}
func (f *fakeStock) ListRecentCostMovements(_ context.Context, _ int64, _, _ int) ([]*stockcontracts.CostMovement, error) {
	return nil, nil
}

func (f *fakeStock) CriticalSummary(_ context.Context, _ int64) (int64, int64, error) {
	return 2, 10, nil
}

func (f *fakeStock) StockReport(_ context.Context, _ int64, _ bool, _, _ int) (*stockcontracts.StockReport, error) {
	return &stockcontracts.StockReport{}, nil
}

func (f *fakeStock) Release(_ context.Context, _, _ int64, _ string) error { return nil }

func (f *fakeStock) Consume(_ context.Context, _ string, _ float64, _ string, _ int64, _ int64) error {
	return nil
}

func (f *fakeStock) Post(_ context.Context, _ stockcontracts.PostRequest) (*stockcontracts.PostResult, error) {
	return &stockcontracts.PostResult{}, nil
}

func (f *fakeStock) Reverse(_ context.Context, _ int64, _ string, _, _ int64) (*stockcontracts.PostResult, error) {
	return &stockcontracts.PostResult{}, nil
}

func (f *fakeStock) DamagedLocation(_ context.Context, _ int64) (*stockcontracts.Location, error) {
	return &stockcontracts.Location{}, nil
}

func (f *fakeStock) HasMovements(_ context.Context, _ int64, _ string, _ int64) (bool, error) {
	return false, nil
}

func (f *fakeStock) CheckLocation(_ context.Context, _, _ int64) error { return nil }

func (f *fakeStock) ListLocations(_ context.Context, _ int64, _ string) ([]*stockcontracts.Location, error) {
	return nil, nil
}

func (f *fakeStock) ListCostMovements(_ context.Context, _, _ int64, _ int) ([]*stockcontracts.CostMovement, error) {
	return nil, nil
}
func (f *fakeStock) TransferSourceBranch(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}

type fakeSales struct {
	listCalls   int
	listStatus  []string
	listLimit   int
	orders      []*salescontracts.OrderSummary
	statusCalls int
	statuses    map[string]int64
	topCalls    int
	topFrom     string
	topTo       string
	topLimit    int
	top         []*salescontracts.TopProduct
	priority    []*salescontracts.PriorityOrder
}

func (f *fakeSales) SwitchToNet(_ context.Context, _, _ int64, _ string, _ int64) error { return nil }

func (f *fakeSales) GetByID(_ context.Context, _ int64) (*salescontracts.SalesOrder, error) {
	return nil, nil
}

func (f *fakeSales) ListByParty(_ context.Context, _, _ int64) ([]*salescontracts.OrderSummary, error) {
	return nil, nil
}

func (f *fakeSales) ListOrders(_ context.Context, _ int64, statuses []string, _, _ string, limit int) ([]*salescontracts.OrderSummary, error) {
	f.listCalls++
	f.listStatus = statuses
	f.listLimit = limit
	return f.orders, nil
}

func (f *fakeSales) GetByNumber(_ context.Context, _ int64, _ string) (*salescontracts.SalesOrder, error) {
	return nil, nil
}

func (f *fakeSales) SetStatus(_ context.Context, _, _ int64, _ string, _ int64) error { return nil }

func (f *fakeSales) StatusCounts(_ context.Context, _ int64) (map[string]int64, error) {
	f.statusCalls++
	return f.statuses, nil
}

func (f *fakeSales) TopProducts(_ context.Context, _ int64, from, to string, limit int) ([]*salescontracts.TopProduct, error) {
	f.topCalls++
	f.topFrom, f.topTo, f.topLimit = from, to, limit
	return f.top, nil
}

func (f *fakeSales) CompletedSales(_ context.Context, _ int64, _, _ string) (int64, int64, error) {
	return 0, 0, nil
}

func (f *fakeSales) CountOrders(_ context.Context, _ int64, _, _ string) (int64, error) {
	return 0, nil
}

func (f *fakeSales) PriorityOrders(_ context.Context, _ int64, limit int) ([]*salescontracts.PriorityOrder, error) {
	f.listCalls++
	f.listLimit = limit
	return f.priority, nil
}

func (f *fakeSales) OpenOrderTotals(_ context.Context, _, _ int64, _ int) ([]salescontracts.OrderTotal, error) {
	return nil, nil
}

func (f *fakeSales) ReportOrders(_ context.Context, _ int64, _, _, _ string, _, _ int) (*salescontracts.OrderReport, error) {
	return &salescontracts.OrderReport{}, nil
}

func (f *fakeSales) CountMissingTaxInvoiceBetween(_ context.Context, _ int64, _, _ string) (int64, error) {
	return 0, nil
}

func (f *fakeSales) CountMissingTaxInvoice(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}

func (f *fakeSales) OpenShipments(_ context.Context, _ int64, _ int) ([]*salescontracts.OpenShipment, error) {
	return nil, nil
}

func (f *fakeSales) SearchOrderIDs(_ context.Context, _ int64, _ string, _ int) ([]int64, error) {
	return nil, nil
}

func (f *fakeSales) OrderParties(_ context.Context, _ []int64) (map[int64]int64, error) {
	return map[int64]int64{}, nil
}

func (f *fakeSales) SummariesByIDs(_ context.Context, _ []int64) (map[int64]*salescontracts.OrderSummary, error) {
	return map[int64]*salescontracts.OrderSummary{}, nil
}

type fakePurchasing struct {
	statusCalls int
	statuses    map[string]int64
	// drafts are the draft orders ReportOrders returns; open the open-order
	// totals (payable base); names resolve SummariesByIDs.
	drafts []*purchasingcontracts.OrderSummary
	open   []purchasingcontracts.OrderTotal
	names  map[int64]*purchasingcontracts.OrderSummary
	// periods answers ReportOrders(status "") per "from|to": the non-cancelled
	// value (GrandTotal) and count (Total) of that range.
	periods map[string]purchasingcontracts.OrderReport
}

func (f *fakePurchasing) SummariesByIDs(_ context.Context, ids []int64) (map[int64]*purchasingcontracts.OrderSummary, error) {
	out := map[int64]*purchasingcontracts.OrderSummary{}
	for _, id := range ids {
		if n, ok := f.names[id]; ok {
			out[id] = n
		}
	}
	return out, nil
}

func (f *fakePurchasing) SwitchToNet(_ context.Context, _, _ int64, _ string, _ int64) error {
	return nil
}

func (f *fakePurchasing) GetByID(_ context.Context, _ int64) (*purchasingcontracts.PurchaseOrder, error) {
	return nil, nil
}

func (f *fakePurchasing) ListByParty(_ context.Context, _, _ int64) ([]*purchasingcontracts.OrderSummary, error) {
	return nil, nil
}

func (f *fakePurchasing) ListOrders(_ context.Context, _ int64, _ []string, _, _ string, _ int) ([]*purchasingcontracts.OrderSummary, error) {
	return nil, nil
}

func (f *fakePurchasing) GetByNumber(_ context.Context, _ int64, _ string) (*purchasingcontracts.PurchaseOrder, error) {
	return nil, nil
}

func (f *fakePurchasing) SetStatus(_ context.Context, _, _ int64, _ string, _ int64) error {
	return nil
}

func (f *fakePurchasing) StatusCounts(_ context.Context, _ int64) (map[string]int64, error) {
	f.statusCalls++
	return f.statuses, nil
}

func (f *fakePurchasing) CountOrders(_ context.Context, _ int64, _, _ string) (int64, error) {
	return 0, nil
}

func (f *fakePurchasing) OpenOrderTotals(_ context.Context, _, after int64, _ int) ([]purchasingcontracts.OrderTotal, error) {
	var out []purchasingcontracts.OrderTotal
	for _, o := range f.open {
		if o.ID > after {
			out = append(out, o)
		}
	}
	return out, nil
}

func (f *fakePurchasing) ReportOrders(_ context.Context, _ int64, status, from, to string, _, _ int) (*purchasingcontracts.OrderReport, error) {
	if status == "" {
		rep := f.periods[from+"|"+to]
		return &rep, nil
	}
	if status == "draft" {
		return &purchasingcontracts.OrderReport{Rows: f.drafts, Total: int64(len(f.drafts))}, nil
	}
	return &purchasingcontracts.OrderReport{}, nil
}

func (f *fakePurchasing) CountMissingSupplierInvoiceBetween(_ context.Context, _ int64, _, _ string) (int64, error) {
	return 0, nil
}

func (f *fakePurchasing) CountMissingSupplierInvoice(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}

func (f *fakePurchasing) SearchOrderIDs(_ context.Context, _ int64, _ string, _ int) ([]int64, error) {
	return nil, nil
}

func (f *fakePurchasing) OrderParties(_ context.Context, _ []int64) (map[int64]int64, error) {
	return map[int64]int64{}, nil
}

func newService(fin *fakeFinance, st *fakeStock, sa *fakeSales, pu *fakePurchasing) *Service {
	return NewService(fin, st, sa, pu)
}

// --- tests ------------------------------------------------------------------

// TestCriticalStockSingleGroupedRead (§7 Tahap F): 1.000 critical products
// must cost exactly one grouped stock read — the dashboard adds no per-row
// call of its own.
func TestCriticalStockSingleGroupedRead(t *testing.T) {
	st := &fakeStock{}
	for i := 1; i <= 1000; i++ {
		st.items = append(st.items, &stockcontracts.CriticalItem{
			ProductID: int64(i), ProductCode: fmt.Sprintf("P-%04d", i),
			Available: 1, MinStock: 10,
		})
	}
	// A healthy product must never be returned by the provider; the dashboard
	// passes the grouped answer through untouched.
	svc := newService(&fakeFinance{}, st, &fakeSales{}, &fakePurchasing{})
	items, err := svc.CriticalStock(context.Background(), 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	if st.criticalCalls != 1 {
		t.Fatalf("CriticalStock calls = %d, want exactly 1 grouped read", st.criticalCalls)
	}
	if len(items) != 1000 {
		t.Fatalf("items = %d, want 1000 passed through", len(items))
	}
	if st.criticalLimit != 20 {
		t.Fatalf("default limit = %d, want 20", st.criticalLimit)
	}
}

// TestPriorityOrdersDelegatesRanking: the urgency score and ordering live in
// the sales read (SQL over ALL live orders), the dashboard only applies the
// display cap — one call.
func TestPriorityOrdersDelegatesRanking(t *testing.T) {
	sa := &fakeSales{priority: []*salescontracts.PriorityOrder{
		{ID: 1, Number: "SO-1", Score: 130}, {ID: 2, Number: "SO-2", Score: 90},
	}}
	svc := newService(&fakeFinance{}, &fakeStock{}, sa, &fakePurchasing{})
	orders, err := svc.PriorityOrders(context.Background(), 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	if sa.listCalls != 1 || sa.listLimit != 5 {
		t.Fatalf("priority calls=%d limit=%d, want 1 call with the default cap 5", sa.listCalls, sa.listLimit)
	}
	if len(orders) != 2 || orders[0].Number != "SO-1" {
		t.Fatalf("orders = %+v, want the sales ranking passed through", orders)
	}
}

// TestOrderStatusMergesFlows: one grouped read per order flow, merged into a
// single payload.
func TestOrderStatusMergesFlows(t *testing.T) {
	sa := &fakeSales{statuses: map[string]int64{"draft": 2, "confirmed": 5}}
	pu := &fakePurchasing{statuses: map[string]int64{"confirmed": 1, "cancelled": 4}}
	svc := newService(&fakeFinance{}, &fakeStock{}, sa, pu)
	status, err := svc.OrderStatus(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if sa.statusCalls != 1 || pu.statusCalls != 1 {
		t.Fatalf("status calls = sales %d purchasing %d, want 1 each", sa.statusCalls, pu.statusCalls)
	}
	if status.Sales["confirmed"] != 5 || status.Sales["draft"] != 2 {
		t.Fatalf("sales = %v, want draft:2 confirmed:5", status.Sales)
	}
	if status.Purchasing["confirmed"] != 1 || status.Purchasing["cancelled"] != 4 {
		t.Fatalf("purchasing = %v, want confirmed:1 cancelled:4", status.Purchasing)
	}
}

// TestTopSellersUsesMonthRange: best sellers always cover month-to-date with
// the default display cap.
func TestTopSellersUsesMonthRange(t *testing.T) {
	sa := &fakeSales{top: []*salescontracts.TopProduct{{ProductID: 9, QtyBase: 12}}}
	svc := newService(&fakeFinance{}, &fakeStock{}, sa, &fakePurchasing{})
	rows, err := svc.TopSellers(context.Background(), 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	if sa.topCalls != 1 {
		t.Fatalf("TopProducts calls = %d, want 1 grouped read", sa.topCalls)
	}
	now := timeutil.NowUTC().In(timeutil.Jakarta)
	if want := now.Format("2006-01") + "-01"; sa.topFrom != want {
		t.Fatalf("from = %s, want month start %s", sa.topFrom, want)
	}
	if want := now.Format("2006-01-02"); sa.topTo != want {
		t.Fatalf("to = %s, want today %s", sa.topTo, want)
	}
	if sa.topLimit != 200 {
		t.Fatalf("fetch limit = %d, want 200 (netted against returns, then capped to 5)", sa.topLimit)
	}
	if len(rows) != 1 || rows[0].ProductID != 9 {
		t.Fatalf("rows = %+v, want passthrough", rows)
	}
}

// TestTopMarginUsesMonthRange: highest margin comes from the books over
// month-to-date, one finance call.
func TestTopMarginUsesMonthRange(t *testing.T) {
	fin := &fakeFinance{}
	svc := newService(fin, &fakeStock{}, &fakeSales{}, &fakePurchasing{})
	rows, err := svc.TopMargin(context.Background(), 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	if fin.marginCalls != 1 {
		t.Fatalf("MarginByProduct calls = %d, want 1", fin.marginCalls)
	}
	now := timeutil.NowUTC().In(timeutil.Jakarta)
	if want := now.Format("2006-01") + "-01"; fin.marginFrom != want {
		t.Fatalf("from = %s, want month start %s", fin.marginFrom, want)
	}
	if want := now.Format("2006-01-02"); fin.marginTo != want {
		t.Fatalf("to = %s, want today %s", fin.marginTo, want)
	}
	if fin.marginLimit != 5 {
		t.Fatalf("limit = %d, want default 5", fin.marginLimit)
	}
	if len(rows) != 1 || rows[0].Gross != 900 {
		t.Fatalf("rows = %+v, want passthrough", rows)
	}
}

// TestSummaryUnchanged: the booked snapshot keeps working after the F1
// constructor change (finance-only path, no operational client touched).
func TestSummaryUnchanged(t *testing.T) {
	fin := &fakeFinance{trendToday: "echo"}
	svc := newService(fin, &fakeStock{}, &fakeSales{}, &fakePurchasing{})
	sum, err := svc.Summary(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if sum.BranchID != 3 {
		t.Fatalf("branch = %d, want 3", sum.BranchID)
	}
	if sum.Today.Revenue != 100 || sum.Today.Profit != 60 {
		t.Fatalf("today = %+v, want revenue:100 profit:60", sum.Today)
	}
	// Month-to-date sums the whole series, today is only its last point.
	if sum.MonthToDate.Revenue != 120 || sum.MonthToDate.Profit != 75 {
		t.Fatalf("month = %+v, want revenue:120 profit:75", sum.MonthToDate)
	}
	if sum.Balances.Payable != 300 {
		t.Fatalf("payable = %d, want 300 (credit-normalised positive)", sum.Balances.Payable)
	}
	if sum.Balances.Cash != 1000 || sum.Balances.Inventory != 700 {
		t.Fatalf("balances = %+v, want cash:1000 inventory:700", sum.Balances)
	}
	if sum.InventoryValue != 700 {
		t.Fatalf("inventory value = %d, want 700", sum.InventoryValue)
	}
}

// TestSummaryPurchasesAndBalancesAsOfToday: "Pembelian" is rupiah (summed
// grand totals of the period's purchase orders) with its own order count —
// never the purchase ORDER COUNT shown as money — and the key balances are
// read as of today (WIB), so future-dated journals do not inflate them.
func TestSummaryPurchasesAndBalancesAsOfToday(t *testing.T) {
	today := timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	monthStart := today[:8] + "01"
	fin := &fakeFinance{trendToday: "echo"}
	pu := &fakePurchasing{periods: map[string]purchasingcontracts.OrderReport{
		today + "|" + today:      {Total: 2, GrandTotal: 250000},
		monthStart + "|" + today: {Total: 5, GrandTotal: 900000},
	}}
	svc := newService(fin, &fakeStock{}, &fakeSales{}, pu)
	sum, err := svc.Summary(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	ops := sum.Operations
	if ops.PurchasesToday != 250000 || ops.PurchaseOrdersToday != 2 {
		t.Fatalf("purchases today = %d (%d PO), want 250000 (2 PO)", ops.PurchasesToday, ops.PurchaseOrdersToday)
	}
	if ops.PurchasesMonth != 900000 || ops.PurchaseOrdersMonth != 5 {
		t.Fatalf("purchases month = %d (%d PO), want 900000 (5 PO)", ops.PurchasesMonth, ops.PurchaseOrdersMonth)
	}
	if fin.balancesAsOf != today {
		t.Fatalf("balances asOf = %q, want today %q (balance sheet of today)", fin.balancesAsOf, today)
	}
}

// TestSummaryReadsAreGrouped pins the query budget of the dashboard's
// headline endpoint. Each of these used to be a per-figure read: two P&L
// computations for two overlapping ranges, one whole-history footing per key
// balance, and a full valued inventory whose product names were discarded.
func TestSummaryReadsAreGrouped(t *testing.T) {
	fin := &fakeFinance{trendToday: "echo"}
	svc := newService(fin, &fakeStock{}, &fakeSales{}, &fakePurchasing{})
	if _, err := svc.Summary(context.Background(), 3); err != nil {
		t.Fatal(err)
	}
	if fin.dailyCalls != 1 {
		t.Fatalf("DailyTrend calls = %d, want 1 (today and month from one series)", fin.dailyCalls)
	}
	if fin.netProfitCalls != 0 {
		t.Fatalf("NetProfit calls = %d, want 0 (both periods come from DailyTrend)", fin.netProfitCalls)
	}
	if fin.balancesCalls != 1 {
		t.Fatalf("AccountBalances calls = %d, want 1 grouped read", fin.balancesCalls)
	}
	if fin.balanceCalls != 0 {
		t.Fatalf("AccountBalance calls = %d, want 0 (each one re-foots the whole ledger)", fin.balanceCalls)
	}
	if fin.inventoryTotalCall != 1 || fin.inventoryValueCall != 0 {
		t.Fatalf("inventory reads = total:%d value:%d, want total:1 value:0 (names are never shown here)",
			fin.inventoryTotalCall, fin.inventoryValueCall)
	}
}

// fakePaid answers PaidForOrders from a fixed map.
type fakePaid map[int64]int64

func (f fakePaid) PaidForOrders(_ context.Context, _ string, ids []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	for _, id := range ids {
		out[id] = f[id]
	}
	return out, nil
}

// TestPriorityOrdersIncludePurchasesWithReasonAndAge (DB-01): purchase orders
// past due and unpaid, and drafts awaiting a decision, join the sales ranking,
// each with its reason and age; a fully paid overdue order stays off the list.
func TestPriorityOrdersIncludePurchasesWithReasonAndAge(t *testing.T) {
	sa := &fakeSales{priority: []*salescontracts.PriorityOrder{
		{ID: 1, Number: "SO-1", Score: 100, Status: "confirmed", DueDate: "2000-01-01", OrderDate: "1999-12-01"},
	}}
	pu := &fakePurchasing{
		drafts: []*purchasingcontracts.OrderSummary{{ID: 7, Number: "PO-7", Status: "draft", PartyName: "Sup A", OrderDate: "2000-01-01"}},
		open: []purchasingcontracts.OrderTotal{
			{ID: 8, Number: "PO-8", GrandTotal: 1000, OrderDate: "2000-01-01", DueDate: "2000-02-01"},
			{ID: 9, Number: "PO-9", GrandTotal: 500, OrderDate: "2000-01-01", DueDate: "2000-02-01"},
			{ID: 10, Number: "PO-10", GrandTotal: 500, OrderDate: "2000-01-01", DueDate: "2999-01-01"},
		},
		names: map[int64]*purchasingcontracts.OrderSummary{8: {ID: 8, Status: "confirmed", PartyName: "Sup B"}},
	}
	svc := newService(&fakeFinance{}, &fakeStock{}, sa, pu)
	svc.SetSources(nil, fakePaid{9: 500})
	items, err := svc.PriorityOrders(context.Background(), 3, 20)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]*dashboardcontracts.PriorityItem{}
	for _, it := range items {
		got[it.Kind+":"+it.Number] = it
	}
	if len(items) != 3 {
		t.Fatalf("items = %d, want SO-1, PO-8 (overdue unpaid), PO-7 (draft): %+v", len(items), items)
	}
	if it := got["purchase:PO-8"]; it == nil || it.Reason != dashboardcontracts.ReasonOverdue || it.PartyName != "Sup B" || it.AgeDays <= 0 {
		t.Fatalf("PO-8 = %+v, want overdue reason, party and age", it)
	}
	if it := got["purchase:PO-7"]; it == nil || it.Reason != dashboardcontracts.ReasonAwaitingDecision {
		t.Fatalf("PO-7 = %+v, want awaiting decision", it)
	}
	if it := got["sales:SO-1"]; it == nil || it.Reason != dashboardcontracts.ReasonOverdue {
		t.Fatalf("SO-1 = %+v, want overdue", it)
	}
	if items[0].Score < items[len(items)-1].Score {
		t.Fatalf("not ordered by score: %+v", items)
	}
}
