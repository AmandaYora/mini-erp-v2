-- Products (F8-18). The product list/count with a per-word search is raw SQL
-- (search_raw.go, ADR-0012): the number of LIKE conditions is variable.

-- name: GetProduct :one
SELECT id, code, name, summary, category_id, type, stock_tracked, base_uom, purchase_uom, sales_uom, purchase_to_base_factor, sales_to_base_factor, purchase_price, selling_price, min_selling_price, min_stock_qty, status, CAST(attributes AS CHAR) AS attributes FROM products WHERE id = ?;

-- name: GetProductByCode :one
SELECT id, code, name, summary, category_id, type, stock_tracked, base_uom, purchase_uom, sales_uom, purchase_to_base_factor, sales_to_base_factor, purchase_price, selling_price, min_selling_price, min_stock_qty, status, CAST(attributes AS CHAR) AS attributes FROM products WHERE code = ?;

-- name: ListProductsByIDs :many
SELECT id, code, name, summary, category_id, type, stock_tracked, base_uom, purchase_uom, sales_uom, purchase_to_base_factor, sales_to_base_factor, purchase_price, selling_price, min_selling_price, min_stock_qty, status, CAST(attributes AS CHAR) AS attributes FROM products WHERE id IN (sqlc.slice('ids'));

-- name: ListProductsByCodes :many
SELECT id, code, name, summary, category_id, type, stock_tracked, base_uom, purchase_uom, sales_uom, purchase_to_base_factor, sales_to_base_factor, purchase_price, selling_price, min_selling_price, min_stock_qty, status, CAST(attributes AS CHAR) AS attributes FROM products WHERE code IN (sqlc.slice('codes'));

-- name: InsertProduct :execresult
INSERT INTO products
  (code, name, category_id, type, stock_tracked, base_uom, purchase_uom, sales_uom,
   purchase_to_base_factor, sales_to_base_factor, purchase_price, selling_price,
   min_selling_price, min_stock_qty, status, summary, attributes, created_by, updated_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CAST(sqlc.narg('attributes') AS CHAR), ?, ?);

-- name: UpdateProduct :exec
UPDATE products SET code = sqlc.arg(code), name = sqlc.arg(name), category_id = sqlc.arg(category_id),
  type = sqlc.arg(type), stock_tracked = sqlc.arg(stock_tracked),
  base_uom = sqlc.arg(base_uom), purchase_uom = sqlc.arg(purchase_uom), sales_uom = sqlc.arg(sales_uom),
  purchase_to_base_factor = sqlc.arg(purchase_to_base_factor), sales_to_base_factor = sqlc.arg(sales_to_base_factor),
  purchase_price = sqlc.arg(purchase_price), selling_price = sqlc.arg(selling_price),
  min_selling_price = sqlc.arg(min_selling_price), min_stock_qty = sqlc.arg(min_stock_qty),
  status = sqlc.arg(status), summary = sqlc.arg(summary), attributes = CAST(sqlc.narg('attributes') AS CHAR),
  updated_at = sqlc.arg(now), updated_by = sqlc.arg(updated_by)
WHERE id = sqlc.arg(id);

-- name: SetProductStatus :exec
UPDATE products SET status = sqlc.arg(status), updated_at = sqlc.arg(now), updated_by = sqlc.arg(updated_by)
WHERE id = sqlc.arg(id);

-- name: ListProductCodesIn :many
SELECT code FROM products WHERE code IN (sqlc.slice('codes')) AND id <> sqlc.arg(exclude_id);

-- name: ListPricingByIDs :many
SELECT id, code, name, status, selling_price, min_selling_price, purchase_price,
       purchase_to_base_factor, sales_to_base_factor
FROM products WHERE id IN (sqlc.slice('ids'));

-- name: SearchProductIDs :many
SELECT id FROM products
WHERE status = 'active' AND (code LIKE sqlc.arg(pattern) OR name LIKE sqlc.arg(pattern))
ORDER BY code ASC
LIMIT ?;

-- name: ListStockedAfter :many
SELECT id, code, name, min_stock_qty FROM products
WHERE stock_tracked = 1 AND status = 'active' AND id > ?
ORDER BY id ASC
LIMIT ?;

-- name: ListQRSources :many
SELECT p.id AS product_id, p.code AS product_code, p.name AS product_name,
       c.name AS category, v.id AS variant_id, v.code AS variant_code, v.name AS variant_name,
       v.barcode AS barcode, v.is_hidden AS variant_hidden
FROM products p
LEFT JOIN product_variants v ON v.product_id = p.id AND v.status = 'active'
LEFT JOIN product_categories c ON c.id = p.category_id
WHERE p.status = 'active' AND p.type = 'barang'
ORDER BY p.code ASC, v.id ASC;

-- name: ListLabelsByIDs :many
SELECT id, code, name FROM products WHERE id IN (sqlc.slice('ids'));

-- name: ListExistingCodes :many
SELECT code FROM products WHERE code IN (sqlc.slice('codes'));

-- name: ListPurchaseBaseCostsByIDs :many
SELECT id, purchase_price, purchase_to_base_factor FROM products
WHERE id IN (sqlc.slice('ids')) AND purchase_price > 0;

-- name: ListPurchasePricesByIDs :many
SELECT id, purchase_price FROM products WHERE id IN (sqlc.slice('ids'));
