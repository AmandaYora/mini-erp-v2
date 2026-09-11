package infrastructure

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
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

func toFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

// MaxCostMovementID returns the costing cursor (0 when nothing costed).
func (r *Repository) MaxCostMovementID(ctx context.Context, branchID int64) (int64, error) {
	var id sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		"SELECT MAX(stock_movement_id) FROM finance_inventory_cost_movements WHERE branch_id = ?",
		branchID).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id.Int64, nil
}

// InsertCostRow records one costed movement.
func (r *Repository) InsertCostRow(ctx context.Context, branchID, productID, variantID, movementID int64, direction string, qty, unit float64, estimated bool, refType string, refID int64) error {
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
		  qty_base, unit_cost, total_cost, is_estimated)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		branchID, productID, variantID, movementID, refType, nullRefID(refID), direction, qty, unit, total, est)
	return err
}

func nullRefID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
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
		 WHERE branch_id = ? AND ref_type = ? AND ref_id = ?
		 GROUP BY product_id, variant_id`, branchID, refType, refID)
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
		out[CostKeyOf(pid, vid)] = MoveValue{Qty: toFloat(qty), Value: toFloat(value)}
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
		p.Qty = toFloat(qty)
		p.Total = toFloat(total)
		out[costKey(p.ProductID, p.VariantID)] = &p
	}
	return out, rows.Err()
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
}

// CreateExpense inserts a draft-posted expense row (journal linked after).
func (r *Repository) CreateExpense(ctx context.Context, e *Expense, actorID int64) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO business_expenses
		 (number, branch_id, expense_account_id, pay_account_id, amount,
		  expense_date, notes, status, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 'posted', ?)`,
		e.Number, e.BranchID, e.ExpenseAccount, e.PayAccount, e.Amount,
		e.Date, nullIfEmpty(e.Notes), actorID)
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
	var notes sql.NullString
	var journal sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		`SELECT id, number, branch_id, expense_account_id, pay_account_id,
			amount, expense_date, notes, journal_entry_id, status
		 FROM business_expenses WHERE id = ?`, id).
		Scan(&e.ID, &e.Number, &e.BranchID, &e.ExpenseAccount, &e.PayAccount,
			&e.Amount, &date, &notes, &journal, &e.Status)
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
	return e, nil
}

// SetExpenseStatus flips posted/cancelled.
func (r *Repository) SetExpenseStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE business_expenses SET status = ? WHERE id = ?", status, id)
	return err
}

// ListExpenses lists a branch's expenses newest-first.
func (r *Repository) ListExpenses(ctx context.Context, branchID int64, limit, offset int) ([]*Expense, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, number, branch_id, expense_account_id, pay_account_id,
			amount, expense_date, notes, journal_entry_id, status
		 FROM business_expenses WHERE branch_id = ? ORDER BY id DESC LIMIT ? OFFSET ?`,
		branchID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*Expense
	for rows.Next() {
		e := &Expense{}
		var date sql.NullTime
		var notes sql.NullString
		var journal sql.NullInt64
		if err := rows.Scan(&e.ID, &e.Number, &e.BranchID, &e.ExpenseAccount,
			&e.PayAccount, &e.Amount, &date, &notes, &journal, &e.Status); err != nil {
			return nil, err
		}
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
func (r *Repository) CountExpenses(ctx context.Context, branchID int64) (int64, error) {
	var total int64
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM business_expenses WHERE branch_id = ?", branchID).Scan(&total)
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
func (r *Repository) SetTaxPeriodStatus(ctx context.Context, branchID int64, year, month int, status string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO finance_tax_periods (branch_id, year, month, status, closed_at)
		 VALUES (?, ?, ?, ?,
			CASE WHEN ? = 'closed' THEN UTC_TIMESTAMP(6) ELSE NULL END)
		 ON DUPLICATE KEY UPDATE status = VALUES(status),
			closed_at = CASE WHEN VALUES(status) = 'closed' THEN UTC_TIMESTAMP(6) ELSE NULL END`,
		branchID, year, month, status, status)
	return err
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
