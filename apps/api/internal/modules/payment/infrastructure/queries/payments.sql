-- name: GetPayment :one
SELECT id, number, branch_id, party_id, party_type, direction,
  amount, method, paid_at, notes, status, client_ref, amount_tendered, reference_number, pair_id,
  credit_source_type, credit_source_id
FROM payments WHERE id = ?;

-- name: GetPaymentByClientRef :one
SELECT id, number, branch_id, party_id, party_type, direction,
  amount, method, paid_at, notes, status, client_ref, amount_tendered, reference_number, pair_id,
  credit_source_type, credit_source_id
FROM payments WHERE branch_id = ? AND client_ref = ?;

-- name: InsertPayment :execresult
INSERT INTO payments
  (number, branch_id, party_id, party_type, direction, amount, method,
   paid_at, notes, status, client_ref, amount_tendered, reference_number,
   credit_source_type, credit_source_id, created_by, updated_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?, ?, ?, ?, ?, ?);

-- name: SetPairID :exec
UPDATE payments SET pair_id = ? WHERE id = ?;

-- name: SetPaymentStatus :exec
UPDATE payments SET status = ?, updated_at = sqlc.arg(now), updated_by = ? WHERE id = ?;

-- name: CancelActiveWithPair :execresult
UPDATE payments SET status = 'cancelled', updated_at = sqlc.arg(now), updated_by = sqlc.arg(actor)
WHERE status = 'active' AND (id = sqlc.arg(id) OR pair_id = sqlc.arg(pair_id));

-- name: ListPayments :many
SELECT id, number, branch_id, party_id, party_type, direction,
  amount, method, paid_at, notes, status, client_ref, amount_tendered, reference_number, pair_id,
  credit_source_type, credit_source_id
FROM payments
WHERE branch_id = sqlc.arg(branch_id)
  AND (sqlc.narg(party_id) IS NULL OR party_id = sqlc.narg(party_id))
  AND (sqlc.narg(status) IS NULL OR status = sqlc.narg(status))
ORDER BY id DESC LIMIT ? OFFSET ?;

-- name: CountPayments :one
SELECT COUNT(*) FROM payments
WHERE branch_id = sqlc.arg(branch_id)
  AND (sqlc.narg(party_id) IS NULL OR party_id = sqlc.narg(party_id))
  AND (sqlc.narg(status) IS NULL OR status = sqlc.narg(status));

-- name: ListCancelledAmong :many
SELECT id FROM payments WHERE branch_id = ? AND status = 'cancelled' AND id IN (sqlc.slice('ids'));

-- name: ListActivePaymentsAfter :many
SELECT id, number, branch_id, party_id, party_type, direction,
  amount, method, paid_at, notes, status, client_ref, amount_tendered, reference_number, pair_id,
  credit_source_type, credit_source_id
FROM payments
WHERE branch_id = ? AND status = 'active' AND id > ? ORDER BY id ASC LIMIT ?;
