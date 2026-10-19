-- User accounts + credentials + links (F8-18). All queries touch only
-- this module's tables, so the per-module schema block in sqlc.yaml keeps
-- enforcing the boundary at generate time. The dynamic user list/count
-- lives in list_raw.go (raw-sql allow-list).

-- name: GetUser :one
SELECT id, username, email, full_name, status, phone, display_title
FROM users WHERE id = ?;

-- name: GetUserByUsername :one
SELECT id, username, email, full_name, status, phone, display_title
FROM users WHERE username = ?;

-- name: GetUserByLogin :one
-- Username match wins deterministically when one user's username is
-- another user's email.
SELECT id, username, email, full_name, status, phone, display_title
FROM users WHERE username = ? OR email = ?
ORDER BY (username = ?) DESC, id ASC LIMIT 1;

-- name: TouchLogin :exec
UPDATE users SET last_login_at = sqlc.arg(now) WHERE id = ?;

-- name: GetPasswordHash :one
SELECT password_hash FROM users WHERE id = ?;

-- name: ExistsByUsername :one
SELECT COUNT(*) AS total FROM users WHERE username = ? AND id <> ?;

-- name: ExistsByEmail :one
SELECT COUNT(*) AS total FROM users WHERE email = ? AND id <> ?;

-- name: InsertUser :execresult
INSERT INTO users (username, email, phone, password_hash, full_name, display_title, status, created_by, updated_by)
VALUES (?, ?, ?, ?, ?, ?, 'active', ?, ?);

-- name: UpdateUserProfile :exec
UPDATE users SET username = ?, email = ?, phone = ?, full_name = ?, display_title = ?,
  updated_at = sqlc.arg(now), updated_by = ? WHERE id = ?;

-- name: SetUserStatus :exec
UPDATE users SET status = ?, updated_at = sqlc.arg(now), updated_by = ? WHERE id = ?;

-- name: UpgradePasswordHash :exec
-- Compare-and-swap on the old hash: only applies while nothing else
-- replaced it, and touches nothing else (no updated_at/updated_by).
UPDATE users SET password_hash = ? WHERE id = ? AND password_hash = ?;

-- name: SetPasswordHash :exec
UPDATE users SET password_hash = ?, updated_at = sqlc.arg(now), updated_by = ? WHERE id = ?;

-- name: DeleteUserRoles :exec
DELETE FROM user_roles WHERE user_id = ?;

-- name: InsertUserRole :exec
INSERT INTO user_roles (user_id, role_id) VALUES (?, ?);

-- name: DeleteUserBranches :exec
DELETE FROM user_branch_access WHERE user_id = ?;

-- name: InsertUserBranch :exec
INSERT INTO user_branch_access (user_id, branch_id, is_default) VALUES (?, ?, ?);

-- name: ListRoleIDsByUser :many
SELECT role_id FROM user_roles WHERE user_id = ? ORDER BY role_id ASC;

-- name: ListBranchAccessByUser :many
SELECT branch_id, is_default FROM user_branch_access WHERE user_id = ? ORDER BY branch_id ASC;

-- name: CountManagers :one
-- Distinct active users holding BOTH manager codes through any of their roles.
SELECT COUNT(DISTINCT ur.user_id) AS total FROM user_roles ur
JOIN users u ON u.id = ur.user_id AND u.status = 'active'
JOIN role_permissions rp ON rp.role_id = ur.role_id
JOIN permissions p ON p.id = rp.permission_id AND p.code = 'roles.manage'
WHERE EXISTS (
  SELECT 1 FROM user_roles ur2
  JOIN role_permissions rp2 ON rp2.role_id = ur2.role_id
  JOIN permissions p2 ON p2.id = rp2.permission_id AND p2.code = 'users.update'
  WHERE ur2.user_id = ur.user_id
);

-- name: NamesByIDs :many
SELECT id, full_name, username FROM users WHERE id IN (sqlc.slice('ids'));
