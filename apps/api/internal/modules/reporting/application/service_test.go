package application

import (
	"context"
	"testing"

	financecontracts "mini-erp/internal/modules/finance/contracts"
)

// fakeFinance is an in-memory FinanceClient double with call counters: it
// proves the trend costs one grouped DailyTrend call and zero per-day
// NetProfit calls (F8, P3).
type fakeFinance struct {
	dailyCalls     int
	dailyFrom      string
	dailyTo        string
	dailyBranch    int64
	netProfitCalls int
	points         []financecontracts.DailyPoint
}

func (f *fakeFinance) AccountBalance(_ context.Context, _ int64, _ string) (int64, int64, error) {
	return 0, 0, nil
}

func (f *fakeFinance) NetProfit(_ context.Context, _ int64, _, _ string) (int64, int64, int64, error) {
	f.netProfitCalls++
	return 0, 0, 0, nil
}

func (f *fakeFinance) InventoryValue(_ context.Context, _ int64) ([]financecontracts.InventoryPosition, int64, error) {
	return nil, 0, nil
}

func (f *fakeFinance) DailyTrend(_ context.Context, branchID int64, from, to string) ([]financecontracts.DailyPoint, error) {
	f.dailyCalls++
	f.dailyBranch, f.dailyFrom, f.dailyTo = branchID, from, to
	return f.points, nil
}

func (f *fakeFinance) MarginByProduct(_ context.Context, _ int64, _, _ string, _ int) ([]financecontracts.ProductMargin, error) {
	return nil, nil
}

// TestSalesTrendSingleGroupedRead (F8): a 30-day trend must cost exactly one
// grouped finance read and zero per-day NetProfit calls — the old loop needed
// 30 sequential reads.
func TestSalesTrendSingleGroupedRead(t *testing.T) {
	fin := &fakeFinance{points: []financecontracts.DailyPoint{
		{Date: "2026-09-01", Revenue: 100, Expense: 40, Profit: 60},
		{Date: "2026-09-30", Revenue: 200, Expense: 50, Profit: 150},
	}}
	svc := NewService(fin)
	points, err := svc.SalesTrend(context.Background(), 7, "2026-09-01", "2026-09-30")
	if err != nil {
		t.Fatal(err)
	}
	if fin.dailyCalls != 1 {
		t.Fatalf("DailyTrend calls = %d, want exactly 1 grouped read", fin.dailyCalls)
	}
	if fin.netProfitCalls != 0 {
		t.Fatalf("NetProfit calls = %d, want 0 (N+1 loop removed)", fin.netProfitCalls)
	}
	if fin.dailyBranch != 7 || fin.dailyFrom != "2026-09-01" || fin.dailyTo != "2026-09-30" {
		t.Fatalf("DailyTrend args = (%d, %s, %s), want branch-scoped passthrough",
			fin.dailyBranch, fin.dailyFrom, fin.dailyTo)
	}
	if len(points) != 2 || points[0].Profit != 60 || points[1].Revenue != 200 {
		t.Fatalf("points = %+v, want grouped answer passed through", points)
	}
}

// TestSalesTrendValidation: bad ranges fail before any finance call.
func TestSalesTrendValidation(t *testing.T) {
	for name, r := range map[string][2]string{
		"bad from":  {"09-01", "2026-09-30"},
		"reversed":  {"2026-09-30", "2026-09-01"},
		"too large": {"2025-01-01", "2026-12-31"},
	} {
		from, to := r[0], r[1]
		t.Run(name, func(t *testing.T) {
			fin := &fakeFinance{}
			svc := NewService(fin)
			if _, err := svc.SalesTrend(context.Background(), 7, from, to); err == nil {
				t.Fatal("want validation error")
			}
			if fin.dailyCalls != 0 || fin.netProfitCalls != 0 {
				t.Fatal("validation must fail before any finance call")
			}
		})
	}
}
