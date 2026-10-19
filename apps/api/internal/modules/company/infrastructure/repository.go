package infrastructure

import (
	"context"
	"database/sql"
	"errors"

	"mini-erp/internal/modules/company/contracts"
	"mini-erp/internal/modules/company/infrastructure/sqlc"
	"mini-erp/internal/shared/timeutil"
)

// Repository owns the company module tables (both singletons).
type Repository struct {
	db *sql.DB
	q  *sqlc.Queries
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: sqlc.New(db)}
}

// GetProfile returns the singleton profile, or nil when never saved.
func (r *Repository) GetProfile(ctx context.Context) (*contracts.Profile, error) {
	row, err := r.q.GetCompanyProfile(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toProfile(row), nil
}

// SaveProfile inserts the first profile or updates the singleton row.
// The optional hook runs in the same transaction so the audit row co-commits
// with the write (ADR-0011).
func (r *Repository) SaveProfile(ctx context.Context, p *contracts.Profile, actorID int64, hook func(ctx context.Context, tx *sql.Tx, id int64) error) (*contracts.Profile, error) {
	existing, err := r.GetProfile(ctx)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	tq := r.q.WithTx(tx)
	if existing == nil {
		res, err := tq.InsertCompanyProfile(ctx, insertParams(p, actorID))
		if err != nil {
			return nil, err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return nil, err
		}
		p.ID = id
		if hook != nil {
			if err := hook(ctx, tx, id); err != nil {
				return nil, err
			}
		}
		return p, tx.Commit()
	}
	if err := tq.UpdateCompanyProfile(ctx, updateParams(p, actorID, existing.ID, timeutil.NowUTC())); err != nil {
		return nil, err
	}
	p.ID = existing.ID
	if hook != nil {
		if err := hook(ctx, tx, existing.ID); err != nil {
			return nil, err
		}
	}
	return p, tx.Commit()
}

// GetSettings returns all settings as key → raw JSON.
func (r *Repository) GetSettings(ctx context.Context) (map[string]string, error) {
	rows, err := r.q.ListCompanySettings(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, row := range rows {
		out[row.SettingKey] = row.SettingValue.String
	}
	return out, nil
}

// SaveSettings merges the given keys (upsert per key). Keys NOT provided are
// left untouched — a single-setting call must never wipe its siblings (KI-30).
func (r *Repository) SaveSettings(ctx context.Context, settings map[string]string, actorID int64, hook func(ctx context.Context, tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	tq := r.q.WithTx(tx)
	now := timeutil.NowUTC()
	for key, value := range settings {
		if err := tq.UpsertCompanySetting(ctx, settingParams(key, value, actorID, now)); err != nil {
			return err
		}
	}
	if hook != nil {
		if err := hook(ctx, tx); err != nil {
			return err
		}
	}
	return tx.Commit()
}
