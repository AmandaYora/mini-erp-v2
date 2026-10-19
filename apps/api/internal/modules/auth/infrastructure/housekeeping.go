package infrastructure

import (
	"context"
	"database/sql"
	"time"

	"mini-erp/internal/modules/auth/infrastructure/sqlc"
)

// PurgeSessions deletes up to limit sessions that were revoked before
// revokedCutoff or whose absolute lifetime ended before expiredCutoff
// (F4-10). Live sessions are never touched.
func (r *Repository) PurgeSessions(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	res, err := r.q.PurgeUserSessions(ctx, sqlc.PurgeUserSessionsParams{
		RevokedCutoff: sql.NullTime{Time: cutoff.UTC(), Valid: true}, ExpiredCutoff: cutoff.UTC(), Limit: int32(limit),
	})
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
