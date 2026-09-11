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

// OrderParties maps order ids to party ids in one grouped read (A3).
func (s *Service) OrderParties(ctx context.Context, ids []int64) (map[int64]int64, error) {
	parties, err := s.repo.OrderParties(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return parties, nil
}
