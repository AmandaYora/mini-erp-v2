package infrastructure

import (
	"context"
	"database/sql"

	"mini-erp/internal/modules/finance/contracts"
)

// --- bank accounts (rekening perusahaan, ADR-0026) ---------------------------

const bankAccountSelect = `SELECT b.id, b.account_id, a.code, a.name, b.bank_name, b.account_number,
	b.account_holder, b.show_on_documents, b.is_default, b.sort_order, b.status
	FROM finance_bank_accounts b JOIN finance_accounts a ON a.id = b.account_id`

func scanBankAccount(row interface{ Scan(...any) error }) (*contracts.BankAccount, error) {
	b := &contracts.BankAccount{}
	var show, def int
	if err := row.Scan(&b.ID, &b.AccountID, &b.AccountCode, &b.AccountName, &b.BankName, &b.AccountNumber,
		&b.AccountHolder, &show, &def, &b.SortOrder, &b.Status); err != nil {
		return nil, err
	}
	b.ShowOnDocuments, b.IsDefault = show == 1, def == 1
	return b, nil
}

// ListBankAccounts returns rekening (optionally one status), display order.
func (r *Repository) ListBankAccounts(ctx context.Context, status string) ([]*contracts.BankAccount, error) {
	query := bankAccountSelect
	var args []any
	if status != "" {
		query += " WHERE b.status = ?"
		args = append(args, status)
	}
	query += " ORDER BY b.status ASC, b.sort_order ASC, b.id ASC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.BankAccount
	for rows.Next() {
		b, err := scanBankAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// BankAccountByID returns nil, nil when missing.
func (r *Repository) BankAccountByID(ctx context.Context, id int64) (*contracts.BankAccount, error) {
	b, err := scanBankAccount(r.db.QueryRowContext(ctx, bankAccountSelect+" WHERE b.id = ?", id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return b, err
}

// BankAccountByAccountID returns the rekening tied to a GL account, nil, nil
// when the account is no rekening.
func (r *Repository) BankAccountByAccountID(ctx context.Context, accountID int64) (*contracts.BankAccount, error) {
	b, err := scanBankAccount(r.db.QueryRowContext(ctx, bankAccountSelect+" WHERE b.account_id = ?", accountID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return b, err
}

// BankPolicy counts active rekening and returns the default one's GL account.
func (r *Repository) BankPolicy(ctx context.Context) (active int64, defaultAccountID int64, err error) {
	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(MAX(CASE WHEN is_default = 1 THEN account_id END), 0)
		 FROM finance_bank_accounts WHERE status = 'active'`).Scan(&active, &defaultAccountID)
	return active, defaultAccountID, err
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// SaveBankAccount inserts (b.ID == 0) or rewrites a rekening. A default
// rekening clears the flag on every other one in the same transaction, so at
// most one default exists.
func (r *Repository) SaveBankAccount(ctx context.Context, b *contracts.BankAccount, actorID int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	id := b.ID
	if id == 0 {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO finance_bank_accounts (account_id, bank_name, account_number, account_holder,
			   show_on_documents, is_default, sort_order, status, created_by, updated_by)
			 VALUES (?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)`,
			b.AccountID, b.BankName, b.AccountNumber, b.AccountHolder,
			boolInt(b.ShowOnDocuments), boolInt(b.IsDefault), b.SortOrder, actorID, actorID)
		if err != nil {
			return 0, err
		}
		if id, err = res.LastInsertId(); err != nil {
			return 0, err
		}
	} else if _, err := tx.ExecContext(ctx,
		`UPDATE finance_bank_accounts SET bank_name = ?, account_number = ?, account_holder = ?,
		   show_on_documents = ?, is_default = ?, sort_order = ?, updated_at = UTC_TIMESTAMP(6), updated_by = ?
		 WHERE id = ?`,
		b.BankName, b.AccountNumber, b.AccountHolder,
		boolInt(b.ShowOnDocuments), boolInt(b.IsDefault), b.SortOrder, actorID, id); err != nil {
		return 0, err
	}
	if b.IsDefault {
		if _, err := tx.ExecContext(ctx,
			`UPDATE finance_bank_accounts SET is_default = 0, updated_at = UTC_TIMESTAMP(6), updated_by = ?
			 WHERE id <> ? AND is_default = 1`, actorID, id); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// SetBankAccountStatus flips active/archived; an archived rekening is never
// the default.
func (r *Repository) SetBankAccountStatus(ctx context.Context, id int64, status string, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE finance_bank_accounts SET status = ?,
		   is_default = CASE WHEN ? = 'active' THEN is_default ELSE 0 END,
		   updated_at = UTC_TIMESTAMP(6), updated_by = ?
		 WHERE id = ?`, status, status, actorID, id)
	return err
}
