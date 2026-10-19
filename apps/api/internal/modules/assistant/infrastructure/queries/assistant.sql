-- Assistant module queries (F8-18). Single-tenant, singleton rows id = 1.

-- name: GetChannel :one
SELECT state, phone, last_error, updated_at FROM assistant_channel WHERE id = 1;

-- name: InsertDefaultChannel :exec
INSERT INTO assistant_channel (id, state) VALUES (1, 'disconnected');

-- name: UpsertChannel :exec
INSERT INTO assistant_channel (id, state, phone, last_error) VALUES (1, ?, ?, ?)
ON DUPLICATE KEY UPDATE state = VALUES(state), phone = VALUES(phone), last_error = VALUES(last_error);

-- name: GetConfig :one
SELECT mode, rate_limit_per_minute, updated_at FROM assistant_config WHERE id = 1;

-- name: InsertDefaultConfig :exec
INSERT INTO assistant_config (id, mode, rate_limit_per_minute) VALUES (1, 'rule_based', 10);

-- name: UpsertConfig :exec
INSERT INTO assistant_config (id, mode, rate_limit_per_minute) VALUES (1, ?, ?)
ON DUPLICATE KEY UPDATE mode = VALUES(mode), rate_limit_per_minute = VALUES(rate_limit_per_minute);

-- name: ListActiveAuthorizations :many
SELECT id, phone, name, access_level, status, is_primary_owner, last_seen_at
FROM assistant_authorizations
WHERE status = 'active'
ORDER BY is_primary_owner DESC, id DESC;

-- name: ListAllAuthorizations :many
SELECT id, phone, name, access_level, status, is_primary_owner, last_seen_at
FROM assistant_authorizations
ORDER BY is_primary_owner DESC, id DESC;

-- name: FindActiveAuthorizationByPhone :one
SELECT id, phone, name, access_level, status, is_primary_owner, last_seen_at
FROM assistant_authorizations
WHERE phone = ? AND status = 'active';

-- name: GetAuthorizationStatusByPhone :one
SELECT status FROM assistant_authorizations WHERE phone = ? AND id <> ?;

-- name: CreateAuthorization :execresult
INSERT INTO assistant_authorizations
  (phone, name, access_level, status, is_primary_owner, created_by, updated_by)
VALUES (?, ?, ?, 'active', ?, ?, ?);

-- name: UpdateActiveAuthorization :execresult
UPDATE assistant_authorizations
SET phone = ?, name = ?, access_level = ?, is_primary_owner = ?, updated_by = ?
WHERE id = ? AND status = 'active';

-- name: ReactivateAuthorization :exec
UPDATE assistant_authorizations
SET name = ?, access_level = ?, is_primary_owner = ?, status = 'active',
    last_seen_at = NULL, updated_by = ?
WHERE id = ?;

-- name: RevokeAuthorization :execresult
UPDATE assistant_authorizations
SET status = 'revoked', is_primary_owner = 0, updated_by = ?
WHERE id = ? AND status = 'active';

-- name: DemoteOtherAuthorizations :exec
UPDATE assistant_authorizations SET is_primary_owner = 0 WHERE id <> ?;

-- name: TouchAuthorizationSeen :exec
UPDATE assistant_authorizations SET last_seen_at = sqlc.arg(now) WHERE id = sqlc.arg(id);

-- name: FindOpenThread :one
SELECT id FROM assistant_threads
WHERE phone = ? AND status = 'open'
ORDER BY id DESC LIMIT 1;

-- name: CreateThread :execresult
INSERT INTO assistant_threads (phone, branch_id) VALUES (?, ?);

-- name: TouchThread :exec
UPDATE assistant_threads SET last_message_at = sqlc.arg(now) WHERE id = sqlc.arg(id);

-- name: InsertMessage :execresult
INSERT INTO assistant_messages (thread_id, direction, body, status, run_id)
VALUES (?, ?, ?, ?, ?);

-- name: SetMessageStatus :exec
UPDATE assistant_messages SET status = ?, run_id = ? WHERE id = ?;

-- name: CreateRun :execresult
INSERT INTO assistant_runs (thread_id, phone, branch_id, actor_id, intent, mode, status)
VALUES (?, ?, ?, ?, ?, ?, 'running');

-- name: FinishRun :exec
UPDATE assistant_runs SET status = ?, answer = ?, duration_ms = ?, failure_reason = ? WHERE id = ?;

-- name: InsertToolExecution :exec
INSERT INTO assistant_tool_executions (run_id, seq, tool, input_json, output_json, duration_ms, status, error_text)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: RunStats :many
SELECT DATE(created_at) AS day, intent, mode,
       COUNT(*) AS total,
       CAST(SUM(status = 'completed') AS SIGNED) AS success,
       CAST(COALESCE(AVG(duration_ms), 0) AS DOUBLE) AS avg_duration_ms
FROM assistant_runs
WHERE created_at >= sqlc.arg(since)
GROUP BY day, intent, mode
ORDER BY day DESC, COUNT(*) DESC;

-- name: PurgeToolExecutions :execresult
DELETE FROM assistant_tool_executions WHERE created_at < sqlc.arg(cutoff) LIMIT ?;

-- name: PurgeMessages :execresult
DELETE FROM assistant_messages WHERE created_at < sqlc.arg(cutoff) LIMIT ?;

-- name: PurgeRuns :execresult
DELETE FROM assistant_runs WHERE created_at < sqlc.arg(cutoff) LIMIT ?;

-- name: PurgeIdleThreads :execresult
DELETE FROM assistant_threads
WHERE last_message_at < sqlc.arg(cutoff)
  AND NOT EXISTS (SELECT 1 FROM assistant_messages m WHERE m.thread_id = assistant_threads.id)
LIMIT ?;
