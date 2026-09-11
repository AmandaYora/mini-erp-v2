package infrastructure

import (
	"context"
	"database/sql"
	"strings"

	"mini-erp/internal/modules/user/contracts"
)

// Repository owns the user module tables. It never touches other modules'
// tables — cross-module data arrives as primitive IDs.
type Repository struct {
	db *sql.DB
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func scanUser(row *sql.Row, u *contracts.User) error {
	var email sql.NullString
	err := row.Scan(&u.ID, &u.Username, &email, &u.FullName, &u.Status)
	if email.Valid {
		u.Email = &email.String
	}
	return err
}

const userColumns = "id, username, email, full_name, status"

// GetByID returns nil, nil when the user does not exist.
func (r *Repository) GetByID(ctx context.Context, id int64) (*contracts.User, error) {
	u := &contracts.User{}
	err := scanUser(r.db.QueryRowContext(ctx,
		"SELECT "+userColumns+" FROM users WHERE id = ?", id), u)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// GetByUsername returns nil, nil when the username does not exist.
func (r *Repository) GetByUsername(ctx context.Context, username string) (*contracts.User, error) {
	u := &contracts.User{}
	err := scanUser(r.db.QueryRowContext(ctx,
		"SELECT "+userColumns+" FROM users WHERE username = ?", username), u)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// GetPasswordHash is credential material for VerifyCredentials. It never
// leaves the module — callers receive only the verified *contracts.User.
func (r *Repository) GetPasswordHash(ctx context.Context, id int64) (string, error) {
	var hash string
	err := r.db.QueryRowContext(ctx, "SELECT password_hash FROM users WHERE id = ?", id).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return hash, err
}

// ExistsByUsername reports whether another row holds the username.
func (r *Repository) ExistsByUsername(ctx context.Context, username string, excludeID int64) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE username = ? AND id <> ?", username, excludeID).Scan(&n)
	return n > 0, err
}

// ExistsByEmail ignores NULL emails: NULL is never equal to NULL, so any
// number of users may have no email (KI-22). Empty strings must already be
// normalized to NULL by the caller.
func (r *Repository) ExistsByEmail(ctx context.Context, email string, excludeID int64) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE email = ? AND id <> ?", email, excludeID).Scan(&n)
	return n > 0, err
}

// CreateUser inserts the account plus role and branch links atomically.
func (r *Repository) CreateUser(ctx context.Context, u *contracts.User, passwordHash string, roleIDs []int64, branches []contracts.BranchAccess, actorID int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO users (username, email, password_hash, full_name, status, created_by, updated_by)
		 VALUES (?, ?, ?, ?, 'active', ?, ?)`,
		u.Username, nullEmail(u.Email), passwordHash, u.FullName, actorID, actorID)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := replaceRoles(ctx, tx, id, roleIDs); err != nil {
		return 0, err
	}
	if err := replaceBranches(ctx, tx, id, branches); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// UpdateUser rewrites profile fields plus role and branch links atomically.
func (r *Repository) UpdateUser(ctx context.Context, u *contracts.User, roleIDs []int64, branches []contracts.BranchAccess, actorID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET username = ?, email = ?, full_name = ?,
		 updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?`,
		u.Username, nullEmail(u.Email), u.FullName, actorID, u.ID); err != nil {
		return err
	}
	if err := replaceRoles(ctx, tx, u.ID, roleIDs); err != nil {
		return err
	}
	if err := replaceBranches(ctx, tx, u.ID, branches); err != nil {
		return err
	}
	return tx.Commit()
}

func nullEmail(email *string) any {
	if email == nil || *email == "" {
		return nil
	}
	return *email
}

func replaceRoles(ctx context.Context, tx *sql.Tx, userID int64, roleIDs []int64) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM user_roles WHERE user_id = ?", userID); err != nil {
		return err
	}
	for _, roleID := range roleIDs {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO user_roles (user_id, role_id) VALUES (?, ?)", userID, roleID); err != nil {
			return err
		}
	}
	return nil
}

func replaceBranches(ctx context.Context, tx *sql.Tx, userID int64, branches []contracts.BranchAccess) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM user_branch_access WHERE user_id = ?", userID); err != nil {
		return err
	}
	for _, b := range branches {
		def := 0
		if b.IsDefault {
			def = 1
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO user_branch_access (user_id, branch_id, is_default) VALUES (?, ?, ?)",
			userID, b.BranchID, def); err != nil {
			return err
		}
	}
	return nil
}

// SetStatus flips active/inactive.
func (r *Repository) SetStatus(ctx context.Context, userID int64, status string, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE users SET status = ?, updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?",
		status, actorID, userID)
	return err
}

// SetPasswordHash replaces the credential (already bcrypt-hashed by caller).
func (r *Repository) SetPasswordHash(ctx context.Context, userID int64, hash string, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE users SET password_hash = ?, updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?",
		hash, actorID, userID)
	return err
}

// ListUsers searches username/full_name/email per word (all words must match
// somewhere — KI-25) with an optional status filter.
func (r *Repository) ListUsers(ctx context.Context, search, status string, limit, offset int) ([]*contracts.User, error) {
	var conds []string
	var args []any
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

	rows, err := r.db.QueryContext(ctx,
		"SELECT "+userColumns+" FROM users "+where+" ORDER BY full_name ASC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []*contracts.User
	for rows.Next() {
		u := &contracts.User{}
		var email sql.NullString
		if err := rows.Scan(&u.ID, &u.Username, &email, &u.FullName, &u.Status); err != nil {
			return nil, err
		}
		if email.Valid {
			u.Email = &email.String
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// CountUsers counts the same filter set as ListUsers.
func (r *Repository) CountUsers(ctx context.Context, search, status string) (int64, error) {
	var conds []string
	var args []any
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
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users "+where, args...).Scan(&total)
	return total, err
}

// GetRoleIDs returns the user's role IDs in stable order.
func (r *Repository) GetRoleIDs(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT role_id FROM user_roles WHERE user_id = ? ORDER BY role_id ASC", userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetBranchAccess returns the user's branch links.
func (r *Repository) GetBranchAccess(ctx context.Context, userID int64) ([]contracts.BranchAccess, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT branch_id, is_default FROM user_branch_access WHERE user_id = ? ORDER BY branch_id ASC", userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []contracts.BranchAccess
	for rows.Next() {
		var b contracts.BranchAccess
		var def int
		if err := rows.Scan(&b.BranchID, &def); err != nil {
			return nil, err
		}
		b.IsDefault = def == 1
		out = append(out, b)
	}
	return out, rows.Err()
}

// RoleByID returns nil, nil when the role does not exist.
func (r *Repository) RoleByID(ctx context.Context, id int64) (*contracts.Role, error) {
	role := &contracts.Role{}
	var isSystem int
	err := r.db.QueryRowContext(ctx,
		"SELECT id, code, name, description, is_system FROM roles WHERE id = ?", id).
		Scan(&role.ID, &role.Code, &role.Name, &role.Description, &isSystem)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	role.IsSystem = isSystem == 1
	return role, err
}

// RoleByCode returns nil, nil when the code does not exist.
func (r *Repository) RoleByCode(ctx context.Context, code string) (*contracts.Role, error) {
	role := &contracts.Role{}
	var isSystem int
	err := r.db.QueryRowContext(ctx,
		"SELECT id, code, name, description, is_system FROM roles WHERE code = ?", code).
		Scan(&role.ID, &role.Code, &role.Name, &role.Description, &isSystem)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	role.IsSystem = isSystem == 1
	return role, err
}

// ListRoles returns all roles ordered by code.
func (r *Repository) ListRoles(ctx context.Context) ([]*contracts.Role, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, code, name, description, is_system FROM roles ORDER BY code ASC")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Role
	for rows.Next() {
		role := &contracts.Role{}
		var isSystem int
		if err := rows.Scan(&role.ID, &role.Code, &role.Name, &role.Description, &isSystem); err != nil {
			return nil, err
		}
		role.IsSystem = isSystem == 1
		out = append(out, role)
	}
	return out, rows.Err()
}

// CreateRole inserts a role row.
func (r *Repository) CreateRole(ctx context.Context, role *contracts.Role) (int64, error) {
	isSystem := 0
	if role.IsSystem {
		isSystem = 1
	}
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO roles (code, name, description, is_system) VALUES (?, ?, ?, ?)",
		role.Code, role.Name, role.Description, isSystem)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateRole rewrites name/description (code is immutable — it anchors
// seed data and audit history).
func (r *Repository) UpdateRole(ctx context.Context, role *contracts.Role) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE roles SET name = ?, description = ?, updated_at = UTC_TIMESTAMP(6) WHERE id = ?",
		role.Name, role.Description, role.ID)
	return err
}

// DeleteRole removes a custom role. Callers must verify it is unused and not
// a system role first.
func (r *Repository) DeleteRole(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, "DELETE FROM role_permissions WHERE role_id = ?", id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM roles WHERE id = ?", id); err != nil {
		return err
	}
	return tx.Commit()
}

// RoleInUse reports whether any user holds the role.
func (r *Repository) RoleInUse(ctx context.Context, id int64) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_roles WHERE role_id = ?", id).Scan(&n)
	return n > 0, err
}

// PermissionCodes returns all catalog codes (for seed + validation).
func (r *Repository) PermissionCodes(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT code FROM permissions ORDER BY code ASC")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		out = append(out, code)
	}
	return out, rows.Err()
}

// ListPermissions returns the full catalog with display names (for the
// role-permissions UI).
func (r *Repository) ListPermissions(ctx context.Context) ([]contracts.PermissionSeed, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT code, name FROM permissions ORDER BY code ASC")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []contracts.PermissionSeed{}
	for rows.Next() {
		var p contracts.PermissionSeed
		if err := rows.Scan(&p.Code, &p.Name); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetPermissionCodes resolves distinct permission codes for the given roles.
func (r *Repository) GetPermissionCodes(ctx context.Context, roleIDs []int64) ([]string, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(roleIDs)-1) + "?"
	args := make([]any, len(roleIDs))
	for i, id := range roleIDs {
		args[i] = id
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT p.code FROM permissions p
		 JOIN role_permissions rp ON rp.permission_id = p.id
		 WHERE rp.role_id IN (`+placeholders+`) ORDER BY p.code ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		out = append(out, code)
	}
	return out, rows.Err()
}

// SetRolePermissions replaces the role's permission set atomically.
func (r *Repository) SetRolePermissions(ctx context.Context, roleID int64, permissionIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, "DELETE FROM role_permissions WHERE role_id = ?", roleID); err != nil {
		return err
	}
	for _, pid := range permissionIDs {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO role_permissions (role_id, permission_id) VALUES (?, ?)", roleID, pid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// PermissionIDByCode resolves a catalog code to its row ID.
func (r *Repository) PermissionIDByCode(ctx context.Context, code string) (int64, bool, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, "SELECT id FROM permissions WHERE code = ?", code).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	return id, true, err
}

// EnsurePermission inserts the catalog code when missing (idempotent seed helper).
func (r *Repository) EnsurePermission(ctx context.Context, code, name string) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO permissions (code, name) VALUES (?, ?) ON DUPLICATE KEY UPDATE name = VALUES(name)",
		code, name)
	return err
}
