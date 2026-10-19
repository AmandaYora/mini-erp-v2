package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"mini-erp/internal/modules/branch/contracts"
	"mini-erp/internal/modules/branch/infrastructure/sqlc"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/timeutil"
)

// Repository owns the branch module tables.
type Repository struct {
	db *sql.DB
	q  *sqlc.Queries
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: sqlc.New(db)}
}

// GetByID returns nil, nil when missing.
func (r *Repository) GetByID(ctx context.Context, id int64) (*contracts.Branch, error) {
	row, err := r.q.GetBranch(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return &contracts.Branch{}, err
	}
	return toBranch(row), nil
}

// GetByIDs resolves known IDs, skipping unknown ones.
func (r *Repository) GetByIDs(ctx context.Context, ids []int64) ([]*contracts.Branch, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.q.GetBranchesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	var out []*contracts.Branch
	for _, row := range rows {
		out = append(out, toBranch(sqlc.GetBranchRow(row)))
	}
	return out, nil
}

// ExistsByCode reports code collisions excluding one row.
func (r *Repository) ExistsByCode(ctx context.Context, code string, excludeID int64) (bool, error) {
	n, err := r.q.CountBranchesByCode(ctx, sqlc.CountBranchesByCodeParams{Code: code, ID: excludeID})
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
	q := r.q.WithTx(tx)

	res, err := q.InsertBranch(ctx, sqlc.InsertBranchParams{
		Code:                 b.Code,
		Name:                 b.Name,
		Address:              nullStr(b.Address),
		City:                 nullStr(b.City),
		Phone:                nullStr(b.Phone),
		IsHead:               b.IsHead,
		DefaultLocationLabel: labelOrDefault(b.DefaultLocationLabel),
		CreatedBy:            actor(actorID),
		UpdatedBy:            actor(actorID),
	})
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := ensureSequences(ctx, q, id, sequenceKeys(kinds, now)); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// Update rewrites editable fields. Code changes go through TryChangeCode.
func (r *Repository) Update(ctx context.Context, b *contracts.Branch, actorID int64) error {
	return r.q.UpdateBranch(ctx, sqlc.UpdateBranchParams{
		Name:                 b.Name,
		Address:              nullStr(b.Address),
		City:                 nullStr(b.City),
		Phone:                nullStr(b.Phone),
		Status:               b.Status,
		IsHead:               b.IsHead,
		DefaultLocationLabel: labelOrDefault(b.DefaultLocationLabel),
		Now:                  timeutil.NowUTC(),
		UpdatedBy:            actor(actorID),
		ID:                   b.ID,
	})
}

// nullStr maps the empty string to SQL NULL.
func nullStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// labelOrDefault keeps the column NOT NULL meaningful when a client omits it.
func labelOrDefault(label string) string {
	if label == "" {
		return "Gudang Utama"
	}
	return label
}

// TryChangeCode changes the code only when the branch never issued documents
// (OQ-A18: prefix consistency of issued documents is untouchable).
func (r *Repository) TryChangeCode(ctx context.Context, branchID int64, code string, actorID int64) (used bool, err error) {
	n, err := r.q.SumSequenceValues(ctx, branchID)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	err = r.q.UpdateBranchCode(ctx, sqlc.UpdateBranchCodeParams{
		Code: code, Now: timeutil.NowUTC(), UpdatedBy: actor(actorID), ID: branchID,
	})
	return false, err
}

// HasDocuments reports whether the branch ever issued a document number.
func (r *Repository) HasDocuments(ctx context.Context, branchID int64) (bool, error) {
	n, err := r.q.SumSequenceValues(ctx, branchID)
	return n > 0, err
}

// sequenceKeys expands kinds into counter keys. Order kinds reset monthly
// (replicating actual legacy behavior); everything else runs continuous
// across years (OQ-A17) — the year label stays informational only.
//
// The month key is computed in Asia/Jakarta (F2a-11): the printed label also
// uses Jakarta, so key and label can never disagree across the WIB midnight
// boundary (UTC keys labeled a Jakarta month collided after 07:00).
func sequenceKeys(kinds []contracts.DocKind, now time.Time) []string {
	month := now.In(timeutil.Jakarta).Format("2006-01")
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

func ensureSequences(ctx context.Context, q *sqlc.Queries, branchID int64, keys []string) error {
	for _, key := range keys {
		if err := q.InsertSequenceIgnore(ctx, sqlc.InsertSequenceIgnoreParams{BranchID: branchID, DocKind: key}); err != nil {
			return err
		}
	}
	return nil
}

// NextNumber atomically allocates the next sequence value for key using
// LAST_INSERT_ID, so concurrent callers never receive the same number.
//
// Hot path is UPDATE-first (F2a-11): the row exists after first use, and a
// bare UPDATE on one row only queues on its lock instead of deadlocking.
// INSERT IGNORE (gap locks) runs solely on first creation, where the retry
// below covers the one-time race. The unit retries on deadlock/lock-timeout:
// hundreds of concurrent order creations collide on the month row.
func (r *Repository) NextNumber(ctx context.Context, branchID int64, key string) (int64, error) {
	var n int64
	bump := sqlc.BumpSequenceParams{BranchID: branchID, DocKind: key}
	// F4-16 fast path: one statement, one round trip, no transaction. The OK
	// packet of UPDATE ... LAST_INSERT_ID(expr) carries expr as the insert id
	// of THIS connection, so concurrent callers never share a number. Anything
	// unusual (row missing, lock wait, deadlock) falls through to the
	// retrying transactional path below.
	if res, err := r.q.BumpSequence(ctx, bump); err == nil {
		if affected, aerr := res.RowsAffected(); aerr == nil && affected == 1 {
			if id, ierr := res.LastInsertId(); ierr == nil && id > 0 {
				return id, nil
			}
		}
	}
	err := dberr.RetryTx(ctx, r.db, func(tx *sql.Tx) error {
		q := r.q.WithTx(tx)
		res, err := q.BumpSequence(ctx, bump)
		if err != nil {
			return err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			if err := ensureSequences(ctx, q, branchID, []string{key}); err != nil {
				return err
			}
			if _, err := q.BumpSequence(ctx, bump); err != nil {
				return err
			}
		}
		n, err = q.LastInsertID(ctx)
		return err
	})
	if err != nil {
		return 0, err
	}
	return n, nil
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
		contracts.DocStockTransfer, contracts.DocGoodsReceipt,
	}
}
