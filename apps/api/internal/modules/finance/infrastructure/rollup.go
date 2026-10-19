package infrastructure

import (
	"context"
	"fmt"
)

// RollupDiff is one (account, day, scope) whose rollup disagrees with the lines.
type RollupDiff struct {
	AccountID    int64  `json:"accountId"`
	Date         string `json:"date"`
	Scope        int    `json:"scope"`
	LinesDebit   int64  `json:"linesDebit"`
	LinesCredit  int64  `json:"linesCredit"`
	RollupDebit  int64  `json:"rollupDebit"`
	RollupCredit int64  `json:"rollupCredit"`
}

type rollupKey struct {
	account int64
	date    string
	scope   int
}

// VerifyRollups compares the daily rollup with the journal lines for a branch
// and returns every disagreement (empty = consistent).
func (r *Repository) VerifyRollups(ctx context.Context, branchID int64) ([]RollupDiff, error) {
	lines := map[rollupKey][2]int64{}
	rows, err := r.db.QueryContext(ctx,
		`SELECT l.account_id, DATE_FORMAT(e.entry_date, '%Y-%m-%d'), e.scope, SUM(l.debit), SUM(l.credit)
		 FROM finance_journal_lines l JOIN finance_journal_entries e ON e.id = l.entry_id
		 WHERE e.branch_id = ? GROUP BY l.account_id, DATE_FORMAT(e.entry_date, '%Y-%m-%d'), e.scope`, branchID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var k rollupKey
		var d, c int64
		if err := rows.Scan(&k.account, &k.date, &k.scope, &d, &c); err != nil {
			_ = rows.Close()
			return nil, err
		}
		lines[k] = [2]int64{d, c}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()
	roll := map[rollupKey][2]int64{}
	rows, err = r.db.QueryContext(ctx,
		`SELECT account_id, DATE_FORMAT(entry_date, '%Y-%m-%d'), scope, debit, credit
		 FROM finance_account_daily WHERE branch_id = ?`, branchID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var k rollupKey
		var d, c int64
		if err := rows.Scan(&k.account, &k.date, &k.scope, &d, &c); err != nil {
			return nil, err
		}
		roll[k] = [2]int64{d, c}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []RollupDiff
	seen := map[rollupKey]bool{}
	for k, l := range lines {
		seen[k] = true
		if ro := roll[k]; ro != l {
			out = append(out, RollupDiff{AccountID: k.account, Date: k.date, Scope: k.scope,
				LinesDebit: l[0], LinesCredit: l[1], RollupDebit: ro[0], RollupCredit: ro[1]})
		}
	}
	for k, ro := range roll {
		if !seen[k] && (ro[0] != 0 || ro[1] != 0) {
			out = append(out, RollupDiff{AccountID: k.account, Date: k.date, Scope: k.scope,
				RollupDebit: ro[0], RollupCredit: ro[1]})
		}
	}
	return out, nil
}

// RebuildRollups recomputes the branch's rollup from the journal lines (ETL and
// repair). One transaction: readers see the old or the new state, never half.
func (r *Repository) RebuildRollups(ctx context.Context, branchID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, "DELETE FROM finance_account_daily WHERE branch_id = ?", branchID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO finance_account_daily (branch_id, account_id, entry_date, scope, debit, credit)
		 SELECT e.branch_id, l.account_id, DATE(e.entry_date), e.scope, SUM(l.debit), SUM(l.credit)
		   FROM finance_journal_lines l JOIN finance_journal_entries e ON e.id = l.entry_id
		  WHERE e.branch_id = ? GROUP BY e.branch_id, l.account_id, DATE(e.entry_date), e.scope`, branchID); err != nil {
		return fmt.Errorf("rebuild rollups: %w", err)
	}
	return tx.Commit()
}
