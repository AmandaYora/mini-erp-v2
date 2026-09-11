package application

import (
	"context"
	"errors"

	dashboardcontracts "mini-erp/internal/modules/dashboard/contracts"
	financecontracts "mini-erp/internal/modules/finance/contracts"
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
}

// NewService wires dashboard reads over module contracts (Rule A:
// contracts-only, never application/infrastructure of other modules).
func NewService(finance financecontracts.FinanceClient, stock stockcontracts.StockClient, sales Sales, purchasing Purchasing) *Service {
	return &Service{finance: finance, stock: stock, sales: sales, purchasing: purchasing}
}

// Summary returns today + month-to-date P&L plus key book balances.
func (s *Service) Summary(ctx context.Context, branchID int64) (*dashboardcontracts.Summary, error) {
	today := timeutil.NowUTC().In(timeutil.Jakarta)
	day := today.Format("2006-01-02")
	monthStart := today.Format("2006-01") + "-01"

	rev, exp, profit, err := s.finance.NetProfit(ctx, branchID, day, day)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	mRev, mExp, mProfit, err := s.finance.NetProfit(ctx, branchID, monthStart, day)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	// bal resolves one key balance. A missing account reads as zero (chart
	// not seeded for the key); any other failure aborts — silently zeroing
	// real outages would present a healthy-looking dashboard.
	bal := func(code string) (int64, error) {
		debit, credit, err := s.finance.AccountBalance(ctx, branchID, code)
		if err != nil {
			var appErr *apperror.AppError
			if errors.As(err, &appErr) && appErr.Code == apperror.CodeNotFound {
				return 0, nil
			}
			return 0, apperror.Internal(err)
		}
		return debit - credit, nil
	}
	cash, err := bal(financecontracts.CodeCash)
	if err != nil {
		return nil, err
	}
	receivable, err := bal(financecontracts.CodeReceivable)
	if err != nil {
		return nil, err
	}
	payable, err := bal(financecontracts.CodePayable)
	if err != nil {
		return nil, err
	}
	inventory, err := bal(financecontracts.CodeInventory)
	if err != nil {
		return nil, err
	}
	_, invTotal, err := s.finance.InventoryValue(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &dashboardcontracts.Summary{
		BranchID:    branchID,
		Today:       dashboardcontracts.Period{From: day, To: day, Revenue: rev, Expense: exp, Profit: profit},
		MonthToDate: dashboardcontracts.Period{From: monthStart, To: day, Revenue: mRev, Expense: mExp, Profit: mProfit},
		Balances: dashboardcontracts.Balances{
			Cash:       cash,
			Receivable: receivable,
			Payable:    -payable,
			Inventory:  inventory,
		},
		InventoryValue: invTotal,
	}, nil
}
