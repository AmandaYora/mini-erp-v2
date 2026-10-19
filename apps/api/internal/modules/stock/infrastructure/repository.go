package infrastructure

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/dbutil"
)

// Repository owns the stock module tables. Balance writes always happen
// inside a transaction together with their movement rows (contract §7.5) —
// single-statement helpers below never write balances alone.
type Repository struct {
	db *sql.DB
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Begin opens a module-scoped transaction for multi-step flows.
func (r *Repository) Begin(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, nil)
}

// --- locations --------------------------------------------------------------

func scanLocation(row *sql.Row, l *contracts.Location) error {
	var parent sql.NullInt64
	var system, picking, primary int
	err := row.Scan(&l.ID, &l.BranchID, &l.Code, &l.Name, &parent, &system, &l.Kind, &l.Status,
		&picking, &primary, &l.SortOrder)
	if parent.Valid {
		l.ParentID = &parent.Int64
	}
	l.IsSystem = system == 1
	l.IsPickingArea, l.IsPrimary = picking == 1, primary == 1
	return err
}

const locationColumns = "id, branch_id, code, name, parent_id, is_system, kind, status, is_picking_area, is_primary, sort_order"

// LocationByID returns nil, nil when missing.
func (r *Repository) LocationByID(ctx context.Context, id int64) (*contracts.Location, error) {
	l := &contracts.Location{}
	err := scanLocation(r.db.QueryRowContext(ctx,
		"SELECT "+locationColumns+" FROM stock_locations WHERE id = ?", id), l)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return l, err
}

// LocationByCode returns nil, nil when the (branch, code) pair is free.
func (r *Repository) LocationByCode(ctx context.Context, branchID int64, code string) (*contracts.Location, error) {
	l := &contracts.Location{}
	err := scanLocation(r.db.QueryRowContext(ctx,
		"SELECT "+locationColumns+" FROM stock_locations WHERE branch_id = ? AND code = ?", branchID, code), l)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return l, err
}

// ListLocations returns a branch's locations ordered by name.
func (r *Repository) ListLocations(ctx context.Context, branchID int64, status string) ([]*contracts.Location, error) {
	query := "SELECT " + locationColumns + " FROM stock_locations WHERE branch_id = ?"
	args := []any{branchID}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY sort_order ASC, name ASC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Location
	for rows.Next() {
		l := &contracts.Location{}
		var parent sql.NullInt64
		var system, picking, primary int
		if err := rows.Scan(&l.ID, &l.BranchID, &l.Code, &l.Name, &parent, &system, &l.Kind, &l.Status,
			&picking, &primary, &l.SortOrder); err != nil {
			return nil, err
		}
		if parent.Valid {
			l.ParentID = &parent.Int64
		}
		l.IsSystem = system == 1
		l.IsPickingArea, l.IsPrimary = picking == 1, primary == 1
		out = append(out, l)
	}
	return out, rows.Err()
}

// HasActiveChildren reports whether the location parents any active location.
func (r *Repository) HasActiveChildren(ctx context.Context, id int64) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM stock_locations WHERE parent_id = ? AND status = 'active'", id).Scan(&n)
	return n > 0, err
}

// IsLeaf reports whether the location has no active children. Only leaves
// hold stock (contract §7.5 resolves to leaf locations).
func (r *Repository) IsLeaf(ctx context.Context, id int64) (bool, error) {
	children, err := r.HasActiveChildren(ctx, id)
	return !children, err
}

// CreateLocation inserts a location row.
func (r *Repository) CreateLocation(ctx context.Context, l *contracts.Location, actorID int64) (int64, error) {
	var parent any
	if l.ParentID != nil {
		parent = *l.ParentID
	}
	system := 0
	if l.IsSystem {
		system = 1
	}
	kind := l.Kind
	if kind == "" {
		kind = "normal"
	}
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO stock_locations (branch_id, code, name, parent_id, is_system, kind, status,
		   is_picking_area, is_primary, sort_order, created_by, updated_by)
		 VALUES (?, ?, ?, ?, ?, ?, 'active', ?, ?, ?, ?, ?)`,
		l.BranchID, l.Code, l.Name, parent, system, kind, boolInt(l.IsPickingArea), boolInt(l.IsPrimary), l.SortOrder, actorID, actorID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CreateLocationTx inserts a location inside the caller's transaction.
func (r *Repository) CreateLocationTx(ctx context.Context, tx *sql.Tx, l *contracts.Location, actorID int64) (int64, error) {
	var parent any
	if l.ParentID != nil {
		parent = *l.ParentID
	}
	kind := l.Kind
	if kind == "" {
		kind = "normal"
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO stock_locations (branch_id, code, name, parent_id, is_system, kind, status,
		   is_picking_area, is_primary, sort_order, created_by, updated_by)
		 VALUES (?, ?, ?, ?, 0, ?, 'active', ?, ?, ?, ?, ?)`,
		l.BranchID, l.Code, l.Name, parent, kind, boolInt(l.IsPickingArea), boolInt(l.IsPrimary), l.SortOrder, actorID, actorID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// UpdateLocationTx rewrites code/name/parent/attributes inside the caller's transaction.
func (r *Repository) UpdateLocationTx(ctx context.Context, tx *sql.Tx, l *contracts.Location, actorID int64) error {
	var parent any
	if l.ParentID != nil {
		parent = *l.ParentID
	}
	_, err := tx.ExecContext(ctx,
		`UPDATE stock_locations SET code = COALESCE(NULLIF(?, ''), code), name = ?, parent_id = ?,
		 is_picking_area = ?, is_primary = ?, sort_order = ?,
		 updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?`,
		l.Code, l.Name, parent, boolInt(l.IsPickingArea), boolInt(l.IsPrimary), l.SortOrder, actorID, l.ID)
	return err
}

// BalancesByLocationTx reads a location's nonzero positions locking them, so
// a relocation moves exactly what it read.
func (r *Repository) BalancesByLocationTx(ctx context.Context, tx *sql.Tx, locationID int64) ([]*contracts.Balance, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT branch_id, product_id, variant_id, on_hand, reserved FROM stock_balances
		 WHERE location_id = ? AND (on_hand <> 0 OR reserved <> 0)
		 ORDER BY product_id, variant_id FOR UPDATE`, locationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Balance
	for rows.Next() {
		b := &contracts.Balance{LocationID: locationID}
		var onHand, reserved string
		if err := rows.Scan(&b.BranchID, &b.ProductID, &b.VariantID, &onHand, &reserved); err != nil {
			return nil, err
		}
		b.OnHand = dbutil.ParseDecimal(onHand)
		b.Reserved = dbutil.ParseDecimal(reserved)
		out = append(out, b)
	}
	return out, rows.Err()
}

// UpdateLocation rewrites name/parent.
func (r *Repository) UpdateLocation(ctx context.Context, l *contracts.Location, actorID int64) error {
	var parent any
	if l.ParentID != nil {
		parent = *l.ParentID
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE stock_locations SET code = COALESCE(NULLIF(?, ''), code), name = ?, parent_id = ?,
		 is_picking_area = ?, is_primary = ?, sort_order = ?,
		 updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?`,
		l.Code, l.Name, parent, boolInt(l.IsPickingArea), boolInt(l.IsPrimary), l.SortOrder, actorID, l.ID)
	return err
}

// SetLocationStatus flips active/archived.
func (r *Repository) SetLocationStatus(ctx context.Context, id int64, status string, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE stock_locations SET status = ?, updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?",
		status, actorID, id)
	return err
}

// EnsureSystemLocation returns the branch's system location for kind,
// creating it idempotently ("pastikan ada", never "buat ulang").
// kind is "default" (code GDG-<branch?>) or "damaged".
func (r *Repository) EnsureSystemLocation(ctx context.Context, branchID int64, kind, code, name string) (*contracts.Location, error) {
	existing, err := r.LocationByCode(ctx, branchID, code)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	locKind := "normal"
	if kind == "damaged" {
		locKind = "damaged"
	}
	id, err := r.CreateLocation(ctx, &contracts.Location{
		BranchID: branchID, Code: code, Name: name, IsSystem: true, Kind: locKind,
	}, 0)
	if err != nil {
		// Lost race with another creator: reread instead of failing.
		if existing, rerr := r.LocationByCode(ctx, branchID, code); rerr == nil && existing != nil {
			return existing, nil
		}
		return nil, err
	}
	return r.LocationByID(ctx, id)
}

// RenameLocation sets a location's display name (system default label sync).
func (r *Repository) RenameLocation(ctx context.Context, id int64, name string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE stock_locations SET name = ?, updated_at = UTC_TIMESTAMP(6) WHERE id = ?", name, id)
	return err
}

// LocationHasBalances reports whether any nonzero position references it.
func (r *Repository) LocationHasBalances(ctx context.Context, id int64) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM stock_balances WHERE location_id = ? AND (on_hand <> 0 OR reserved <> 0)", id).Scan(&n)
	return n > 0, err
}

// --- balances + movements (transactional primitives) ------------------------

// EnsureBalanceTx inserts the zero row for a position when missing.
func (r *Repository) EnsureBalanceTx(ctx context.Context, tx *sql.Tx, branchID, productID, variantID, locationID int64) error {
	// Plain read first: INSERT IGNORE on an existing row takes a SHARED lock
	// on it, and two transactions holding S then asking for X (FOR UPDATE)
	// deadlock each other. Only insert when the row is really missing.
	var one int
	err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM stock_balances
		 WHERE branch_id = ? AND product_id = ? AND variant_id = ? AND location_id = ?`,
		branchID, productID, variantID, locationID).Scan(&one)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`INSERT IGNORE INTO stock_balances (branch_id, product_id, variant_id, location_id, on_hand, reserved)
		 VALUES (?, ?, ?, ?, 0, 0)`,
		branchID, productID, variantID, locationID)
	return err
}

// AddBalanceTx shifts on_hand/reserved. Callers must have ensured the row.
func (r *Repository) AddBalanceTx(ctx context.Context, tx *sql.Tx, branchID, productID, variantID, locationID int64, onHandDelta, reservedDelta float64) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE stock_balances SET on_hand = on_hand + ?, reserved = reserved + ?
		 WHERE branch_id = ? AND product_id = ? AND variant_id = ? AND location_id = ?`,
		onHandDelta, reservedDelta, branchID, productID, variantID, locationID)
	return err
}

// MovementRow is one movement to record. Before/After are the position's
// on-hand around the change; MovedAt is the effective time (zero = now).
type MovementRow struct {
	BranchID, ProductID, VariantID, LocationID int64
	Direction, MovementType                    string
	Qty, Before, After                         float64
	UnitCost                                   float64
	RefType                                    string
	RefID                                      int64
	Notes                                      string
	ActorID                                    int64
	MovedAt                                    time.Time
}

// InsertMovementTx records one movement row. Every balance change in this
// module flows through here — no silent adjustments exist. A duplicate
// (ref_type, ref_id, position, direction) surfaces as MySQL 1062; Post treats
// that as a replay.
func (r *Repository) InsertMovementTx(ctx context.Context, tx *sql.Tx, m MovementRow) error {
	moved := m.MovedAt
	if moved.IsZero() {
		moved = time.Now()
	}
	_, err := tx.ExecContext(ctx,
		`INSERT INTO stock_movements
		 (branch_id, product_id, variant_id, location_id, direction, movement_type,
		  qty_base, balance_before, balance_after, unit_cost, ref_type, ref_id, notes, created_by, moved_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.BranchID, m.ProductID, m.VariantID, m.LocationID, m.Direction, m.MovementType,
		m.Qty, m.Before, m.After, nullPositive(m.UnitCost), m.RefType, dbutil.NullIfZero(m.RefID), dbutil.NullIfEmpty(m.Notes), m.ActorID,
		moved.UTC().Format("2006-01-02 15:04:05.000000"))
	return err
}

// MovementExistsTx reports whether the exact document movement already exists.
func (r *Repository) MovementExistsTx(ctx context.Context, tx *sql.Tx, refType string, refID, productID, variantID, locationID int64, direction string) (bool, error) {
	var one int
	err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM stock_movements
		 WHERE ref_type = ? AND ref_id = ? AND product_id = ? AND variant_id = ?
		   AND location_id = ? AND direction = ? LIMIT 1`,
		refType, refID, productID, variantID, locationID, direction).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

// DocMovement is one persisted movement of a document (for Reverse).
type DocMovement struct {
	BranchID, ProductID, VariantID, LocationID int64
	Direction                                  string
	Qty                                        float64
}

// MovementsByRefTx lists a document's movements ordered by position then
// direction, the same order Post locks balances in.
func (r *Repository) MovementsByRefTx(ctx context.Context, tx *sql.Tx, branchID int64, refType string, refID int64) ([]DocMovement, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT branch_id, product_id, variant_id, location_id, direction, qty_base
		 FROM stock_movements WHERE branch_id = ? AND ref_type = ? AND ref_id = ?
		 ORDER BY product_id, variant_id, location_id, direction`,
		branchID, refType, refID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []DocMovement
	for rows.Next() {
		var m DocMovement
		var qty string
		if err := rows.Scan(&m.BranchID, &m.ProductID, &m.VariantID, &m.LocationID, &m.Direction, &qty); err != nil {
			return nil, err
		}
		m.Qty = dbutil.ParseDecimal(qty)
		out = append(out, m)
	}
	return out, rows.Err()
}

// HasRefMovements reports whether a document wrote any movement.
func (r *Repository) HasRefMovements(ctx context.Context, branchID int64, refType string, refID int64) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM stock_movements WHERE branch_id = ? AND ref_type = ? AND ref_id = ? LIMIT 1`,
		branchID, refType, refID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func nullPositive(v float64) any {
	if v > 0 {
		return v
	}
	return nil
}

// GetBalance reads one position; missing rows read as zero.
func (r *Repository) GetBalance(ctx context.Context, branchID, productID, variantID, locationID int64) (*contracts.Balance, error) {
	b := &contracts.Balance{BranchID: branchID, ProductID: productID, VariantID: variantID, LocationID: locationID}
	var onHand, reserved string
	err := r.db.QueryRowContext(ctx,
		`SELECT on_hand, reserved FROM stock_balances
		 WHERE branch_id = ? AND product_id = ? AND variant_id = ? AND location_id = ?`,
		branchID, productID, variantID, locationID).Scan(&onHand, &reserved)
	if err == sql.ErrNoRows {
		return b, nil
	}
	if err != nil {
		return nil, err
	}
	b.OnHand = dbutil.ParseDecimal(onHand)
	b.Reserved = dbutil.ParseDecimal(reserved)
	return b, nil
}

// ProductAvailability sums (on_hand − reserved) per product in a branch
// (all variants and locations merged). Own tables only.
func (r *Repository) ProductAvailability(ctx context.Context, branchID int64) (map[int64]float64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT b.product_id, SUM(b.on_hand - b.reserved) FROM stock_balances b
		 JOIN stock_locations l ON l.id = b.location_id
		 WHERE b.branch_id = ? AND l.kind <> 'damaged' AND l.status = 'active' GROUP BY b.product_id`, branchID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[int64]float64{}
	for rows.Next() {
		var id int64
		var avail string
		if err := rows.Scan(&id, &avail); err != nil {
			return nil, err
		}
		out[id] = dbutil.ParseDecimal(avail)
	}
	return out, rows.Err()
}

// ProductTotals sums on_hand and reserved per product in a branch (damaged and
// inactive locations excluded).
func (r *Repository) ProductTotals(ctx context.Context, branchID int64) (map[int64][2]float64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT b.product_id, SUM(b.on_hand), SUM(b.reserved) FROM stock_balances b
		 JOIN stock_locations l ON l.id = b.location_id
		 WHERE b.branch_id = ? AND l.kind <> 'damaged' AND l.status = 'active' GROUP BY b.product_id`, branchID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[int64][2]float64{}
	for rows.Next() {
		var id int64
		var onHand, reserved string
		if err := rows.Scan(&id, &onHand, &reserved); err != nil {
			return nil, err
		}
		out[id] = [2]float64{dbutil.ParseDecimal(onHand), dbutil.ParseDecimal(reserved)}
	}
	return out, rows.Err()
}

// GetBalanceTx reads one position locking the row (FOR UPDATE), so
// check-then-move sequences inside the same tx cannot race.
func (r *Repository) GetBalanceTx(ctx context.Context, tx *sql.Tx, branchID, productID, variantID, locationID int64) (*contracts.Balance, error) {
	b := &contracts.Balance{BranchID: branchID, ProductID: productID, VariantID: variantID, LocationID: locationID}
	var onHand, reserved string
	err := tx.QueryRowContext(ctx,
		`SELECT on_hand, reserved FROM stock_balances
		 WHERE branch_id = ? AND product_id = ? AND variant_id = ? AND location_id = ? FOR UPDATE`,
		branchID, productID, variantID, locationID).Scan(&onHand, &reserved)
	if err == sql.ErrNoRows {
		return b, nil
	}
	if err != nil {
		return nil, err
	}
	b.OnHand = dbutil.ParseDecimal(onHand)
	b.Reserved = dbutil.ParseDecimal(reserved)
	return b, nil
}

// ExpireReservationsTx sweeps expired active holds for a position, returning
// the reserved quantity to balances. It takes NO gap locks: the SELECT runs
// unlocked and each UPDATE is conditional (WHERE status='active'), so only
// the winner of a concurrent sweep decrements. Never SELECT ... FOR UPDATE
// here — the resulting gap locks would block this same flow's later INSERT
// into stock_reservations on a different connection (self-deadlock).
func (r *Repository) ExpireReservationsTx(ctx context.Context, tx *sql.Tx, branchID, productID, variantID, locationID int64) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, qty_remaining, location_id FROM stock_reservations
		 WHERE branch_id = ? AND product_id = ? AND variant_id = ?
		   AND status = 'active' AND expires_at <= UTC_TIMESTAMP(6)`,
		branchID, productID, variantID)
	if err != nil {
		return err
	}
	type expired struct {
		id       int64
		qty      float64
		location sql.NullInt64
	}
	var list []expired
	for rows.Next() {
		var e expired
		var qty string
		if err := rows.Scan(&e.id, &qty, &e.location); err != nil {
			_ = rows.Close()
			return err
		}
		e.qty = dbutil.ParseDecimal(qty)
		list = append(list, e)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()
	for _, e := range list {
		res, err := tx.ExecContext(ctx,
			"UPDATE stock_reservations SET status = 'expired' WHERE id = ? AND status = 'active'",
			e.id)
		if err != nil {
			return err
		}
		won, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if won == 0 || !e.location.Valid {
			continue
		}
		if err := r.ensureBalanceTxLoc(ctx, tx, branchID, productID, variantID, e.location.Int64); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE stock_balances SET reserved = reserved - ?
			 WHERE branch_id = ? AND product_id = ? AND variant_id = ? AND location_id = ?`,
			e.qty, branchID, productID, variantID, e.location.Int64); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) ensureBalanceTxLoc(ctx context.Context, tx *sql.Tx, branchID, productID, variantID, locationID int64) error {
	return r.EnsureBalanceTx(ctx, tx, branchID, productID, variantID, locationID)
}

// BalancesByLocation lists nonzero positions at one location.
func (r *Repository) BalancesByLocation(ctx context.Context, locationID int64) ([]*contracts.Balance, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT branch_id, product_id, variant_id, on_hand, reserved FROM stock_balances
		 WHERE location_id = ? AND (on_hand <> 0 OR reserved <> 0)`,
		locationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Balance
	for rows.Next() {
		b := &contracts.Balance{LocationID: locationID}
		var onHand, reserved string
		if err := rows.Scan(&b.BranchID, &b.ProductID, &b.VariantID, &onHand, &reserved); err != nil {
			return nil, err
		}
		b.OnHand = dbutil.ParseDecimal(onHand)
		b.Reserved = dbutil.ParseDecimal(reserved)
		out = append(out, b)
	}
	return out, rows.Err()
}
func (r *Repository) BalancesByProduct(ctx context.Context, branchID, productID int64) ([]*contracts.Balance, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT product_id, variant_id, location_id, on_hand, reserved FROM stock_balances
		 WHERE branch_id = ? AND product_id = ? AND (on_hand <> 0 OR reserved <> 0)`,
		branchID, productID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Balance
	for rows.Next() {
		b := &contracts.Balance{BranchID: branchID}
		var onHand, reserved string
		if err := rows.Scan(&b.ProductID, &b.VariantID, &b.LocationID, &onHand, &reserved); err != nil {
			return nil, err
		}
		b.OnHand = dbutil.ParseDecimal(onHand)
		b.Reserved = dbutil.ParseDecimal(reserved)
		out = append(out, b)
	}
	return out, rows.Err()
}

// BalanceFilter narrows the branch-wide balance list.
type BalanceFilter struct {
	ProductID  int64
	LocationID int64
	Negative   bool // only positions with available < 0
	// RestrictIDs limits the list to ProductIDs (empty = no rows), for
	// search/critical filters resolved by the product module.
	RestrictIDs    bool
	ProductIDs     []int64
	IncludeDamaged bool
	Page           int
	Limit          int
}

// ListBalances pages the branch's nonzero positions (product/location filters) with the exact total.
func (r *Repository) ListBalances(ctx context.Context, branchID int64, f BalanceFilter) ([]*contracts.Balance, int64, error) {
	where := "b.branch_id = ? AND (b.on_hand <> 0 OR b.reserved <> 0)"
	args := []any{branchID}
	if f.ProductID > 0 {
		where += " AND b.product_id = ?"
		args = append(args, f.ProductID)
	}
	if f.LocationID > 0 {
		where += " AND b.location_id = ?"
		args = append(args, f.LocationID)
	}
	if f.Negative {
		where += " AND (b.on_hand - b.reserved) < 0"
	}
	if !f.IncludeDamaged {
		where += " AND b.location_id NOT IN (SELECT id FROM stock_locations WHERE kind = 'damaged')"
	}
	if f.RestrictIDs {
		if len(f.ProductIDs) == 0 {
			where += " AND 1 = 0"
		} else {
			where += " AND b.product_id IN (" + strings.Repeat("?,", len(f.ProductIDs)-1) + "?)"
			for _, id := range f.ProductIDs {
				args = append(args, id)
			}
		}
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM stock_balances b WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit, offset := f.Limit, (f.Page-1)*f.Limit
	if limit <= 0 {
		limit, offset = 50, 0
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT b.product_id, b.variant_id, b.location_id, b.on_hand, b.reserved FROM stock_balances b
		 WHERE `+where+` ORDER BY b.product_id, b.variant_id, b.location_id LIMIT ? OFFSET ?`,
		append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Balance
	for rows.Next() {
		b := &contracts.Balance{BranchID: branchID}
		var onHand, reserved string
		if err := rows.Scan(&b.ProductID, &b.VariantID, &b.LocationID, &onHand, &reserved); err != nil {
			return nil, 0, err
		}
		b.OnHand = dbutil.ParseDecimal(onHand)
		b.Reserved = dbutil.ParseDecimal(reserved)
		out = append(out, b)
	}
	return out, total, rows.Err()
}

// SuggestLocations lists leaf positions with available stock in allocation
// priority (ST-01, legacy sortAllocationCandidates): picking area, then primary
// location, then the branch default (the system normal location), then the
// largest available quantity, then sort order, then id.
func (r *Repository) SuggestLocations(ctx context.Context, branchID, productID, variantID int64) ([]*contracts.Balance, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT b.location_id, b.on_hand, b.reserved FROM stock_balances b
		 JOIN stock_locations l ON l.id = b.location_id
		 WHERE b.branch_id = ? AND b.product_id = ? AND b.variant_id = ?
		   AND l.status = 'active' AND l.kind <> 'damaged' AND (b.on_hand - b.reserved) > 0
		 ORDER BY l.is_picking_area DESC, l.is_primary DESC,
		   (l.is_system = 1 AND l.kind = 'normal') DESC,
		   (b.on_hand - b.reserved) DESC, l.sort_order ASC, b.location_id ASC`, branchID, productID, variantID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Balance
	for rows.Next() {
		b := &contracts.Balance{BranchID: branchID, ProductID: productID, VariantID: variantID}
		var onHand, reserved string
		if err := rows.Scan(&b.LocationID, &onHand, &reserved); err != nil {
			return nil, err
		}
		b.OnHand = dbutil.ParseDecimal(onHand)
		b.Reserved = dbutil.ParseDecimal(reserved)
		out = append(out, b)
	}
	return out, rows.Err()
}

// RecentCostMovements lists movements created within the last `minutes` minutes.
func (r *Repository) RecentCostMovements(ctx context.Context, branchID int64, minutes, limit int) ([]*contracts.CostMovement, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, branch_id, product_id, variant_id, direction,
			movement_type, qty_base, ref_type, ref_id, created_at, COALESCE(unit_cost, 0), moved_at
		 FROM stock_movements WHERE branch_id = ? AND created_at > UTC_TIMESTAMP(6) - INTERVAL ? MINUTE
		 ORDER BY id ASC LIMIT ?`, branchID, minutes, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.CostMovement
	for rows.Next() {
		m := &contracts.CostMovement{BranchID: branchID}
		var qty, mtype, unitCost string
		var refID sql.NullInt64
		var at, movedAt string
		if err := rows.Scan(&m.ID, &m.BranchID, &m.ProductID, &m.VariantID,
			&m.Direction, &mtype, &qty, &m.RefType, &refID, &at, &unitCost, &movedAt); err != nil {
			return nil, err
		}
		m.QtyBase = dbutil.ParseDecimal(qty)
		m.UnitCost = dbutil.ParseDecimal(unitCost)
		m.MovedAt = movedAt
		m.RefID = refID.Int64
		m.CreatedAt = at
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListCostMovements streams movement rows after an id for HPP costing.
func (r *Repository) ListCostMovements(ctx context.Context, branchID, afterID int64, limit int) ([]*contracts.CostMovement, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, branch_id, product_id, variant_id, direction,
			movement_type, qty_base, ref_type, ref_id, created_at, COALESCE(unit_cost, 0), DATE_FORMAT(moved_at, '%Y-%m-%d %H:%i:%s.%f')
		 FROM stock_movements WHERE branch_id = ? AND id > ? ORDER BY id ASC LIMIT ?`,
		branchID, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.CostMovement
	for rows.Next() {
		m := &contracts.CostMovement{BranchID: branchID}
		var qty, mtype, unitCost string
		var refID sql.NullInt64
		var at, movedAt string
		if err := rows.Scan(&m.ID, &m.BranchID, &m.ProductID, &m.VariantID,
			&m.Direction, &mtype, &qty, &m.RefType, &refID, &at, &unitCost, &movedAt); err != nil {
			return nil, err
		}
		m.QtyBase = dbutil.ParseDecimal(qty)
		m.UnitCost = dbutil.ParseDecimal(unitCost)
		m.MovedAt = movedAt
		m.RefID = refID.Int64
		m.CreatedAt = at
		out = append(out, m)
	}
	return out, rows.Err()
}

// HasMovements reports whether any movement ever touched the position —
// the integrity anchor for product UOM/type locks.
func (r *Repository) HasMovements(ctx context.Context, branchID, productID, variantID int64) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM stock_movements
		 WHERE branch_id = ? AND product_id = ? AND variant_id = ?`,
		branchID, productID, variantID).Scan(&n)
	return n > 0, err
}

// HasMovementsAnywhere reports history across all branches/locations.
func (r *Repository) HasMovementsAnywhere(ctx context.Context, productID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM stock_movements WHERE product_id = ? LIMIT 1)", productID).Scan(&exists)
	return exists, err
}

// Movement is one history row for listings.
type Movement struct {
	ID         int64
	BranchID   int64
	ProductID  int64
	VariantID  int64
	LocationID int64
	Direction  string
	Type       string
	Qty        float64
	RefType    string
	RefID      int64
	Notes      string
	CreatedBy  int64
	CreatedAt  string
	// BalanceBefore/After are the position's on-hand around the movement (ST-04);
	// nil when the row predates the snapshot. MovedAt is the effective time.
	BalanceBefore *float64
	BalanceAfter  *float64
	MovedAt       string
}

// ListMovements filters history (branch/product/variant/location/type/date),
// newest first.
func (r *Repository) ListMovements(ctx context.Context, branchID, productID, variantID, locationID int64, movementType, dateFrom, dateTo string, limit, offset int) ([]*Movement, error) {
	conds := []string{}
	args := []any{}
	if branchID != 0 {
		conds = append(conds, "branch_id = ?")
		args = append(args, branchID)
	}
	if productID != 0 {
		conds = append(conds, "product_id = ?")
		args = append(args, productID)
	}
	if variantID != 0 {
		conds = append(conds, "variant_id = ?")
		args = append(args, variantID)
	}
	if locationID != 0 {
		conds = append(conds, "location_id = ?")
		args = append(args, locationID)
	}
	if movementType != "" {
		conds = append(conds, "movement_type = ?")
		args = append(args, movementType)
	}
	if dateFrom != "" {
		conds = append(conds, "moved_at >= ?")
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		conds = append(conds, "moved_at <= ?")
		args = append(args, dateTo)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, branch_id, product_id, variant_id, location_id, direction,
			movement_type, qty_base, ref_type, ref_id, notes, created_by, created_at,
			balance_before, balance_after, moved_at
		 FROM stock_movements `+where+` ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*Movement
	for rows.Next() {
		m := &Movement{}
		var qty string
		var refID sql.NullInt64
		var notes sql.NullString
		var by sql.NullInt64
		var at, moved string
		var before, after sql.NullString
		if err := rows.Scan(&m.ID, &m.BranchID, &m.ProductID, &m.VariantID, &m.LocationID,
			&m.Direction, &m.Type, &qty, &m.RefType, &refID, &notes, &by, &at,
			&before, &after, &moved); err != nil {
			return nil, err
		}
		if before.Valid {
			v := dbutil.ParseDecimal(before.String)
			m.BalanceBefore = &v
		}
		if after.Valid {
			v := dbutil.ParseDecimal(after.String)
			m.BalanceAfter = &v
		}
		m.MovedAt = moved
		m.Qty = dbutil.ParseDecimal(qty)
		m.RefID = refID.Int64
		m.Notes = notes.String
		if by.Valid {
			m.CreatedBy = by.Int64
		}
		m.CreatedAt = at
		out = append(out, m)
	}
	return out, rows.Err()
}

// CountMovements counts the List filter set.
func (r *Repository) CountMovements(ctx context.Context, branchID, productID, variantID, locationID int64, movementType, dateFrom, dateTo string) (int64, error) {
	conds := []string{}
	args := []any{}
	if branchID != 0 {
		conds = append(conds, "branch_id = ?")
		args = append(args, branchID)
	}
	if productID != 0 {
		conds = append(conds, "product_id = ?")
		args = append(args, productID)
	}
	if variantID != 0 {
		conds = append(conds, "variant_id = ?")
		args = append(args, variantID)
	}
	if locationID != 0 {
		conds = append(conds, "location_id = ?")
		args = append(args, locationID)
	}
	if movementType != "" {
		conds = append(conds, "movement_type = ?")
		args = append(args, movementType)
	}
	if dateFrom != "" {
		conds = append(conds, "moved_at >= ?")
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		conds = append(conds, "moved_at <= ?")
		args = append(args, dateTo)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	var total int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM stock_movements "+where, args...).Scan(&total)
	return total, err
}

// --- transfers --------------------------------------------------------------

// Transfer is the document header with its lines.
type Transfer struct {
	ID           int64
	Number       string
	FromBranchID int64
	ToBranchID   int64
	FromLocation int64
	ToLocation   int64
	Status       string
	Notes        string
	// Driver, vehicle and date of the physical movement (ST-02). TransferDate is
	// the Jakarta calendar day, "YYYY-MM-DD" ("" = unknown).
	DriverName   string
	VehiclePlate string
	TransferDate string
	Items        []TransferItem
}

// TransferItem is one document line (base UOM).
type TransferItem struct {
	ProductID int64
	VariantID int64
	Qty       float64
	Notes     string
	// ToLocation is this line's own destination (0 = the header destination).
	ToLocation int64
}

// transferDateSQL turns "YYYY-MM-DD" into the stored midnight value ("" stays "").
func transferDateSQL(d string) string {
	if len(d) >= 10 {
		return d[:10] + " 00:00:00"
	}
	return ""
}

// CreateTransfer inserts the draft document with lines.
func (r *Repository) CreateTransfer(ctx context.Context, t *Transfer, actorID int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO stock_transfers
		 (number, from_branch_id, to_branch_id, from_location_id, to_location_id,
		  status, notes, driver_name, vehicle_plate, transfer_date, created_by, updated_by)
		 VALUES (?, ?, ?, ?, ?, 'draft', ?, ?, ?, ?, ?, ?)`,
		t.Number, t.FromBranchID, t.ToBranchID, t.FromLocation, t.ToLocation,
		dbutil.NullIfEmpty(t.Notes), dbutil.NullIfEmpty(t.DriverName), dbutil.NullIfEmpty(t.VehiclePlate),
		dbutil.NullIfEmpty(transferDateSQL(t.TransferDate)), actorID, actorID)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, it := range t.Items {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO stock_transfer_items (transfer_id, product_id, variant_id, qty_base, notes, to_location_id) VALUES (?, ?, ?, ?, ?, ?)",
			id, it.ProductID, it.VariantID, it.Qty, dbutil.NullIfEmpty(it.Notes), nullIfZeroID(it.ToLocation)); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// GetTransfer loads a document with lines, or nil when missing.
func (r *Repository) GetTransfer(ctx context.Context, id int64) (*Transfer, error) {
	t := &Transfer{}
	var notes, driver, plate sql.NullString
	var dispatched, received sql.NullString
	var date sql.NullTime
	err := r.db.QueryRowContext(ctx,
		`SELECT id, number, from_branch_id, to_branch_id, from_location_id,
			to_location_id, status, notes, driver_name, vehicle_plate, transfer_date, dispatched_at, received_at
		 FROM stock_transfers WHERE id = ?`, id).
		Scan(&t.ID, &t.Number, &t.FromBranchID, &t.ToBranchID, &t.FromLocation,
			&t.ToLocation, &t.Status, &notes, &driver, &plate, &date, &dispatched, &received)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.Notes = notes.String
	t.DriverName, t.VehiclePlate = driver.String, plate.String
	if date.Valid {
		t.TransferDate = date.Time.Format("2006-01-02")
	}
	rows, err := r.db.QueryContext(ctx,
		"SELECT product_id, variant_id, qty_base, notes, to_location_id FROM stock_transfer_items WHERE transfer_id = ?",
		id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var it TransferItem
		var qty string
		var inotes sql.NullString
		var itemTo sql.NullInt64
		if err := rows.Scan(&it.ProductID, &it.VariantID, &qty, &inotes, &itemTo); err != nil {
			return nil, err
		}
		it.ToLocation = itemTo.Int64
		it.Qty = dbutil.ParseDecimal(qty)
		it.Notes = inotes.String
		t.Items = append(t.Items, it)
	}
	return t, rows.Err()
}

// LockTransferStatusTx reads a transfer's status locking the row, so every
// transition of the document serializes at the database (the stock module
// holds no process lock). Returns "" when the document does not exist.
func (r *Repository) LockTransferStatusTx(ctx context.Context, tx *sql.Tx, id int64) (string, error) {
	var status string
	err := tx.QueryRowContext(ctx,
		"SELECT status FROM stock_transfers WHERE id = ? FOR UPDATE", id).Scan(&status)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return status, err
}

// TransitionTransferTx moves a transfer between statuses only from an allowed
// origin (CAS). Returns false when the status was not one of from.
func (r *Repository) TransitionTransferTx(ctx context.Context, tx *sql.Tx, id int64, from []string, to string, actorID int64) (bool, error) {
	if len(from) == 0 {
		return false, nil
	}
	stamp := ""
	switch to {
	case "dispatched":
		stamp = ", dispatched_at = UTC_TIMESTAMP(6)"
	case "received":
		stamp = ", received_at = UTC_TIMESTAMP(6)"
	}
	args := []any{to, actorID, id}
	for _, f := range from {
		args = append(args, f)
	}
	res, err := tx.ExecContext(ctx,
		"UPDATE stock_transfers SET status = ?, updated_at = UTC_TIMESTAMP(6), updated_by = ?"+stamp+
			" WHERE id = ? AND status IN ("+strings.Repeat("?,", len(from)-1)+"?)", args...)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	return affected == 1, err
}

// CreateTransferTx inserts a transfer document with the given initial status
// inside the caller's transaction (one-step location moves are born
// "received", with both stamps set).
func (r *Repository) CreateTransferTx(ctx context.Context, tx *sql.Tx, t *Transfer, status string, actorID int64) (int64, error) {
	stamps, stampVals := "", ""
	if status == "received" {
		stamps, stampVals = ", dispatched_at, received_at", ", UTC_TIMESTAMP(6), UTC_TIMESTAMP(6)"
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO stock_transfers
		 (number, from_branch_id, to_branch_id, from_location_id, to_location_id,
		  status, notes, driver_name, vehicle_plate, transfer_date, created_by, updated_by`+stamps+`)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?`+stampVals+`)`,
		t.Number, t.FromBranchID, t.ToBranchID, t.FromLocation, t.ToLocation,
		status, dbutil.NullIfEmpty(t.Notes), dbutil.NullIfEmpty(t.DriverName), dbutil.NullIfEmpty(t.VehiclePlate),
		dbutil.NullIfEmpty(transferDateSQL(t.TransferDate)), actorID, actorID)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, it := range t.Items {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO stock_transfer_items (transfer_id, product_id, variant_id, qty_base, notes) VALUES (?, ?, ?, ?, ?)",
			id, it.ProductID, it.VariantID, it.Qty, dbutil.NullIfEmpty(it.Notes)); err != nil {
			return 0, err
		}
	}
	return id, nil
}

// SetTransferStatusTx moves the document lifecycle inside the caller's tx,
// so status and stock never diverge on commit failure.
func (r *Repository) SetTransferStatusTx(ctx context.Context, tx *sql.Tx, id int64, status string, actorID int64) error {
	stamp := ""
	switch status {
	case "dispatched":
		stamp = ", dispatched_at = UTC_TIMESTAMP(6)"
	case "received":
		stamp = ", received_at = UTC_TIMESTAMP(6)"
	}
	_, err := tx.ExecContext(ctx,
		"UPDATE stock_transfers SET status = ?, updated_at = UTC_TIMESTAMP(6), updated_by = ?"+stamp+" WHERE id = ?",
		status, actorID, id)
	return err
}

// SetTransferStatus moves the document lifecycle (standalone, no tx held).
func (r *Repository) SetTransferStatus(ctx context.Context, id int64, status string, actorID int64) error {
	stamp := ""
	switch status {
	case "dispatched":
		stamp = ", dispatched_at = UTC_TIMESTAMP(6)"
	case "received":
		stamp = ", received_at = UTC_TIMESTAMP(6)"
	}
	_, err := r.db.ExecContext(ctx,
		"UPDATE stock_transfers SET status = ?, updated_at = UTC_TIMESTAMP(6), updated_by = ?"+stamp+" WHERE id = ?",
		status, actorID, id)
	return err
}

// transferCols are the list columns of a transfer row.
const transferCols = `id, number, from_branch_id, to_branch_id, from_location_id, to_location_id, status, driver_name, vehicle_plate, transfer_date`

// scanTransferRow reads one transferCols row.
func scanTransferRow(rows *sql.Rows) (*Transfer, error) {
	t := &Transfer{}
	var driver, plate sql.NullString
	var date sql.NullTime
	if err := rows.Scan(&t.ID, &t.Number, &t.FromBranchID, &t.ToBranchID,
		&t.FromLocation, &t.ToLocation, &t.Status, &driver, &plate, &date); err != nil {
		return nil, err
	}
	t.DriverName, t.VehiclePlate = driver.String, plate.String
	if date.Valid {
		t.TransferDate = date.Time.Format("2006-01-02")
	}
	return t, nil
}

// ListTransfers filters by branch involvement + status, newest first. The
// "from OR to" condition is split into two index-driven branches
// (ix_stock_transfers_from / _to) combined with UNION ALL; a same-branch
// transfer (from = to) appears only in the first branch.
func (r *Repository) ListTransfers(ctx context.Context, branchID int64, status string, limit, offset int) ([]*Transfer, error) {
	statusCond := ""
	var args []any
	inner := limit + offset
	args = append(args, branchID)
	if status != "" {
		statusCond = " AND status = ?"
		args = append(args, status)
	}
	args = append(args, inner, branchID, branchID)
	if status != "" {
		args = append(args, status)
	}
	args = append(args, inner, limit, offset)
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+transferCols+` FROM (
			(SELECT `+transferCols+` FROM stock_transfers WHERE from_branch_id = ?`+statusCond+` ORDER BY id DESC LIMIT ?)
			UNION ALL
			(SELECT `+transferCols+` FROM stock_transfers WHERE to_branch_id = ? AND from_branch_id <> ?`+statusCond+` ORDER BY id DESC LIMIT ?)
		 ) t ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*Transfer
	for rows.Next() {
		t, err := scanTransferRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// transferDirCond is the extra condition of the direction filter (ST-06),
// relative to the session branch: out = leaves this branch for another, in =
// arrives from another, internal = both ends here.
func transferDirCond(direction string) (string, bool) {
	switch direction {
	case "out":
		return "from_branch_id = ? AND to_branch_id <> ?", true
	case "in":
		return "to_branch_id = ? AND from_branch_id <> ?", true
	case "internal":
		return "from_branch_id = ? AND to_branch_id = ?", true
	}
	return "", false
}

// ListTransfersDir is ListTransfers narrowed by direction (ST-06); an empty
// direction is the union listing.
func (r *Repository) ListTransfersDir(ctx context.Context, branchID int64, status, direction string, limit, offset int) ([]*Transfer, error) {
	cond, ok := transferDirCond(direction)
	if !ok {
		return r.ListTransfers(ctx, branchID, status, limit, offset)
	}
	args := []any{branchID, branchID}
	if status != "" {
		cond += " AND status = ?"
		args = append(args, status)
	}
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+transferCols+` FROM stock_transfers WHERE `+cond+` ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*Transfer
	for rows.Next() {
		t, err := scanTransferRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// CountTransfersDir counts the ListTransfersDir set.
func (r *Repository) CountTransfersDir(ctx context.Context, branchID int64, status, direction string) (int64, error) {
	cond, ok := transferDirCond(direction)
	if !ok {
		return r.CountTransfers(ctx, branchID, status)
	}
	args := []any{branchID, branchID}
	if status != "" {
		cond += " AND status = ?"
		args = append(args, status)
	}
	var n int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM stock_transfers WHERE "+cond, args...).Scan(&n)
	return n, err
}

// CountTransfers counts the List filter set (two index-only counts).
func (r *Repository) CountTransfers(ctx context.Context, branchID int64, status string) (int64, error) {
	statusCond := ""
	args1 := []any{branchID}
	args2 := []any{branchID, branchID}
	if status != "" {
		statusCond = " AND status = ?"
		args1 = append(args1, status)
		args2 = append(args2, status)
	}
	var a, b int64
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM stock_transfers WHERE from_branch_id = ?"+statusCond, args1...).Scan(&a); err != nil {
		return 0, err
	}
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM stock_transfers WHERE to_branch_id = ? AND from_branch_id <> ?"+statusCond, args2...).Scan(&b); err != nil {
		return 0, err
	}
	return a + b, nil
}

// --- reservations -----------------------------------------------------------

// Reservation is one hold row.
type Reservation struct {
	ID          int64
	Key         string
	BranchID    int64
	ProductID   int64
	VariantID   int64
	LocationID  int64
	HasLocation bool
	Total       float64
	Remaining   float64
	Status      string
	ExpiresAt   string
}

// CreateReservationTx inserts an active hold inside the caller's tx — never
// on a separate connection while this tx holds locks (self-deadlock).
func (r *Repository) CreateReservationTx(ctx context.Context, tx *sql.Tx, res *Reservation, actorID int64) (int64, error) {
	var loc any
	if res.HasLocation {
		loc = res.LocationID
	}
	dbRes, err := tx.ExecContext(ctx,
		`INSERT INTO stock_reservations
		 (reservation_key, branch_id, product_id, variant_id, location_id,
		  qty_total, qty_remaining, status, expires_at, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)`,
		res.Key, res.BranchID, res.ProductID, res.VariantID, loc,
		res.Total, res.Remaining, res.ExpiresAt, actorID)
	if err != nil {
		return 0, err
	}
	return dbRes.LastInsertId()
}

// CreateReservation inserts an active hold (standalone, no tx held).
func (r *Repository) CreateReservation(ctx context.Context, res *Reservation, actorID int64) (int64, error) {
	var loc any
	if res.HasLocation {
		loc = res.LocationID
	}
	dbRes, err := r.db.ExecContext(ctx,
		`INSERT INTO stock_reservations
		 (reservation_key, branch_id, product_id, variant_id, location_id,
		  qty_total, qty_remaining, status, expires_at, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)`,
		res.Key, res.BranchID, res.ProductID, res.VariantID, loc,
		res.Total, res.Remaining, res.ExpiresAt, actorID)
	if err != nil {
		return 0, err
	}
	return dbRes.LastInsertId()
}

// ReservationByKey loads by key, or nil when unknown.
func (r *Repository) ReservationByKey(ctx context.Context, key string) (*Reservation, error) {
	res := &Reservation{}
	var loc sql.NullInt64
	var total, remaining, expires string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, reservation_key, branch_id, product_id, variant_id, location_id,
			qty_total, qty_remaining, status, expires_at
		 FROM stock_reservations WHERE reservation_key = ?`, key).
		Scan(&res.ID, &res.Key, &res.BranchID, &res.ProductID, &res.VariantID,
			&loc, &total, &remaining, &res.Status, &expires)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if loc.Valid {
		res.LocationID, res.HasLocation = loc.Int64, true
	}
	res.Total = dbutil.ParseDecimal(total)
	res.Remaining = dbutil.ParseDecimal(remaining)
	res.ExpiresAt = expires
	return res, nil
}

// ReservationByKeyTx loads by key locking the row; nil when unknown.
func (r *Repository) ReservationByKeyTx(ctx context.Context, tx *sql.Tx, key string) (*Reservation, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM stock_reservations WHERE reservation_key = ?`, key).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.GetReservationTx(ctx, tx, id)
}

// GetReservationTx re-reads a reservation locking the row, so concurrent
// release/consume calls serialize instead of double-spending the hold.
func (r *Repository) GetReservationTx(ctx context.Context, tx *sql.Tx, id int64) (*Reservation, error) {
	res := &Reservation{}
	var loc sql.NullInt64
	var total, remaining, expires string
	err := tx.QueryRowContext(ctx,
		`SELECT id, reservation_key, branch_id, product_id, variant_id, location_id,
			qty_total, qty_remaining, status, expires_at
		 FROM stock_reservations WHERE id = ? FOR UPDATE`, id).
		Scan(&res.ID, &res.Key, &res.BranchID, &res.ProductID, &res.VariantID,
			&loc, &total, &remaining, &res.Status, &expires)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if loc.Valid {
		res.LocationID, res.HasLocation = loc.Int64, true
	}
	res.Total = dbutil.ParseDecimal(total)
	res.Remaining = dbutil.ParseDecimal(remaining)
	res.ExpiresAt = expires
	return res, nil
}

// ReservationProbeTx returns the id of the hold with this key, or 0. The read
// is unlocked on purpose (see Hold: gap locks on a missing unique key).
func (r *Repository) ReservationProbeTx(ctx context.Context, tx *sql.Tx, key string) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM stock_reservations WHERE reservation_key = ?`, key).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return id, err
}

// RewriteReservationTx replaces a hold's quantities, status and expiry.
func (r *Repository) RewriteReservationTx(ctx context.Context, tx *sql.Tx, id int64, total, remaining float64, status, expiresAt string) error {
	_, err := tx.ExecContext(ctx,
		"UPDATE stock_reservations SET qty_total = ?, qty_remaining = ?, status = ?, expires_at = ? WHERE id = ?",
		total, remaining, status, expiresAt, id)
	return err
}

// ExpiredReservations lists up to limit active holds past their expiry.
func (r *Repository) ExpiredReservations(ctx context.Context, limit int) ([]*Reservation, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, branch_id, product_id, variant_id, location_id FROM stock_reservations
		 WHERE status = 'active' AND expires_at <= UTC_TIMESTAMP(6) ORDER BY expires_at ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*Reservation
	for rows.Next() {
		e := &Reservation{Status: "active"}
		var loc sql.NullInt64
		if err := rows.Scan(&e.ID, &e.BranchID, &e.ProductID, &e.VariantID, &loc); err != nil {
			return nil, err
		}
		if loc.Valid {
			e.LocationID, e.HasLocation = loc.Int64, true
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// UpdateReservationTx writes qty_remaining + status inside the caller's tx.
func (r *Repository) UpdateReservationTx(ctx context.Context, tx *sql.Tx, id int64, remaining float64, status string) error {
	_, err := tx.ExecContext(ctx,
		"UPDATE stock_reservations SET qty_remaining = ?, status = ? WHERE id = ?",
		remaining, status, id)
	return err
}

// UpdateReservation writes qty_remaining + status (standalone, no tx held).
func (r *Repository) UpdateReservation(ctx context.Context, id int64, remaining float64, status string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE stock_reservations SET qty_remaining = ?, status = ? WHERE id = ?",
		remaining, status, id)
	return err
}

// ActiveReservations lists live holds for a position (for availability math
// outside transactions — informational; enforcement re-reads inside tx).
func (r *Repository) ActiveReservations(ctx context.Context, branchID, productID, variantID int64) ([]*Reservation, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, reservation_key, location_id, qty_total, qty_remaining, expires_at
		 FROM stock_reservations
		 WHERE branch_id = ? AND product_id = ? AND variant_id = ?
		   AND status = 'active' AND expires_at > UTC_TIMESTAMP(6)`,
		branchID, productID, variantID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*Reservation
	for rows.Next() {
		res := &Reservation{BranchID: branchID, ProductID: productID, VariantID: variantID, Status: "active"}
		var loc sql.NullInt64
		var total, remaining, expires string
		if err := rows.Scan(&res.ID, &res.Key, &loc, &total, &remaining, &expires); err != nil {
			return nil, err
		}
		if loc.Valid {
			res.LocationID, res.HasLocation = loc.Int64, true
		}
		res.Total = dbutil.ParseDecimal(total)
		res.Remaining = dbutil.ParseDecimal(remaining)
		res.ExpiresAt = expires
		out = append(out, res)
	}
	return out, rows.Err()
}

// MovedPositions returns the set of positions in a branch that already have
// movement history, keyed "productID:variantID:locationID".
//
// Opening-balance import asks "does this position already have history?" per
// row, to stay resumable. One grouped read answers it for the whole file;
// the caller adds positions it posts so rows later in the same file still
// see them.
func (r *Repository) MovedPositions(ctx context.Context, branchID int64) (map[string]bool, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT product_id, variant_id, location_id
		 FROM stock_movements WHERE branch_id = ?`, branchID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]bool{}
	for rows.Next() {
		var productID, variantID, locationID int64
		if err := rows.Scan(&productID, &variantID, &locationID); err != nil {
			return nil, err
		}
		out[PositionKey(productID, variantID, locationID)] = true
	}
	return out, rows.Err()
}

// PositionKey builds the map key MovedPositions uses.
func PositionKey(productID, variantID, locationID int64) string {
	return strconv.FormatInt(productID, 10) + ":" +
		strconv.FormatInt(variantID, 10) + ":" +
		strconv.FormatInt(locationID, 10)
}

// DB exposes the pool so the application layer can wrap a whole unit of work
// in dberr.RetryTx.
func (r *Repository) DB() *sql.DB { return r.db }

func nullIfZeroID(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}
