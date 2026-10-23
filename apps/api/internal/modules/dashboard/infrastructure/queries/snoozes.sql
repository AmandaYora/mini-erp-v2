-- name: ListSnoozes :many
-- Every snooze row of a branch (at most one per alert); expiry is applied by
-- the caller against the clock it was given.
SELECT id, branch_id, alert_id, reason, duration_days, snoozed_until,
       count_at_snooze, amount_at_snooze, created_at, updated_at, created_by, updated_by
FROM dashboard_alert_snoozes
WHERE branch_id = ?;

-- name: GetSnooze :one
SELECT id, branch_id, alert_id, reason, duration_days, snoozed_until,
       count_at_snooze, amount_at_snooze, created_at, updated_at, created_by, updated_by
FROM dashboard_alert_snoozes
WHERE branch_id = ? AND alert_id = ?;

-- name: UpsertSnooze :exec
INSERT INTO dashboard_alert_snoozes
  (branch_id, alert_id, reason, duration_days, snoozed_until, count_at_snooze, amount_at_snooze,
   created_at, updated_at, created_by, updated_by)
VALUES (sqlc.arg(branch_id), sqlc.arg(alert_id), sqlc.arg(reason), sqlc.arg(duration_days), sqlc.arg(snoozed_until),
        sqlc.arg(count_at_snooze), sqlc.arg(amount_at_snooze), sqlc.arg(now), sqlc.arg(now), sqlc.arg(actor_id), sqlc.arg(actor_id))
ON DUPLICATE KEY UPDATE
  reason = VALUES(reason), duration_days = VALUES(duration_days), snoozed_until = VALUES(snoozed_until),
  count_at_snooze = VALUES(count_at_snooze), amount_at_snooze = VALUES(amount_at_snooze),
  updated_at = VALUES(updated_at), updated_by = VALUES(updated_by);

-- name: DeleteSnooze :exec
DELETE FROM dashboard_alert_snoozes WHERE id = ?;
