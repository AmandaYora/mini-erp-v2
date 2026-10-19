package application

import (
	"context"
	"math"

	"mini-erp/internal/shared/apperror"
)

// OpeningPosted implements the stock-facing guard: true once the branch's
// cutover balance journal exists.
func (s *Service) OpeningPosted(ctx context.Context, branchID int64) (bool, error) {
	e, err := s.OpeningStatus(ctx, branchID)
	if err != nil {
		return false, err
	}
	return e != nil, nil
}

// AverageUnitCost returns the position's moving-average cost in whole rupiah
// (0 when there is no quantity or no ledger yet). The cost ledger is drained
// first (N-42): a receipt posted a moment ago must already count, otherwise
// callers such as the replacement-goods cost guard would see a stale zero and
// refuse goods that do have a purchase history. The drain is one cursor read
// when nothing is new.
func (s *Service) AverageUnitCost(ctx context.Context, branchID, productID, variantID int64) (int64, error) {
	if err := s.drainCosting(ctx, branchID); err != nil {
		return 0, err
	}
	p, err := s.repo.CostPositionOf(ctx, branchID, productID, variantID)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return int64(math.Round(p.Average())), nil
}
