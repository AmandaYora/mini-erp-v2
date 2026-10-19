package infrastructure

import (
	"context"
	"strings"

	"mini-erp/internal/modules/finance/contracts"
)

// TaxEntryRow is one journal entry of a month aggregated to its tax legs.
type TaxEntryRow struct {
	EntryID    int64
	BranchID   int64
	Number     string
	Date       string
	Memo       string
	SourceType string
	SourceID   int64
	Revenue    int64 // credit - debit on the income accounts (sales returns land negative)
	PpnOut     int64 // credit - debit on PPN Keluaran
	PpnIn      int64 // debit - credit on PPN Masukan
	// Payable is credit - debit on the payable account: the tax-inclusive value
	// of a receipt (positive) or purchase return (negative), at the order's own
	// prices. Purchase DPP = Payable - PpnIn.
	Payable int64
}

// TaxEntries aggregates, in SQL, the revenue / PPN legs of every entry of the
// given source types in a date range, ordered by date, with an exact total.
// Nothing is loaded and summed in memory, so a month of any size is one
// grouped query per page.
func (r *Repository) TaxEntries(ctx context.Context, branchID int64, from, to string, ppnOutID, ppnInID, payableID int64, sourceTypes []string, limit, offset int) ([]TaxEntryRow, int64, error) {
	ids := branches(ctx, branchID)
	conds := "e.branch_id IN (" + strings.Repeat("?,", len(ids)-1) + "?) AND e.entry_date >= ? AND e.entry_date <= ?" +
		commercialOnly + " AND e.status = 'posted' AND e.source_doc_type IN (" + strings.Repeat("?,", len(sourceTypes)-1) + "?)"
	args := make([]any, 0, len(ids)+len(sourceTypes)+2)
	for _, id := range ids {
		args = append(args, id)
	}
	args = append(args, from, to)
	for _, t := range sourceTypes {
		args = append(args, t)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM finance_journal_entries e WHERE `+conds, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := `SELECT e.id, e.branch_id, e.number, DATE_FORMAT(e.entry_date, '%Y-%m-%d'), e.memo,
			COALESCE(e.source_doc_type, ''), COALESCE(e.source_doc_id, 0),
			COALESCE(SUM(CASE WHEN a.type = ? THEN l.credit - l.debit ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN l.account_id = ? THEN l.credit - l.debit ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN l.account_id = ? THEN l.debit - l.credit ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN l.account_id = ? THEN l.credit - l.debit ELSE 0 END), 0)
		FROM finance_journal_entries e
		LEFT JOIN finance_journal_lines l ON l.entry_id = e.id
		LEFT JOIN finance_accounts a ON a.id = l.account_id
		WHERE ` + conds + `
		GROUP BY e.id, e.branch_id, e.number, e.entry_date, e.memo, e.source_doc_type, e.source_doc_id
		ORDER BY e.entry_date, e.id LIMIT ? OFFSET ?`
	qargs := append([]any{contracts.AccountIncome, ppnOutID, ppnInID, payableID}, args...)
	qargs = append(qargs, limit, offset)
	rows, err := r.db.QueryContext(ctx, q, qargs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	var out []TaxEntryRow
	for rows.Next() {
		var t TaxEntryRow
		if err := rows.Scan(&t.EntryID, &t.BranchID, &t.Number, &t.Date, &t.Memo, &t.SourceType, &t.SourceID, &t.Revenue, &t.PpnOut, &t.PpnIn, &t.Payable); err != nil {
			return nil, 0, err
		}
		out = append(out, t)
	}
	return out, total, rows.Err()
}
