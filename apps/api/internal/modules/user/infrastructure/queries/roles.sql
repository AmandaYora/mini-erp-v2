-- Roles, permissions, links, seed bookkeeping (F8-18). Same module, same
-- boundary as users.sql.

-- name: GetRole :one
SELECT id, code, name, description, is_system, is_reserved FROM roles WHERE id = ?;

-- name: GetRoleByCode :one
SELECT id, code, name, description, is_system, is_reserved FROM roles WHERE code = ?;

-- name: ListRoles :many
SELECT id, code, name, description, is_system, is_reserved FROM roles ORDER BY code ASC;

-- name: InsertRole :execresult
INSERT INTO roles (code, name, description, is_system, is_reserved) VALUES (?, ?, ?, ?, ?);

-- name: UpdateRole :exec
UPDATE roles SET name = ?, description = ?, updated_at = sqlc.arg(now) WHERE id = ?;

-- name: DeleteRolePermissions :exec
DELETE FROM role_permissions WHERE role_id = ?;

-- name: DeleteRole :exec
DELETE FROM roles WHERE id = ?;

-- name: RoleInUse :one
SELECT COUNT(*) AS total FROM user_roles WHERE role_id = ?;

-- name: UserHasRoleCode :one
SELECT COUNT(*) AS total FROM user_roles ur
JOIN roles r ON r.id = ur.role_id AND r.code = ?
WHERE ur.user_id = ?;

-- name: UserHasReservedRole :one
SELECT COUNT(*) AS total FROM user_roles ur
JOIN roles r ON r.id = ur.role_id AND r.is_reserved = 1
WHERE ur.user_id = ?;

-- name: ListPermissionCodes :many
SELECT code FROM permissions ORDER BY code ASC;

-- name: ListPermissions :many
SELECT code, name FROM permissions ORDER BY code ASC;

-- name: GetPermissionCodesByRoles :many
SELECT DISTINCT p.code FROM permissions p
JOIN role_permissions rp ON rp.permission_id = p.id
WHERE rp.role_id IN (sqlc.slice('role_ids')) ORDER BY p.code ASC;

-- name: InsertRolePermission :exec
INSERT INTO role_permissions (role_id, permission_id) VALUES (?, ?);

-- name: AddRolePermission :exec
INSERT IGNORE INTO role_permissions (role_id, permission_id) VALUES (?, ?);

-- name: PermissionIDByCode :one
SELECT id FROM permissions WHERE code = ?;

-- name: EnsurePermission :exec
INSERT INTO permissions (code, name) VALUES (?, ?)
ON DUPLICATE KEY UPDATE name = VALUES(name);

-- name: SetRoleSystem :exec
UPDATE roles SET is_system = 1 WHERE code = ?;

-- name: SetRoleReserved :exec
UPDATE roles SET is_reserved = 1, is_system = 1 WHERE code = ?;

-- name: ListSeedGranted :many
SELECT permission_code FROM authz_seed_applied WHERE role_code = ?;

-- name: MarkSeedGranted :exec
INSERT IGNORE INTO authz_seed_applied (role_code, permission_code) VALUES (?, ?);
