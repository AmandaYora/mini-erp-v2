-- name: ListActiveAddressesByParty :many
SELECT id, party_id, label, recipient_name, phone, address_text, is_primary, sort_order, status
FROM party_delivery_addresses WHERE party_id = ? AND status = 'active'
ORDER BY is_primary DESC, sort_order ASC, id ASC;

-- name: GetAddress :one
SELECT id, party_id, label, recipient_name, phone, address_text, is_primary, sort_order, status
FROM party_delivery_addresses WHERE id = ? AND party_id = ?;

-- name: InsertAddress :execresult
INSERT INTO party_delivery_addresses
  (party_id, label, recipient_name, phone, address_text, is_primary, sort_order, status)
VALUES (?, ?, ?, ?, ?, ?, ?, 'active');

-- name: UpdateAddress :exec
UPDATE party_delivery_addresses SET label = ?, recipient_name = ?, phone = ?,
  address_text = ?, is_primary = ?, sort_order = ?, updated_at = sqlc.arg(now)
WHERE id = ?;

-- name: ClearPrimaryAddresses :exec
UPDATE party_delivery_addresses SET is_primary = 0 WHERE party_id = ? AND status = 'active';

-- name: PromoteOldestActiveAddress :exec
UPDATE party_delivery_addresses SET is_primary = 1 WHERE id = (
  SELECT t.id FROM (SELECT a.id FROM party_delivery_addresses a
  WHERE a.party_id = ? AND a.status = 'active' ORDER BY a.id ASC LIMIT 1) t);

-- name: ArchiveAddress :exec
UPDATE party_delivery_addresses SET status = sqlc.arg(status), archived_at = sqlc.arg(archived_at),
  updated_at = sqlc.arg(now) WHERE id = ?;

-- name: RestoreAddress :exec
UPDATE party_delivery_addresses SET status = sqlc.arg(status), archived_at = NULL,
  updated_at = sqlc.arg(now) WHERE id = ?;
