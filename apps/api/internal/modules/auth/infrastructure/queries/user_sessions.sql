-- name: InsertUserSession :execresult
INSERT INTO user_sessions
  (user_id, refresh_hash, id_active_branch, id_active_role, user_agent, ip_address,
   expires_at, absolute_expires_at, last_activity_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetUserSessionByID :one
SELECT id, user_id, refresh_hash, prev_refresh_hash, rotated_at, id_active_branch,
       id_active_role, expires_at, absolute_expires_at, revoked_at
FROM user_sessions WHERE id = ?;

-- name: GetUserSessionByRefreshHash :one
SELECT id, user_id, refresh_hash, prev_refresh_hash, rotated_at, id_active_branch,
       id_active_role, expires_at, absolute_expires_at, revoked_at
FROM user_sessions WHERE refresh_hash = ?;

-- name: GetUserSessionByPrevHash :one
SELECT id, user_id, refresh_hash, prev_refresh_hash, rotated_at, id_active_branch,
       id_active_role, expires_at, absolute_expires_at, revoked_at
FROM user_sessions WHERE prev_refresh_hash = ?;

-- CAS rotation (F1-10): only the holder of the current hash wins.
-- name: RotateUserSession :execresult
UPDATE user_sessions SET prev_refresh_hash = refresh_hash, refresh_hash = sqlc.arg(new_hash),
  rotated_at = sqlc.arg(rotated_at), expires_at = sqlc.arg(new_expiry), last_activity_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND refresh_hash = sqlc.arg(old_hash) AND revoked_at IS NULL;

-- name: RevokeUserSession :exec
UPDATE user_sessions SET revoked_at = sqlc.arg(now) WHERE id = sqlc.arg(id);

-- name: RevokeAllUserSessions :exec
UPDATE user_sessions SET revoked_at = sqlc.arg(now) WHERE user_id = sqlc.arg(user_id) AND revoked_at IS NULL;

-- name: RevokeUserSessionsExcept :execresult
UPDATE user_sessions SET revoked_at = sqlc.arg(now)
WHERE user_id = sqlc.arg(user_id) AND revoked_at IS NULL AND id <> sqlc.arg(except_id);

-- name: UpdateUserSessionBranchRole :exec
UPDATE user_sessions SET id_active_branch = sqlc.arg(branch_id), id_active_role = sqlc.arg(role_id),
  last_activity_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: PurgeUserSessions :execresult
DELETE FROM user_sessions
WHERE (revoked_at IS NOT NULL AND revoked_at < sqlc.arg(revoked_cutoff)) OR absolute_expires_at < sqlc.arg(expired_cutoff)
LIMIT ?;
