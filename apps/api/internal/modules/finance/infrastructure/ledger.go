package infrastructure

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/shared/dbutil"
)

// CostPosition is the running quantity/value of one (product, variant).
type CostPosition struct {
	ProductID int64
	VariantID int64
	Qty       float64
	Total     float64
}

// Average returns the moving-average unit cost (0 when no quantity).
func (p CostPosition) Average() float64 {
	if p.Qty <= 0 {
		return 0
	}
	return p.Total / p.Qty
}

// CostRow is one cost ledger row.
type CostRow struct {
	MovementID int64
	Direction  string
	Qty        float64
	UnitCost   float64
	Total      float64
	Estimated  bool
}

// MaxCostMovementID returns the costing cursor (0 when nothing costed).
func (r *Repository) MaxCostMovementID(ctx context.Context, branchID int64) (int64, error) {
	var id sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		"SELECT MAX(stock_movement_id) FROM finance_inventory_cost_movements WHERE branch_id = ? AND stock_movement_id < ?",
		branchID, SyntheticCostBase).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id.Int64, nil
}

// InsertCostRow records one costed movement.
func (r *Repository) InsertCostRow(ctx context.Context, branchID, productID, variantID, movementID int64, direction string, qty, unit float64, estimated bool, refType string, refID int64, movedAt string) error {
	total := qty * unit
	if direction == "out" {
		total = -total
	}
	est := 0
	if estimated {
		est = 1
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO finance_inventory_cost_movements
		 (branch_id, product_id, variant_id, stock_movement_id, ref_type, ref_id, direction,
		  qty_base, unit_cost, total_cost, is_estimated, moved_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		branchID, productID, variantID, movementID, refType, dbutil.NullIfZero(refID), direction, qty, unit, total, est, dbutil.NullIfEmpty(normalizeStamp(movedAt)))
	return err
}

// normalizeStamp turns the driver's RFC3339 timestamp into a MySQL DATETIME
// literal (empty stays empty).
func normalizeStamp(v string) string {
	if v == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
		return t.UTC().Format("2006-01-02 15:04:05.000000")
	}
	return v
}

// StockMovementRow is one costed stock movement for the stock-difference journal.
type StockMovementRow struct {
	MovementID int64
	ProductID  int64
	VariantID  int64
	Direction  string
	RefType    string
	Qty        float64
	TotalCost  float64
	Estimated  bool
	MovedAt    string
}

// CostRowByMovement returns the cost ledger row of one stock movement, or nil.
func (r *Repository) CostRowByMovement(ctx context.Context, branchID, movementID int64) (*StockMovementRow, error) {
	var m StockMovementRow
	var qty, total string
	var est int
	var moved sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT stock_movement_id, product_id, variant_id, direction, ref_type, qty_base,
			total_cost, is_estimated, moved_at
		 FROM finance_inventory_cost_movements WHERE branch_id = ? AND stock_movement_id = ?`,
		branchID, movementID).Scan(&m.MovementID, &m.ProductID, &m.VariantID, &m.Direction, &m.RefType, &qty, &total, &est, &moved)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.Qty, m.TotalCost, m.Estimated, m.MovedAt = dbutil.ParseDecimal(qty), dbutil.ParseDecimal(total), est == 1, moved.String
	return &m, nil
}

// UnpostedStockMovements lists adjustment / write-off movements that have no
// live journal yet, oldest first, bounded. The anti-join stays inside finance's
// own tables (the cost ledger and the journal index).
func (r *Repository) UnpostedStockMovements(ctx context.Context, branchID, afterID int64, limit int) ([]StockMovementRow, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT c.stock_movement_id, c.product_id, c.variant_id, c.direction, c.ref_type,
			c.qty_base, c.total_cost, c.is_estimated, c.moved_at
		 FROM finance_inventory_cost_movements c
		 WHERE c.branch_id = ? AND c.ref_type IN ('adjustment', 'write_off')
		   AND c.stock_movement_id > ?
		   AND NOT EXISTS (SELECT 1 FROM finance_journal_entries e
		                    WHERE e.branch_id = c.branch_id AND e.source_doc_type = 'stock_movement'
		                      AND e.source_doc_id = c.stock_movement_id
		                      AND (e.status = 'posted' OR (e.status = 'reversed' AND e.requeue = 0)))
		 ORDER BY c.stock_movement_id ASC LIMIT ?`, branchID, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []StockMovementRow
	for rows.Next() {
		var m StockMovementRow
		var qty, total string
		var est int
		var moved sql.NullString
		if err := rows.Scan(&m.MovementID, &m.ProductID, &m.VariantID, &m.Direction, &m.RefType, &qty, &total, &est, &moved); err != nil {
			return nil, err
		}
		m.Qty, m.TotalCost, m.Estimated, m.MovedAt = dbutil.ParseDecimal(qty), dbutil.ParseDecimal(total), est == 1, moved.String
		out = append(out, m)
	}
	return out, rows.Err()
}

// MoveValue is the recorded cost of one product in one source document.
type MoveValue struct {
	Qty   float64
	Value float64
}

// MoveValues sums recorded movement costs per (product, variant) for one
// source document (one grouped query). Journals value a document's own
// movements at these historical costs instead of the live average: after an
// out-movement empties a position, the live average reads zero while the
// relieved units were really worth the pre-out average — which is exactly
// what their cost rows recorded.
func (r *Repository) MoveValues(ctx context.Context, branchID int64, refType string, refID int64) (map[string]MoveValue, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT product_id, variant_id,
			COALESCE(SUM(CASE WHEN direction = 'in' THEN qty_base ELSE -qty_base END), 0),
			COALESCE(SUM(total_cost), 0)
		 FROM finance_inventory_cost_movements
		 WHERE branch_id = ? AND ref_type IN (?, ?, ?) AND ref_id = ?
		 GROUP BY product_id, variant_id`, branchID, refType, refType+"_rollback", refType+"_cancel", refID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]MoveValue{}
	for rows.Next() {
		var pid, vid int64
		var qty, value string
		if err := rows.Scan(&pid, &vid, &qty, &value); err != nil {
			return nil, err
		}
		out[CostKeyOf(pid, vid)] = MoveValue{Qty: dbutil.ParseDecimal(qty), Value: dbutil.ParseDecimal(value)}
	}
	return out, rows.Err()
}

// LastInCosts returns the most recent explicit inflow unit cost per
// (product, variant) — what a return into an emptied position is worth.
func (r *Repository) LastInCosts(ctx context.Context, branchID int64) (map[string]float64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT c.product_id, c.variant_id, c.unit_cost
		 FROM finance_inventory_cost_movements c
		 JOIN (SELECT product_id, variant_id, MAX(stock_movement_id) AS mid
		         FROM finance_inventory_cost_movements
		        WHERE branch_id = ? AND direction = 'in' AND unit_cost > 0
		        GROUP BY product_id, variant_id) m
		   ON m.product_id = c.product_id AND m.variant_id = c.variant_id AND m.mid = c.stock_movement_id
		WHERE c.branch_id = ?`, branchID, branchID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]float64{}
	for rows.Next() {
		var pid, vid int64
		var unit string
		if err := rows.Scan(&pid, &vid, &unit); err != nil {
			return nil, err
		}
		out[CostKeyOf(pid, vid)] = dbutil.ParseDecimal(unit)
	}
	return out, rows.Err()
}

// CostPositions returns running qty/value per (product, variant).
func (r *Repository) CostPositions(ctx context.Context, branchID int64) (map[string]*CostPosition, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT product_id, variant_id,
			COALESCE(SUM(CASE WHEN direction = 'in' THEN qty_base ELSE -qty_base END), 0),
			COALESCE(SUM(total_cost), 0)
		 FROM finance_inventory_cost_movements WHERE branch_id = ? GROUP BY product_id, variant_id`,
		branchID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]*CostPosition{}
	for rows.Next() {
		var p CostPosition
		var qty, total string
		if err := rows.Scan(&p.ProductID, &p.VariantID, &qty, &total); err != nil {
			return nil, err
		}
		p.Qty = dbutil.ParseDecimal(qty)
		p.Total = dbutil.ParseDecimal(total)
		out[costKey(p.ProductID, p.VariantID)] = &p
	}
	return out, rows.Err()
}

// CostPositionsFor reads the running positions of just the given keys (the
// products a costing batch touches) — never the whole branch ledger.
func (r *Repository) CostPositionsFor(ctx context.Context, branchID int64, keys [][2]int64) (map[string]*CostPosition, error) {
	out := map[string]*CostPosition{}
	for _, k := range keys {
		p, err := r.CostPositionOf(ctx, branchID, k[0], k[1])
		if err != nil {
			return nil, err
		}
		out[CostKeyOf(k[0], k[1])] = p
	}
	return out, nil
}

// LastInCostsFor is LastInCosts restricted to the given keys.
func (r *Repository) LastInCostsFor(ctx context.Context, branchID int64, keys [][2]int64) (map[string]float64, error) {
	out := map[string]float64{}
	for _, k := range keys {
		var unit sql.NullString
		err := r.db.QueryRowContext(ctx,
			`SELECT unit_cost FROM finance_inventory_cost_movements
			 WHERE branch_id = ? AND product_id = ? AND variant_id = ? AND direction = 'in' AND unit_cost > 0
			 ORDER BY stock_movement_id DESC LIMIT 1`, branchID, k[0], k[1]).Scan(&unit)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		out[CostKeyOf(k[0], k[1])] = dbutil.ParseDecimal(unit.String)
	}
	return out, nil
}

// CostMovementIDsPresent returns which of the stock movement ids already have a
// ledger row.
func (r *Repository) CostMovementIDsPresent(ctx context.Context, branchID int64, ids []int64) (map[int64]bool, error) {
	out := map[int64]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	args := []any{branchID}
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx,
		"SELECT stock_movement_id FROM finance_inventory_cost_movements WHERE branch_id = ? AND stock_movement_id IN ("+
			strings.Repeat("?,", len(ids)-1)+"?)", args...)
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

// CostPositionOf reads one position's quantity and value.
func (r *Repository) CostPositionOf(ctx context.Context, branchID, productID, variantID int64) (*CostPosition, error) {
	var qty, total string
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(CASE WHEN direction = 'in' THEN qty_base ELSE -qty_base END), 0),
			COALESCE(SUM(total_cost), 0)
		 FROM finance_inventory_cost_movements WHERE branch_id = ? AND product_id = ? AND variant_id = ?`,
		branchID, productID, variantID).Scan(&qty, &total)
	if err != nil {
		return nil, err
	}
	return &CostPosition{ProductID: productID, VariantID: variantID, Qty: dbutil.ParseDecimal(qty), Total: dbutil.ParseDecimal(total)}, nil
}

func costKey(productID, variantID int64) string {
	return strconv.FormatInt(productID, 10) + "/" + strconv.FormatInt(variantID, 10)
}

// CostKeyOf exposes the position map key.
func CostKeyOf(productID, variantID int64) string {
	return costKey(productID, variantID)
}

// EstimatedProducts lists products holding estimated cost rows.
func (r *Repository) EstimatedProducts(ctx context.Context, branchID int64) ([][2]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT product_id, variant_id FROM finance_inventory_cost_movements
		 WHERE branch_id = ? AND is_estimated = 1`, branchID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out [][2]int64
	for rows.Next() {
		var p, v int64
		if err := rows.Scan(&p, &v); err != nil {
			return nil, err
		}
		out = append(out, [2]int64{p, v})
	}
	return out, rows.Err()
}

// --- expenses ---------------------------------------------------------------

// Expense is one business expense row.
type Expense struct {
	ID             int64
	Number         string
	BranchID       int64
	ExpenseAccount int64
	PayAccount     int64
	Amount         int64
	Date           string
	Notes          string
	JournalID      int64
	Status         string
	CancelReason   string
	// PartyID is the optional counterparty (FN-10); PartyName is resolved by the
	// service, never stored.
	PartyID   int64
	PartyName string
}

// ExpenseFilter narrows the expense list.
type ExpenseFilter struct {
	Q                string
	From, To         string
	Status           string
	ExpenseAccountID int64
	PayAccountID     int64
	PartyID          int64
}

func (f ExpenseFilter) where(branchID int64) (string, []any) {
	conds := "branch_id = ?"
	args := []any{branchID}
	if q := strings.TrimSpace(f.Q); q != "" {
		conds += " AND (number LIKE ? OR notes LIKE ?)"
		like := "%" + q + "%"
		args = append(args, like, like)
	}
	if f.From != "" {
		conds += " AND expense_date >= ?"
		args = append(args, f.From)
	}
	if f.To != "" {
		conds += " AND expense_date <= ?"
		args = append(args, f.To)
	}
	if f.Status != "" {
		conds += " AND status = ?"
		args = append(args, f.Status)
	}
	if f.ExpenseAccountID != 0 {
		conds += " AND expense_account_id = ?"
		args = append(args, f.ExpenseAccountID)
	}
	if f.PayAccountID != 0 {
		conds += " AND pay_account_id = ?"
		args = append(args, f.PayAccountID)
	}
	if f.PartyID != 0 {
		conds += " AND party_id = ?"
		args = append(args, f.PartyID)
	}
	return conds, args
}

// CreateExpenseWithEntry stores the expense row, its number and its journal in
// ONE transaction: no expense without a journal, no journal without an expense,
// no number burnt by a failure (N8).
func (r *Repository) CreateExpenseWithEntry(ctx context.Context, e *Expense, nb *contracts.Numbering, entry *contracts.JournalEntry, actorID int64, hook EntryHook) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	number, err := numberInTx(ctx, tx, e.BranchID, nb)
	if err != nil {
		return 0, err
	}
	e.Number = number
	res, err := tx.ExecContext(ctx,
		`INSERT INTO business_expenses
		 (number, branch_id, expense_account_id, pay_account_id, amount,
		  expense_date, notes, status, created_by, party_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 'posted', ?, ?)`,
		e.Number, e.BranchID, e.ExpenseAccount, e.PayAccount, e.Amount,
		e.Date, dbutil.NullIfEmpty(e.Notes), actorID, dbutil.NullIfZero(e.PartyID))
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	entry.SourceID = id
	entry.Memo = "Biaya " + number + ": " + strings.TrimSpace(e.Notes)
	entryID, err := createEntryTx(ctx, tx, entry, actorID)
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE business_expenses SET journal_entry_id = ? WHERE id = ?", entryID, id); err != nil {
		return 0, err
	}
	if hook != nil {
		if err := hook(ctx, tx, id, e.Number); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// CancelExpense marks a posted expense cancelled with its reason (CAS).
// The audit hook runs in the CAS transaction only when this call won, so a
// replay that lost the CAS never writes a second audit row (ADR-0011/0016).
func (r *Repository) CancelExpense(ctx context.Context, id int64, reason string, actorID int64, hook func(ctx context.Context, tx *sql.Tx) error) (bool, error) {
	won := false
	err := r.WithTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE business_expenses SET status = 'cancelled', cancel_reason = ?, cancelled_at = UTC_TIMESTAMP(6), cancelled_by = ?
		 WHERE id = ? AND status = 'posted'`, reason, actorID, id)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return nil
		}
		won = true
		if hook != nil {
			return hook(ctx, tx)
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return won, nil
}

// CreateExpense inserts a draft-posted expense row (journal linked after).
func (r *Repository) CreateExpense(ctx context.Context, e *Expense, actorID int64) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO business_expenses
		 (number, branch_id, expense_account_id, pay_account_id, amount,
		  expense_date, notes, status, created_by, party_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 'posted', ?, ?)`,
		e.Number, e.BranchID, e.ExpenseAccount, e.PayAccount, e.Amount,
		e.Date, dbutil.NullIfEmpty(e.Notes), actorID, dbutil.NullIfZero(e.PartyID))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// LinkExpenseJournal attaches the auto-posted entry.
func (r *Repository) LinkExpenseJournal(ctx context.Context, id, journalID int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE business_expenses SET journal_entry_id = ? WHERE id = ?", journalID, id)
	return err
}

// GetExpense returns nil, nil when missing.
func (r *Repository) GetExpense(ctx context.Context, id int64) (*Expense, error) {
	e := &Expense{}
	var date sql.NullTime
	var notes, cancelReason sql.NullString
	var journal, party sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		`SELECT id, number, branch_id, expense_account_id, pay_account_id,
			amount, expense_date, notes, journal_entry_id, status, cancel_reason, party_id
		 FROM business_expenses WHERE id = ?`, id).
		Scan(&e.ID, &e.Number, &e.BranchID, &e.ExpenseAccount, &e.PayAccount,
			&e.Amount, &date, &notes, &journal, &e.Status, &cancelReason, &party)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if date.Valid {
		e.Date = date.Time.Format("2006-01-02 15:04:05")
	}
	e.Notes = notes.String
	e.JournalID = journal.Int64
	e.CancelReason = cancelReason.String
	e.PartyID = party.Int64
	return e, nil
}

// SetExpenseStatus flips posted/cancelled.
func (r *Repository) SetExpenseStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE business_expenses SET status = ? WHERE id = ?", status, id)
	return err
}

// ListExpenses lists a branch's expenses newest-first.
func (r *Repository) ListExpenses(ctx context.Context, branchID int64, f ExpenseFilter, limit, offset int) ([]*Expense, error) {
	conds, args := f.where(branchID)
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, number, branch_id, expense_account_id, pay_account_id,
			amount, expense_date, notes, journal_entry_id, status, party_id
		 FROM business_expenses WHERE `+conds+` ORDER BY id DESC LIMIT ? OFFSET ?`,
		append(args, limit, offset)...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*Expense
	for rows.Next() {
		e := &Expense{}
		var date sql.NullTime
		var notes sql.NullString
		var journal, party sql.NullInt64
		if err := rows.Scan(&e.ID, &e.Number, &e.BranchID, &e.ExpenseAccount,
			&e.PayAccount, &e.Amount, &date, &notes, &journal, &e.Status, &party); err != nil {
			return nil, err
		}
		e.PartyID = party.Int64
		if date.Valid {
			e.Date = date.Time.Format("2006-01-02 15:04:05")
		}
		e.Notes = notes.String
		e.JournalID = journal.Int64
		out = append(out, e)
	}
	return out, rows.Err()
}

// CountExpenses counts the List filter set.
func (r *Repository) CountExpenses(ctx context.Context, branchID int64, f ExpenseFilter) (int64, error) {
	conds, args := f.where(branchID)
	var total int64
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM business_expenses WHERE "+conds, args...).Scan(&total)
	return total, err
}

// --- tax periods ------------------------------------------------------------

// TaxPeriod is one month's tax lock.
type TaxPeriod struct {
	ID     int64
	Year   int
	Month  int
	Status string // open | closed
}

// GetTaxPeriod returns nil, nil when untouched (treated open).
func (r *Repository) GetTaxPeriod(ctx context.Context, branchID int64, year, month int) (*TaxPeriod, error) {
	p := &TaxPeriod{}
	err := r.db.QueryRowContext(ctx,
		"SELECT id, year, month, status FROM finance_tax_periods WHERE branch_id = ? AND year = ? AND month = ?",
		branchID, year, month).Scan(&p.ID, &p.Year, &p.Month, &p.Status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

// SetTaxPeriodStatus inserts or updates a month lock.
// snapshot (non-empty, closing only) is stored in the same transaction; the
// hook receives the previous status and co-commits the audit row (ADR-0011).
func (r *Repository) SetTaxPeriodStatus(ctx context.Context, branchID int64, year, month int, status, snapshot string, actorID int64, hook func(ctx context.Context, tx *sql.Tx, before string) error) error {
	return r.WithTx(ctx, func(tx *sql.Tx) error {
		before := "open"
		err := tx.QueryRowContext(ctx,
			"SELECT status FROM finance_tax_periods WHERE branch_id = ? AND year = ? AND month = ? FOR UPDATE",
			branchID, year, month).Scan(&before)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO finance_tax_periods (branch_id, year, month, status, closed_at)
		 VALUES (?, ?, ?, ?,
			CASE WHEN ? = 'closed' THEN UTC_TIMESTAMP(6) ELSE NULL END)
		 ON DUPLICATE KEY UPDATE status = VALUES(status),
			closed_at = CASE WHEN VALUES(status) = 'closed' THEN UTC_TIMESTAMP(6) ELSE NULL END`,
			branchID, year, month, status, status); err != nil {
			return err
		}
		if snapshot != "" {
			var pid int64
			if err := tx.QueryRowContext(ctx,
				"SELECT id FROM finance_tax_periods WHERE branch_id = ? AND year = ? AND month = ?",
				branchID, year, month).Scan(&pid); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO finance_tax_report_snapshots (tax_period_id, snapshot_json, created_by) VALUES (?, ?, ?)",
				pid, snapshot, actorID); err != nil {
				return err
			}
		}
		if hook != nil {
			return hook(ctx, tx, before)
		}
		return nil
	})
}

// ListTaxPeriods returns touched tax months, newest first. Untouched months
// are treated open (read-time rule); snapshots freeze filed figures while
// live summaries may keep moving when the accounting period stays open.
func (r *Repository) ListTaxPeriods(ctx context.Context, branchID int64) ([]*TaxPeriod, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, year, month, status FROM finance_tax_periods WHERE branch_id = ? ORDER BY year DESC, month DESC",
		branchID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*TaxPeriod
	for rows.Next() {
		var p TaxPeriod
		if err := rows.Scan(&p.ID, &p.Year, &p.Month, &p.Status); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}
func (r *Repository) SaveTaxSnapshot(ctx context.Context, taxPeriodID int64, snapshot string, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO finance_tax_report_snapshots (tax_period_id, snapshot_json, created_by) VALUES (?, ?, ?)",
		taxPeriodID, snapshot, actorID)
	return err
}
