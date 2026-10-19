package application

import (
	"context"
	"time"

	"mini-erp/internal/shared/apperror"
)

// PurgeSessions trims revoked/expired session rows older than cutoff
// (F4-10 retention); returns how many were deleted (bounded by limit).
func (s *Service) PurgeSessions(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	n, err := s.repo.PurgeSessions(ctx, cutoff, limit)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return n, nil
}
