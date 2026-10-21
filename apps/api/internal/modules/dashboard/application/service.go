package application

import (
	"context"

	dashboardcontracts "mini-erp/internal/modules/dashboard/contracts"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
)

// Sales is the sales surface the operational dashboard consumes: document
// reads plus grouped operational aggregates. Implemented by the sales module;
// main wires the concrete service, tests wire fakes.
type Sales interface {
	salescontracts.SalesOrderClient
	salescontracts.SalesOpsClient
}

// Purchasing is the purchasing surface the operational dashboard consumes:
// document reads plus grouped operational aggregates.
type Purchasing interface {
	purchasingcontracts.PurchaseOrderClient
	purchasingcontracts.PurchasingOpsClient
}

// Service composes the operational snapshot from finance (posted books) and
// the operational widgets from stock/sales/purchasing (live documents). The
// dashboard owns no tables: every figure below costs a stated, constant
// number of grouped reads (P3).
type Service struct {
	finance    financecontracts.FinanceClient
	stock      stockcontracts.StockClient
	sales      Sales
	purchasing Purchasing
	returns    Returns
	payments   Payments
}

// Returns is the slice of salesreturn the dashboard nets sales and best
// sellers with.
type Returns interface {
	ReturnedByProduct(ctx context.Context, branchID int64, from, to string) (map[int64]float64, error)
	NetAdjustment(ctx context.Context, branchID int64, from, to string) (int64, error)
}

// Payments is the slice of payment the outstanding figures need: what has been
// paid per order, in one grouped read per page.
type Payments interface {
	PaidForOrders(ctx context.Context, orderType string, ids []int64) (map[int64]int64, error)
}

// SetSources wires returns and payments (built after the dashboard's base
// readers). Without them the sales/outstanding figures stay zero rather than
// invented.
func (s *Service) SetSources(returns Returns, payments Payments) {
	s.returns, s.payments = returns, payments
}

// NewService wires dashboard reads over module contracts (Rule A:
// contracts-only, never application/infrastructure of other modules).
func NewService(finance financecontracts.FinanceClient, stock stockcontracts.StockClient, sales Sales, purchasing Purchasing) *Service {
	return &Service{finance: finance, stock: stock, sales: sales, purchasing: purchasing}
}

// Summary returns today + month-to-date P&L plus key book balances.
//
// Cost: three grouped finance reads, constant in the size of the books.
//
//   - DailyTrend over the month gives BOTH periods. Today is the last point
//     of month-to-date, so asking for the month and summing it answers both
//     questions from one pass; two NetProfit calls computed the same P&L
//     twice, the second one a superset of the first.
//   - AccountBalances resolves all four key figures from one footing. Four
//     AccountBalance calls each aggregated the branch's entire posting
//     history to read a single account out of the result.
//   - InventoryTotal skips per-position product naming that this endpoint
//     never displays — it uses only the total.
func (s *Service) Summary(ctx context.Context, branchID int64) (*dashboardcontracts.Summary, error) {
	today := timeutil.NowUTC().In(timeutil.Jakarta)
	day := today.Format("2006-01-02")
	monthStart := today.Format("2006-01") + "-01"

	points, err := s.finance.DailyTrend(ctx, branchID, monthStart, day)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	// DailyTrend emits a dense series, so the last point is always today.
	var rev, exp, profit, mRev, mExp, mProfit int64
	for _, p := range points {
		mRev += p.Revenue
		mExp += p.Expense
		mProfit += p.Profit
		if p.Date == day {
			rev, exp, profit = p.Revenue, p.Expense, p.Profit
		}
	}

	balances, err := s.finance.BalancesByKey(ctx, branchID, []string{
		"cash", "bank", "receivable", "payable", "inventory",
	})
	if err != nil {
		return nil, apperror.Internal(err)
	}

	invTotal, err := s.finance.InventoryTotal(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	ops, err := s.operations(ctx, branchID, monthStart, day)
	if err != nil {
		return nil, err
	}
	return &dashboardcontracts.Summary{
		Operations:  *ops,
		BranchID:    branchID,
		Today:       dashboardcontracts.Period{From: day, To: day, Revenue: rev, Expense: exp, Profit: profit},
		MonthToDate: dashboardcontracts.Period{From: monthStart, To: day, Revenue: mRev, Expense: mExp, Profit: mProfit},
		Balances: dashboardcontracts.Balances{
			// "Kas & Bank": cash and bank are both money on hand (N9).
			Cash:       balances["cash"] + balances["bank"],
			Receivable: balances["receivable"],
			Payable:    -balances["payable"],
			Inventory:  balances["inventory"],
		},
		InventoryValue: invTotal,
	}, nil
}

// operations reads the live-document figures: sales completed (net of
// returns), order counts, per-order outstanding and critical-stock counts.
//
// Query count: constant in the data size — 2 completed-sales reads, 2 + 2
// order counts, 2 return adjustments, 1 critical summary, plus one grouped
// payment read per 1.000 open orders (receivable and payable).
func (s *Service) operations(ctx context.Context, branchID int64, monthStart, today string) (*dashboardcontracts.Operations, error) {
	ops := &dashboardcontracts.Operations{}
	var err error
	var n int64
	if ops.SalesToday, ops.SalesOrdersToday, err = s.sales.CompletedSales(ctx, branchID, today, today); err != nil {
		return nil, apperror.Internal(err)
	}
	if ops.SalesMonth, ops.SalesOrdersMonth, err = s.sales.CompletedSales(ctx, branchID, monthStart, today); err != nil {
		return nil, apperror.Internal(err)
	}
	if s.returns != nil {
		adjToday, err := s.returns.NetAdjustment(ctx, branchID, today, today)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		adjMonth, err := s.returns.NetAdjustment(ctx, branchID, monthStart, today)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		ops.SalesToday += adjToday
		ops.SalesMonth += adjMonth
	}
	if ops.OrdersToday, err = s.sales.CountOrders(ctx, branchID, today, today); err != nil {
		return nil, apperror.Internal(err)
	}
	if ops.OrdersMonth, err = s.sales.CountOrders(ctx, branchID, monthStart, today); err != nil {
		return nil, apperror.Internal(err)
	}
	if n, err = s.purchasing.CountOrders(ctx, branchID, today, today); err != nil {
		return nil, apperror.Internal(err)
	}
	ops.PurchasesToday = n
	if n, err = s.purchasing.CountOrders(ctx, branchID, monthStart, today); err != nil {
		return nil, apperror.Internal(err)
	}
	ops.PurchasesMonth = n
	if ops.CriticalCount, ops.TrackedCount, err = s.stock.CriticalSummary(ctx, branchID); err != nil {
		return nil, apperror.Internal(err)
	}
	// Cost-basis status of the margin figures (DB-01).
	if ops.MissingCostCount, err = s.finance.UncostedCount(ctx, branchID); err != nil {
		return nil, apperror.Internal(err)
	}
	ops.CostBasis = "complete"
	if ops.MissingCostCount > 0 {
		ops.CostBasis = "estimated"
	}
	if s.payments != nil {
		if ops.OutstandingReceivable, err = s.outstanding(ctx, branchID, paymentcontracts.OrderSales); err != nil {
			return nil, err
		}
		if ops.OutstandingPayable, err = s.outstanding(ctx, branchID, paymentcontracts.OrderPurchase); err != nil {
			return nil, err
		}
	}
	return ops, nil
}

// outstanding sums, over every confirmed/completed order, what is still owed
// per order (grand total minus paid, never below zero) — legacy definition.
func (s *Service) outstanding(ctx context.Context, branchID int64, orderType string) (int64, error) {
	var total, after int64
	for {
		var ids []int64
		var grand map[int64]int64
		if orderType == paymentcontracts.OrderSales {
			rows, err := s.sales.OpenOrderTotals(ctx, branchID, after, 1000)
			if err != nil {
				return 0, apperror.Internal(err)
			}
			grand = map[int64]int64{}
			for _, r := range rows {
				ids = append(ids, r.ID)
				grand[r.ID] = r.GrandTotal
			}
		} else {
			rows, err := s.purchasing.OpenOrderTotals(ctx, branchID, after, 1000)
			if err != nil {
				return 0, apperror.Internal(err)
			}
			grand = map[int64]int64{}
			for _, r := range rows {
				ids = append(ids, r.ID)
				grand[r.ID] = r.GrandTotal
			}
		}
		if len(ids) == 0 {
			return total, nil
		}
		paid, err := s.payments.PaidForOrders(ctx, orderType, ids)
		if err != nil {
			return 0, apperror.Internal(err)
		}
		for _, id := range ids {
			if owed := grand[id] - paid[id]; owed > 0 {
				total += owed
			}
		}
		after = ids[len(ids)-1]
		if len(ids) < 1000 {
			return total, nil
		}
	}
}
