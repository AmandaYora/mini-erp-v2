-- name: GetMemberType :one
SELECT id, code, name, description_text, price_basis,
  price_adjustment_direction, price_adjustment_type, price_adjustment_value,
  rounding_mode, rounding_increment, status
FROM member_types WHERE id = ?;

-- name: GetMemberTypeByCode :one
SELECT id, code, name, description_text, price_basis,
  price_adjustment_direction, price_adjustment_type, price_adjustment_value,
  rounding_mode, rounding_increment, status
FROM member_types WHERE code = ?;

-- name: InsertMemberType :execresult
INSERT INTO member_types
  (code, name, description_text, price_basis, price_adjustment_direction,
   price_adjustment_type, price_adjustment_value, rounding_mode, rounding_increment,
   status, created_by, updated_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?);

-- name: UpdateMemberType :exec
UPDATE member_types SET name = ?, description_text = ?, price_basis = ?,
  price_adjustment_direction = ?, price_adjustment_type = ?, price_adjustment_value = ?,
  rounding_mode = ?, rounding_increment = ?,
  updated_at = sqlc.arg(now), updated_by = ? WHERE id = ?;

-- name: ArchiveMemberType :exec
UPDATE member_types SET status = sqlc.arg(status), archived_at = sqlc.arg(archived_at),
  updated_at = sqlc.arg(now), updated_by = ? WHERE id = ?;

-- name: RestoreMemberType :exec
UPDATE member_types SET status = sqlc.arg(status), archived_at = NULL,
  updated_at = sqlc.arg(now), updated_by = ? WHERE id = ?;

-- name: CountActiveCustomersByMemberType :one
SELECT COUNT(*) FROM parties
WHERE party_type = 'customer' AND status = 'active' AND member_type_id = ?;
