package application

import (
	"context"
	"sort"
	"time"

	dashboardcontracts "mini-erp/internal/modules/dashboard/contracts"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
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

// PriorityOrders returns the orders that need action (DB-01), most urgent
// first. Sales: the live (draft/confirmed) orders ranked in SQL by the legacy
// score (past due 100, due within 2 days 80, pending 60, otherwise 40, plus the
// order's age, max 30). Purchases: confirmed/completed orders past their due
// date that are still unpaid (100) and drafts waiting for a decision (60), with
// the same age bonus. Every item names its reason and age in days.
//
// Query count: sales 1 ranked read + 1 party-name batch read; purchases 1
// summary page for drafts, one cursor page per 1.000 open orders plus one
// grouped payment read per page, and 1 summaries read for the overdue names.
func (s *Service) PriorityOrders(ctx context.Context, branchID int64, limit int) ([]*dashboardcontracts.PriorityItem, error) {
	limit = clampWidgetLimit(limit)
	today := timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	rows, err := s.sales.PriorityOrders(ctx, branchID, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]*dashboardcontracts.PriorityItem, 0, len(rows)+limit)
	for _, r := range rows {
		reason := salesReason(r, today)
		out = append(out, &dashboardcontracts.PriorityItem{ID: r.ID, Kind: "sales", Number: r.Number, PartyID: r.PartyID,
			PartyName: r.PartyName, GrandTotal: r.GrandTotal, Status: r.Status, OrderDate: r.OrderDate, DueDate: r.DueDate,
			Score: r.Score, Reason: reason, AgeDays: ageDays(r.OrderDate, today)})
	}
	pos, err := s.purchasePriority(ctx, branchID, today, limit)
	if err != nil {
		return nil, err
	}
	out = append(out, pos...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].DueDate != out[j].DueDate {
			return out[i].DueDate < out[j].DueDate
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func salesReason(r *salescontracts.PriorityOrder, today string) string {
	switch {
	case r.DueDate != "" && r.DueDate[:10] < today:
		return dashboardcontracts.ReasonOverdue
	case r.Status == "draft":
		return dashboardcontracts.ReasonAwaitingDecision
	case r.DueDate != "" && r.Score >= 80:
		return dashboardcontracts.ReasonDueSoon
	default:
		return dashboardcontracts.ReasonOpen
	}
}

// ageDays is the whole days between an order date and today (0 when unknown).
func ageDays(orderDate, today string) int {
	if len(orderDate) < 10 {
		return 0
	}
	from, err1 := time.Parse("2006-01-02", orderDate[:10])
	to, err2 := time.Parse("2006-01-02", today)
	if err1 != nil || err2 != nil || to.Before(from) {
		return 0
	}
	return int(to.Sub(from).Hours() / 24)
}

func ageBonus(age int) int {
	if age > 30 {
		return 30
	}
	return age
}

// purchasePriority lists the purchase orders that need action: overdue and
// still unpaid, and drafts awaiting a decision.
func (s *Service) purchasePriority(ctx context.Context, branchID int64, today string, limit int) ([]*dashboardcontracts.PriorityItem, error) {
	var out []*dashboardcontracts.PriorityItem
	// Drafts: waiting for a decision.
	drafts, err := s.purchasing.ReportOrders(ctx, branchID, "draft", "", "", 1, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if drafts != nil {
		for _, d := range drafts.Rows {
			age := ageDays(d.OrderDate, today)
			out = append(out, &dashboardcontracts.PriorityItem{ID: d.ID, Kind: "purchase", Number: d.Number, PartyID: d.PartyID,
				PartyName: d.PartyName, GrandTotal: d.GrandTotal, Status: d.Status, OrderDate: d.OrderDate, DueDate: d.DueDate,
				Score: 60 + ageBonus(age), Reason: dashboardcontracts.ReasonAwaitingDecision, AgeDays: age})
		}
	}
	if s.payments == nil {
		return out, nil
	}
	// Overdue and unpaid: scan the open orders (payable base) page by page.
	type overdue struct {
		row  purchasingcontracts.OrderTotal
		owed int64
	}
	var late []overdue
	var after int64
	for {
		rows, err := s.purchasing.OpenOrderTotals(ctx, branchID, after, 1000)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if len(rows) == 0 {
			break
		}
		var ids []int64
		for _, r := range rows {
			if r.DueDate != "" && r.DueDate[:10] < today {
				ids = append(ids, r.ID)
			}
		}
		if len(ids) > 0 {
			paid, err := s.payments.PaidForOrders(ctx, paymentcontracts.OrderPurchase, ids)
			if err != nil {
				return nil, apperror.Internal(err)
			}
			for _, r := range rows {
				if r.DueDate == "" || r.DueDate[:10] >= today {
					continue
				}
				if owed := r.GrandTotal - paid[r.ID]; owed > 0 {
					late = append(late, overdue{row: r, owed: owed})
				}
			}
		}
		after = rows[len(rows)-1].ID
		if len(rows) < 1000 {
			break
		}
	}
	sort.SliceStable(late, func(i, j int) bool { return late[i].row.DueDate < late[j].row.DueDate })
	if len(late) > limit {
		late = late[:limit]
	}
	if len(late) == 0 {
		return out, nil
	}
	ids := make([]int64, 0, len(late))
	for _, l := range late {
		ids = append(ids, l.row.ID)
	}
	names, err := s.purchasing.SummariesByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for _, l := range late {
		age := ageDays(l.row.OrderDate, today)
		item := &dashboardcontracts.PriorityItem{ID: l.row.ID, Kind: "purchase", Number: l.row.Number, PartyID: l.row.PartyID,
			GrandTotal: l.row.GrandTotal, Status: "confirmed", OrderDate: l.row.OrderDate, DueDate: l.row.DueDate,
			Score: 100 + ageBonus(age), Reason: dashboardcontracts.ReasonOverdue, AgeDays: age}
		if sum := names[l.row.ID]; sum != nil {
			item.PartyName, item.Status = sum.PartyName, sum.Status
		}
		out = append(out, item)
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

// TopSellers returns this month's best sellers: orders COMPLETED this month,
// net of the quantities returned (confirmed returns) in the same month.
//
// Query count: 1 grouped sales read + 1 grouped return read.
func (s *Service) TopSellers(ctx context.Context, branchID int64, limit int) ([]*salescontracts.TopProduct, error) {
	monthStart, today := monthRange()
	limit = clampWidgetLimit(limit)
	rows, err := s.sales.TopProducts(ctx, branchID, monthStart, today, 200)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if rows == nil {
		rows = []*salescontracts.TopProduct{}
	}
	if s.returns != nil {
		returned, err := s.returns.ReturnedByProduct(ctx, branchID, monthStart, today)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		kept := rows[:0]
		for _, r := range rows {
			r.QtyBase -= returned[r.ProductID]
			if r.QtyBase > 0 {
				kept = append(kept, r)
			}
		}
		rows = kept
		sort.Slice(rows, func(i, j int) bool { return rows[i].QtyBase > rows[j].QtyBase })
	}
	if len(rows) > limit {
		rows = rows[:limit]
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
