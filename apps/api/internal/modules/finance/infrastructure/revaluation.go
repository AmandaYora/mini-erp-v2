package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/dbutil"
	"mini-erp/internal/shared/timeutil"
)

// RefInventoryRevaluation is the ledger ref_type of a revaluation row: a
// cost-only row (qty 0, signed value, synthetic movement id) whose ref_id is
// the finance_inventory_revaluations header. Like an opening_cost row it moves
// value, never quantity, so every later outflow is priced at the new average.
const RefInventoryRevaluation = "inventory_revaluation"

// Revaluation is one committed inventory revaluation (one journal).
type Revaluation struct {
	ID         int64
	BranchID   int64
	Date       string
	Reason     string
	TotalDelta int64
	JournalID  int64
	JournalNo  string
	ClientRef  string
	CreatedBy  int64
	CreatedAt  string // WIB wall clock (API wire form)
	Lines      []RevaluationLine
}

// RevaluationLine is one position of a revaluation.
type RevaluationLine struct {
	ProductID      int64
	VariantID      int64
	Qty            float64
	UnitCostBefore float64
	UnitCostAfter  float64
	ValueBefore    int64
	ValueAfter     int64
	Delta          int64
	// CostMovementID is the synthetic stock_movement_id of its ledger row.
	CostMovementID int64
}

// ErrRevaluationClientRef means another request with the same client_ref
// committed first (unique key): the caller reads that one back.
var ErrRevaluationClientRef = errors.New("revaluation client_ref already used")

// CommitRevaluation writes a revaluation in ONE transaction: the header, the
// journal (source inventory_revaluation, source id = the header), one
// cost-only ledger row per line, the lines, and the audit row (ADR-0011). Any
// failure rolls everything back, so the books and the cost ledger always move
// together (ADR-0020).
func (r *Repository) CommitRevaluation(ctx context.Context, rev *Revaluation, entry *contracts.JournalEntry, actorID int64,
	audit func(ctx context.Context, tx auditcontracts.AuditTx, id, entryID int64, number string) error) (int64, int64, error) {
	var id, entryID int64
	err := r.WithTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO finance_inventory_revaluations
			 (branch_id, revaluation_date, reason, total_delta, client_ref, created_by)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			rev.BranchID, rev.Date, rev.Reason, rev.TotalDelta, dbutil.NullIfEmpty(rev.ClientRef), dbutil.NullIfZero(actorID))
		if err != nil {
			if dberr.IsDuplicate(err) && rev.ClientRef != "" {
				return ErrRevaluationClientRef
			}
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}
		entry.SourceID = id
		if entryID, err = createEntryTx(ctx, tx, entry, actorID); err != nil {
			return err
		}
		for _, l := range rev.Lines {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO finance_inventory_cost_movements
				 (branch_id, product_id, variant_id, stock_movement_id, ref_type, ref_id, direction,
				  qty_base, unit_cost, total_cost, is_estimated, moved_at)
				 VALUES (?, ?, ?, ?, ?, ?, 'in', 0, NULL, ?, 0, UTC_TIMESTAMP(6))`,
				rev.BranchID, l.ProductID, l.VariantID, l.CostMovementID, RefInventoryRevaluation, id, l.Delta); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO finance_inventory_revaluation_lines
				 (revaluation_id, product_id, variant_id, qty_base, unit_cost_before, unit_cost_after,
				  value_before, value_after, delta, cost_movement_id)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				id, l.ProductID, l.VariantID, l.Qty, l.UnitCostBefore, l.UnitCostAfter,
				l.ValueBefore, l.ValueAfter, l.Delta, l.CostMovementID); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE finance_inventory_revaluations SET journal_entry_id = ? WHERE id = ?", entryID, id); err != nil {
			return err
		}
		return audit(ctx, tx, id, entryID, entry.Number)
	})
	if err != nil {
		return 0, 0, err
	}
	return id, entryID, nil
}

const revaluationCols = `v.id, v.branch_id, v.revaluation_date, v.reason, v.total_delta,
	COALESCE(v.journal_entry_id, 0), COALESCE(e.number, ''), COALESCE(v.client_ref, ''),
	COALESCE(v.created_by, 0), v.created_at`

func scanRevaluation(sc interface{ Scan(...any) error }) (*Revaluation, error) {
	var v Revaluation
	var date, created sql.NullTime
	if err := sc.Scan(&v.ID, &v.BranchID, &date, &v.Reason, &v.TotalDelta, &v.JournalID, &v.JournalNo,
		&v.ClientRef, &v.CreatedBy, &created); err != nil {
		return nil, err
	}
	if date.Valid {
		v.Date = date.Time.Format("2006-01-02")
	}
	if created.Valid {
		v.CreatedAt = created.Time.UTC().In(timeutil.Jakarta).Format("2006-01-02 15:04:05")
	}
	return &v, nil
}

// RevaluationByClientRef returns the branch's revaluation committed under
// clientRef (with its lines), or nil.
func (r *Repository) RevaluationByClientRef(ctx context.Context, branchID int64, clientRef string) (*Revaluation, error) {
	v, err := scanRevaluation(r.db.QueryRowContext(ctx,
		`SELECT `+revaluationCols+`
		 FROM finance_inventory_revaluations v LEFT JOIN finance_journal_entries e ON e.id = v.journal_entry_id
		 WHERE v.branch_id = ? AND v.client_ref = ?`, branchID, clientRef))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	lines, err := r.RevaluationLines(ctx, []int64{v.ID})
	if err != nil {
		return nil, err
	}
	v.Lines = lines[v.ID]
	return v, nil
}

// ListRevaluations pages a branch's revaluations newest first, with lines.
func (r *Repository) ListRevaluations(ctx context.Context, branchID int64, limit, offset int) ([]*Revaluation, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM finance_inventory_revaluations WHERE branch_id = ?", branchID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+revaluationCols+`
		 FROM finance_inventory_revaluations v LEFT JOIN finance_journal_entries e ON e.id = v.journal_entry_id
		 WHERE v.branch_id = ? ORDER BY v.id DESC LIMIT ? OFFSET ?`, branchID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	var out []*Revaluation
	for rows.Next() {
		v, err := scanRevaluation(rows)
		if err != nil {
			_ = rows.Close()
			return nil, 0, err
		}
		out = append(out, v)
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	if len(out) == 0 {
		return out, total, nil
	}
	ids := make([]int64, 0, len(out))
	for _, v := range out {
		ids = append(ids, v.ID)
	}
	lines, err := r.RevaluationLines(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	for _, v := range out {
		v.Lines = lines[v.ID]
	}
	return out, total, nil
}

// RevaluationLines reads the lines of the given revaluations in one query.
func (r *Repository) RevaluationLines(ctx context.Context, ids []int64) (map[int64][]RevaluationLine, error) {
	out := map[int64][]RevaluationLine{}
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT revaluation_id, product_id, variant_id, qty_base, unit_cost_before, unit_cost_after,
			value_before, value_after, delta, cost_movement_id
		 FROM finance_inventory_revaluation_lines
		 WHERE revaluation_id IN (`+strings.Repeat("?,", len(ids)-1)+`?)
		 ORDER BY revaluation_id, product_id, variant_id`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var rid int64
		var l RevaluationLine
		var qty, before, after string
		if err := rows.Scan(&rid, &l.ProductID, &l.VariantID, &qty, &before, &after,
			&l.ValueBefore, &l.ValueAfter, &l.Delta, &l.CostMovementID); err != nil {
			return nil, err
		}
		l.Qty, l.UnitCostBefore, l.UnitCostAfter = dbutil.ParseDecimal(qty), dbutil.ParseDecimal(before), dbutil.ParseDecimal(after)
		out[rid] = append(out[rid], l)
	}
	return out, rows.Err()
}

// PositionsOfProduct reads every ledger position (one per variant) of one
// product: quantity and value.
func (r *Repository) PositionsOfProduct(ctx context.Context, branchID, productID int64) ([]CostPosition, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT variant_id,
			COALESCE(SUM(CASE WHEN direction = 'in' THEN qty_base ELSE -qty_base END), 0),
			COALESCE(SUM(total_cost), 0)
		 FROM finance_inventory_cost_movements WHERE branch_id = ? AND product_id = ?
		 GROUP BY variant_id ORDER BY variant_id`, branchID, productID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []CostPosition
	for rows.Next() {
		p := CostPosition{ProductID: productID}
		var qty, total string
		if err := rows.Scan(&p.VariantID, &qty, &total); err != nil {
			return nil, err
		}
		p.Qty, p.Total = dbutil.ParseDecimal(qty), dbutil.ParseDecimal(total)
		out = append(out, p)
	}
	return out, rows.Err()
}

// LastReceiptUnitCost returns the unit cost of the position's latest goods
// receipt in the ledger (what the books last paid per base unit), 0 = none.
func (r *Repository) LastReceiptUnitCost(ctx context.Context, branchID, productID, variantID int64) (float64, error) {
	var unit sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT unit_cost FROM finance_inventory_cost_movements
		 WHERE branch_id = ? AND product_id = ? AND variant_id = ? AND ref_type = 'goods_receipt'
		   AND direction = 'in' AND qty_base > 0
		 ORDER BY stock_movement_id DESC LIMIT 1`, branchID, productID, variantID).Scan(&unit)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return dbutil.ParseDecimal(unit.String), nil
}
