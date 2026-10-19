-- Product categories (F8-18).

-- name: GetCategoryByID :one
SELECT id, code, name, parent_id, sort_order, status FROM product_categories WHERE id = ?;

-- name: GetCategoryByCode :one
SELECT id, code, name, parent_id, sort_order, status FROM product_categories WHERE code = ?;

-- name: ListCategories :many
SELECT id, code, name, parent_id, sort_order, status FROM product_categories ORDER BY sort_order ASC, name ASC, id ASC;

-- name: CreateCategory :execresult
INSERT INTO product_categories (code, name, parent_id, sort_order, status, created_by, updated_by)
VALUES (?, ?, ?, ?, 'active', ?, ?);

-- name: UpdateCategory :exec
UPDATE product_categories
SET name = sqlc.arg(name), parent_id = sqlc.arg(parent_id), sort_order = sqlc.arg(sort_order), updated_at = sqlc.arg(now), updated_by = sqlc.arg(updated_by)
WHERE id = sqlc.arg(id);

-- name: SetCategoryStatus :exec
UPDATE product_categories
SET status = sqlc.arg(status), updated_at = sqlc.arg(now), updated_by = sqlc.arg(updated_by)
WHERE id = sqlc.arg(id);

-- name: GetCategoryParent :one
SELECT parent_id FROM product_categories WHERE id = ?;

-- name: CountActiveSubcategories :one
SELECT COUNT(*) FROM product_categories WHERE parent_id = ? AND status = 'active';

-- name: CountActiveProductsByCategory :one
SELECT COUNT(*) FROM products WHERE category_id = ? AND status = 'active';

-- name: ListCategoryNamesByIDs :many
SELECT id, name FROM product_categories WHERE id IN (sqlc.slice('ids'));

-- name: GetCategoryIDByCode :one
SELECT id FROM product_categories WHERE code = ?;

-- name: UpdateImportedCategory :exec
UPDATE product_categories
SET name = sqlc.arg(name), parent_id = sqlc.arg(parent_id), sort_order = sqlc.arg(sort_order),
    status = sqlc.arg(status), updated_at = sqlc.arg(now), updated_by = sqlc.arg(updated_by)
WHERE id = sqlc.arg(id);

-- name: InsertImportedCategory :execresult
INSERT INTO product_categories (code, name, parent_id, sort_order, status, created_by, updated_by)
VALUES (?, ?, ?, ?, ?, ?, ?);
