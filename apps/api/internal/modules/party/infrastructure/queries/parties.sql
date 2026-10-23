-- name: MaxPartyCodeNumber :one
SELECT CAST(COALESCE(MAX(CAST(SUBSTRING(code, sqlc.arg(start)) AS UNSIGNED)), 0) AS SIGNED) AS max_number FROM parties
WHERE party_type = sqlc.arg(party_type) AND code LIKE sqlc.arg(code_like)
  AND REGEXP_LIKE(SUBSTRING(code, sqlc.arg(start_check)), '^[0-9]+$');

-- name: GetParty :one
SELECT id, code, party_type, name, phone, email, address_text,
  notes, member_type_id, status, COALESCE(attributes, '{}') AS attributes, archived_at
FROM parties WHERE id = ?;

-- name: GetPartyByCode :one
SELECT id, code, party_type, name, phone, email, address_text,
  notes, member_type_id, status, COALESCE(attributes, '{}') AS attributes, archived_at
FROM parties WHERE code = ? AND party_type = ?;

-- name: ListPartyNames :many
SELECT id, name FROM parties WHERE id IN (sqlc.slice('ids'));

-- name: ListPartyLabels :many
SELECT id, code, name FROM parties WHERE id IN (sqlc.slice('ids'));

-- name: InsertParty :execresult
INSERT INTO parties
  (code, party_type, name, phone, email, address_text, notes, member_type_id,
   attributes, status, created_by, updated_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?);

-- name: UpdateParty :exec
UPDATE parties SET name = ?, phone = ?, email = ?, address_text = ?, notes = ?,
  member_type_id = ?, attributes = ?, updated_at = sqlc.arg(now), updated_by = ? WHERE id = ?;

-- name: ArchiveParty :exec
UPDATE parties SET status = sqlc.arg(status), archived_at = sqlc.arg(archived_at),
  updated_at = sqlc.arg(now), updated_by = ? WHERE id = ?;

-- name: RestoreParty :exec
UPDATE parties SET status = sqlc.arg(status), archived_at = NULL,
  updated_at = sqlc.arg(now), updated_by = ? WHERE id = ?;

-- name: ListDuplicatePartyNameGroups :many
SELECT LOWER(TRIM(name)) AS norm_name, COUNT(*) AS rows_count
FROM parties WHERE party_type = ? AND status = 'active'
GROUP BY norm_name HAVING COUNT(*) > 1
ORDER BY rows_count DESC, norm_name ASC;
