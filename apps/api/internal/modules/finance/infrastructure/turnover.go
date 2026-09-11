package infrastructure

import (
	"context"

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
