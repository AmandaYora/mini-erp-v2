package infrastructure

import (
	"context"
	"strings"

	"mini-erp/internal/modules/branch/contracts"
	"mini-erp/internal/modules/branch/infrastructure/sqlc"
)

// searchWhere builds the branch list filter: an optional status plus one
// (code|name|city LIKE) condition per whitespace-separated search word, all
// ANDed. The number of conditions follows the number of words.
func searchWhere(search, status string) (string, []any) {
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
	if len(conds) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}

// List returns branches with optional search and status filter.
func (r *Repository) List(ctx context.Context, search, status string, limit, offset int) ([]*contracts.Branch, error) {
	where, args := searchWhere(search, status)
	args = append(args, limit, offset)
	// raw-sql: variable-arity AND of one LIKE group per search word; sqlc.narg/slice cannot express a per-word condition list
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, code, name, address, city, phone, status, is_head, default_location_label FROM branches "+
			where+" ORDER BY name ASC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Branch
	for rows.Next() {
		var row sqlc.GetBranchRow
		if err := rows.Scan(&row.ID, &row.Code, &row.Name, &row.Address, &row.City, &row.Phone,
			&row.Status, &row.IsHead, &row.DefaultLocationLabel); err != nil {
			return nil, err
		}
		out = append(out, toBranch(row))
	}
	return out, rows.Err()
}

// Count counts the List filter set.
func (r *Repository) Count(ctx context.Context, search, status string) (int64, error) {
	where, args := searchWhere(search, status)
	var total int64
	// raw-sql: same variable-arity per-word filter as List (kept in one place so list and count cannot drift)
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM branches "+where, args...).Scan(&total)
	return total, err
}
