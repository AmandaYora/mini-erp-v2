package infrastructure

import (
	"context"
	"database/sql"
	"strings"

	"mini-erp/internal/modules/audit/contracts"
)

// Repository owns the audit_logs table.
type Repository struct {
	db *sql.DB
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Log inserts one trail record.
func (r *Repository) Log(ctx context.Context, e contracts.Entry) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO audit_logs (action_key, entity_type, entity_id, branch_id, actor_id, note)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		e.Action, e.Entity, nullIfZero(e.EntityID), nullIfZero(e.BranchID),
		nullIfZero(e.ActorID), nullIfEmpty(e.Note))
	return err
}

// List returns newest-first records for one branch with optional filters.
func (r *Repository) List(ctx context.Context, branchID int64, action, entity string, limit, offset int) ([]contracts.Record, error) {
	var conds []string
	var args []any
	if branchID > 0 {
		// Branch scope plus global events (branch NULL: users/roles/company)
		// which every branch may see.
		conds = append(conds, "(branch_id = ? OR branch_id IS NULL)")
		args = append(args, branchID)
	}
	if action != "" {
		conds = append(conds, "action_key = ?")
		args = append(args, action)
	}
	if entity != "" {
		conds = append(conds, "entity_type = ?")
		args = append(args, entity)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, action_key, entity_type, entity_id, branch_id, actor_id, note, created_at
		 FROM audit_logs `+where+`
		 ORDER BY id DESC LIMIT ? OFFSET ?`,
		append(args, limit, offset)...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []contracts.Record{}
	for rows.Next() {
		var rec contracts.Record
		var entityID, branchID, actorID sql.NullInt64
		var note sql.NullString
		var created sql.NullTime
		if err := rows.Scan(&rec.ID, &rec.Action, &rec.Entity,
			&entityID, &branchID, &actorID, &note, &created); err != nil {
			return nil, err
		}
		rec.EntityID, rec.BranchID, rec.ActorID = entityID.Int64, branchID.Int64, actorID.Int64
		rec.Note = note.String
		if created.Valid {
			rec.CreatedAt = created.Time.Format("2006-01-02 15:04:05")
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// Count returns the total rows matching the same filters as List.
func (r *Repository) Count(ctx context.Context, branchID int64, action, entity string) (int64, error) {
	var conds []string
	var args []any
	if branchID > 0 {
		conds = append(conds, "(branch_id = ? OR branch_id IS NULL)")
		args = append(args, branchID)
	}
	if action != "" {
		conds = append(conds, "action_key = ?")
		args = append(args, action)
	}
	if entity != "" {
		conds = append(conds, "entity_type = ?")
		args = append(args, entity)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	var total int64
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM audit_logs `+where, args...).Scan(&total)
	return total, err
}

func nullIfZero(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
