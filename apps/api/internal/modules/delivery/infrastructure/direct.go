package infrastructure

import (
	"context"
	"database/sql"
	"errors"

	"mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/modules/delivery/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
)

// Storage of SJ Langsung (ADR-0022): direct notes drafted before their order
// exists. Everything here is sqlc; the CAS statements decide the races.

// NotesByIDs reads many headers in one query; unknown ids are absent.
func (r *Repository) NotesByIDs(ctx context.Context, ids []int64) (map[int64]*contracts.DeliveryNote, error) {
	out := map[int64]*contracts.DeliveryNote{}
	args := distinctIDs(ids)
	if len(args) == 0 {
		return out, nil
	}
	rows, err := r.q.NotesByIDs(ctx, args)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = toNote(sqlc.GetNoteRow(row))
	}
	return out, nil
}

// ItemsByDeliveries reads the lines of many notes in one query, keyed by note
// id, in row order.
func (r *Repository) ItemsByDeliveries(ctx context.Context, ids []int64) (map[int64][]*contracts.Item, error) {
	out := map[int64][]*contracts.Item{}
	args := distinctIDs(ids)
	if len(args) == 0 {
		return out, nil
	}
	rows, err := r.q.ListItemsByDeliveries(ctx, args)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.DeliveryID] = append(out[row.DeliveryID], toItemOfMany(row))
	}
	return out, nil
}

// ListOpenDirect lists confirmed direct notes without an order, oldest first
// (partyID 0 = every customer), bounded by limit.
func (r *Repository) ListOpenDirect(ctx context.Context, branchID, partyID int64, limit int) ([]*contracts.DeliveryNote, error) {
	rows, err := r.q.ListOpenDirect(ctx, sqlc.ListOpenDirectParams{
		BranchID: branchID, PartyID: dbutil.NullInt64(partyID), Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*contracts.DeliveryNote, 0, len(rows))
	for _, row := range rows {
		out = append(out, toNote(sqlc.GetNoteRow(row)))
	}
	return out, nil
}

// OpenDirectSummary counts the confirmed direct notes without an order and
// names the oldest delivery day (partyID 0 = every customer).
func (r *Repository) OpenDirectSummary(ctx context.Context, branchID, partyID int64) (contracts.OpenDirectSummary, error) {
	row, err := r.q.OpenDirectSummary(ctx, sqlc.OpenDirectSummaryParams{
		BranchID: branchID, PartyID: dbutil.NullInt64(partyID),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return contracts.OpenDirectSummary{}, nil
	}
	if err != nil {
		return contracts.OpenDirectSummary{}, err
	}
	return contracts.OpenDirectSummary{Count: row.Total, OldestDate: oldestDay(row.Oldest)}, nil
}

// LinkDirect attaches every note to the order in ONE transaction: each row is
// a CAS (origin direct, confirmed, unordered), and a single lost CAS rolls the
// whole set back and reports that note's id. The hook (audit row) runs inside
// the same transaction after every CAS won (ADR-0011). ids must be distinct.
func (r *Repository) LinkDirect(ctx context.Context, soID int64, ids []int64, actorID int64, hook func(ctx context.Context, tx *sql.Tx) error) (lost int64, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	t := now()
	q := r.q.WithTx(tx)
	for _, id := range ids {
		res, err := q.LinkDirectNote(ctx, sqlc.LinkDirectNoteParams{
			SalesOrderID: soID, Stamp: sql.NullTime{Time: t, Valid: true},
			ActorID: actor(actorID), Now: t, ID: id,
		})
		if err != nil {
			return 0, err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return 0, err
		}
		if affected != 1 {
			return id, nil
		}
	}
	if hook != nil {
		if err := hook(ctx, tx); err != nil {
			return 0, err
		}
	}
	return 0, tx.Commit()
}

// CancelDirect voids an unordered direct note (draft or confirmed) with its
// reason in one CAS; false = the note moved meanwhile (linked, cancelled). The
// hook co-commits the audit row only when the CAS won.
func (r *Repository) CancelDirect(ctx context.Context, id int64, reason string, actorID int64, hook func(ctx context.Context, tx *sql.Tx) error) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	t := now()
	res, err := r.q.WithTx(tx).CancelDirectNote(ctx, sqlc.CancelDirectNoteParams{
		Reason: dbutil.NullString(reason), Stamp: sql.NullTime{Time: t, Valid: true},
		ActorID: actor(actorID), Now: t, ID: id,
	})
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil || affected != 1 {
		return false, err
	}
	if hook != nil {
		if err := hook(ctx, tx); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

// distinctIDs drops zeros and duplicates, keeping the first order.
func distinctIDs(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
