package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
)

// Tx is the transaction handle hooks receive. Application code names it through
// this alias so it never imports database/sql itself (ADR-0011).
type Tx = sql.Tx

// Posting ignores (FN-01) and tax-adjustment metadata (FN-07): small tables
// owned by finance. Ignoring a source document never touches the document's
// own module — the marker lives here, keyed by (branch, docType, docId).

// PostingIgnore is one document taken out of the posting queue on purpose.
type PostingIgnore struct {
	DocType   string
	DocID     int64
	Reason    string
	IgnoredBy int64
	IgnoredAt string
}

// IgnoreKey is the lookup key of PostingIgnores.
func IgnoreKey(docType string, docID int64) string {
	return docType + ":" + strconv.FormatInt(docID, 10)
}

// PostingIgnores returns every ignored document of a branch (the set is small:
// it holds only stuck documents an accountant dealt with by hand).
func (r *Repository) PostingIgnores(ctx context.Context, branchID int64) (map[string]PostingIgnore, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT doc_type, doc_id, reason, COALESCE(ignored_by, 0), ignored_at
		 FROM finance_posting_ignores WHERE branch_id = ?`, branchID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]PostingIgnore{}
	for rows.Next() {
		var p PostingIgnore
		var at sql.NullTime
		if err := rows.Scan(&p.DocType, &p.DocID, &p.Reason, &p.IgnoredBy, &at); err != nil {
			return nil, err
		}
		if at.Valid {
			p.IgnoredAt = at.Time.Format("2006-01-02 15:04:05")
		}
		out[IgnoreKey(p.DocType, p.DocID)] = p
	}
	return out, rows.Err()
}

// IgnoreSource marks one document ignored (idempotent: a second call keeps the
// first reason and writes no second audit row). hook runs in the same
// transaction only when this call created the marker.
func (r *Repository) IgnoreSource(ctx context.Context, branchID int64, docType string, docID int64, reason string, actorID int64, hook func(ctx context.Context, tx *sql.Tx) error) (bool, error) {
	created := false
	err := r.WithTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`INSERT IGNORE INTO finance_posting_ignores (branch_id, doc_type, doc_id, reason, ignored_by)
			 VALUES (?, ?, ?, ?, ?)`, branchID, docType, docID, reason, actorID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return nil
		}
		created = true
		if hook != nil {
			return hook(ctx, tx)
		}
		return nil
	})
	return created, err
}

// RestoreSource removes the marker. Returns the removed row (nil when the
// document was not ignored); hook runs in the same transaction on removal.
func (r *Repository) RestoreSource(ctx context.Context, branchID int64, docType string, docID int64, hook func(ctx context.Context, tx *sql.Tx, was PostingIgnore) error) (*PostingIgnore, error) {
	var was *PostingIgnore
	err := r.WithTx(ctx, func(tx *sql.Tx) error {
		var p PostingIgnore
		err := tx.QueryRowContext(ctx,
			`SELECT doc_type, doc_id, reason, COALESCE(ignored_by, 0) FROM finance_posting_ignores
			 WHERE branch_id = ? AND doc_type = ? AND doc_id = ? FOR UPDATE`, branchID, docType, docID).
			Scan(&p.DocType, &p.DocID, &p.Reason, &p.IgnoredBy)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM finance_posting_ignores WHERE branch_id = ? AND doc_type = ? AND doc_id = ?`,
			branchID, docType, docID); err != nil {
			return err
		}
		was = &p
		if hook != nil {
			return hook(ctx, tx, p)
		}
		return nil
	})
	return was, err
}

// --- tax adjustment metadata (FN-07) ----------------------------------------

// TaxAdjustMeta classifies one fiscal-correction journal and carries the
// accountant's note. Attachments live in the media module (owner
// tax_adjustment/<entry id>).
type TaxAdjustMeta struct {
	EntryID        int64
	Classification string
	Note           string
}

// InsertTaxAdjustMetaTx stores the metadata inside the journal's transaction.
func InsertTaxAdjustMetaTx(ctx context.Context, tx *sql.Tx, m TaxAdjustMeta) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO finance_tax_adjustment_meta (entry_id, classification, note) VALUES (?, ?, ?)`,
		m.EntryID, m.Classification, nullable(m.Note))
	return err
}

func nullable(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// TaxAdjustMetaByEntries resolves the metadata of many entries in one read.
func (r *Repository) TaxAdjustMetaByEntries(ctx context.Context, ids []int64) (map[int64]TaxAdjustMeta, error) {
	out := map[int64]TaxAdjustMeta{}
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT entry_id, classification, COALESCE(note, '') FROM finance_tax_adjustment_meta
		 WHERE entry_id IN (`+strings.Repeat("?,", len(ids)-1)+`?)`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var m TaxAdjustMeta
		if err := rows.Scan(&m.EntryID, &m.Classification, &m.Note); err != nil {
			return nil, err
		}
		out[m.EntryID] = m
	}
	return out, rows.Err()
}

// ClassFooting is the debit/credit footing of fiscal corrections of one class
// on one account. A reversal mirror counts under the class of the entry it
// reverses, so a cancelled correction nets to zero inside its own class.
type ClassFooting struct {
	Classification string // "" = journal without metadata (booked before FN-07)
	AccountID      int64
	Debit          int64
	Credit         int64
	Entries        int64
}

// TaxAdjustmentFootingsByClass mirrors TaxAdjustmentFootings, split by class.
func (r *Repository) TaxAdjustmentFootingsByClass(ctx context.Context, branchID int64, from, to string) ([]ClassFooting, error) {
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
		`SELECT COALESCE(m.classification, ''), l.account_id,
			COALESCE(SUM(l.debit), 0), COALESCE(SUM(l.credit), 0), COUNT(DISTINCT e.id)
		 FROM finance_journal_lines l
		 JOIN finance_journal_entries e ON e.id = l.entry_id
		 LEFT JOIN finance_tax_adjustment_meta m
		   ON m.entry_id = CASE WHEN e.source_doc_type = 'reversal' THEN e.source_doc_id ELSE e.id END
		 WHERE `+conds+`
		 GROUP BY COALESCE(m.classification, ''), l.account_id`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []ClassFooting
	for rows.Next() {
		var f ClassFooting
		if err := rows.Scan(&f.Classification, &f.AccountID, &f.Debit, &f.Credit, &f.Entries); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// --- daily close overview (FN-02) --------------------------------------------

// DaySourceFooting is the footing of the live commercial journals of one
// source type on one account within a time window.
type DaySourceFooting struct {
	SourceType string
	AccountID  int64
	Debit      int64
	Credit     int64
}

// DaySourceFootings sums, per source type and account, the live (posted)
// commercial journals dated within [from, to], plus the number of journals per
// source type. A cancelled document's journal is reversed: the original is no
// longer 'posted' and the mirror carries source type 'reversal', so a cancelled
// document contributes nothing.
func (r *Repository) DaySourceFootings(ctx context.Context, branchID int64, from, to string) ([]DaySourceFooting, map[string]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT COALESCE(e.source_doc_type, ''), l.account_id,
			COALESCE(SUM(l.debit), 0), COALESCE(SUM(l.credit), 0)
		 FROM finance_journal_lines l
		 JOIN finance_journal_entries e ON e.id = l.entry_id
		 WHERE e.branch_id = ? AND e.status = 'posted' AND e.scope = 0
		   AND e.entry_date >= ? AND e.entry_date <= ?
		 GROUP BY COALESCE(e.source_doc_type, ''), l.account_id`, branchID, from, to)
	if err != nil {
		return nil, nil, err
	}
	var out []DaySourceFooting
	for rows.Next() {
		var f DaySourceFooting
		if err := rows.Scan(&f.SourceType, &f.AccountID, &f.Debit, &f.Credit); err != nil {
			_ = rows.Close()
			return nil, nil, err
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, nil, err
	}
	_ = rows.Close()
	cnt, err := r.db.QueryContext(ctx,
		`SELECT COALESCE(e.source_doc_type, ''), COUNT(*) FROM finance_journal_entries e
		 WHERE e.branch_id = ? AND e.status = 'posted' AND e.scope = 0
		   AND e.entry_date >= ? AND e.entry_date <= ?
		 GROUP BY COALESCE(e.source_doc_type, '')`, branchID, from, to)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = cnt.Close() }()
	counts := map[string]int64{}
	for cnt.Next() {
		var t string
		var n int64
		if err := cnt.Scan(&t, &n); err != nil {
			return nil, nil, err
		}
		counts[t] = n
	}
	return out, counts, cnt.Err()
}

// --- general ledger of every account (FN-04 package sheet, FN-08) -----------

// LedgerAllRow is one posted commercial leg with its account, for the
// all-accounts ledger.
type LedgerAllRow struct {
	BranchID    int64
	AccountID   int64
	EntryID     int64
	Number      string
	Date        string
	Memo        string
	Debit       int64
	Credit      int64
	PartyID     int64
	ProductID   int64
	VariantID   int64
	Description string
}

// ErrLedgerTooLarge means the range holds more legs than the caller allowed.
var ErrLedgerTooLarge = errors.New("buku besar melebihi batas baris")

// LedgerLinesAll lists every commercial leg in the window ordered by account,
// date, entry, line. max bounds the result: more rows than that is
// ErrLedgerTooLarge, never a silent truncation.
func (r *Repository) LedgerLinesAll(ctx context.Context, branchID int64, from, to string, max int) ([]LedgerAllRow, error) {
	ids := branches(ctx, branchID)
	conds := "e.branch_id IN (" + strings.Repeat("?,", len(ids)-1) + "?)" + commercialOnly
	args := make([]any, 0, len(ids)+3)
	for _, id := range ids {
		args = append(args, id)
	}
	if from != "" {
		conds += " AND e.entry_date >= ?"
		args = append(args, from)
	}
	if to != "" {
		conds += " AND e.entry_date <= ?"
		args = append(args, to)
	}
	args = append(args, max+1)
	rows, err := r.db.QueryContext(ctx,
		`SELECT e.branch_id, l.account_id, e.id, e.number, DATE_FORMAT(e.entry_date, '%Y-%m-%d'), e.memo, l.debit, l.credit,
			COALESCE(l.party_id, 0), COALESCE(l.product_id, 0), COALESCE(l.variant_id, 0), COALESCE(l.description, '')
		 FROM finance_journal_lines l JOIN finance_journal_entries e ON e.id = l.entry_id
		 WHERE `+conds+` ORDER BY l.account_id, e.entry_date, e.id, l.id LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []LedgerAllRow
	for rows.Next() {
		var l LedgerAllRow
		if err := rows.Scan(&l.BranchID, &l.AccountID, &l.EntryID, &l.Number, &l.Date, &l.Memo, &l.Debit, &l.Credit,
			&l.PartyID, &l.ProductID, &l.VariantID, &l.Description); err != nil {
			return nil, err
		}
		out = append(out, l)
		if len(out) > max {
			return nil, ErrLedgerTooLarge
		}
	}
	return out, rows.Err()
}

// TaxAdjustmentCountsByClass counts the live correction journals (mirrors of
// reversals excluded) per class inside the window.
func (r *Repository) TaxAdjustmentCountsByClass(ctx context.Context, branchID int64, from, to string) (map[string]int64, error) {
	bc, args := BranchCond(ctx, branchID, "e.branch_id")
	conds := bc + " AND e.scope = 1 AND e.source_doc_type = 'tax_adjustment' AND e.status = 'posted'"
	if from != "" {
		conds += " AND e.entry_date >= ?"
		args = append(args, from)
	}
	if to != "" {
		conds += " AND e.entry_date <= ?"
		args = append(args, to)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT COALESCE(m.classification, ''), COUNT(*) FROM finance_journal_entries e
		 LEFT JOIN finance_tax_adjustment_meta m ON m.entry_id = e.id
		 WHERE `+conds+` GROUP BY COALESCE(m.classification, '')`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]int64{}
	for rows.Next() {
		var c string
		var n int64
		if err := rows.Scan(&c, &n); err != nil {
			return nil, err
		}
		out[c] = n
	}
	return out, rows.Err()
}
