package application

import (
	"context"

	"mini-erp/internal/modules/purchasing/contracts"
	"mini-erp/internal/shared/apperror"
)

// StatusCounts returns one branch's order counts keyed by derived status.
//
// Query count: 1 grouped repo read for all statuses. The four fixed statuses
// are zero-filled so readers never nil-check; unknown values pass through
// instead of failing — a report must not break on data it does not know.
func (s *Service) StatusCounts(ctx context.Context, branchID int64) (map[string]int64, error) {
	counts, err := s.repo.StatusCounts(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := map[string]int64{
		contracts.StatusDraft:     0,
		contracts.StatusConfirmed: 0,
		contracts.StatusCompleted: 0,
		contracts.StatusCancelled: 0,
	}
	for status, n := range counts {
		out[status] = n
	}
	return out, nil
}

// CountMissingSupplierInvoice counts taxed orders past draft without a
// supplier invoice number — one COUNT query for the readiness gate.
func (s *Service) CountMissingSupplierInvoice(ctx context.Context, branchID int64) (int64, error) {
	n, err := s.repo.CountMissingSupplierInvoice(ctx, branchID)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return n, nil
}

// CountMissingSupplierInvoiceBetween counts them within an order-date range.
func (s *Service) CountMissingSupplierInvoiceBetween(ctx context.Context, branchID int64, from, to string) (int64, error) {
	n, err := s.repo.CountMissingSupplierInvoiceBetween(ctx, branchID, from, to)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return n, nil
}

// OrderParties maps order ids to party ids in one grouped read (A3).
func (s *Service) OrderParties(ctx context.Context, ids []int64) (map[int64]int64, error) {
	parties, err := s.repo.OrderParties(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return parties, nil
}

// CountOrders implements the contract (see there).
func (s *Service) CountOrders(ctx context.Context, branchID int64, from, to string) (int64, error) {
	n, err := s.repo.CountOrders(ctx, branchID, from, to)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return n, nil
}

// OpenOrderTotals implements the contract (see there).
func (s *Service) OpenOrderTotals(ctx context.Context, branchID, afterID int64, limit int) ([]contracts.OrderTotal, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	out, err := s.repo.OpenOrderTotals(ctx, branchID, afterID, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}

// ReportOrders implements the contract (see there).
func (s *Service) ReportOrders(ctx context.Context, branchID int64, status, from, to string, page, limit int) (*contracts.OrderReport, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, total, sum, err := s.repo.ReportOrders(ctx, branchID, status, from, to, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.PartyID)
	}
	names, err := s.parties.NamesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		r.PartyName = names[r.PartyID]
	}
	if rows == nil {
		rows = []*contracts.OrderSummary{}
	}
	return &contracts.OrderReport{Rows: rows, Total: total, GrandTotal: sum}, nil
}
