package application

import (
	"context"
	"sort"

	dashboardcontracts "mini-erp/internal/modules/dashboard/contracts"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
)

// Operational widget bounds. All are display caps, constant and small, so
// downstream per-row work (names, lookups) stays bounded by a constant.
const (
	// PriorityWindowDays bounds F3: confirmed SO due within the next 7 days
	// or already past due. Anything further out is planning, not priority.
	PriorityWindowDays = 7
	// PriorityFetch caps the confirmed-order scan F3 filters in memory.
	// ListOrders itself caps at 50; asking for more would silently truncate.
	PriorityFetch = 50

	defaultWidgetLimit = 5
	maxWidgetLimit     = 20
	maxCriticalLimit   = 50
)

func clampWidgetLimit(limit int) int {
	if limit <= 0 {
		return defaultWidgetLimit
	}
	if limit > maxWidgetLimit {
		return maxWidgetLimit
	}
	return limit
}

// monthRange returns (monthStart, today) in Jakarta as YYYY-MM-DD — the range
// every "this month" widget (F5, F6) reads.
func monthRange() (monthStart, today string) {
	now := timeutil.NowUTC().In(timeutil.Jakarta)
	today = now.Format("2006-01-02")
	return now.Format("2006-01") + "-01", today
}

// CriticalStock returns tracked products at/below minimum, scarcest first.
//
// Query count: 1 stock contract call = 1 GROUP BY availability read + 1
// stocked-product read inside stock (its documented shape). The dashboard
// adds no per-product read of its own — 1.000 critical products still cost a
// single grouped pass (§7 test).
func (s *Service) CriticalStock(ctx context.Context, branchID int64, limit int) ([]*stockcontracts.CriticalItem, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > maxCriticalLimit {
		limit = maxCriticalLimit
	}
	items, err := s.stock.CriticalStock(ctx, branchID, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if items == nil {
		items = []*stockcontracts.CriticalItem{}
	}
	return items, nil
}

// PriorityOrders returns confirmed sales orders near/past due, most overdue
// first.
//
// Query count: 1 sales ListOrders call (one grouped query; party names ride
// the summaries via the sales module's own resolution, bounded by the
// constant PriorityFetch) plus in-memory due filtering and sorting here.
func (s *Service) PriorityOrders(ctx context.Context, branchID int64, limit int) ([]*salescontracts.OrderSummary, error) {
	limit = clampWidgetLimit(limit)
	rows, err := s.sales.ListOrders(ctx, branchID,
		[]string{salescontracts.StatusConfirmed}, "", "", PriorityFetch)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	horizon := timeutil.NowUTC().In(timeutil.Jakarta).AddDate(0, 0, PriorityWindowDays).Format("2006-01-02")
	out := make([]*salescontracts.OrderSummary, 0, len(rows))
	for _, r := range rows {
		if len(r.DueDate) < 10 {
			continue // COD / no due date: not schedulable, not priority.
		}
		if due := r.DueDate[:10]; due <= horizon {
			out = append(out, r)
		}
	}
	// Most overdue first; ties break by order number for a stable widget.
	sort.Slice(out, func(i, j int) bool {
		di, dj := out[i].DueDate[:10], out[j].DueDate[:10]
		if di == dj {
			return out[i].Number < out[j].Number
		}
		return di < dj
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// OrderStatus returns sales + purchasing counts per derived status.
//
// Query count: 2 — one GROUP BY status read per order flow. The statuses are
// the fixed in-code state machine read straight off the status column; no
// status-config tables exist by design (D4).
func (s *Service) OrderStatus(ctx context.Context, branchID int64) (*dashboardcontracts.OrderStatus, error) {
	sales, err := s.sales.StatusCounts(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	purchasing, err := s.purchasing.StatusCounts(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &dashboardcontracts.OrderStatus{Sales: sales, Purchasing: purchasing}, nil
}

// TopSellers returns this month's best sellers by base qty.
//
// Query count: 1 grouped sales read for the whole ranking.
func (s *Service) TopSellers(ctx context.Context, branchID int64, limit int) ([]*salescontracts.TopProduct, error) {
	monthStart, today := monthRange()
	rows, err := s.sales.TopProducts(ctx, branchID, monthStart, today, clampWidgetLimit(limit))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if rows == nil {
		rows = []*salescontracts.TopProduct{}
	}
	return rows, nil
}

// TopMargin returns this month's highest-margin products, from the books
// (Tahap-1 MarginByProduct: revenue nets returns, COGS nets restocks).
//
// Query count: 1 finance call = 1 grouped product-footing read + 3 account
// resolutions + one product lookup per ranked row (bounded by the constant
// limit, never per journal line).
func (s *Service) TopMargin(ctx context.Context, branchID int64, limit int) ([]financecontracts.ProductMargin, error) {
	monthStart, today := monthRange()
	rows, err := s.finance.MarginByProduct(ctx, branchID, monthStart, today, clampWidgetLimit(limit))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if rows == nil {
		rows = []financecontracts.ProductMargin{}
	}
	return rows, nil
}
