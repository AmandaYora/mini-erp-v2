package application

import (
	"context"

	"mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/shared/apperror"
)

// UnshippedOrderTotals pages the draft/confirmed orders — money may already be
// received on them while goods are still due (data-health A12, ADR-0025).
func (s *Service) UnshippedOrderTotals(ctx context.Context, branchID, afterID int64, limit int) ([]contracts.OrderTotal, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	out, err := s.repo.UnshippedOrderTotals(ctx, branchID, afterID, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}
