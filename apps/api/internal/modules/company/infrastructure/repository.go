package infrastructure

import (
	"context"
	"database/sql"

	"mini-erp/internal/modules/company/contracts"
)

// Repository owns the company module tables (both singletons).
type Repository struct {
	db *sql.DB
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// GetProfile returns the singleton profile, or nil when never saved.
func (r *Repository) GetProfile(ctx context.Context) (*contracts.Profile, error) {
	p := &contracts.Profile{}
	var legal, address, city, phone, email, taxID sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, legal_name, address, city, phone, email, tax_id
		 FROM company_profile ORDER BY id ASC LIMIT 1`).
		Scan(&p.ID, &p.Name, &legal, &address, &city, &phone, &email, &taxID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.LegalName, p.Address, p.City = legal.String, address.String, city.String
	p.Phone, p.Email, p.TaxID = phone.String, email.String, taxID.String
	return p, nil
}

// SaveProfile inserts the first profile or updates the singleton row.
func (r *Repository) SaveProfile(ctx context.Context, p *contracts.Profile, actorID int64) (*contracts.Profile, error) {
	existing, err := r.GetProfile(ctx)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		res, err := r.db.ExecContext(ctx,
			`INSERT INTO company_profile
			 (name, legal_name, address, city, phone, email, tax_id, created_by, updated_by)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			p.Name, nullIfEmpty(p.LegalName), nullIfEmpty(p.Address), nullIfEmpty(p.City),
			nullIfEmpty(p.Phone), nullIfEmpty(p.Email), nullIfEmpty(p.TaxID), actorID, actorID)
		if err != nil {
			return nil, err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return nil, err
		}
		p.ID = id
		return p, nil
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE company_profile SET name = ?, legal_name = ?, address = ?, city = ?,
		 phone = ?, email = ?, tax_id = ?, updated_at = UTC_TIMESTAMP(6), updated_by = ?
		 WHERE id = ?`,
		p.Name, nullIfEmpty(p.LegalName), nullIfEmpty(p.Address), nullIfEmpty(p.City),
		nullIfEmpty(p.Phone), nullIfEmpty(p.Email), nullIfEmpty(p.TaxID), actorID, existing.ID)
	if err != nil {
		return nil, err
	}
	p.ID = existing.ID
	return p, nil
}

// GetSettings returns all settings as key → raw JSON.
func (r *Repository) GetSettings(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT setting_key, setting_value FROM company_settings")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var key string
		var value sql.NullString
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		out[key] = value.String
	}
	return out, rows.Err()
}

// SaveSettings merges the given keys (upsert per key). Keys NOT provided are
// left untouched — a single-setting call must never wipe its siblings (KI-30).
func (r *Repository) SaveSettings(ctx context.Context, settings map[string]string, actorID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for key, value := range settings {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO company_settings (setting_key, setting_value, updated_by)
			 VALUES (?, ?, ?)
			 ON DUPLICATE KEY UPDATE setting_value = VALUES(setting_value),
			 updated_at = UTC_TIMESTAMP(6), updated_by = VALUES(updated_by)`,
			key, nullIfEmpty(value), actorID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
