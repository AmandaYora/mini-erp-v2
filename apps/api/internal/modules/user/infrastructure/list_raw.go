package infrastructure

import (
	"context"
	"database/sql"
	"strings"

	"mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/modules/user/infrastructure/sqlc"
)

// ListUsers searches username/full_name/email per word (all words must match
// somewhere — KI-25) with an optional status filter. With hideReserved,
// holders of the superadmin role are excluded (F1-7: invisible to
// non-superadmin sessions).
func (r *Repository) ListUsers(ctx context.Context, search, status string, limit, offset int, hideReserved bool) ([]*contracts.User, error) {
	var conds []string
	var args []any
	if hideReserved {
		conds = append(conds, `NOT EXISTS (
		   SELECT 1 FROM user_roles ur JOIN roles r ON r.id = ur.role_id
		   AND r.is_reserved = 1 WHERE ur.user_id = users.id)`)
	}
	if status != "" {
		conds = append(conds, "status = ?")
		args = append(args, status)
	}
	for _, word := range strings.Fields(search) {
		like := "%" + word + "%"
		conds = append(conds, "(username LIKE ? OR full_name LIKE ? OR email LIKE ?)")
		args = append(args, like, like, like)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, limit, offset)

	// raw-sql: one (username|full_name|email LIKE) group per search word (arity
	// follows the word count) plus an optional reserved-holder exclusion;
	// sqlc.narg/slice cannot express it. Shared shape with CountUsers.
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, username, email, full_name, status, phone, display_title, last_login_at FROM users "+where+" ORDER BY full_name ASC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []*contracts.User
	for rows.Next() {
		var row sqlc.GetUserRow
		var lastLogin sql.NullTime
		if err := rows.Scan(&row.ID, &row.Username, &row.Email, &row.FullName, &row.Status, &row.Phone, &row.DisplayTitle, &lastLogin); err != nil {
			return nil, err
		}
		u := toUser(row)
		if lastLogin.Valid {
			t := lastLogin.Time
			u.LastLoginAt = &t
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// CountUsers counts the same filter set as ListUsers (including the
// reserved-holder exclusion when hideReserved).
func (r *Repository) CountUsers(ctx context.Context, search, status string, hideReserved bool) (int64, error) {
	var conds []string
	var args []any
	if hideReserved {
		conds = append(conds, `NOT EXISTS (
		   SELECT 1 FROM user_roles ur JOIN roles r ON r.id = ur.role_id
		   AND r.is_reserved = 1 WHERE ur.user_id = users.id)`)
	}
	if status != "" {
		conds = append(conds, "status = ?")
		args = append(args, status)
	}
	for _, word := range strings.Fields(search) {
		like := "%" + word + "%"
		conds = append(conds, "(username LIKE ? OR full_name LIKE ? OR email LIKE ?)")
		args = append(args, like, like, like)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	var total int64
	// raw-sql: same variable-arity filter as ListUsers (one builder shape so list and count cannot drift)
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users "+where, args...).Scan(&total)
	return total, err
}

// ListMetaRow is one role or branch-access link of a listed user.
type ListMetaRow struct {
	UserID    int64
	Kind      string // "role" | "branch"
	RefID     int64  // role id or branch id
	RoleName  string // role only
	IsDefault bool   // branch only
}

// ListMetaByUsers reads the role and branch-access links of many users in ONE
// query (U-01: the list decoration must not grow with the row count). Role
// names are this module's own table; branch names are resolved by the caller
// through the branch contract, never joined here.
func (r *Repository) ListMetaByUsers(ctx context.Context, userIDs []int64) ([]ListMetaRow, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(userIDs)), ",")
	args := make([]any, 0, len(userIDs)*2)
	for i := 0; i < 2; i++ {
		for _, id := range userIDs {
			args = append(args, id)
		}
	}
	// raw-sql: UNION ALL of two own-module link tables over a variable-length
	// IN list (sqlc.slice cannot be repeated across a UNION); one round trip.
	rows, err := r.db.QueryContext(ctx,
		"SELECT ur.user_id, 'role', r.id, r.name, 0 FROM user_roles ur JOIN roles r ON r.id = ur.role_id WHERE ur.user_id IN ("+marks+") "+
			"UNION ALL SELECT user_id, 'branch', branch_id, '', is_default FROM user_branch_access WHERE user_id IN ("+marks+") "+
			"ORDER BY 1, 2, 3", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []ListMetaRow
	for rows.Next() {
		var m ListMetaRow
		if err := rows.Scan(&m.UserID, &m.Kind, &m.RefID, &m.RoleName, &m.IsDefault); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
