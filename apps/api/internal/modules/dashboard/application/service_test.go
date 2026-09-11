package application

import (
	"context"
	"fmt"
	"testing"

	financecontracts "mini-erp/internal/modules/finance/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/timeutil"
)

// --- fakes (in-memory contract doubles with call counters) ------------------

type fakeFinance struct {
	netProfitCalls int
	dailyCalls     int
	marginCalls    int
	marginFrom     string
	marginTo       string
	marginLimit    int
}

func (f *fakeFinance) AccountBalance(_ context.Context, _ int64, code string) (int64, int64, error) {
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

func (f *fakeFinance) NetProfit(_ context.Context, _ int64, _, _ string) (int64, int64, int64, error) {
	f.netProfitCalls++
	return 100, 40, 60, nil
}

func (f *fakeFinance) InventoryValue(_ context.Context, _ int64) ([]financecontracts.InventoryPosition, int64, error) {
	return []financecontracts.InventoryPosition{}, 700, nil
}

func (f *fakeFinance) DailyTrend(_ context.Context, _ int64, _, _ string) ([]financecontracts.DailyPoint, error) {
	f.dailyCalls++
	return []financecontracts.DailyPoint{}, nil
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

func (f *fakeStock) Release(_ context.Context, _ int64, _ string) error { return nil }

func (f *fakeStock) Consume(_ context.Context, _ string, _ float64, _ string, _ int64, _ int64) error {
	return nil
}

func (f *fakeStock) MoveIn(_ context.Context, _, _, _, _ int64, _ float64, _ string, _ int64, _ int64) error {
	return nil
}

func (f *fakeStock) MoveOut(_ context.Context, _ int64, _, _, _ int64, _ float64, _ string, _ int64, _ int64) error {
	return nil
}

func (f *fakeStock) CheckLocation(_ context.Context, _, _ int64) error { return nil }

func (f *fakeStock) ListLocations(_ context.Context, _ int64, _ string) ([]*stockcontracts.Location, error) {
	return nil, nil
}

func (f *fakeStock) ListCostMovements(_ context.Context, _, _ int64, _ int) ([]*stockcontracts.CostMovement, error) {
	return nil, nil
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
}

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

func (f *fakeSales) CountMissingTaxInvoice(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}

func (f *fakeSales) OpenShipments(_ context.Context, _ int64, _ int) ([]*salescontracts.OpenShipment, error) {
	return nil, nil
}

func (f *fakeSales) OrderParties(_ context.Context, _ []int64) (map[int64]int64, error) {
	return map[int64]int64{}, nil
}

type fakePurchasing struct {
	statusCalls int
	statuses    map[string]int64
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

func (f *fakePurchasing) CountMissingSupplierInvoice(_ context.Context, _ int64) (int64, error) {
	return 0, nil
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

// TestPriorityOrdersFiltersAndSorts: only confirmed orders with a due date on
// or before today+7d surface, most overdue first, from a single ListOrders
// call.
func TestPriorityOrdersFiltersAndSorts(t *testing.T) {
	now := timeutil.NowUTC().In(timeutil.Jakarta)
	day := func(delta int) string {
		return now.AddDate(0, 0, delta).Format("2006-01-02") + " 00:00:00"
	}
	sa := &fakeSales{orders: []*salescontracts.OrderSummary{
		{ID: 1, Number: "SO-1", DueDate: day(-3)}, // overdue, oldest first
		{ID: 2, Number: "SO-2", DueDate: day(-1)}, // overdue
		{ID: 3, Number: "SO-3", DueDate: day(2)},  // inside window
		{ID: 4, Number: "SO-4", DueDate: day(30)}, // planning, excluded
		{ID: 5, Number: "SO-5", DueDate: ""},      // COD, excluded
		{ID: 6, Number: "SO-6", DueDate: day(7)},  // window edge, included
		{ID: 7, Number: "SO-7", DueDate: day(8)},  // past window, excluded
	}}
	svc := newService(&fakeFinance{}, &fakeStock{}, sa, &fakePurchasing{})
	orders, err := svc.PriorityOrders(context.Background(), 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	if sa.listCalls != 1 {
		t.Fatalf("ListOrders calls = %d, want 1", sa.listCalls)
	}
	if len(sa.listStatus) != 1 || sa.listStatus[0] != salescontracts.StatusConfirmed {
		t.Fatalf("ListOrders statuses = %v, want [confirmed]", sa.listStatus)
	}
	want := []string{"SO-1", "SO-2", "SO-3", "SO-6"}
	if len(orders) != len(want) {
		t.Fatalf("orders = %d, want %d", len(orders), len(want))
	}
	for i, n := range want {
		if orders[i].Number != n {
			t.Fatalf("orders[%d] = %s, want %s (full: %v)", i, orders[i].Number, n, want)
		}
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
	if sa.topLimit != 5 {
		t.Fatalf("limit = %d, want default 5", sa.topLimit)
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
	svc := newService(&fakeFinance{}, &fakeStock{}, &fakeSales{}, &fakePurchasing{})
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
	if sum.Balances.Payable != 300 {
		t.Fatalf("payable = %d, want 300 (credit-normalised positive)", sum.Balances.Payable)
	}
}
