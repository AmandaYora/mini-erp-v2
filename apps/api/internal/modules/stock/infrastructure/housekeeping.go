package infrastructure

import (
	"context"
	"time"
)

// PurgeReservations deletes up to limit finished holds (expired, consumed,
// released) whose expiry passed before cutoff (F4-10). Active holds are never
// touched. Idempotent; served by ix_stock_reservations_exp.
func (r *Repository) PurgeReservations(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM stock_reservations WHERE status <> 'active' AND expires_at < ? LIMIT ?`,
		cutoff.UTC(), limit)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
