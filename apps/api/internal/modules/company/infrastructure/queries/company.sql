-- name: GetCompanyProfile :one
SELECT id, name, legal_name, address, city, phone, email, tax_id
FROM company_profile ORDER BY id ASC LIMIT 1;

-- name: InsertCompanyProfile :execresult
INSERT INTO company_profile
  (name, legal_name, address, city, phone, email, tax_id, created_by, updated_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateCompanyProfile :exec
UPDATE company_profile SET name = ?, legal_name = ?, address = ?, city = ?,
  phone = ?, email = ?, tax_id = ?, updated_at = sqlc.arg(now), updated_by = ?
WHERE id = ?;

-- name: ListCompanySettings :many
SELECT setting_key, setting_value FROM company_settings;

-- name: UpsertCompanySetting :exec
INSERT INTO company_settings (setting_key, setting_value, updated_by)
VALUES (?, ?, ?)
ON DUPLICATE KEY UPDATE setting_value = VALUES(setting_value),
  updated_at = sqlc.arg(now), updated_by = VALUES(updated_by);
