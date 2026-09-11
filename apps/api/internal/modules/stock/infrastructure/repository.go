package infrastructure

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"mini-erp/internal/modules/stock/contracts"
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

func parseDecimal(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

// --- locations --------------------------------------------------------------

func scanLocation(row *sql.Row, l *contracts.Location) error {
	var parent sql.NullInt64
	var system int
	err := row.Scan(&l.ID, &l.BranchID, &l.Code, &l.Name, &parent, &system, &l.Status)
	if parent.Valid {
		l.ParentID = &parent.Int64
	}
	l.IsSystem = system == 1
	return err
}

const locationColumns = "id, branch_id, code, name, parent_id, is_system, status"

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
	query += " ORDER BY name ASC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Location
	for rows.Next() {
		l := &contracts.Location{}
		var parent sql.NullInt64
		var system int
		if err := rows.Scan(&l.ID, &l.BranchID, &l.Code, &l.Name, &parent, &system, &l.Status); err != nil {
			return nil, err
		}
		if parent.Valid {
			l.ParentID = &parent.Int64
		}
		l.IsSystem = system == 1
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
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO stock_locations (branch_id, code, name, parent_id, is_system, status, created_by, updated_by)
		 VALUES (?, ?, ?, ?, ?, 'active', ?, ?)`,
		l.BranchID, l.Code, l.Name, parent, system, actorID, actorID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateLocation rewrites name/parent.
func (r *Repository) UpdateLocation(ctx context.Context, l *contracts.Location, actorID int64) error {
	var parent any
	if l.ParentID != nil {
		parent = *l.ParentID
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE stock_locations SET name = ?, parent_id = ?,
		 updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?`,
		l.Name, parent, actorID, l.ID)
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
	id, err := r.CreateLocation(ctx, &contracts.Location{
		BranchID: branchID, Code: code, Name: name, IsSystem: true,
	}, 0)
	if err != nil {
		// Lost race with another creator: reread instead of failing.
		if existing, rerr := r.LocationByCode(ctx, branchID, code); rerr == nil && existing != nil {
			return existing, nil
		}
		return nil, err
	}
	_ = kind
	return r.LocationByID(ctx, id)
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
	_, err := tx.ExecContext(ctx,
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

// InsertMovementTx records one movement row. Every balance change in this
// module flows through here — no silent adjustments exist.
func (r *Repository) InsertMovementTx(ctx context.Context, tx *sql.Tx, branchID, productID, variantID, locationID int64, direction, movementType string, qty float64, refType string, refID int64, notes string, actorID int64) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO stock_movements
		 (branch_id, product_id, variant_id, location_id, direction, movement_type,
		  qty_base, ref_type, ref_id, notes, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		branchID, productID, variantID, locationID, direction, movementType,
		qty, refType, nullRefID(refID), nullIfEmpty(notes), actorID)
	return err
}

func nullRefID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
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
	b.OnHand = parseDecimal(onHand)
	b.Reserved = parseDecimal(reserved)
	return b, nil
}

// ProductAvailability sums (on_hand − reserved) per product in a branch
// (all variants and locations merged). Own tables only.
func (r *Repository) ProductAvailability(ctx context.Context, branchID int64) (map[int64]float64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT product_id, SUM(on_hand - reserved) FROM stock_balances
		 WHERE branch_id = ? GROUP BY product_id`, branchID)
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
		out[id] = parseDecimal(avail)
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
	b.OnHand = parseDecimal(onHand)
	b.Reserved = parseDecimal(reserved)
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
		e.qty = parseDecimal(qty)
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
		b.OnHand = parseDecimal(onHand)
		b.Reserved = parseDecimal(reserved)
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
		b.OnHand = parseDecimal(onHand)
		b.Reserved = parseDecimal(reserved)
		out = append(out, b)
	}
	return out, rows.Err()
}

// SuggestLocations lists leaf positions with available stock, oldest
// locations first (FIFO-ish picking order).
func (r *Repository) SuggestLocations(ctx context.Context, branchID, productID, variantID int64) ([]*contracts.Balance, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT b.location_id, b.on_hand, b.reserved FROM stock_balances b
		 JOIN stock_locations l ON l.id = b.location_id
		 WHERE b.branch_id = ? AND b.product_id = ? AND b.variant_id = ?
		   AND l.status = 'active' AND (b.on_hand - b.reserved) > 0
		 ORDER BY b.location_id ASC`, branchID, productID, variantID)
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
		b.OnHand = parseDecimal(onHand)
		b.Reserved = parseDecimal(reserved)
		out = append(out, b)
	}
	return out, rows.Err()
}

// ListCostMovements streams movement rows after an id for HPP costing.
func (r *Repository) ListCostMovements(ctx context.Context, branchID, afterID int64, limit int) ([]*contracts.CostMovement, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, branch_id, product_id, variant_id, direction,
			movement_type, qty_base, ref_type, ref_id, created_at
		 FROM stock_movements WHERE branch_id = ? AND id > ? ORDER BY id ASC LIMIT ?`,
		branchID, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.CostMovement
	for rows.Next() {
		m := &contracts.CostMovement{BranchID: branchID}
		var qty, mtype string
		var refID sql.NullInt64
		var at string
		if err := rows.Scan(&m.ID, &m.BranchID, &m.ProductID, &m.VariantID,
			&m.Direction, &mtype, &qty, &m.RefType, &refID, &at); err != nil {
			return nil, err
		}
		m.QtyBase = parseDecimal(qty)
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

// HasMovementsAt reports history for one exact position (opening guard).
func (r *Repository) HasMovementsAt(ctx context.Context, branchID, productID, variantID, locationID int64) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM stock_movements
		 WHERE branch_id = ? AND product_id = ? AND variant_id = ? AND location_id = ?`,
		branchID, productID, variantID, locationID).Scan(&n)
	return n > 0, err
}

// HasMovementsAnywhere reports history across all branches/locations.
func (r *Repository) HasMovementsAnywhere(ctx context.Context, productID int64) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM stock_movements WHERE product_id = ?", productID).Scan(&n)
	return n > 0, err
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
		conds = append(conds, "created_at >= ?")
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		conds = append(conds, "created_at <= ?")
		args = append(args, dateTo)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, branch_id, product_id, variant_id, location_id, direction,
			movement_type, qty_base, ref_type, ref_id, notes, created_by, created_at
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
		var at string
		if err := rows.Scan(&m.ID, &m.BranchID, &m.ProductID, &m.VariantID, &m.LocationID,
			&m.Direction, &m.Type, &qty, &m.RefType, &refID, &notes, &by, &at); err != nil {
			return nil, err
		}
		m.Qty = parseDecimal(qty)
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
		conds = append(conds, "created_at >= ?")
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		conds = append(conds, "created_at <= ?")
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
	Items        []TransferItem
}

// TransferItem is one document line (base UOM).
type TransferItem struct {
	ProductID int64
	VariantID int64
	Qty       float64
	Notes     string
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
		  status, notes, created_by, updated_by)
		 VALUES (?, ?, ?, ?, ?, 'draft', ?, ?, ?)`,
		t.Number, t.FromBranchID, t.ToBranchID, t.FromLocation, t.ToLocation,
		nullIfEmpty(t.Notes), actorID, actorID)
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
			id, it.ProductID, it.VariantID, it.Qty, nullIfEmpty(it.Notes)); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// GetTransfer loads a document with lines, or nil when missing.
func (r *Repository) GetTransfer(ctx context.Context, id int64) (*Transfer, error) {
	t := &Transfer{}
	var notes sql.NullString
	var dispatched, received sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, number, from_branch_id, to_branch_id, from_location_id,
			to_location_id, status, notes, dispatched_at, received_at
		 FROM stock_transfers WHERE id = ?`, id).
		Scan(&t.ID, &t.Number, &t.FromBranchID, &t.ToBranchID, &t.FromLocation,
			&t.ToLocation, &t.Status, &notes, &dispatched, &received)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.Notes = notes.String
	rows, err := r.db.QueryContext(ctx,
		"SELECT product_id, variant_id, qty_base, notes FROM stock_transfer_items WHERE transfer_id = ?",
		id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var it TransferItem
		var qty string
		var inotes sql.NullString
		if err := rows.Scan(&it.ProductID, &it.VariantID, &qty, &inotes); err != nil {
			return nil, err
		}
		it.Qty = parseDecimal(qty)
		it.Notes = inotes.String
		t.Items = append(t.Items, it)
	}
	return t, rows.Err()
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

// ListTransfers filters by branch involvement + status, newest first.
func (r *Repository) ListTransfers(ctx context.Context, branchID int64, status string, limit, offset int) ([]*Transfer, error) {
	conds := []string{"(from_branch_id = ? OR to_branch_id = ?)"}
	args := []any{branchID, branchID}
	if status != "" {
		conds = append(conds, "status = ?")
		args = append(args, status)
	}
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, number, from_branch_id, to_branch_id, from_location_id,
			to_location_id, status FROM stock_transfers
		 WHERE `+strings.Join(conds, " AND ")+` ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*Transfer
	for rows.Next() {
		t := &Transfer{}
		if err := rows.Scan(&t.ID, &t.Number, &t.FromBranchID, &t.ToBranchID,
			&t.FromLocation, &t.ToLocation, &t.Status); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// CountTransfers counts the List filter set.
func (r *Repository) CountTransfers(ctx context.Context, branchID int64, status string) (int64, error) {
	conds := []string{"(from_branch_id = ? OR to_branch_id = ?)"}
	args := []any{branchID, branchID}
	if status != "" {
		conds = append(conds, "status = ?")
		args = append(args, status)
	}
	var total int64
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM stock_transfers WHERE "+strings.Join(conds, " AND "), args...).Scan(&total)
	return total, err
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
	res.Total = parseDecimal(total)
	res.Remaining = parseDecimal(remaining)
	res.ExpiresAt = expires
	return res, nil
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
	res.Total = parseDecimal(total)
	res.Remaining = parseDecimal(remaining)
	res.ExpiresAt = expires
	return res, nil
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
		res.Total = parseDecimal(total)
		res.Remaining = parseDecimal(remaining)
		res.ExpiresAt = expires
		out = append(out, res)
	}
	return out, rows.Err()
}
