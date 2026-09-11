package infrastructure

import (
	"context"
	"database/sql"

	"mini-erp/internal/modules/finance/contracts"
)

// Repository owns the finance module tables.
type Repository struct {
	db *sql.DB
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// --- accounts ---------------------------------------------------------------

func scanAccount(row *sql.Row, a *contracts.Account) error {
	var cash int
	err := row.Scan(&a.ID, &a.Code, &a.Name, &a.Type, &cash, &a.Status)
	a.IsCash = cash == 1
	return err
}

const accountColumns = "id, code, name, type, is_cash, status"

// AccountByID returns nil, nil when missing.
func (r *Repository) AccountByID(ctx context.Context, id int64) (*contracts.Account, error) {
	a := &contracts.Account{}
	err := scanAccount(r.db.QueryRowContext(ctx,
		"SELECT "+accountColumns+" FROM finance_accounts WHERE id = ?", id), a)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

// AccountByCode returns nil, nil when missing.
func (r *Repository) AccountByCode(ctx context.Context, code string) (*contracts.Account, error) {
	a := &contracts.Account{}
	err := scanAccount(r.db.QueryRowContext(ctx,
		"SELECT "+accountColumns+" FROM finance_accounts WHERE code = ?", code), a)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

// ListAccounts returns all accounts ordered by code.
func (r *Repository) ListAccounts(ctx context.Context, status string) ([]*contracts.Account, error) {
	query := "SELECT " + accountColumns + " FROM finance_accounts"
	var args []any
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY code ASC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Account
	for rows.Next() {
		a := &contracts.Account{}
		var cash int
		if err := rows.Scan(&a.ID, &a.Code, &a.Name, &a.Type, &cash, &a.Status); err != nil {
			return nil, err
		}
		a.IsCash = cash == 1
		out = append(out, a)
	}
	return out, rows.Err()
}

// CreateAccount inserts an account row.
func (r *Repository) CreateAccount(ctx context.Context, a *contracts.Account) (int64, error) {
	cash := 0
	if a.IsCash {
		cash = 1
	}
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO finance_accounts (code, name, type, is_cash, status) VALUES (?, ?, ?, ?, 'active')",
		a.Code, a.Name, a.Type, cash)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateAccount rewrites name/type/cash flag (code immutable — journals
// reference codes in reports and history).
func (r *Repository) UpdateAccount(ctx context.Context, a *contracts.Account) error {
	cash := 0
	if a.IsCash {
		cash = 1
	}
	_, err := r.db.ExecContext(ctx,
		"UPDATE finance_accounts SET name = ?, type = ?, is_cash = ?, updated_at = UTC_TIMESTAMP(6) WHERE id = ?",
		a.Name, a.Type, cash, a.ID)
	return err
}

// SetAccountStatus flips active/archived.
func (r *Repository) SetAccountStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE finance_accounts SET status = ?, updated_at = UTC_TIMESTAMP(6) WHERE id = ?",
		status, id)
	return err
}

// AccountInUse reports whether any journal line references the account.
func (r *Repository) AccountInUse(ctx context.Context, id int64) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM finance_journal_lines WHERE account_id = ?", id).Scan(&n)
	return n > 0, err
}

// EnsureAccount inserts the account when the code is free (idempotent seed).
func (r *Repository) EnsureAccount(ctx context.Context, code, name, accType string, isCash bool) error {
	cash := 0
	if isCash {
		cash = 1
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO finance_accounts (code, name, type, is_cash, status)
		 VALUES (?, ?, ?, ?, 'active')
		 ON DUPLICATE KEY UPDATE name = VALUES(name)`,
		code, name, accType, cash)
	return err
}

// MappingAccount resolves a mapping key to its account, or nil when unset.
func (r *Repository) MappingAccount(ctx context.Context, key string) (*contracts.Account, error) {
	a := &contracts.Account{}
	var cash int
	err := r.db.QueryRowContext(ctx,
		`SELECT a.id, a.code, a.name, a.type, a.is_cash, a.status
		 FROM finance_accounts a JOIN finance_account_mappings m ON m.account_id = a.id
		 WHERE m.mapping_key = ?`, key).
		Scan(&a.ID, &a.Code, &a.Name, &a.Type, &cash, &a.Status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.IsCash = cash == 1
	return a, nil
}

// ListMappings returns key → account rows.
func (r *Repository) ListMappings(ctx context.Context) (map[string]*contracts.Account, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT m.mapping_key, a.id, a.code, a.name, a.type, a.is_cash, a.status
		 FROM finance_account_mappings m JOIN finance_accounts a ON a.id = m.account_id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]*contracts.Account{}
	for rows.Next() {
		var key string
		a := &contracts.Account{}
		var cash int
		if err := rows.Scan(&key, &a.ID, &a.Code, &a.Name, &a.Type, &cash, &a.Status); err != nil {
			return nil, err
		}
		a.IsCash = cash == 1
		out[key] = a
	}
	return out, rows.Err()
}

// SetMapping points a key at an account (upsert).
func (r *Repository) SetMapping(ctx context.Context, key string, accountID, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO finance_account_mappings (mapping_key, account_id, updated_by)
		 VALUES (?, ?, ?)
		 ON DUPLICATE KEY UPDATE account_id = VALUES(account_id),
		 updated_at = UTC_TIMESTAMP(6), updated_by = VALUES(updated_by)`,
		key, accountID, actorID)
	return err
}

// --- periods ----------------------------------------------------------------

// Period is one accounting month lock.
type Period struct {
	ID     int64
	Year   int
	Month  int
	Status string // open | closed
}

// GetPeriod returns nil, nil when the month was never touched (treated open).
func (r *Repository) GetPeriod(ctx context.Context, branchID int64, year, month int) (*Period, error) {
	p := &Period{}
	err := r.db.QueryRowContext(ctx,
		"SELECT id, year, month, status FROM finance_periods WHERE branch_id = ? AND year = ? AND month = ?",
		branchID, year, month).Scan(&p.ID, &p.Year, &p.Month, &p.Status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

// ListPeriods returns a branch's touched months, newest first.
func (r *Repository) ListPeriods(ctx context.Context, branchID int64) ([]*Period, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, year, month, status FROM finance_periods WHERE branch_id = ? ORDER BY year DESC, month DESC",
		branchID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*Period
	for rows.Next() {
		var p Period
		if err := rows.Scan(&p.ID, &p.Year, &p.Month, &p.Status); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

// SetPeriodStatus inserts or updates a month lock.
func (r *Repository) SetPeriodStatus(ctx context.Context, branchID int64, year, month int, status string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO finance_periods (branch_id, year, month, status, closed_at)
		 VALUES (?, ?, ?, ?,
			CASE WHEN ? = 'closed' THEN UTC_TIMESTAMP(6) ELSE NULL END)
		 ON DUPLICATE KEY UPDATE status = VALUES(status),
			closed_at = CASE WHEN VALUES(status) = 'closed' THEN UTC_TIMESTAMP(6) ELSE NULL END`,
		branchID, year, month, status, status)
	return err
}

// --- sequences --------------------------------------------------------------

// NextSequence atomically allocates the next counter for key (LAST_INSERT_ID
// trick — concurrent callers never share a number).
func (r *Repository) NextSequence(ctx context.Context, branchID int64, key string) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx,
		`INSERT IGNORE INTO finance_document_sequences (branch_id, seq_key, current_value)
		 VALUES (?, ?, 0)`, branchID, key); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE finance_document_sequences SET current_value = LAST_INSERT_ID(current_value + 1)
		 WHERE branch_id = ? AND seq_key = ?`, branchID, key); err != nil {
		return 0, err
	}
	var n int64
	if err := tx.QueryRowContext(ctx, "SELECT LAST_INSERT_ID()").Scan(&n); err != nil {
		return 0, err
	}
	return n, tx.Commit()
}
