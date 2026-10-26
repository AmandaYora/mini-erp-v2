-- name: GetPayment :one
SELECT id, number, branch_id, party_id, party_type, direction,
  amount, method, paid_at, notes, status, client_ref, amount_tendered, reference_number, pair_id,
  cash_account_id, credit_source_type, credit_source_id, corrects_payment_id, cancel_reason, cancelled_at, cancelled_by
FROM payments WHERE id = ?;

-- name: GetPaymentByClientRef :one
SELECT id, number, branch_id, party_id, party_type, direction,
  amount, method, paid_at, notes, status, client_ref, amount_tendered, reference_number, pair_id,
  cash_account_id, credit_source_type, credit_source_id, corrects_payment_id, cancel_reason, cancelled_at, cancelled_by
FROM payments WHERE branch_id = ? AND client_ref = ?;

-- name: InsertPayment :execresult
INSERT INTO payments
  (number, branch_id, party_id, party_type, direction, amount, method,
   paid_at, notes, status, client_ref, amount_tendered, reference_number, cash_account_id,
   credit_source_type, credit_source_id, corrects_payment_id, created_by, updated_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: SetPairID :exec
UPDATE payments SET pair_id = ? WHERE id = ?;

-- name: SetPaymentStatus :exec
UPDATE payments SET status = ?, updated_at = sqlc.arg(now), updated_by = ? WHERE id = ?;

-- name: CancelActiveWithPair :execresult
-- MySQL applies single-table assignments left to right, so cancelled_at/by
-- take the values just written to updated_at/by.
UPDATE payments SET status = 'cancelled', updated_at = sqlc.arg(now), updated_by = sqlc.arg(actor),
  cancelled_at = updated_at, cancelled_by = updated_by, cancel_reason = sqlc.narg(reason)
WHERE status = 'active' AND (id = sqlc.arg(id) OR pair_id = sqlc.arg(pair_id));

-- name: GetCorrectionOf :one
-- The payment that replaced (corrected) this one, newest first; a corrected
-- payment is cancelled in the same transaction, so there is at most one.
SELECT id, number FROM payments
WHERE corrects_payment_id = sqlc.arg(corrects_payment_id) AND branch_id = sqlc.arg(branch_id)
ORDER BY id DESC LIMIT 1;

-- name: ListPayments :many
SELECT id, number, branch_id, party_id, party_type, direction,
  amount, method, paid_at, notes, status, client_ref, amount_tendered, reference_number, pair_id,
  cash_account_id, credit_source_type, credit_source_id, corrects_payment_id, cancel_reason, cancelled_at, cancelled_by
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
  cash_account_id, credit_source_type, credit_source_id, corrects_payment_id, cancel_reason, cancelled_at, cancelled_by
FROM payments
WHERE branch_id = ? AND status = 'active' AND id > ? ORDER BY id ASC LIMIT ?;
