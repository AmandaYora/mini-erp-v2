package infrastructure

import (
	"context"
	"database/sql"
	"errors"

	"mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/audit/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
	"mini-erp/internal/shared/timeutil"
)

// execOnly adapts the borrowed contracts.AuditTx (ExecContext only, by design:
// the audit module may only INSERT into the caller's transaction) to the
// sqlc.DBTX interface. The other methods are never reached by LogTx's insert
// queries; they fail loudly instead of touching the caller's transaction.
type execOnly struct{ contracts.AuditTx }

var errExecOnly = errors.New("audit: borrowed transaction only supports Exec")

func (execOnly) PrepareContext(context.Context, string) (*sql.Stmt, error) { return nil, errExecOnly }

func (execOnly) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errExecOnly
}

// QueryRowContext cannot return an error row; it is unreachable (no :one/:many
// query runs through execOnly) and would panic, never silently succeed.
func (execOnly) QueryRowContext(context.Context, string, ...interface{}) *sql.Row { return nil }

// filter is the translated, bindable form of contracts.ListFilter.
type filter struct {
	actionLike sql.NullString
	entity     sql.NullString
	entityID   sql.NullInt64
	actorID    sql.NullInt64
	from, to   sql.NullTime
}

func toFilter(f contracts.ListFilter) filter {
	out := filter{
		entity:   dbutil.NullString(f.Entity),
		entityID: positive(f.EntityID),
		actorID:  positive(f.ActorID),
	}
	if f.Action != "" {
		out.actionLike = sql.NullString{String: likePrefix(f.Action), Valid: true}
	}
	if d, err := timeutil.ParseDateInput(f.From); err == nil && f.From != "" {
		out.from = sql.NullTime{Time: timeutil.StartOfDayWIB(d), Valid: true}
	}
	if d, err := timeutil.ParseDateInput(f.To); err == nil && f.To != "" {
		out.to = sql.NullTime{Time: timeutil.EndOfDayWIB(d), Valid: true}
	}
	return out
}

// positive maps non-positive ids to "no filter".
func positive(v int64) sql.NullInt64 {
	if v > 0 {
		return sql.NullInt64{Int64: v, Valid: true}
	}
	return sql.NullInt64{}
}

func branchArg(scoped bool, branchID int64) sql.NullInt64 {
	if !scoped {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: branchID, Valid: true}
}

func (f filter) count(scoped bool, branchID int64) sqlc.CountAuditLogsParams {
	return sqlc.CountAuditLogsParams{
		ActionLike: f.actionLike, Entity: f.entity, EntityID: f.entityID, ActorID: f.actorID,
		FromTime: f.from, ToTime: f.to, BranchID: branchArg(scoped, branchID),
	}
}

func (f filter) countGlobal() sqlc.CountAuditLogsGlobalParams {
	return sqlc.CountAuditLogsGlobalParams{
		ActionLike: f.actionLike, Entity: f.entity, EntityID: f.entityID, ActorID: f.actorID,
		FromTime: f.from, ToTime: f.to,
	}
}

// page binds both legs, each capped at offset+limit ids, then the outer page.
func (f filter) page(scoped bool, branchID int64, offset, limit int) sqlc.QueryAuditLogsParams {
	with := int64(0)
	if scoped {
		with = 1
	}
	return sqlc.QueryAuditLogsParams{
		ActionLike: f.actionLike, Entity: f.entity, EntityID: f.entityID, ActorID: f.actorID,
		FromTime: f.from, ToTime: f.to, BranchID: branchArg(scoped, branchID),
		Limit: int32(offset + limit), WithGlobal: with, Limit_2: int32(offset + limit),
		Limit_3: int32(limit), Offset: int32(offset),
	}
}

func toRecord(r sqlc.QueryAuditLogsRow) contracts.Record {
	rec := contracts.Record{
		ID: r.ID, Action: r.ActionKey, Entity: r.EntityType, EntityID: r.EntityID.Int64,
		BranchID: r.BranchID.Int64, ActorType: r.ActorType, ActorID: r.ActorID.Int64, Note: r.Note.String,
		Before: rawJSON(r.BeforeJson), After: rawJSON(r.AfterJson), Metadata: rawJSON(r.MetadataJson),
	}
	if !r.CreatedAt.IsZero() {
		rec.CreatedAt = r.CreatedAt.Format("2006-01-02 15:04:05")
	}
	return rec
}

// rawJSON reads a CAST(json AS CHAR) column (sqlc types it interface{}):
// NULL stays nil, text becomes bytes.
func rawJSON(v any) []byte {
	switch x := v.(type) {
	case []byte:
		return x
	case string:
		return []byte(x)
	}
	return nil
}
