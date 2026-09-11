package infrastructure

import (
	"context"
	"strings"

	"mini-erp/internal/modules/finance/contracts"
)

// TurnoverEntry is one journal entry reduced to what the gross-turnover
// ceiling needs: when it happened, which document produced it, and how much
// revenue it carries.
//
// Read COMPANY-WIDE on purpose — no branch filter. The Rp4,8 M ceiling is a
// property of the taxpayer, not of a branch; summing per-branch ceilings
// would silently allow the company past the real one.
type TurnoverEntry struct {
	EntryID    int64
	BranchID   int64
	Date       string
	SourceType string
	SourceID   int64
	// Revenue is the net income-account footing (credit − debit). Sales
	// returns land here as a NEGATIVE amount, which is exactly right:
	// peredaran bruto is turnover net of returns.
	Revenue int64
}

// TurnoverEntries lists every commercial entry in an inclusive YYYY-MM-DD
// range with its revenue footing, ordered chronologically.
//
// One query for the whole ceiling computation. Entries with no revenue come
// back too (revenue 0) because the exclusion set still needs them: dropping
// an order means dropping its COGS and payment legs as well, and those legs
// live in entries that carry no revenue of their own.
//
// Fiscal-correction entries are excluded via commercialOnly — the ceiling is
// a commercial-turnover rule, and tax adjustments are not turnover.
func (r *Repository) TurnoverEntries(ctx context.Context, from, to string) ([]TurnoverEntry, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT e.id, e.branch_id, DATE_FORMAT(e.entry_date, '%Y-%m-%d'),
		        COALESCE(e.source_doc_type, ''), COALESCE(e.source_doc_id, 0),
		        COALESCE(SUM(CASE WHEN a.type = ? THEN l.credit - l.debit ELSE 0 END), 0)
		 FROM finance_journal_entries e
		 JOIN finance_journal_lines l ON l.entry_id = e.id
		 JOIN finance_accounts a ON a.id = l.account_id
		 WHERE e.entry_date >= ? AND e.entry_date <= ?`+commercialOnly+`
		 GROUP BY e.id, e.branch_id, e.entry_date, e.source_doc_type, e.source_doc_id
		 ORDER BY e.entry_date, e.id`,
		contracts.AccountIncome, from, to)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []TurnoverEntry
	for rows.Next() {
		var t TurnoverEntry
		if err := rows.Scan(&t.EntryID, &t.BranchID, &t.Date,
			&t.SourceType, &t.SourceID, &t.Revenue); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// footingChunk bounds how many entry ids go into one IN (...) list.
const footingChunk = 1000

// AccountFootingsExcluding returns account footings for the window with the
// given journal entries removed.
//
// Implemented as (full − excluded) rather than a giant `NOT IN (...)`:
// footings are plain sums, so subtracting the excluded slice is exact, and
// the excluded side pages through ids in chunks instead of building one
// enormous predicate. Both sides run the SAME window and commercial filter,
// so the subtraction can never drift.
//
// Every report in this module funnels through AccountFootings, which is why
// excluding here is enough to keep an excluded transaction out of EVERY
// sheet at once — profit & loss, balance sheet, trial balance, VAT summary.
func (r *Repository) AccountFootingsExcluding(ctx context.Context, branchID int64, from, to string, excluded []int64) (map[int64][2]int64, error) {
	full, err := r.AccountFootings(ctx, branchID, from, to)
	if err != nil {
		return nil, err
	}
	if len(excluded) == 0 {
		return full, nil
	}
	drop, err := r.footingsOfEntries(ctx, branchID, from, to, excluded)
	if err != nil {
		return nil, err
	}
	for id, d := range drop {
		f := full[id]
		f[0] -= d[0]
		f[1] -= d[1]
		if f[0] == 0 && f[1] == 0 {
			delete(full, id)
			continue
		}
		full[id] = f
	}
	return full, nil
}

// footingsOfEntries sums debit/credit per account for the named entries,
// restricted to the same window as the full footing query.
func (r *Repository) footingsOfEntries(ctx context.Context, branchID int64, from, to string, ids []int64) (map[int64][2]int64, error) {
	out := map[int64][2]int64{}
	seen := make(map[int64]bool, len(ids))
	uniq := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		uniq = append(uniq, id)
	}
	for start := 0; start < len(uniq); start += footingChunk {
		end := start + footingChunk
		if end > len(uniq) {
			end = len(uniq)
		}
		chunk := uniq[start:end]
		conds := "e.branch_id = ?"
		args := []any{branchID}
		if from != "" {
			conds += " AND e.entry_date >= ?"
			args = append(args, from)
		}
		if to != "" {
			conds += " AND e.entry_date <= ?"
			args = append(args, to)
		}
		placeholders := strings.Repeat("?,", len(chunk)-1) + "?"
		for _, id := range chunk {
			args = append(args, id)
		}
		rows, err := r.db.QueryContext(ctx,
			`SELECT l.account_id, COALESCE(SUM(l.debit), 0), COALESCE(SUM(l.credit), 0)
			 FROM finance_journal_lines l
			 JOIN finance_journal_entries e ON e.id = l.entry_id
			 WHERE `+conds+commercialOnly+` AND e.id IN (`+placeholders+`)
			 GROUP BY l.account_id`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id, debit, credit int64
			if err := rows.Scan(&id, &debit, &credit); err != nil {
				_ = rows.Close()
				return nil, err
			}
			f := out[id]
			f[0] += debit
			f[1] += credit
			out[id] = f
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
	}
	return out, nil
}
