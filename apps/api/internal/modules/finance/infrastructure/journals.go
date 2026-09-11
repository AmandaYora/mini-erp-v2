package infrastructure

import (
	"context"
	"database/sql"
	"errors"

	"mini-erp/internal/modules/finance/contracts"
)

// errUnbalanced is a programmer-error guard: services validate balance
// before calling, so this fires only on a service bug, never user input.
var errUnbalanced = errors.New("journal out of balance")

// CreateEntry inserts a balanced entry with lines atomically. Balance is
// enforced here (not trusted to callers): total debit must equal total
// credit and both must be positive. Source uniqueness (one entry per source
// document) rides the DB unique key — duplicates surface as conflicts.
func (r *Repository) CreateEntry(ctx context.Context, e *contracts.JournalEntry, actorID int64) (int64, error) {
	var debit, credit int64
	for _, l := range e.Lines {
		debit += l.Debit
		credit += l.Credit
	}
	if debit <= 0 || debit != credit {
		return 0, errUnbalanced
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO finance_journal_entries
		 (number, branch_id, entry_date, memo, source_doc_type, source_doc_id,
		  status, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, 'posted', ?)`,
		e.Number, e.BranchID, e.Date, e.Memo,
		nullIfEmpty(e.SourceType), nullInt64(e.SourceID), actorID)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, l := range e.Lines {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO finance_journal_lines
			 (entry_id, account_id, product_id, variant_id, party_id, debit, credit, description)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, l.AccountID, nullInt64(l.ProductID), nullInt64(l.VariantID),
			nullInt64(l.PartyID), l.Debit, l.Credit, nullIfEmpty(l.Description)); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullInt64(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

// GetEntry returns nil, nil when missing.
func (r *Repository) GetEntry(ctx context.Context, id int64) (*contracts.JournalEntry, error) {
	e := &contracts.JournalEntry{}
	var sourceType sql.NullString
	var sourceID sql.NullInt64
	var reversedBy sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		`SELECT id, number, branch_id, entry_date, memo, source_doc_type,
			source_doc_id, status, reversed_by
		 FROM finance_journal_entries WHERE id = ?`, id).
		Scan(&e.ID, &e.Number, &e.BranchID, &e.Date, &e.Memo,
			&sourceType, &sourceID, &e.Status, &reversedBy)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	e.SourceType, e.SourceID = sourceType.String, sourceID.Int64
	e.ReversedBy = reversedBy.Int64
	lines, err := r.linesByEntry(ctx, id)
	if err != nil {
		return nil, err
	}
	e.Lines = lines
	return e, nil
}

// EntryBySource returns the posted entry for a source document, or nil.
func (r *Repository) EntryBySource(ctx context.Context, branchID int64, sourceType string, sourceID int64) (*contracts.JournalEntry, error) {
	var id int64
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM finance_journal_entries
		 WHERE branch_id = ? AND source_doc_type = ? AND source_doc_id = ?`,
		branchID, sourceType, sourceID).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.GetEntry(ctx, id)
}

func (r *Repository) linesByEntry(ctx context.Context, entryID int64) ([]*contracts.JournalLine, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT l.account_id, a.code, a.name, l.debit, l.credit,
			l.product_id, l.variant_id, l.party_id, l.description
		 FROM finance_journal_lines l JOIN finance_accounts a ON a.id = l.account_id
		 WHERE l.entry_id = ? ORDER BY l.id ASC`, entryID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.JournalLine
	for rows.Next() {
		var l contracts.JournalLine
		var productID, variantID, partyID sql.NullInt64
		var description sql.NullString
		if err := rows.Scan(&l.AccountID, &l.AccountCode, &l.AccountName, &l.Debit, &l.Credit,
			&productID, &variantID, &partyID, &description); err != nil {
			return nil, err
		}
		l.ProductID, l.VariantID, l.PartyID = productID.Int64, variantID.Int64, partyID.Int64
		l.Description = description.String
		out = append(out, &l)
	}
	return out, rows.Err()
}

// MarkReversed flags an entry reversed by another.
func (r *Repository) MarkReversed(ctx context.Context, id, byID int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE finance_journal_entries SET status = 'reversed', reversed_by = ? WHERE id = ?",
		byID, id)
	return err
}

// ListEntries searches a branch's entries newest-first. excludeFiscal drops
// fiscal-adjustment journals (source_doc_type 'tax_adjustment'): commercial
// reports must never see them, while the journal browser keeps them visible.
func (r *Repository) ListEntries(ctx context.Context, branchID int64, from, to string, accountID int64, limit, offset int, skipFiscal bool) ([]*contracts.JournalEntry, error) {
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
	join := ""
	if accountID != 0 {
		join = "JOIN finance_journal_lines l ON l.entry_id = e.id AND l.account_id = ?"
		args = append([]any{accountID}, args...)
	}
 	args = append(args, limit, offset)
 	// SELECT with a JOIN arg first: reorder — placeholder order must match.
 	rows, err := r.db.QueryContext(ctx,
 		`SELECT DISTINCT e.id, e.number, e.branch_id, e.entry_date, e.memo,
			e.source_doc_type, e.source_doc_id, e.status, e.reversed_by
		 FROM finance_journal_entries e `+join+`
		 WHERE `+conds+excludeFiscal(skipFiscal)+` ORDER BY e.id DESC LIMIT ? OFFSET ?`, args...)
 	if err != nil {
 		return nil, err
 	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.JournalEntry
	for rows.Next() {
		e := &contracts.JournalEntry{}
		var sourceType sql.NullString
		var sourceID, reversedBy sql.NullInt64
		var date sql.NullTime
		if err := rows.Scan(&e.ID, &e.Number, &e.BranchID, &date, &e.Memo,
			&sourceType, &sourceID, &e.Status, &reversedBy); err != nil {
			return nil, err
		}
		if date.Valid {
			e.Date = date.Time.Format("2006-01-02 15:04:05")
		}
		e.SourceType, e.SourceID = sourceType.String, sourceID.Int64
		e.ReversedBy = reversedBy.Int64
		out = append(out, e)
	}
	return out, rows.Err()
}

// CountEntries counts the List filter set.
func (r *Repository) CountEntries(ctx context.Context, branchID int64, from, to string, accountID int64) (int64, error) {
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
	join := ""
	if accountID != 0 {
		join = "JOIN finance_journal_lines l ON l.entry_id = e.id AND l.account_id = ?"
		args = append([]any{accountID}, args...)
	}
	var total int64
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(DISTINCT e.id) FROM finance_journal_entries e "+join+" WHERE "+conds, args...).Scan(&total)
	return total, err
}

// commercialOnly is the shared fiscal-exclusion fragment. Commercial books
// never see tax-adjustment journals (they live in the same ledger for audit,
// but every commercial footing/ledger excludes them by construction — one
// forgotten filter is all it takes to misstate profit, so the rule lives
// here, not in callers).
const commercialOnly = ` AND (e.source_doc_type IS NULL OR e.source_doc_type <> 'tax_adjustment')`

// excludeFiscal returns commercialOnly when true, "" otherwise (journal
// browser keeps fiscal entries visible and reversible).
func excludeFiscal(on bool) string {
	if on {
		return commercialOnly
	}
	return ""
}

// AccountFootings returns lifetime debit/credit sums per account.
func (r *Repository) AccountFootings(ctx context.Context, branchID int64, from, to string) (map[int64][2]int64, error) {
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
	rows, err := r.db.QueryContext(ctx,
		`SELECT l.account_id, COALESCE(SUM(l.debit), 0), COALESCE(SUM(l.credit), 0)
		 FROM finance_journal_lines l
		 JOIN finance_journal_entries e ON e.id = l.entry_id
		 WHERE `+conds+commercialOnly+` GROUP BY l.account_id`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[int64][2]int64{}
	for rows.Next() {
		var id, debit, credit int64
		if err := rows.Scan(&id, &debit, &credit); err != nil {
			return nil, err
		}
		out[id] = [2]int64{debit, credit}
	}
	return out, rows.Err()
}

// ProductFooting is one product position's debit/credit footing on one
// account, for a date range.
type ProductFooting struct {
	ProductID int64
	VariantID int64
	AccountID int64
	Debit     int64
	Credit    int64
}

// ProductFootings sums product-attributed legs on the given accounts, in one
// pass. Callers net the sides per account role — the repository does not know
// which account is revenue and which is COGS.
//
// One query for the whole report: the alternative (a query per product) is
// the N+1 the analytic dimension exists to avoid.
func (r *Repository) ProductFootings(ctx context.Context, branchID int64, from, to string, accountIDs []int64) ([]ProductFooting, error) {
	if len(accountIDs) == 0 {
		return nil, nil
	}
	args := []any{branchID}
	conds := "e.branch_id = ? AND l.product_id IS NOT NULL"
	if from != "" {
		conds += " AND e.entry_date >= ?"
		args = append(args, from)
	}
	if to != "" {
		conds += " AND e.entry_date <= ?"
		args = append(args, to)
	}
	placeholders := make([]byte, 0, len(accountIDs)*2)
	for i, id := range accountIDs {
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholders = append(placeholders, '?')
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT l.product_id, l.variant_id, l.account_id,
			COALESCE(SUM(l.debit), 0), COALESCE(SUM(l.credit), 0)
		 FROM finance_journal_lines l
		 JOIN finance_journal_entries e ON e.id = l.entry_id
		 WHERE `+conds+commercialOnly+` AND l.account_id IN (`+string(placeholders)+`)
		 GROUP BY l.product_id, l.variant_id, l.account_id`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []ProductFooting
	for rows.Next() {
		var f ProductFooting
		if err := rows.Scan(&f.ProductID, &f.VariantID, &f.AccountID, &f.Debit, &f.Credit); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// PartyFooting is one counterparty's debit/credit footing on one account.
type PartyFooting struct {
	PartyID int64
	Debit   int64
	Credit  int64
}

// PartyFootings sums party-attributed legs on one account up to `asOf`
// inclusive. Receivable and payable are *balances*, not period flows, so the
// window is open-ended on the left — a range filter would silently drop
// invoices older than the range and understate what is owed.
func (r *Repository) PartyFootings(ctx context.Context, branchID, accountID int64, asOf string) ([]PartyFooting, error) {
	conds := "e.branch_id = ? AND l.account_id = ? AND l.party_id IS NOT NULL"
	args := []any{branchID, accountID}
	if asOf != "" {
		conds += " AND e.entry_date <= ?"
		args = append(args, asOf)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT l.party_id, COALESCE(SUM(l.debit), 0), COALESCE(SUM(l.credit), 0)
		 FROM finance_journal_lines l
		 JOIN finance_journal_entries e ON e.id = l.entry_id
		 WHERE `+conds+commercialOnly+` GROUP BY l.party_id`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []PartyFooting
	for rows.Next() {
		var f PartyFooting
		if err := rows.Scan(&f.PartyID, &f.Debit, &f.Credit); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// TaxAdjustmentFootings mirrors AccountFootings over fiscal-adjustment
// journals only — the fiscal leg of the commercial→fiscal worksheet.
func (r *Repository) TaxAdjustmentFootings(ctx context.Context, branchID int64, from, to string) (map[int64][2]int64, error) {
	conds := "e.branch_id = ? AND e.source_doc_type = 'tax_adjustment'"
	args := []any{branchID}
	if from != "" {
		conds += " AND e.entry_date >= ?"
		args = append(args, from)
	}
	if to != "" {
		conds += " AND e.entry_date <= ?"
		args = append(args, to)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT l.account_id, COALESCE(SUM(l.debit), 0), COALESCE(SUM(l.credit), 0)
		 FROM finance_journal_lines l
		 JOIN finance_journal_entries e ON e.id = l.entry_id
		 WHERE `+conds+` GROUP BY l.account_id`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[int64][2]int64{}
	for rows.Next() {
		var id, debit, credit int64
		if err := rows.Scan(&id, &debit, &credit); err != nil {
			return nil, err
		}
		out[id] = [2]int64{debit, credit}
	}
	return out, rows.Err()
}

// PostedSourceIDs returns the set of source document IDs already journaled
// for one source type — the journal side of the derived posting queue.
// One grouped query per type; the queue diffs it against module candidates
// in memory (P3: no per-document EntryBySource loop).
func (r *Repository) PostedSourceIDs(ctx context.Context, branchID int64, sourceType string) (map[int64]bool, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT e.source_doc_id FROM finance_journal_entries e
		 WHERE e.branch_id = ? AND e.source_doc_type = ? AND e.source_doc_id IS NOT NULL`,
		branchID, sourceType)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// CountUnbalanced counts entries whose legs do not net to zero in range.
// Inserts enforce balance, so a nonzero here means migrated or hand-touched
// data — exactly what the safe-close gate must catch before sealing books.
func (r *Repository) CountUnbalanced(ctx context.Context, branchID int64, from, to string) (int64, error) {
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
	var n int64
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM (
			SELECT l.entry_id
			FROM finance_journal_lines l
			JOIN finance_journal_entries e ON e.id = l.entry_id
			WHERE `+conds+`
			GROUP BY l.entry_id
			HAVING COALESCE(SUM(l.debit), 0) <> COALESCE(SUM(l.credit), 0)
			   OR COALESCE(SUM(l.debit), 0) <= 0
		) u`, args...).Scan(&n)
	return n, err
}

// EntriesBySourceType lists entries of one source type newest-first for
// worksheets (tax adjustments) that live as typed journals, not tables.
func (r *Repository) EntriesBySourceType(ctx context.Context, branchID int64, sourceType string, limit, offset int) ([]*contracts.JournalEntry, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT e.id, e.number, e.branch_id, e.entry_date, e.memo,
			e.source_doc_type, e.source_doc_id, e.status, e.reversed_by
		 FROM finance_journal_entries e
		 WHERE e.branch_id = ? AND e.source_doc_type = ?
		 ORDER BY e.id DESC LIMIT ? OFFSET ?`, branchID, sourceType, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.JournalEntry
	for rows.Next() {
		e := &contracts.JournalEntry{}
		var sourceType sql.NullString
		var sourceID, reversedBy sql.NullInt64
		var date sql.NullTime
		if err := rows.Scan(&e.ID, &e.Number, &e.BranchID, &date, &e.Memo,
			&sourceType, &sourceID, &e.Status, &reversedBy); err != nil {
			return nil, err
		}
		if date.Valid {
			e.Date = date.Time.Format("2006-01-02 15:04:05")
		}
		e.SourceType, e.SourceID = sourceType.String, sourceID.Int64
		e.ReversedBy = reversedBy.Int64
		out = append(out, e)
	}
	return out, rows.Err()
}
