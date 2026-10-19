package infrastructure

import (
	"context"
	"time"

	"mini-erp/internal/modules/assistant/infrastructure/sqlc"
)

// PurgeToolExecutions deletes up to limit bot tool-execution rows created
// before cutoff (F4-10).
func (r *Repository) PurgeToolExecutions(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	res, err := r.q.PurgeToolExecutions(ctx, sqlc.PurgeToolExecutionsParams{Cutoff: cutoff.UTC(), Limit: int32(limit)})
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PurgeMessages deletes up to limit bot messages created before cutoff.
func (r *Repository) PurgeMessages(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	res, err := r.q.PurgeMessages(ctx, sqlc.PurgeMessagesParams{Cutoff: cutoff.UTC(), Limit: int32(limit)})
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PurgeRuns deletes up to limit bot runs created before cutoff.
func (r *Repository) PurgeRuns(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	res, err := r.q.PurgeRuns(ctx, sqlc.PurgeRunsParams{Cutoff: cutoff.UTC(), Limit: int32(limit)})
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PurgeIdleThreads deletes up to limit threads with no activity since cutoff
// and no remaining messages.
func (r *Repository) PurgeIdleThreads(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	res, err := r.q.PurgeIdleThreads(ctx, sqlc.PurgeIdleThreadsParams{Cutoff: cutoff.UTC(), Limit: int32(limit)})
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
