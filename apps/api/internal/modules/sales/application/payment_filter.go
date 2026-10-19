package application

import (
	"context"
	"sort"
	"strings"

	"mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/listquery"
	"mini-erp/internal/shared/timeutil"
)

// Payment-status filter of the order list/export (SL-02). What is paid lives in
// the payment module, so no SQL join can express "unpaid/partial/paid": the SQL
// narrows the candidates (live statuses, and for "overdue" the net orders past
// due), then the service reads the paid amounts per page of candidates through
// the payment contract and keeps the matching ones. Drafts and cancelled orders
// never match (nothing is owed on them).

// maxPaymentScan caps how many candidate orders one filtered request walks.
// Past it the caller is asked to narrow the date range instead of paying an
// unbounded scan.
const maxPaymentScan = 30000

// preparePaymentFilter validates the filter value and turns it into the SQL
// pre-filter. Requested statuses are intersected with the live ones.
func (s *Service) preparePaymentFilter(f *ListFilter) error {
	if err := listquery.ValidateEnum("paymentStatus", f.PaymentStatus, PayUnpaid, PayPartial, PayPaid, PayOverdue); err != nil {
		return err
	}
	f.OverdueBefore = ""
	if f.PaymentStatus == "" {
		return nil
	}
	live := []string{contracts.StatusConfirmed, contracts.StatusCompleted}
	if len(f.Statuses) > 0 {
		var kept []string
		for _, st := range f.Statuses {
			if st == contracts.StatusConfirmed || st == contracts.StatusCompleted {
				kept = append(kept, st)
			}
		}
		if len(kept) == 0 {
			// Only draft/cancelled were asked for: nothing can match.
			kept = []string{"none"}
		}
		live = kept
	}
	f.Statuses = live
	if f.PaymentStatus == PayOverdue {
		f.OverdueBefore = timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	}
	return nil
}

// matchesPayment reports whether a summary satisfies the filter value.
func matchesPayment(sum PaymentSummary, want string) bool {
	switch want {
	case PayOverdue:
		return sum.IsOverdue
	case "":
		return true
	default:
		return sum.PaymentStatus == want
	}
}

// filterByPayment keeps the orders of a batch that match the payment filter.
func (s *Service) filterByPayment(ctx context.Context, batch []*contracts.SalesOrder, want string) ([]*contracts.SalesOrder, map[int64]PaymentSummary, error) {
	sums, err := s.PaymentSummaries(ctx, batch)
	if err != nil {
		return nil, nil, err
	}
	var kept []*contracts.SalesOrder
	for _, o := range batch {
		if matchesPayment(sums[o.ID], want) {
			kept = append(kept, o)
		}
	}
	return kept, sums, nil
}

// listByPaymentStatus is List for a payment-status filter: walks the candidates
// by keyset (the export's pager), keeps the matches, sorts them the way the list
// asks and cuts the page. Total is the exact match count.
func (s *Service) listByPaymentStatus(ctx context.Context, branchID int64, f ListFilter, page, limit int) (*ListResult, error) {
	var matched []*contracts.SalesOrder
	var after int64
	scanned := 0
	for {
		batch, err := s.repo.ListAfter(ctx, branchID, f, after, exportPageSize)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if len(batch) == 0 {
			break
		}
		after = batch[len(batch)-1].ID
		scanned += len(batch)
		kept, _, err := s.filterByPayment(ctx, batch, f.PaymentStatus)
		if err != nil {
			return nil, err
		}
		matched = append(matched, kept...)
		if len(batch) < exportPageSize {
			break
		}
		if scanned >= maxPaymentScan {
			return nil, apperror.Validation("Terlalu banyak order untuk filter status bayar. Persempit rentang tanggal.",
				[]apperror.FieldError{{Field: "paymentStatus", Message: "persempit rentang tanggal"}})
		}
	}
	sortOrders(matched, f.OrderBy)
	total := int64(len(matched))
	start := (page - 1) * limit
	if start > len(matched) {
		start = len(matched)
	}
	end := start + limit
	if end > len(matched) {
		end = len(matched)
	}
	pageRows := matched[start:end]
	if pageRows == nil {
		pageRows = []*contracts.SalesOrder{}
	}
	if err := s.decorateParties(ctx, pageRows); err != nil {
		return nil, err
	}
	return &ListResult{Orders: pageRows, Total: total}, nil
}

// sortOrders applies a whitelisted "expr ASC|DESC" (list_raw orderBy columns)
// in memory; ties fall back to id DESC like the SQL does.
func sortOrders(orders []*contracts.SalesOrder, orderBy string) {
	col, desc := "id", true
	if fs := strings.Fields(orderBy); len(fs) >= 1 {
		col = strings.ToLower(strings.TrimSuffix(fs[0], ","))
		desc = len(fs) < 2 || !strings.EqualFold(strings.TrimSuffix(fs[1], ","), "ASC")
	}
	less := func(a, b *contracts.SalesOrder) int {
		switch col {
		case "order_date":
			return strings.Compare(a.OrderDate, b.OrderDate)
		case "number":
			return strings.Compare(a.Number, b.Number)
		case "grand_total":
			switch {
			case a.GrandTotal < b.GrandTotal:
				return -1
			case a.GrandTotal > b.GrandTotal:
				return 1
			}
			return 0
		case "status":
			return strings.Compare(a.Status, b.Status)
		}
		return 0
	}
	sort.SliceStable(orders, func(i, j int) bool {
		c := less(orders[i], orders[j])
		if c != 0 {
			if desc {
				return c > 0
			}
			return c < 0
		}
		return orders[i].ID > orders[j].ID
	})
}

// DecorateList fills the readable user names of a list page in one batch read.
func (s *Service) DecorateList(ctx context.Context, orders []*contracts.SalesOrder) error {
	return s.decorateUsers(ctx, orders)
}
