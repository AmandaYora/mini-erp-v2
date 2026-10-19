-- Product variants (F8-18).

-- name: ListActiveVariantsByProduct :many
SELECT v.id, v.product_id, v.code, v.name, v.barcode, v.is_default, v.status, v.is_hidden, v.sort_order, CAST(v.attributes AS CHAR) AS attributes FROM product_variants v
WHERE v.product_id = ? AND v.status = 'active'
ORDER BY v.is_default DESC, v.sort_order ASC, v.code ASC;

-- name: ListActiveVariantsByProducts :many
SELECT v.id, v.product_id, v.code, v.name, v.barcode, v.is_default, v.status, v.is_hidden, v.sort_order, CAST(v.attributes AS CHAR) AS attributes FROM product_variants v
WHERE v.product_id IN (sqlc.slice('product_ids')) AND v.status = 'active'
ORDER BY v.product_id, v.is_default DESC, v.sort_order ASC, v.code ASC;

-- name: ListAllVariantsByProduct :many
SELECT v.id, v.product_id, v.code, v.name, v.barcode, v.is_default, v.status, v.is_hidden, v.sort_order, CAST(v.attributes AS CHAR) AS attributes FROM product_variants v
WHERE v.product_id = ?
ORDER BY v.id ASC;

-- name: ListAllVariantsByProducts :many
SELECT v.id, v.product_id, v.code, v.name, v.barcode, v.is_default, v.status, v.is_hidden, v.sort_order, CAST(v.attributes AS CHAR) AS attributes FROM product_variants v
WHERE v.product_id IN (sqlc.slice('product_ids'))
ORDER BY v.product_id, v.id;

-- name: GetActiveVariantByBarcode :one
SELECT v.id, v.product_id, v.code, v.name, v.barcode, v.is_default, v.status, v.is_hidden, v.sort_order, CAST(v.attributes AS CHAR) AS attributes FROM product_variants v
WHERE v.barcode = ? AND v.status = 'active'
LIMIT 1;

-- name: ListActiveVariantsByCode :many
SELECT v.id, v.product_id, v.code, v.name, v.barcode, v.is_default, v.status, v.is_hidden, v.sort_order, CAST(v.attributes AS CHAR) AS attributes FROM product_variants v
WHERE v.code = sqlc.arg(code) AND v.status = 'active'
ORDER BY v.product_id ASC
LIMIT ?;

-- name: GetDefaultVariant :one
SELECT v.id, v.product_id, v.code, v.name, v.barcode, v.is_default, v.status, v.is_hidden, v.sort_order, CAST(v.attributes AS CHAR) AS attributes FROM product_variants v
WHERE v.product_id = ? AND v.status = 'active'
ORDER BY v.is_default DESC, v.id ASC
LIMIT 1;

-- name: ListOtherVariantsUsingKeys :many
SELECT product_id, code, barcode
FROM product_variants
WHERE (code IN (sqlc.slice('codes')) OR barcode IN (sqlc.slice('barcodes'))) AND product_id <> sqlc.arg(exclude_product_id);

-- name: UpdateVariant :exec
UPDATE product_variants
SET code = sqlc.arg(code), name = sqlc.arg(name), barcode = sqlc.arg(barcode),
    is_default = sqlc.arg(is_default), is_hidden = sqlc.arg(is_hidden), sort_order = sqlc.arg(sort_order),
    attributes = CAST(sqlc.narg('attributes') AS CHAR), status = sqlc.arg(status), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND product_id = sqlc.arg(product_id);

-- name: InsertVariant :exec
INSERT INTO product_variants (product_id, code, name, barcode, is_default, is_hidden, sort_order, attributes, status)
VALUES (?, ?, ?, ?, ?, ?, ?, CAST(sqlc.narg('attributes') AS CHAR), ?);
