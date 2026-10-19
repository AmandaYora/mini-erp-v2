package application

import (
	"context"
	"time"

	"mini-erp/internal/shared/apperror"
)

// PurgeReservations trims finished reservation rows older than cutoff
// (F4-10 retention); returns how many were deleted (bounded by limit).
func (s *Service) PurgeReservations(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	n, err := s.repo.PurgeReservations(ctx, cutoff, limit)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return n, nil
}
