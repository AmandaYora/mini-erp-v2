package infrastructure

import (
	"context"
	"database/sql"
	"fmt"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/shared/dbutil"
)

// RestateRow is one cost ledger row as the restatement tool reads it (OP-04).
type RestateRow struct {
	ID         int64
	ProductID  int64
	VariantID  int64
	MovementID int64
	Direction  string
	RefType    string
	RefID      int64
	Qty        float64
	Unit       float64
	Total      float64
}

// CostRowsOrdered returns every ledger row of a branch in chain order: per
// (product, variant), by stock movement id. Synthetic cost-only rows carry ids
// above every real movement, so they land after the movements they were
// applied to — the order they actually took effect in.
func (r *Repository) CostRowsOrdered(ctx context.Context, branchID int64) ([]RestateRow, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, product_id, variant_id, stock_movement_id, direction, ref_type, COALESCE(ref_id, 0),
			qty_base, unit_cost, total_cost
		 FROM finance_inventory_cost_movements WHERE branch_id = ?
		 ORDER BY product_id, variant_id, stock_movement_id`, branchID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []RestateRow
	for rows.Next() {
		var x RestateRow
		var qty string
		var unit, total sql.NullString
		if err := rows.Scan(&x.ID, &x.ProductID, &x.VariantID, &x.MovementID, &x.Direction, &x.RefType, &x.RefID, &qty, &unit, &total); err != nil {
			return nil, err
		}
		x.Qty, x.Unit, x.Total = dbutil.ParseDecimal(qty), dbutil.ParseDecimal(unit.String), dbutil.ParseDecimal(total.String)
		out = append(out, x)
	}
	return out, rows.Err()
}

// RestateUpdate is the new price of one ledger row. Only unit/total change:
// quantity and identity never do.
type RestateUpdate struct {
	ID    int64
	Unit  float64
	Total float64
}

// TrialBalanceError is the commit gate refusing: the books do not foot once the
// correction is in place, so the whole restatement rolled back.
type TrialBalanceError struct{ Debit, Credit, Unbalanced int64 }

func (e *TrialBalanceError) Error() string {
	return fmt.Sprintf("neraca saldo tidak seimbang setelah restatement (debit %d, kredit %d, jurnal timpang %d)", e.Debit, e.Credit, e.Unbalanced)
}

// CommitRestatement applies a cost restatement in ONE transaction: the ledger
// rows are rewritten, the correction journal (nil = sub-rupiah drift, none
// needed) is posted, the trial balance is footed as the hard gate, and the
// audit row co-commits (ADR-0011). Any failure rolls everything back.
func (r *Repository) CommitRestatement(ctx context.Context, branchID, actorID int64, entry *contracts.JournalEntry, updates []RestateUpdate,
	audit func(ctx context.Context, tx auditcontracts.AuditTx, entryID int64, number string) error) (int64, error) {
	var entryID int64
	err := r.WithTx(ctx, func(tx *sql.Tx) error {
		if entry != nil {
			id, err := createEntryTx(ctx, tx, entry, actorID)
			if err != nil {
				return err
			}
			entryID = id
		}
		for _, u := range updates {
			if _, err := tx.ExecContext(ctx,
				"UPDATE finance_inventory_cost_movements SET unit_cost = ?, total_cost = ? WHERE id = ?", u.Unit, u.Total, u.ID); err != nil {
				return err
			}
		}
		d, c, bad, err := trialBalance(ctx, tx, branchID)
		if err != nil {
			return err
		}
		if d != c || bad != 0 {
			return &TrialBalanceError{Debit: d, Credit: c, Unbalanced: bad}
		}
		number := ""
		if entry != nil {
			number = entry.Number
		}
		return audit(ctx, tx, entryID, number)
	})
	return entryID, err
}

// rowQuerier is what *sql.DB and *sql.Tx share.
type rowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func trialBalance(ctx context.Context, q rowQuerier, branchID int64) (debit, credit, unbalanced int64, err error) {
	if err = q.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(l.debit), 0), COALESCE(SUM(l.credit), 0)
		 FROM finance_journal_lines l JOIN finance_journal_entries e ON e.id = l.entry_id
		 WHERE e.branch_id = ?`, branchID).Scan(&debit, &credit); err != nil {
		return
	}
	err = q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM (
			SELECT l.entry_id FROM finance_journal_lines l
			JOIN finance_journal_entries e ON e.id = l.entry_id
			WHERE e.branch_id = ? GROUP BY l.entry_id
			HAVING COALESCE(SUM(l.debit), 0) <> COALESCE(SUM(l.credit), 0)
		) u`, branchID).Scan(&unbalanced)
	return
}

// TrialBalance foots every journal line of a branch: total debit, total credit
// and the number of entries that do not net to zero on their own.
func (r *Repository) TrialBalance(ctx context.Context, branchID int64) (debit, credit, unbalanced int64, err error) {
	return trialBalance(ctx, r.db, branchID)
}

// NextSourceID is the next source_doc_id for a typed source (max + 1, 1 when
// none): journals that have no source document of their own still need a
// unique key.
func (r *Repository) NextSourceID(ctx context.Context, branchID int64, sourceType string) (int64, error) {
	var max sql.NullInt64
	if err := r.db.QueryRowContext(ctx,
		"SELECT MAX(source_doc_id) FROM finance_journal_entries WHERE branch_id = ? AND source_doc_type = ?",
		branchID, sourceType).Scan(&max); err != nil {
		return 0, err
	}
	return max.Int64 + 1, nil
}
