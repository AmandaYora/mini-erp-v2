package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"time"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/dashboard/domain"
	"mini-erp/internal/modules/dashboard/infrastructure/sqlc"
)

// Repository owns dashboard_alert_snoozes, the dashboard's only table
// (ADR-0025). Alerts themselves are computed, never stored.
type Repository struct {
	db *sql.DB
	q  *sqlc.Queries
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: sqlc.New(db)}
}

// TxHook runs inside a snooze write's transaction (the audit row, ADR-0011)
// with the row written (or removed) and, for an upsert, the row it replaced
// (nil when new); an error rolls the write back.
type TxHook func(ctx context.Context, tx auditcontracts.AuditTx, snooze, previous *domain.Snooze) error

// ListSnoozes returns every snooze row of a branch, expired ones included.
func (r *Repository) ListSnoozes(ctx context.Context, branchID int64) ([]domain.Snooze, error) {
	rows, err := r.q.ListSnoozes(ctx, branchID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Snooze, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSnooze(row))
	}
	return out, nil
}

// GetSnooze returns nil, nil when the alert has no snooze row in the branch.
func (r *Repository) GetSnooze(ctx context.Context, branchID int64, alertID string) (*domain.Snooze, error) {
	return getSnooze(ctx, r.q, branchID, alertID)
}

func getSnooze(ctx context.Context, q *sqlc.Queries, branchID int64, alertID string) (*domain.Snooze, error) {
	row, err := q.GetSnooze(ctx, sqlc.GetSnoozeParams{BranchID: branchID, AlertID: alertID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s := toSnooze(row)
	return &s, nil
}

// UpsertSnooze writes (or replaces) the branch's snooze of one alert and runs
// hook with the stored row in the same transaction. previous is the row it
// replaced (nil when new).
func (r *Repository) UpsertSnooze(ctx context.Context, s domain.Snooze, actorID int64, now time.Time, hook TxHook) (stored, previous *domain.Snooze, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.q.WithTx(tx)
	if previous, err = getSnooze(ctx, q, s.BranchID, s.AlertID); err != nil {
		return nil, nil, err
	}
	amount := sql.NullInt64{}
	if s.AmountAt != nil {
		amount = sql.NullInt64{Int64: *s.AmountAt, Valid: true}
	}
	if err := q.UpsertSnooze(ctx, sqlc.UpsertSnoozeParams{
		BranchID: s.BranchID, AlertID: s.AlertID, Reason: s.Reason, DurationDays: uint8(s.Days),
		SnoozedUntil: s.Until, CountAtSnooze: s.CountAt, AmountAtSnooze: amount,
		Now: now, ActorID: sql.NullInt64{Int64: actorID, Valid: true},
	}); err != nil {
		return nil, nil, err
	}
	if stored, err = getSnooze(ctx, q, s.BranchID, s.AlertID); err != nil {
		return nil, nil, err
	}
	if stored == nil {
		return nil, nil, errors.New("snooze row missing after upsert")
	}
	if hook != nil {
		if err := hook(ctx, tx, stored, previous); err != nil {
			return nil, nil, err
		}
	}
	return stored, previous, tx.Commit()
}

// DeleteSnooze removes the branch's snooze of one alert and runs hook with the
// removed row in the same transaction. It returns nil, nil when there was none.
func (r *Repository) DeleteSnooze(ctx context.Context, branchID int64, alertID string, hook TxHook) (*domain.Snooze, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.q.WithTx(tx)
	old, err := getSnooze(ctx, q, branchID, alertID)
	if err != nil || old == nil {
		return nil, err
	}
	if err := q.DeleteSnooze(ctx, old.ID); err != nil {
		return nil, err
	}
	if hook != nil {
		if err := hook(ctx, tx, old, nil); err != nil {
			return nil, err
		}
	}
	return old, tx.Commit()
}

func toSnooze(row sqlc.DashboardAlertSnooze) domain.Snooze {
	s := domain.Snooze{
		ID: row.ID, BranchID: row.BranchID, AlertID: row.AlertID, Reason: row.Reason,
		Days: int(row.DurationDays), Until: row.SnoozedUntil.UTC(), CountAt: row.CountAtSnooze,
		CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(), UpdatedBy: row.UpdatedBy.Int64,
	}
	if row.AmountAtSnooze.Valid {
		v := row.AmountAtSnooze.Int64
		s.AmountAt = &v
	}
	return s
}
