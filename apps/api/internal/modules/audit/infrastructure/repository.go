package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/audit/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
)

// Repository owns the audit_logs table.
type Repository struct {
	db *sql.DB
	q  *sqlc.Queries
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: sqlc.New(db)}
}

// maxDetailBytes caps one JSON detail at 16 KB (F4-9/F3b-8): larger payloads
// are truncated with a marker instead of widening rows or failing writes.
const maxDetailBytes = 16 * 1024

// marshalDetail renders one diff value: nil/empty renders NULL (no detail
// row), oversize renders a truncation marker object.
func marshalDetail(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	if len(raw) == 0 || string(raw) == "null" || string(raw) == `""` || string(raw) == "[]" || string(raw) == "{}" {
		return nil
	}
	if len(raw) > maxDetailBytes {
		raw, _ = json.Marshal(map[string]any{"_truncated": true, "bytes": len(raw)})
	}
	return json.RawMessage(raw)
}

func actorTypeOf(e contracts.Entry) string {
	if strings.TrimSpace(e.ActorType) == "system" {
		return "system"
	}
	return "user"
}

// Log inserts one trail record plus its detail row when a diff is present
// (two INSERTs in one short transaction — still best-effort overall: the
// service logs storage failures and never fails the caller).
func (r *Repository) Log(ctx context.Context, e contracts.Entry) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := logTx(ctx, tx, e); err != nil {
		return err
	}
	return tx.Commit()
}

// LogTx inserts one trail record inside the caller's transaction (ADR-0011).
func (r *Repository) LogTx(ctx context.Context, tx contracts.AuditTx, e contracts.Entry) error {
	return logTx(ctx, tx, e)
}

func logTx(ctx context.Context, tx contracts.AuditTx, e contracts.Entry) error {
	q := sqlc.New(execOnly{tx})
	res, err := q.InsertAuditLog(ctx, sqlc.InsertAuditLogParams{
		ActionKey: e.Action, EntityType: e.Entity,
		EntityID: dbutil.NullInt64(e.EntityID), BranchID: dbutil.NullInt64(e.BranchID),
		ActorType: actorTypeOf(e), ActorID: dbutil.NullInt64(e.ActorID), Note: dbutil.NullString(e.Note),
	})
	if err != nil {
		return err
	}
	before, after, meta := marshalDetail(e.Before), marshalDetail(e.After), marshalDetail(e.Metadata)
	if before == nil && after == nil && meta == nil {
		return nil
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	return q.InsertAuditLogDetail(ctx, sqlc.InsertAuditLogDetailParams{
		LogID: id, BeforeJson: before, AfterJson: after, MetadataJson: meta,
	})
}

// likePrefix escapes LIKE wildcards and appends the prefix wildcard.
func likePrefix(s string) string {
	r := strings.NewReplacer(`\`, `\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s) + "%"
}

// Query returns newest-first records (with before/after detail) and the
// exact total for the filter. A branch-scoped view reads two index-friendly
// legs (the branch, then global events); an unscoped view reads one.
func (r *Repository) Query(ctx context.Context, f contracts.ListFilter) ([]contracts.Record, int64, error) {
	scoped := !(f.AllBranches || f.BranchID <= 0)
	flt := toFilter(f)
	total, err := r.q.CountAuditLogs(ctx, flt.count(scoped, f.BranchID))
	if err != nil {
		return nil, 0, err
	}
	if scoped {
		n, err := r.q.CountAuditLogsGlobal(ctx, flt.countGlobal())
		if err != nil {
			return nil, 0, err
		}
		total += n
	}
	offset := (f.Page - 1) * f.Limit
	rows, err := r.q.QueryAuditLogs(ctx, flt.page(scoped, f.BranchID, offset, f.Limit))
	if err != nil {
		return nil, 0, err
	}
	out := make([]contracts.Record, 0, len(rows))
	for _, row := range rows {
		out = append(out, toRecord(row))
	}
	return out, total, nil
}

// Purge deletes up to limit audit rows created before cutoff, oldest first
// (details cascade). Idempotent and bounded: callers loop until it returns
// fewer rows than limit.
func (r *Repository) Purge(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 5000
	}
	res, err := r.q.PurgeAuditLogs(ctx, sqlc.PurgeAuditLogsParams{Cutoff: cutoff.UTC(), Limit: int32(limit)})
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// List returns newest-first records for one branch (plus global) with
// optional filters; kept for the AuditClient contract.
func (r *Repository) List(ctx context.Context, branchID int64, action, entity string, limit, offset int) ([]contracts.Record, error) {
	page := 1
	if limit > 0 {
		page = offset/limit + 1
	}
	recs, _, err := r.Query(ctx, contracts.ListFilter{BranchID: branchID, AllBranches: branchID <= 0,
		Action: action, Entity: entity, Page: page, Limit: limit})
	return recs, err
}

// Count returns the total rows matching the same filters as List.
func (r *Repository) Count(ctx context.Context, branchID int64, action, entity string) (int64, error) {
	_, total, err := r.Query(ctx, contracts.ListFilter{BranchID: branchID, AllBranches: branchID <= 0,
		Action: action, Entity: entity, Page: 1, Limit: 1})
	return total, err
}
