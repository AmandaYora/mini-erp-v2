package infrastructure

import (
	"context"
	"database/sql"
	"fmt"

	"mini-erp/internal/modules/finance/contracts"
)

// Repository owns the finance module tables.
type Repository struct {
	db *sql.DB
	// chart memoizes the chart of accounts and its mappings — read on every
	// journal line, written only by administrators (see chartcache.go).
	chart *chartCache
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, chart: newChartCache()}
}

// --- accounts ---------------------------------------------------------------

const accountColumns = "id, code, name, type, is_cash, status"

// AccountByID returns nil, nil when missing.
func (r *Repository) AccountByID(ctx context.Context, id int64) (*contracts.Account, error) {
	return r.chartLookup(ctx, func(c *chartCache) (contracts.Account, bool) {
		a, ok := c.byID[id]
		return a, ok
	})
}

// AccountByCode returns nil, nil when missing. Served from the cached chart
// (chartcache.go): posting resolves a code per journal line, and that is a
// map lookup, not a query.
func (r *Repository) AccountByCode(ctx context.Context, code string) (*contracts.Account, error) {
	return r.chartLookup(ctx, func(c *chartCache) (contracts.Account, bool) {
		a, ok := c.byCode[code]
		return a, ok
	})
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
	r.chart.invalidate()
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
	r.chart.invalidate()
	return err
}

// SetAccountStatus flips active/archived.
func (r *Repository) SetAccountStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE finance_accounts SET status = ?, updated_at = UTC_TIMESTAMP(6) WHERE id = ?",
		status, id)
	r.chart.invalidate()
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
	r.chart.invalidate()
	return err
}

// MappingAccount resolves a mapping key to its account, or nil when unset.
// Served from the cached chart.
func (r *Repository) MappingAccount(ctx context.Context, key string) (*contracts.Account, error) {
	return r.chartLookup(ctx, func(c *chartCache) (contracts.Account, bool) {
		a, ok := c.byMapKey[key]
		return a, ok
	})
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
	r.chart.invalidate()
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
// hook (optional) runs in the same transaction with the month's previous
// status ("open" when the month was untouched) so the audit row co-commits
// with the change (ADR-0011).
func (r *Repository) SetPeriodStatus(ctx context.Context, branchID int64, year, month int, status string, hook func(ctx context.Context, tx *sql.Tx, before string) error) error {
	return r.WithTx(ctx, func(tx *sql.Tx) error {
		before := "open"
		err := tx.QueryRowContext(ctx,
			"SELECT status FROM finance_periods WHERE branch_id = ? AND year = ? AND month = ? FOR UPDATE",
			branchID, year, month).Scan(&before)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO finance_periods (branch_id, year, month, status, closed_at)
		 VALUES (?, ?, ?, ?,
			CASE WHEN ? = 'closed' THEN UTC_TIMESTAMP(6) ELSE NULL END)
		 ON DUPLICATE KEY UPDATE status = VALUES(status),
			closed_at = CASE WHEN VALUES(status) = 'closed' THEN UTC_TIMESTAMP(6) ELSE NULL END`,
			branchID, year, month, status, status); err != nil {
			return err
		}
		if hook != nil {
			return hook(ctx, tx, before)
		}
		return nil
	})
}

// --- sequences --------------------------------------------------------------

// NextSequence atomically allocates the next counter for key (LAST_INSERT_ID
// trick — concurrent callers never share a number).
func NextSequenceTx(ctx context.Context, tx *sql.Tx, branchID int64, key string) (int64, error) {
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
	return n, nil
}

// numberInTx allocates PREFIX-CODE/YEAR/00001 inside the caller's transaction.
func numberInTx(ctx context.Context, tx *sql.Tx, branchID int64, nb *contracts.Numbering) (string, error) {
	n, err := NextSequenceTx(ctx, tx, branchID, nb.Prefix+":"+nb.Year)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s/%s/%05d", nb.Prefix, nb.BranchCode, nb.Year, n), nil
}

func (r *Repository) NextSequence(ctx context.Context, branchID int64, key string) (int64, error) {
	// F4-16 fast path: a single UPDATE ... LAST_INSERT_ID(expr) statement (no
	// transaction); the missing-row / error cases use the transactional path.
	if res, err := r.db.ExecContext(ctx,
		`UPDATE finance_document_sequences SET current_value = LAST_INSERT_ID(current_value + 1)
		 WHERE branch_id = ? AND seq_key = ?`, branchID, key); err == nil {
		if affected, aerr := res.RowsAffected(); aerr == nil && affected == 1 {
			if id, ierr := res.LastInsertId(); ierr == nil && id > 0 {
				return id, nil
			}
		}
	}
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
