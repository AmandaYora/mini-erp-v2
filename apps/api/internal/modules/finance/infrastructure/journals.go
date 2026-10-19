package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"

	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/shared/dbutil"
)

// errUnbalanced is a programmer-error guard: services validate balance
// before calling, so this fires only on a service bug, never user input.
var errUnbalanced = errors.New("journal out of balance")

// CreateEntry inserts a balanced entry with lines atomically. Balance is
// enforced here (not trusted to callers): total debit must equal total
// credit and both must be positive. Source uniqueness (one entry per source
// document) rides the DB unique key — duplicates surface as conflicts.
// hook (optional) runs inside the same transaction after the rows are written
// and receives the new entry's id and number — the audit trail co-commits with
// the journal (ADR-0011); a hook error rolls the whole entry back.
func (r *Repository) CreateEntry(ctx context.Context, e *contracts.JournalEntry, actorID int64, hook EntryHook) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	id, err := createEntryTx(ctx, tx, e, actorID)
	if err != nil {
		return 0, err
	}
	if hook != nil {
		if err := hook(ctx, tx, id, e.Number); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// EntryHook runs inside the journal's transaction with the stored entry's
// identity; it is where the audit row is written (ADR-0011).
type EntryHook func(ctx context.Context, tx *sql.Tx, id int64, number string) error

// WithTx runs fn in one transaction (commit on nil, rollback otherwise).
func (r *Repository) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// ErrAlreadyReversed means another writer reversed the entry first (CAS lost).
var ErrAlreadyReversed = errors.New("journal already reversed")

// ReverseEntry stores the mirror and flags the original in ONE transaction. The
// flag is a CAS (status must still be posted): two simultaneous reversals
// produce one mirror and one ErrAlreadyReversed, never two mirrors.
func (r *Repository) ReverseEntry(ctx context.Context, mirror *contracts.JournalEntry, origID int64, requeue bool, actorID int64, hook EntryHook) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	// Claim the original first: if it is no longer posted nothing is written.
	rq := 0
	if requeue {
		rq = 1
	}
	res, err := tx.ExecContext(ctx,
		"UPDATE finance_journal_entries SET status = 'reversed', requeue = ? WHERE id = ? AND status = 'posted'", rq, origID)
	if err != nil {
		return 0, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return 0, ErrAlreadyReversed
	}
	id, err := createEntryTx(ctx, tx, mirror, actorID)
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE finance_journal_entries SET reversed_by = ? WHERE id = ?", id, origID); err != nil {
		return 0, err
	}
	if hook != nil {
		if err := hook(ctx, tx, id, mirror.Number); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

func createEntryTx(ctx context.Context, tx *sql.Tx, e *contracts.JournalEntry, actorID int64) (int64, error) {
	if err := guardPeriodTx(ctx, tx, e.BranchID, e.Date); err != nil {
		return 0, err
	}
	if e.Number == "" && e.Numbering != nil {
		number, err := numberInTx(ctx, tx, e.BranchID, e.Numbering)
		if err != nil {
			return 0, err
		}
		e.Number = number
	}
	var debit, credit int64
	for _, l := range e.Lines {
		debit += l.Debit
		credit += l.Credit
	}
	if debit <= 0 || debit != credit {
		return 0, errUnbalanced
	}
	scope := e.Scope
	if e.SourceType == "tax_adjustment" {
		scope = 1
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO finance_journal_entries
		 (number, branch_id, entry_date, memo, source_doc_type, source_doc_id,
		  status, scope, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, 'posted', ?, ?)`,
		e.Number, e.BranchID, e.Date, e.Memo,
		dbutil.NullIfEmpty(e.SourceType), dbutil.NullIfZero(e.SourceID), scope, actorID)
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
			id, l.AccountID, dbutil.NullIfZero(l.ProductID), dbutil.NullIfZero(l.VariantID),
			dbutil.NullIfZero(l.PartyID), l.Debit, l.Credit, dbutil.NullIfEmpty(l.Description)); err != nil {
			return 0, err
		}
	}
	if err := bumpDailyTx(ctx, tx, e.BranchID, e.Date, scope, e.Lines); err != nil {
		return 0, err
	}
	return id, nil
}

// bumpDailyTx adds the entry's legs to the daily account rollup, inside the
// entry's own transaction. Accounts are touched in ascending id order so two
// concurrent entries can never deadlock on each other's rollup rows.
func bumpDailyTx(ctx context.Context, tx *sql.Tx, branchID int64, date string, scope int, lines []*contracts.JournalLine) error {
	if len(date) < 10 {
		return nil
	}
	type sums struct{ debit, credit int64 }
	byAccount := map[int64]*sums{}
	for _, l := range lines {
		s := byAccount[l.AccountID]
		if s == nil {
			s = &sums{}
			byAccount[l.AccountID] = s
		}
		s.debit += l.Debit
		s.credit += l.Credit
	}
	ids := make([]int64, 0, len(byAccount))
	for id := range byAccount {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		s := byAccount[id]
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO finance_account_daily (branch_id, account_id, entry_date, scope, debit, credit)
			 VALUES (?, ?, ?, ?, ?, ?)
			 ON DUPLICATE KEY UPDATE debit = debit + VALUES(debit), credit = credit + VALUES(credit)`,
			branchID, id, date[:10], scope, s.debit, s.credit); err != nil {
			return err
		}
	}
	return nil
}

// GetEntry returns nil, nil when missing.
func (r *Repository) GetEntry(ctx context.Context, id int64) (*contracts.JournalEntry, error) {
	e := &contracts.JournalEntry{}
	var sourceType sql.NullString
	var sourceID sql.NullInt64
	var reversedBy sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		`SELECT id, number, branch_id, entry_date, memo, source_doc_type,
			source_doc_id, status, reversed_by, scope
		 FROM finance_journal_entries WHERE id = ?`, id).
		Scan(&e.ID, &e.Number, &e.BranchID, &e.Date, &e.Memo,
			&sourceType, &sourceID, &e.Status, &reversedBy, &e.Scope)
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
		 WHERE branch_id = ? AND source_doc_type = ? AND source_doc_id = ? AND status = 'posted'`,
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

// LinesByEntryIDs resolves the lines of many entries in one grouped read,
// keyed by entry id.
//
// Reports walk a list of entries and need each one's legs. Doing that with
// GetEntry per entry costs two queries per row — a 10k-entry ledger became
// 20k round trips for data one JOIN already has. Ids are chunked at
// footingChunk so a wide range cannot build an unbounded IN (...) list.
func (r *Repository) LinesByEntryIDs(ctx context.Context, ids []int64) (map[int64][]*contracts.JournalLine, error) {
	out := map[int64][]*contracts.JournalLine{}
	seen := map[int64]bool{}
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
		batch := uniq[start:end]
		args := make([]any, len(batch))
		for i, id := range batch {
			args[i] = id
		}
		placeholders := strings.Repeat("?,", len(batch)-1) + "?"
		if err := r.appendLines(ctx, out, placeholders, args); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// appendLines reads one chunk into out. Rows are fully drained before
// returning: the connection must be free before the next chunk asks for it.
func (r *Repository) appendLines(ctx context.Context, out map[int64][]*contracts.JournalLine, placeholders string, args []any) error {
	rows, err := r.db.QueryContext(ctx,
		`SELECT l.entry_id, l.account_id, a.code, a.name, l.debit, l.credit,
			l.product_id, l.variant_id, l.party_id, l.description
		 FROM finance_journal_lines l JOIN finance_accounts a ON a.id = l.account_id
		 WHERE l.entry_id IN (`+placeholders+`) ORDER BY l.entry_id ASC, l.id ASC`, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var entryID int64
		var l contracts.JournalLine
		var productID, variantID, partyID sql.NullInt64
		var description sql.NullString
		if err := rows.Scan(&entryID, &l.AccountID, &l.AccountCode, &l.AccountName,
			&l.Debit, &l.Credit, &productID, &variantID, &partyID, &description); err != nil {
			return err
		}
		l.ProductID, l.VariantID, l.PartyID = productID.Int64, variantID.Int64, partyID.Int64
		l.Description = description.String
		line := l
		out[entryID] = append(out[entryID], &line)
	}
	return rows.Err()
}

// MarkReversed flags an entry reversed by another.
func (r *Repository) MarkReversed(ctx context.Context, id, byID int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE finance_journal_entries SET status = 'reversed', reversed_by = ? WHERE id = ?",
		byID, id)
	return err
}

// LedgerRow is one posted leg on an account with its running net (debit -
// credit) over the filtered range, in date order.
type LedgerRow struct {
	EntryID int64
	Number  string
	Date    string
	Memo    string
	Debit   int64
	Credit  int64
	Running int64
	// Dimensions of the leg (FN-08): counterparty / product, and the leg's own
	// description. Zero / empty when the leg carries none.
	PartyID     int64
	ProductID   int64
	VariantID   int64
	Description string
}

// LedgerPage returns a window of an account's legs ordered by date, entry,
// line, with the running net computed in SQL (window function) — the whole
// range is never loaded into memory. total is the exact leg count.
func (r *Repository) LedgerPage(ctx context.Context, branchID, accountID int64, from, to string, limit, offset int) ([]LedgerRow, int64, error) {
	ids := branches(ctx, branchID)
	conds := "e.branch_id IN (" + strings.Repeat("?,", len(ids)-1) + "?) AND l.account_id = ?" + commercialOnly
	args := make([]any, 0, len(ids)+4)
	for _, id := range ids {
		args = append(args, id)
	}
	args = append(args, accountID)
	if from != "" {
		conds += " AND e.entry_date >= ?"
		args = append(args, from)
	}
	if to != "" {
		conds += " AND e.entry_date <= ?"
		args = append(args, to)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM finance_journal_lines l JOIN finance_journal_entries e ON e.id = l.entry_id WHERE `+conds,
		args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, number, DATE_FORMAT(entry_date, '%Y-%m-%d'), memo, debit, credit, running,
		        party_id, product_id, variant_id, description FROM (
		   SELECT e.id, e.number, e.entry_date, e.memo, l.id AS line_id, l.debit, l.credit,
		          COALESCE(l.party_id, 0) AS party_id, COALESCE(l.product_id, 0) AS product_id,
		          COALESCE(l.variant_id, 0) AS variant_id, COALESCE(l.description, '') AS description,
		          SUM(l.debit - l.credit) OVER (ORDER BY e.entry_date, e.id, l.id
		            ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS running
		     FROM finance_journal_lines l JOIN finance_journal_entries e ON e.id = l.entry_id
		    WHERE `+conds+`
		 ) t ORDER BY entry_date, id, line_id LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	var out []LedgerRow
	for rows.Next() {
		var l LedgerRow
		if err := rows.Scan(&l.EntryID, &l.Number, &l.Date, &l.Memo, &l.Debit, &l.Credit, &l.Running,
			&l.PartyID, &l.ProductID, &l.VariantID, &l.Description); err != nil {
			return nil, 0, err
		}
		out = append(out, l)
	}
	return out, total, rows.Err()
}

// ListEntries searches a branch's entries newest-first. excludeFiscal drops
// fiscal-adjustment journals (source_doc_type 'tax_adjustment'): commercial
// reports must never see them, while the journal browser keeps them visible.
func (r *Repository) ListEntries(ctx context.Context, branchID int64, from, to string, accountID int64, limit, offset int, skipFiscal bool, f ...EntryFilter) ([]*contracts.JournalEntry, error) {
	conds := "e.branch_id = ?"
	args := []any{branchID}
	if len(f) > 0 {
		c, a := f[0].where()
		conds += c
		args = append(args, a...)
	}
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
func (r *Repository) CountEntries(ctx context.Context, branchID int64, from, to string, accountID int64, f ...EntryFilter) (int64, error) {
	conds := "e.branch_id = ?"
	args := []any{branchID}
	if len(f) > 0 {
		c, a := f[0].where()
		conds += c
		args = append(args, a...)
	}
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

// EntryFilter narrows the journal browser: text in number/memo, the source
// document type, and the entry status.
type EntryFilter struct {
	Q          string
	SourceType string
	Status     string
}

func (f EntryFilter) where() (string, []any) {
	var conds string
	var args []any
	if q := strings.TrimSpace(f.Q); q != "" {
		conds += " AND (e.number LIKE ? OR e.memo LIKE ?)"
		like := "%" + q + "%"
		args = append(args, like, like)
	}
	if f.SourceType != "" {
		conds += " AND e.source_doc_type = ?"
		args = append(args, f.SourceType)
	}
	if f.Status != "" {
		conds += " AND e.status = ?"
		args = append(args, f.Status)
	}
	return conds, args
}

// commercialOnly is the shared fiscal-exclusion fragment. Commercial books
// never see tax-adjustment journals (they live in the same ledger for audit,
// but every commercial footing/ledger excludes them by construction — one
// forgotten filter is all it takes to misstate profit, so the rule lives
// here, not in callers).
const commercialOnly = ` AND e.scope = 0`

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
	ids := branches(ctx, branchID)
	if len(ids) == 1 {
		return r.accountFootingsOne(ctx, ids[0], from, to)
	}
	out := map[int64][2]int64{}
	for _, b := range ids {
		one, err := r.accountFootingsOne(ctx, b, from, to)
		if err != nil {
			return nil, err
		}
		for acc, f := range one {
			cur := out[acc]
			out[acc] = [2]int64{cur[0] + f[0], cur[1] + f[1]}
		}
	}
	return out, nil
}

func (r *Repository) accountFootingsOne(ctx context.Context, branchID int64, from, to string) (map[int64][2]int64, error) {
	conds := "branch_id = ? AND scope = 0"
	args := []any{branchID}
	if from != "" {
		conds += " AND entry_date >= ?"
		args = append(args, from[:min(10, len(from))])
	}
	if to != "" {
		conds += " AND entry_date <= ?"
		args = append(args, to[:min(10, len(to))])
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT account_id, COALESCE(SUM(debit), 0), COALESCE(SUM(credit), 0)
		 FROM finance_account_daily
		 WHERE `+conds+` GROUP BY account_id`, args...)
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
	var out []ProductFooting
	for _, b := range branches(ctx, branchID) {
		one, err := r.productFootingsOne(ctx, b, from, to, accountIDs)
		if err != nil {
			return nil, err
		}
		out = append(out, one...)
	}
	return out, nil
}

func (r *Repository) productFootingsOne(ctx context.Context, branchID int64, from, to string, accountIDs []int64) ([]ProductFooting, error) {
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
	ids := branches(ctx, branchID)
	if len(ids) == 1 {
		return r.partyFootingsOne(ctx, ids[0], accountID, asOf)
	}
	merged := map[int64]*PartyFooting{}
	var order []int64
	for _, b := range ids {
		one, err := r.partyFootingsOne(ctx, b, accountID, asOf)
		if err != nil {
			return nil, err
		}
		for _, f := range one {
			cur, ok := merged[f.PartyID]
			if !ok {
				c := f
				merged[f.PartyID] = &c
				order = append(order, f.PartyID)
				continue
			}
			cur.Debit += f.Debit
			cur.Credit += f.Credit
		}
	}
	out := make([]PartyFooting, 0, len(order))
	for _, id := range order {
		out = append(out, *merged[id])
	}
	return out, nil
}

func (r *Repository) partyFootingsOne(ctx context.Context, branchID, accountID int64, asOf string) ([]PartyFooting, error) {
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
	bc, args := BranchCond(ctx, branchID, "e.branch_id")
	conds := bc + " AND e.scope = 1"
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

// PostedSourceIDsAfter pages the source ids with a live (posted) journal of
// the type by ascending id — the other half of the orphan check.
func (r *Repository) PostedSourceIDsAfter(ctx context.Context, branchID int64, sourceType string, afterID int64, limit int) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT source_doc_id FROM finance_journal_entries
		 WHERE branch_id = ? AND source_doc_type = ? AND status = 'posted' AND source_doc_id > ?
		 ORDER BY source_doc_id ASC LIMIT ?`, branchID, sourceType, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ErrPeriodClosed means the entry's month was closed while the entry was
// being written (the period row lock serialises posting against closing).
var ErrPeriodClosed = errors.New("period closed")

// guardPeriodTx takes a SHARED lock on the entry month's period row inside the
// insert transaction. A concurrent safe-close holds the row FOR UPDATE, so a
// posting either lands before the close (and the gate sees it) or is refused
// after it — it can never slip into a month that was just sealed.
func guardPeriodTx(ctx context.Context, tx *sql.Tx, branchID int64, entryDate string) error {
	if len(entryDate) < 7 {
		return nil
	}
	year, month := entryDate[:4], entryDate[5:7]
	if _, err := tx.ExecContext(ctx,
		`INSERT IGNORE INTO finance_periods (branch_id, year, month, status) VALUES (?, ?, ?, 'open')`,
		branchID, year, month); err != nil {
		return err
	}
	var status string
	if err := tx.QueryRowContext(ctx,
		`SELECT status FROM finance_periods WHERE branch_id = ? AND year = ? AND month = ? FOR SHARE`,
		branchID, year, month).Scan(&status); err != nil {
		return err
	}
	if status == "closed" {
		return ErrPeriodClosed
	}
	return nil
}

// CloseWithGate seals a month atomically: it locks the period row FOR UPDATE,
// runs gate (the readiness evaluation, on other connections), and closes only
// if the gate passes — while any posting into the month waits on the lock.
func (r *Repository) CloseWithGate(ctx context.Context, branchID int64, year, month int, gate func() error, hook func(ctx context.Context, tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx,
		`INSERT IGNORE INTO finance_periods (branch_id, year, month, status) VALUES (?, ?, ?, 'open')`,
		branchID, year, month); err != nil {
		return err
	}
	var status string
	if err := tx.QueryRowContext(ctx,
		`SELECT status FROM finance_periods WHERE branch_id = ? AND year = ? AND month = ? FOR UPDATE`,
		branchID, year, month).Scan(&status); err != nil {
		return err
	}
	if status == "closed" {
		return ErrPeriodClosed
	}
	if err := gate(); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE finance_periods SET status = 'closed', closed_at = UTC_TIMESTAMP(6)
		 WHERE branch_id = ? AND year = ? AND month = ?`, branchID, year, month); err != nil {
		return err
	}
	if hook != nil {
		if err := hook(ctx, tx); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// PostedAmong returns which of ids already have a live journal of sourceType
// (posted, or reversed with requeue=0 — a cancelled document). One indexed IN
// query per page of the queue scan.
func (r *Repository) PostedAmong(ctx context.Context, branchID int64, sourceType string, ids []int64) (map[int64]bool, error) {
	out := map[int64]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	args := []any{branchID, sourceType}
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT e.source_doc_id FROM finance_journal_entries e
		 WHERE e.branch_id = ? AND e.source_doc_type = ?
		   AND e.source_doc_id IN (`+strings.Repeat("?,", len(ids)-1)+`?)
		   AND (e.status = 'posted' OR (e.status = 'reversed' AND e.requeue = 0))`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// Cutovers returns the covered-through id per document type for a branch.
func (r *Repository) Cutovers(ctx context.Context, branchID int64) (map[string]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT doc_type, covered_through_id FROM finance_cutover WHERE branch_id = ?", branchID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]int64{}
	for rows.Next() {
		var t string
		var id int64
		if err := rows.Scan(&t, &id); err != nil {
			return nil, err
		}
		out[t] = id
	}
	return out, rows.Err()
}

// SetCutover upserts one boundary.
func (r *Repository) SetCutover(ctx context.Context, branchID int64, docType string, coveredThrough, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO finance_cutover (branch_id, doc_type, covered_through_id, set_by)
		 VALUES (?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE covered_through_id = VALUES(covered_through_id), set_by = VALUES(set_by), set_at = UTC_TIMESTAMP(6)`,
		branchID, docType, coveredThrough, actorID)
	return err
}

// MinPostedSourceID returns the smallest source id with a live journal of the
// type at or below limit (0 when none) — a cutover must not claim documents
// this system already journalled itself.
func (r *Repository) MinPostedSourceID(ctx context.Context, branchID int64, docType string, limit int64) (int64, error) {
	var id sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		`SELECT MIN(source_doc_id) FROM finance_journal_entries
		 WHERE branch_id = ? AND source_doc_type = ? AND status = 'posted' AND source_doc_id <= ?`,
		branchID, docType, limit).Scan(&id)
	return id.Int64, err
}

// PostedSourceIDs returns the set of source document IDs already journaled
// for one source type — the journal side of the derived posting queue.
// One grouped query per type; the queue diffs it against module candidates
// in memory (P3: no per-document EntryBySource loop).
func (r *Repository) PostedSourceIDs(ctx context.Context, branchID int64, sourceType string) (map[int64]bool, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT e.source_doc_id FROM finance_journal_entries e
		 WHERE e.branch_id = ? AND e.source_doc_type = ? AND e.source_doc_id IS NOT NULL
		   AND (e.status = 'posted' OR (e.status = 'reversed' AND e.requeue = 0))`,
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
