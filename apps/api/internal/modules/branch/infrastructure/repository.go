package infrastructure

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"mini-erp/internal/modules/branch/contracts"
)

// Repository owns the branch module tables.
type Repository struct {
	db *sql.DB
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func scanBranch(row *sql.Row, b *contracts.Branch) error {
	var address, city, phone sql.NullString
	var isHead int
	err := row.Scan(&b.ID, &b.Code, &b.Name, &address, &city, &phone, &b.Status, &isHead)
	b.Address, b.City, b.Phone = address.String, city.String, phone.String
	b.IsHead = isHead == 1
	return err
}

func scanBranchRows(rows *sql.Rows, b *contracts.Branch) error {
	var address, city, phone sql.NullString
	var isHead int
	err := rows.Scan(&b.ID, &b.Code, &b.Name, &address, &city, &phone, &b.Status, &isHead)
	b.Address, b.City, b.Phone = address.String, city.String, phone.String
	b.IsHead = isHead == 1
	return err
}

const branchColumns = "id, code, name, address, city, phone, status, is_head"

// GetByID returns nil, nil when missing.
func (r *Repository) GetByID(ctx context.Context, id int64) (*contracts.Branch, error) {
	b := &contracts.Branch{}
	err := scanBranch(r.db.QueryRowContext(ctx,
		"SELECT "+branchColumns+" FROM branches WHERE id = ?", id), b)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return b, err
}

// GetByIDs resolves known IDs, skipping unknown ones.
func (r *Repository) GetByIDs(ctx context.Context, ids []int64) ([]*contracts.Branch, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(ids)-1) + "?"
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+branchColumns+" FROM branches WHERE id IN ("+placeholders+") ORDER BY name ASC", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Branch
	for rows.Next() {
		b := &contracts.Branch{}
		if err := scanBranchRows(rows, b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ExistsByCode reports code collisions excluding one row.
func (r *Repository) ExistsByCode(ctx context.Context, code string, excludeID int64) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM branches WHERE code = ? AND id <> ?", code, excludeID).Scan(&n)
	return n > 0, err
}

// Create inserts the branch plus its document sequences in one transaction —
// a branch is never born half-provisioned (legacy "gagal-bersama" pattern).
func (r *Repository) Create(ctx context.Context, b *contracts.Branch, actorID int64, kinds []contracts.DocKind, now time.Time) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO branches (code, name, address, city, phone, status, is_head, created_by, updated_by)
		 VALUES (?, ?, ?, ?, ?, 'active', ?, ?, ?)`,
		b.Code, b.Name, nullIfEmpty(b.Address), nullIfEmpty(b.City), nullIfEmpty(b.Phone),
		boolToInt(b.IsHead), actorID, actorID)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := ensureSequences(ctx, tx, id, sequenceKeys(kinds, now)); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// Update rewrites editable fields. Code changes go through TryChangeCode.
func (r *Repository) Update(ctx context.Context, b *contracts.Branch, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE branches SET name = ?, address = ?, city = ?, phone = ?,
		 status = ?, is_head = ?, updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?`,
		b.Name, nullIfEmpty(b.Address), nullIfEmpty(b.City), nullIfEmpty(b.Phone),
		b.Status, boolToInt(b.IsHead), actorID, b.ID)
	return err
}

// TryChangeCode changes the code only when the branch never issued documents
// (OQ-A18: prefix consistency of issued documents is untouchable).
func (r *Repository) TryChangeCode(ctx context.Context, branchID int64, code string, actorID int64) (used bool, err error) {
	var n int
	if err := r.db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(current_value), 0) FROM branch_document_sequences WHERE branch_id = ?",
		branchID).Scan(&n); err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	_, err = r.db.ExecContext(ctx,
		"UPDATE branches SET code = ?, updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?",
		code, actorID, branchID)
	return false, err
}

// HasDocuments reports whether the branch ever issued a document number.
func (r *Repository) HasDocuments(ctx context.Context, branchID int64) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(current_value), 0) FROM branch_document_sequences WHERE branch_id = ?",
		branchID).Scan(&n)
	return n > 0, err
}

// List returns branches with optional search and status filter.
func (r *Repository) List(ctx context.Context, search, status string, limit, offset int) ([]*contracts.Branch, error) {
	var conds []string
	var args []any
	if status != "" {
		conds = append(conds, "status = ?")
		args = append(args, status)
	}
	for _, word := range strings.Fields(search) {
		like := "%" + word + "%"
		conds = append(conds, "(code LIKE ? OR name LIKE ? OR city LIKE ?)")
		args = append(args, like, like, like)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+branchColumns+" FROM branches "+where+" ORDER BY name ASC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Branch
	for rows.Next() {
		b := &contracts.Branch{}
		if err := scanBranchRows(rows, b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Count counts the List filter set.
func (r *Repository) Count(ctx context.Context, search, status string) (int64, error) {
	var conds []string
	var args []any
	if status != "" {
		conds = append(conds, "status = ?")
		args = append(args, status)
	}
	for _, word := range strings.Fields(search) {
		like := "%" + word + "%"
		conds = append(conds, "(code LIKE ? OR name LIKE ? OR city LIKE ?)")
		args = append(args, like, like, like)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	var total int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM branches "+where, args...).Scan(&total)
	return total, err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// sequenceKeys expands kinds into counter keys. Order kinds reset monthly
// (replicating actual legacy behavior); everything else runs continuous
// across years (OQ-A17) — the year label stays informational only.
func sequenceKeys(kinds []contracts.DocKind, now time.Time) []string {
	month := now.Format("2006-01")
	keys := make([]string, 0, len(kinds))
	for _, k := range kinds {
		switch k {
		case contracts.DocPurchaseOrder, contracts.DocSalesOrder:
			keys = append(keys, string(k)+":"+month)
		default:
			keys = append(keys, string(k))
		}
	}
	return keys
}

func ensureSequences(ctx context.Context, tx *sql.Tx, branchID int64, keys []string) error {
	for _, key := range keys {
		if _, err := tx.ExecContext(ctx,
			"INSERT IGNORE INTO branch_document_sequences (branch_id, doc_kind, current_value) VALUES (?, ?, 0)",
			branchID, key); err != nil {
			return err
		}
	}
	return nil
}

// NextNumber atomically allocates the next sequence value for key using
// LAST_INSERT_ID, so concurrent callers never receive the same number.
func (r *Repository) NextNumber(ctx context.Context, branchID int64, key string) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureSequences(ctx, tx, branchID, []string{key}); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE branch_document_sequences SET current_value = LAST_INSERT_ID(current_value + 1)
		 WHERE branch_id = ? AND doc_kind = ?`, branchID, key); err != nil {
		return 0, err
	}
	var n int64
	if err := tx.QueryRowContext(ctx, "SELECT LAST_INSERT_ID()").Scan(&n); err != nil {
		return 0, err
	}
	return n, tx.Commit()
}

// SequenceKey builds the counter key for kind at now. Order kinds reset
// monthly (replicating actual legacy behavior); everything else runs
// continuous across years (OQ-A17).
func SequenceKey(kind contracts.DocKind, now time.Time) string {
	keys := sequenceKeys([]contracts.DocKind{kind}, now)
	return keys[0]
}

// AllDocKinds lists every sequence kind the branch provisions.
func AllDocKinds() []contracts.DocKind {
	return []contracts.DocKind{
		contracts.DocPurchaseOrder, contracts.DocSalesOrder, contracts.DocPayment,
		contracts.DocDeliveryNote, contracts.DocSalesReturn, contracts.DocPurchaseReturn,
		contracts.DocStockTransfer,
	}
}
