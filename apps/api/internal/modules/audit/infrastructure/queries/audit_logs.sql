-- name: InsertAuditLog :execresult
INSERT INTO audit_logs (action_key, entity_type, entity_id, branch_id, actor_type, actor_id, note)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: InsertAuditLogDetail :exec
INSERT INTO audit_log_details (log_id, before_json, after_json, metadata_json)
VALUES (?, ?, ?, ?);

-- Branch scoping (F4-9): "branch OR NULL" is split into two index-friendly legs.
-- Leg 1 (this query): the session branch, or every row when branch_id is NULL
-- (unscoped view). The global (branch-less) leg is CountAuditLogsGlobal.
-- name: CountAuditLogs :one
SELECT COUNT(*) FROM audit_logs
WHERE (sqlc.narg(action_like) IS NULL OR action_key LIKE sqlc.narg(action_like))
  AND (sqlc.narg(entity) IS NULL OR entity_type = sqlc.narg(entity))
  AND (sqlc.narg(entity_id) IS NULL OR entity_id = sqlc.narg(entity_id))
  AND (sqlc.narg(actor_id) IS NULL OR actor_id = sqlc.narg(actor_id))
  AND (sqlc.narg(from_time) IS NULL OR created_at >= sqlc.narg(from_time))
  AND (sqlc.narg(to_time) IS NULL OR created_at <= sqlc.narg(to_time))
  AND (sqlc.narg(branch_id) IS NULL OR branch_id = sqlc.narg(branch_id));

-- Leg 2: global (branch-less) events, only used for branch-scoped views.
-- name: CountAuditLogsGlobal :one
SELECT COUNT(*) FROM audit_logs
WHERE (sqlc.narg(action_like) IS NULL OR action_key LIKE sqlc.narg(action_like))
  AND (sqlc.narg(entity) IS NULL OR entity_type = sqlc.narg(entity))
  AND (sqlc.narg(entity_id) IS NULL OR entity_id = sqlc.narg(entity_id))
  AND (sqlc.narg(actor_id) IS NULL OR actor_id = sqlc.narg(actor_id))
  AND (sqlc.narg(from_time) IS NULL OR created_at >= sqlc.narg(from_time))
  AND (sqlc.narg(to_time) IS NULL OR created_at <= sqlc.narg(to_time))
  AND branch_id IS NULL;

-- JSON columns are read through CAST ... AS CHAR: database/sql cannot scan a
-- NULL json.RawMessage, while []byte scans NULL as nil (same as before).
-- Newest-first page: each leg is capped at offset+limit ids (index seek), the
-- union is ordered and paged, then joined to the row and its detail.
-- name: QueryAuditLogs :many
SELECT a.id, a.action_key, a.entity_type, a.entity_id, a.branch_id, a.actor_type, a.actor_id,
       a.note, a.created_at, CAST(d.before_json AS CHAR) AS before_json, CAST(d.after_json AS CHAR) AS after_json,
       CAST(d.metadata_json AS CHAR) AS metadata_json
FROM (
  SELECT u.id FROM (
    (SELECT l1.id FROM audit_logs l1
      WHERE (sqlc.narg(action_like) IS NULL OR l1.action_key LIKE sqlc.narg(action_like))
        AND (sqlc.narg(entity) IS NULL OR l1.entity_type = sqlc.narg(entity))
        AND (sqlc.narg(entity_id) IS NULL OR l1.entity_id = sqlc.narg(entity_id))
        AND (sqlc.narg(actor_id) IS NULL OR l1.actor_id = sqlc.narg(actor_id))
        AND (sqlc.narg(from_time) IS NULL OR l1.created_at >= sqlc.narg(from_time))
        AND (sqlc.narg(to_time) IS NULL OR l1.created_at <= sqlc.narg(to_time))
        AND (sqlc.narg(branch_id) IS NULL OR l1.branch_id = sqlc.narg(branch_id))
      ORDER BY l1.id DESC LIMIT ?)
    UNION ALL
    (SELECT l2.id FROM audit_logs l2
      WHERE (sqlc.narg(action_like) IS NULL OR l2.action_key LIKE sqlc.narg(action_like))
        AND (sqlc.narg(entity) IS NULL OR l2.entity_type = sqlc.narg(entity))
        AND (sqlc.narg(entity_id) IS NULL OR l2.entity_id = sqlc.narg(entity_id))
        AND (sqlc.narg(actor_id) IS NULL OR l2.actor_id = sqlc.narg(actor_id))
        AND (sqlc.narg(from_time) IS NULL OR l2.created_at >= sqlc.narg(from_time))
        AND (sqlc.narg(to_time) IS NULL OR l2.created_at <= sqlc.narg(to_time))
        AND CAST(sqlc.arg(with_global) AS SIGNED) = 1 AND l2.branch_id IS NULL
      ORDER BY l2.id DESC LIMIT ?)
  ) u ORDER BY u.id DESC LIMIT ? OFFSET ?
) p
JOIN audit_logs a ON a.id = p.id
LEFT JOIN audit_log_details d ON d.log_id = a.id
ORDER BY a.id DESC;

-- name: PurgeAuditLogs :execresult
DELETE FROM audit_logs WHERE created_at < sqlc.arg(cutoff) ORDER BY created_at, id LIMIT ?;
