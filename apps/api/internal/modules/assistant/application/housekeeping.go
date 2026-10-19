package application

import (
	"context"
	"time"

	"mini-erp/internal/shared/apperror"
)

func wrapPurge(n int64, err error) (int64, error) {
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return n, nil
}

// PurgeToolExecutions, PurgeMessages, PurgeRuns and PurgeIdleThreads trim the
// bot's history (F4-10 retention). Each is bounded by limit and idempotent.
func (s *Service) PurgeToolExecutions(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	return wrapPurge(s.repo.PurgeToolExecutions(ctx, cutoff, limit))
}

func (s *Service) PurgeMessages(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	return wrapPurge(s.repo.PurgeMessages(ctx, cutoff, limit))
}

func (s *Service) PurgeRuns(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	return wrapPurge(s.repo.PurgeRuns(ctx, cutoff, limit))
}

func (s *Service) PurgeIdleThreads(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	return wrapPurge(s.repo.PurgeIdleThreads(ctx, cutoff, limit))
}
