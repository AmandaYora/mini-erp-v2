package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/modules/user/infrastructure/sqlc"
)

// Repository owns the user module tables. It never touches other modules'
// tables — cross-module data arrives as primitive IDs. Data access goes
// through the sqlc-generated Queries (ADR-0012); only the dynamic user
// list/count in list_raw.go is hand-written (raw-sql allow-list).
type Repository struct {
	db *sql.DB
	q  *sqlc.Queries
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: sqlc.New(db)}
}

// GetByID returns nil, nil when the user does not exist.
func (r *Repository) GetByID(ctx context.Context, id int64) (*contracts.User, error) {
	row, err := r.q.GetUser(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toUser(row), nil
}

// NamesByIDs resolves display names for many users in one read (full_name,
// else username). Unknown ids are absent from the map.
func (r *Repository) NamesByIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	out := map[int64]string{}
	seen := map[int64]bool{}
	args := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		args = append(args, id)
	}
	for start := 0; start < len(args); start += 1000 {
		end := start + 1000
		if end > len(args) {
			end = len(args)
		}
		rows, err := r.q.NamesByIDs(ctx, args[start:end])
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			out[row.ID] = displayName(row.FullName, row.Username)
		}
	}
	return out, nil
}

// GetByUsername returns nil, nil when the username does not exist.
func (r *Repository) GetByUsername(ctx context.Context, username string) (*contracts.User, error) {
	row, err := r.q.GetUserByUsername(ctx, username)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toUser(sqlc.GetUserRow(row)), nil
}

// GetByLogin resolves a user by username OR email. When two different users
// match (A's username = B's email) the username match wins deterministically.
// An empty identifier never matches (NULL emails are not "").
func (r *Repository) GetByLogin(ctx context.Context, identifier string) (*contracts.User, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, nil
	}
	row, err := r.q.GetUserByLogin(ctx, sqlc.GetUserByLoginParams{
		Username: identifier, Email: nullStr(identifier), Username_2: identifier,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toUser(sqlc.GetUserRow(row)), nil
}

// TouchLogin stamps the last successful login.
func (r *Repository) TouchLogin(ctx context.Context, userID int64) error {
	return r.q.TouchLogin(ctx, sqlc.TouchLoginParams{Now: sql.NullTime{Time: now(), Valid: true}, ID: userID})
}

// GetPasswordHash is credential material for VerifyCredentials. It never
// leaves the module — callers receive only the verified *contracts.User.
func (r *Repository) GetPasswordHash(ctx context.Context, id int64) (string, error) {
	hash, err := r.q.GetPasswordHash(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return hash, err
}

// ExistsByUsername reports whether another row holds the username.
func (r *Repository) ExistsByUsername(ctx context.Context, username string, excludeID int64) (bool, error) {
	n, err := r.q.ExistsByUsername(ctx, sqlc.ExistsByUsernameParams{Username: username, ID: excludeID})
	return n > 0, err
}

// ExistsByEmail ignores NULL emails: NULL is never equal to NULL, so any
// number of users may have no email (KI-22). Empty strings must already be
// normalized to NULL by the caller.
func (r *Repository) ExistsByEmail(ctx context.Context, email string, excludeID int64) (bool, error) {
	n, err := r.q.ExistsByEmail(ctx, sqlc.ExistsByEmailParams{Email: nullStr(email), ID: excludeID})
	return n > 0, err
}

// TxHook runs inside a write's own transaction after its rows are written, so
// the audit row co-commits with the change (ADR-0011); an error rolls the
// whole write back.
type TxHook func(ctx context.Context, tx *sql.Tx) error

func runHooks(ctx context.Context, tx *sql.Tx, hooks []TxHook) error {
	for _, h := range hooks {
		if h == nil {
			continue
		}
		if err := h(ctx, tx); err != nil {
			return err
		}
	}
	return nil
}

// CreateUser inserts the account plus role and branch links atomically. The
// optional hook receives the new id inside the same transaction.
func (r *Repository) CreateUser(ctx context.Context, u *contracts.User, passwordHash string, roleIDs []int64, branches []contracts.BranchAccess, actorID int64, hook ...func(ctx context.Context, tx *sql.Tx, id int64) error) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.q.WithTx(tx)
	res, err := q.InsertUser(ctx, sqlc.InsertUserParams{
		Username: u.Username, Email: nullEmail(u.Email), Phone: nullStr(u.Phone),
		PasswordHash: passwordHash, FullName: u.FullName, DisplayTitle: nullStr(u.DisplayTitle),
		CreatedBy: actor(actorID), UpdatedBy: actor(actorID),
	})
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := replaceRoles(ctx, q, id, roleIDs); err != nil {
		return 0, err
	}
	if err := replaceBranches(ctx, q, id, branches); err != nil {
		return 0, err
	}
	for _, h := range hook {
		if h == nil {
			continue
		}
		if err := h(ctx, tx, id); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// UpdateUser rewrites profile fields plus role and branch links atomically.
// When guard is true the write is refused when it would leave zero managers
// (F1-7 last-admin rule); pass false for composition-root writes that only
// ever add capabilities.
func (r *Repository) UpdateUser(ctx context.Context, u *contracts.User, roleIDs []int64, branches []contracts.BranchAccess, actorID int64, guard bool, hooks ...TxHook) error {
	return r.writeGuarded(ctx, guard, func(tx *sql.Tx, q *sqlc.Queries) error {
		if err := q.UpdateUserProfile(ctx, sqlc.UpdateUserProfileParams{
			Username: u.Username, Email: nullEmail(u.Email), Phone: nullStr(u.Phone),
			FullName: u.FullName, DisplayTitle: nullStr(u.DisplayTitle),
			Now: now(), UpdatedBy: actor(actorID), ID: u.ID,
		}); err != nil {
			return err
		}
		if err := replaceRoles(ctx, q, u.ID, roleIDs); err != nil {
			return err
		}
		if err := replaceBranches(ctx, q, u.ID, branches); err != nil {
			return err
		}
		return runHooks(ctx, tx, hooks)
	})
}

func replaceRoles(ctx context.Context, q *sqlc.Queries, userID int64, roleIDs []int64) error {
	if err := q.DeleteUserRoles(ctx, userID); err != nil {
		return err
	}
	for _, roleID := range roleIDs {
		if err := q.InsertUserRole(ctx, sqlc.InsertUserRoleParams{UserID: userID, RoleID: roleID}); err != nil {
			return err
		}
	}
	return nil
}

func replaceBranches(ctx context.Context, q *sqlc.Queries, userID int64, branches []contracts.BranchAccess) error {
	if err := q.DeleteUserBranches(ctx, userID); err != nil {
		return err
	}
	for _, b := range branches {
		if err := q.InsertUserBranch(ctx, sqlc.InsertUserBranchParams{
			UserID: userID, BranchID: b.BranchID, IsDefault: b.IsDefault,
		}); err != nil {
			return err
		}
	}
	return nil
}

// SetStatus flips active/inactive. With guard it runs in a transaction that
// refuses to leave zero managers (F1-7).
func (r *Repository) SetStatus(ctx context.Context, userID int64, status string, actorID int64, guard bool, hooks ...TxHook) error {
	return r.writeGuarded(ctx, guard, func(tx *sql.Tx, q *sqlc.Queries) error {
		if err := q.SetUserStatus(ctx, sqlc.SetUserStatusParams{
			Status: status, Now: now(), UpdatedBy: actor(actorID), ID: userID,
		}); err != nil {
			return err
		}
		return runHooks(ctx, tx, hooks)
	})
}

// UpgradePasswordHash swaps a verified hash for a stronger one of the same
// password. It touches nothing else (no updated_at/updated_by: the user did not
// change anything) and only applies while the stored hash is still oldHash.
func (r *Repository) UpgradePasswordHash(ctx context.Context, userID int64, oldHash, newHash string) error {
	return r.q.UpgradePasswordHash(ctx, sqlc.UpgradePasswordHashParams{
		PasswordHash: newHash, ID: userID, PasswordHash_2: oldHash,
	})
}

// SetPasswordHash replaces the credential (already bcrypt-hashed by caller).
func (r *Repository) SetPasswordHash(ctx context.Context, userID int64, hash string, actorID int64, hooks ...TxHook) error {
	return r.writeGuarded(ctx, false, func(tx *sql.Tx, q *sqlc.Queries) error {
		if err := q.SetPasswordHash(ctx, sqlc.SetPasswordHashParams{
			PasswordHash: hash, Now: now(), UpdatedBy: actor(actorID), ID: userID,
		}); err != nil {
			return err
		}
		return runHooks(ctx, tx, hooks)
	})
}

// GetRoleIDs returns the user's role IDs in stable order.
func (r *Repository) GetRoleIDs(ctx context.Context, userID int64) ([]int64, error) {
	return r.q.ListRoleIDsByUser(ctx, userID)
}

// GetBranchAccess returns the user's branch links.
func (r *Repository) GetBranchAccess(ctx context.Context, userID int64) ([]contracts.BranchAccess, error) {
	rows, err := r.q.ListBranchAccessByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	var out []contracts.BranchAccess
	for _, row := range rows {
		out = append(out, contracts.BranchAccess{BranchID: row.BranchID, IsDefault: row.IsDefault})
	}
	return out, nil
}

// RoleByID returns nil, nil when the role does not exist.
func (r *Repository) RoleByID(ctx context.Context, id int64) (*contracts.Role, error) {
	row, err := r.q.GetRole(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toRole(row), nil
}

// RoleByCode returns nil, nil when the code does not exist.
func (r *Repository) RoleByCode(ctx context.Context, code string) (*contracts.Role, error) {
	row, err := r.q.GetRoleByCode(ctx, code)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toRole(sqlc.GetRoleRow(row)), nil
}

// ListRoles returns all roles ordered by code.
func (r *Repository) ListRoles(ctx context.Context) ([]*contracts.Role, error) {
	rows, err := r.q.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	var out []*contracts.Role
	for _, row := range rows {
		out = append(out, toRole(sqlc.GetRoleRow(row)))
	}
	return out, nil
}

// CreateRole inserts a role row.
func (r *Repository) CreateRole(ctx context.Context, role *contracts.Role, hook ...func(ctx context.Context, tx *sql.Tx, id int64) error) (int64, error) {
	isSystem, isReserved := false, false
	if role.IsSystem {
		isSystem = true
	}
	if role.IsReserved {
		isReserved = true
		isSystem = true
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := r.q.WithTx(tx).InsertRole(ctx, sqlc.InsertRoleParams{
		Code: role.Code, Name: role.Name, Description: nullStr(role.Description),
		IsSystem: isSystem, IsReserved: isReserved,
	})
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, h := range hook {
		if h == nil {
			continue
		}
		if err := h(ctx, tx, id); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// UpdateRole rewrites name/description (code is immutable — it anchors
// seed data and audit history).
func (r *Repository) UpdateRole(ctx context.Context, role *contracts.Role, hooks ...TxHook) error {
	return r.writeGuarded(ctx, false, func(tx *sql.Tx, q *sqlc.Queries) error {
		if err := q.UpdateRole(ctx, sqlc.UpdateRoleParams{
			Name: role.Name, Description: nullStr(role.Description), Now: now(), ID: role.ID,
		}); err != nil {
			return err
		}
		return runHooks(ctx, tx, hooks)
	})
}

// DeleteRole removes a custom role. Callers must verify it is unused and not
// a system role first.
func (r *Repository) DeleteRole(ctx context.Context, id int64, hooks ...TxHook) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.q.WithTx(tx)
	if err := q.DeleteRolePermissions(ctx, id); err != nil {
		return err
	}
	if err := q.DeleteRole(ctx, id); err != nil {
		return err
	}
	if err := runHooks(ctx, tx, hooks); err != nil {
		return err
	}
	return tx.Commit()
}

// RoleInUse reports whether any user holds the role.
func (r *Repository) RoleInUse(ctx context.Context, id int64) (bool, error) {
	n, err := r.q.RoleInUse(ctx, id)
	return n > 0, err
}

// PermissionCodes returns all catalog codes (for seed + validation).
func (r *Repository) PermissionCodes(ctx context.Context) ([]string, error) {
	return r.q.ListPermissionCodes(ctx)
}

// ListPermissions returns the full catalog with display names (for the
// role-permissions UI).
func (r *Repository) ListPermissions(ctx context.Context) ([]contracts.PermissionSeed, error) {
	rows, err := r.q.ListPermissions(ctx)
	if err != nil {
		return nil, err
	}
	out := []contracts.PermissionSeed{}
	for _, row := range rows {
		out = append(out, contracts.PermissionSeed{Code: row.Code, Name: row.Name})
	}
	return out, nil
}

// GetPermissionCodes resolves distinct permission codes for the given roles.
func (r *Repository) GetPermissionCodes(ctx context.Context, roleIDs []int64) ([]string, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}
	return r.q.GetPermissionCodesByRoles(ctx, roleIDs)
}

// SetRolePermissions replaces the role's permission set atomically. With
// guard the replacement is refused when it would leave zero managers (F1-7).
func (r *Repository) SetRolePermissions(ctx context.Context, roleID int64, permissionIDs []int64, guard bool, hooks ...TxHook) error {
	return r.writeGuarded(ctx, guard, func(tx *sql.Tx, q *sqlc.Queries) error {
		if err := q.DeleteRolePermissions(ctx, roleID); err != nil {
			return err
		}
		for _, pid := range permissionIDs {
			if err := q.InsertRolePermission(ctx, sqlc.InsertRolePermissionParams{RoleID: roleID, PermissionID: pid}); err != nil {
				return err
			}
		}
		return runHooks(ctx, tx, hooks)
	})
}

// ErrNoManagerLeft aborts writes that would leave zero pengelola (F1-7).
// The service maps it to 409 Conflict with a business message.
var ErrNoManagerLeft = errors.New("harus tersisa minimal satu pengelola aktif")

// writeGuarded runs fn in a transaction; with guard it additionally refuses
// to commit when managers drop from >0 to 0 inside the same tx. Recovery
// stays possible: when zero managers already exist (before == 0) writes are
// allowed so an admin can repair.
func (r *Repository) writeGuarded(ctx context.Context, guard bool, fn func(tx *sql.Tx, q *sqlc.Queries) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.q.WithTx(tx)
	var before int64
	if guard {
		if before, err = q.CountManagers(ctx); err != nil {
			return err
		}
	}
	if err := fn(tx, q); err != nil {
		return err
	}
	if guard {
		after, err := q.CountManagers(ctx)
		if err != nil {
			return err
		}
		if before > 0 && after == 0 {
			return ErrNoManagerLeft
		}
	}
	return tx.Commit()
}

// CountManagers reports the current pengelola count (for tests and gates).
func (r *Repository) CountManagers(ctx context.Context) (int, error) {
	n, err := r.q.CountManagers(ctx)
	return int(n), err
}

// UserHasRoleCode reports whether the user holds the role code.
func (r *Repository) UserHasRoleCode(ctx context.Context, userID int64, code string) (bool, error) {
	n, err := r.q.UserHasRoleCode(ctx, sqlc.UserHasRoleCodeParams{Code: code, UserID: userID})
	return n > 0, err
}

// UserHasReservedRole reports whether the user holds any is_reserved role.
func (r *Repository) UserHasReservedRole(ctx context.Context, userID int64) (bool, error) {
	n, err := r.q.UserHasReservedRole(ctx, userID)
	return n > 0, err
}

// SetRoleReserved flips a role to reserved (+ system). Bootstrap only.
func (r *Repository) SetRoleReserved(ctx context.Context, code string) error {
	return r.q.SetRoleReserved(ctx, code)
}

// PermissionIDByCode resolves a catalog code to its row ID.
func (r *Repository) PermissionIDByCode(ctx context.Context, code string) (int64, bool, error) {
	id, err := r.q.PermissionIDByCode(ctx, code)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return id, true, err
}

// EnsurePermission inserts the catalog code when missing (idempotent seed helper).
func (r *Repository) EnsurePermission(ctx context.Context, code, name string) error {
	return r.q.EnsurePermission(ctx, sqlc.EnsurePermissionParams{Code: code, Name: name})
}

// SetRoleSystem flips an existing role to is_system (F1-6: upgrades roles
// created before the bootstrap, e.g. owner from the old seed).
func (r *Repository) SetRoleSystem(ctx context.Context, code string) error {
	return r.q.SetRoleSystem(ctx, code)
}

// AddRolePermissions attaches permissions without removing existing ones
// (F1-6: grant-once defaults; SetRolePermissions replaces instead).
func (r *Repository) AddRolePermissions(ctx context.Context, roleID int64, permissionIDs []int64) error {
	for _, pid := range permissionIDs {
		if err := r.q.AddRolePermission(ctx, sqlc.AddRolePermissionParams{RoleID: roleID, PermissionID: pid}); err != nil {
			return err
		}
	}
	return nil
}

// SeedGranted returns the permission codes already granted once to the role
// (F1-6: admin customizations are never overwritten by the seed).
func (r *Repository) SeedGranted(ctx context.Context, roleCode string) (map[string]bool, error) {
	rows, err := r.q.ListSeedGranted(ctx, roleCode)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, code := range rows {
		out[code] = true
	}
	return out, nil
}

// MarkSeedGranted records one granted-once default (idempotent).
func (r *Repository) MarkSeedGranted(ctx context.Context, roleCode, permCode string) error {
	return r.q.MarkSeedGranted(ctx, sqlc.MarkSeedGrantedParams{RoleCode: roleCode, PermissionCode: permCode})
}
