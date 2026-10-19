-- name: GetBranch :one
SELECT id, code, name, address, city, phone, status, is_head, default_location_label
FROM branches WHERE id = ?;

-- name: GetBranchesByIDs :many
SELECT id, code, name, address, city, phone, status, is_head, default_location_label
FROM branches WHERE id IN (sqlc.slice('ids')) ORDER BY name ASC;

-- name: CountBranchesByCode :one
SELECT COUNT(*) FROM branches WHERE code = ? AND id <> ?;

-- name: InsertBranch :execresult
INSERT INTO branches (code, name, address, city, phone, status, is_head, default_location_label, created_by, updated_by)
VALUES (?, ?, ?, ?, ?, 'active', ?, ?, ?, ?);

-- name: UpdateBranch :exec
UPDATE branches SET name = ?, address = ?, city = ?, phone = ?,
  status = ?, is_head = ?, default_location_label = ?, updated_at = sqlc.arg(now), updated_by = ?
WHERE id = ?;

-- name: UpdateBranchCode :exec
UPDATE branches SET code = ?, updated_at = sqlc.arg(now), updated_by = ? WHERE id = ?;
