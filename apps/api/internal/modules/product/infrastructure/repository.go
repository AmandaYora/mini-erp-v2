package infrastructure

import (
	"context"
	"database/sql"
	"strings"

	"mini-erp/internal/modules/product/contracts"
)

// Repository owns the product module tables.
type Repository struct {
	db *sql.DB
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// --- categories -------------------------------------------------------------

func scanCategory(row *sql.Row, c *contracts.Category) error {
	var parent sql.NullInt64
	err := row.Scan(&c.ID, &c.Code, &c.Name, &parent, &c.Status)
	if parent.Valid {
		c.ParentID = &parent.Int64
	}
	return err
}

// CategoryByID returns nil, nil when missing.
func (r *Repository) CategoryByID(ctx context.Context, id int64) (*contracts.Category, error) {
	c := &contracts.Category{}
	err := scanCategory(r.db.QueryRowContext(ctx,
		"SELECT id, code, name, parent_id, status FROM product_categories WHERE id = ?", id), c)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return c, err
}

// CategoryByCode returns nil, nil when missing.
func (r *Repository) CategoryByCode(ctx context.Context, code string) (*contracts.Category, error) {
	c := &contracts.Category{}
	err := scanCategory(r.db.QueryRowContext(ctx,
		"SELECT id, code, name, parent_id, status FROM product_categories WHERE code = ?", code), c)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return c, err
}

// ListCategories returns all categories ordered by name.
func (r *Repository) ListCategories(ctx context.Context) ([]*contracts.Category, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, code, name, parent_id, status FROM product_categories ORDER BY name ASC")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Category
	for rows.Next() {
		c := &contracts.Category{}
		var parent sql.NullInt64
		if err := rows.Scan(&c.ID, &c.Code, &c.Name, &parent, &c.Status); err != nil {
			return nil, err
		}
		if parent.Valid {
			c.ParentID = &parent.Int64
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateCategory inserts a category row.
func (r *Repository) CreateCategory(ctx context.Context, c *contracts.Category, actorID int64) (int64, error) {
	var parent any
	if c.ParentID != nil {
		parent = *c.ParentID
	}
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO product_categories (code, name, parent_id, status, created_by, updated_by) VALUES (?, ?, ?, 'active', ?, ?)",
		c.Code, c.Name, parent, actorID, actorID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateCategory rewrites name/parent (code is immutable).
func (r *Repository) UpdateCategory(ctx context.Context, c *contracts.Category, actorID int64) error {
	var parent any
	if c.ParentID != nil {
		parent = *c.ParentID
	}
	_, err := r.db.ExecContext(ctx,
		"UPDATE product_categories SET name = ?, parent_id = ?, updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?",
		c.Name, parent, actorID, c.ID)
	return err
}

// SetCategoryStatus flips active/archived.
func (r *Repository) SetCategoryStatus(ctx context.Context, id int64, status string, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE product_categories SET status = ?, updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?",
		status, actorID, id)
	return err
}

// HasAncestor walks the parent chain of startID looking for targetID.
// It bounds the walk (corrupt chains terminate) to detect cycles.
func (r *Repository) HasAncestor(ctx context.Context, startID, targetID int64) (bool, error) {
	current := startID
	for i := 0; i < 20; i++ {
		var parent sql.NullInt64
		err := r.db.QueryRowContext(ctx,
			"SELECT parent_id FROM product_categories WHERE id = ?", current).Scan(&parent)
		if err == sql.ErrNoRows || !parent.Valid {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if parent.Int64 == targetID {
			return true, nil
		}
		current = parent.Int64
	}
	return true, nil
}

// CountActiveSubcategories counts non-archived direct children.
func (r *Repository) CountActiveSubcategories(ctx context.Context, categoryID int64) (int64, error) {
	var n int64
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM product_categories WHERE parent_id = ? AND status = 'active'", categoryID).Scan(&n)
	return n, err
}

// CountActiveProductsByCategory counts non-archived products in a category.
func (r *Repository) CountActiveProductsByCategory(ctx context.Context, categoryID int64) (int64, error) {
	var n int64
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM products WHERE category_id = ? AND status = 'active'", categoryID).Scan(&n)
	return n, err
}

// --- products ---------------------------------------------------------------

const productColumns = `id, code, name, category_id, type, stock_tracked,
	base_uom, purchase_uom, sales_uom, purchase_to_base_factor, sales_to_base_factor,
	purchase_price, selling_price, min_selling_price, min_stock_qty, status`

func scanProduct(row *sql.Row, p *contracts.Product) error {
	var category sql.NullInt64
	var tracked int
	err := row.Scan(&p.ID, &p.Code, &p.Name, &category, &p.Type, &tracked,
		&p.BaseUOM, &p.PurchaseUOM, &p.SalesUOM, &p.PurchaseFactor, &p.SalesFactor,
		&p.PurchasePrice, &p.SellingPrice, &p.MinSellingPrice, &p.MinStock, &p.Status)
	if category.Valid {
		p.CategoryID = &category.Int64
	}
	p.Tracked = tracked == 1
	return err
}

// GetProduct returns nil, nil when missing.
func (r *Repository) GetProduct(ctx context.Context, id int64) (*contracts.Product, error) {
	p := &contracts.Product{}
	err := scanProduct(r.db.QueryRowContext(ctx,
		"SELECT "+productColumns+" FROM products WHERE id = ?", id), p)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

// ProductByCode returns nil, nil when missing.
func (r *Repository) ProductByCode(ctx context.Context, code string) (*contracts.Product, error) {
	p := &contracts.Product{}
	err := scanProduct(r.db.QueryRowContext(ctx,
		"SELECT "+productColumns+" FROM products WHERE code = ?", code), p)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

// CreateProduct inserts the product plus its variants atomically.
func (r *Repository) CreateProduct(ctx context.Context, p *contracts.Product, actorID int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO products
		 (code, name, category_id, type, stock_tracked, base_uom, purchase_uom, sales_uom,
		  purchase_to_base_factor, sales_to_base_factor, purchase_price, selling_price,
		  min_selling_price, min_stock_qty, status, created_by, updated_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)`,
		p.Code, p.Name, nullInt64(p.CategoryID), p.Type, boolToInt(p.Tracked),
		p.BaseUOM, p.PurchaseUOM, p.SalesUOM, p.PurchaseFactor, p.SalesFactor,
		p.PurchasePrice, p.SellingPrice, p.MinSellingPrice, p.MinStock, actorID, actorID)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := replaceVariants(ctx, tx, id, p.Variants); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// UpdateProduct rewrites the product plus its variants atomically.
func (r *Repository) UpdateProduct(ctx context.Context, p *contracts.Product, actorID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`UPDATE products SET code = ?, name = ?, category_id = ?, type = ?, stock_tracked = ?,
		 base_uom = ?, purchase_uom = ?, sales_uom = ?,
		 purchase_to_base_factor = ?, sales_to_base_factor = ?,
		 purchase_price = ?, selling_price = ?, min_selling_price = ?, min_stock_qty = ?,
		 updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?`,
		p.Code, p.Name, nullInt64(p.CategoryID), p.Type, boolToInt(p.Tracked),
		p.BaseUOM, p.PurchaseUOM, p.SalesUOM, p.PurchaseFactor, p.SalesFactor,
		p.PurchasePrice, p.SellingPrice, p.MinSellingPrice, p.MinStock, actorID, p.ID); err != nil {
		return err
	}
	if err := replaceVariants(ctx, tx, p.ID, p.Variants); err != nil {
		return err
	}
	return tx.Commit()
}

func replaceVariants(ctx context.Context, tx *sql.Tx, productID int64, variants []*contracts.Variant) error {
	// Load existing rows (any status) so stable IDs survive updates — future
	// stock rows will reference variant IDs, which must never be recycled.
	rows, err := tx.QueryContext(ctx,
		"SELECT id, code FROM product_variants WHERE product_id = ?", productID)
	if err != nil {
		return err
	}
	existing := map[string]int64{}
	for rows.Next() {
		var id int64
		var code string
		if err := rows.Scan(&id, &code); err != nil {
			_ = rows.Close()
			return err
		}
		existing[code] = id
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	keep := map[string]bool{}
	for _, v := range variants {
		def := 0
		if v.IsDefault {
			def = 1
		}
		keep[v.Code] = true
		if id, ok := existing[v.Code]; ok {
			if _, err := tx.ExecContext(ctx,
				`UPDATE product_variants SET name = ?, barcode = ?, is_default = ?, status = 'active'
				 WHERE id = ?`,
				v.Name, nullIfEmpty(v.Barcode), def, id); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO product_variants (product_id, code, name, barcode, is_default, status)
			 VALUES (?, ?, ?, ?, ?, 'active')`,
			productID, v.Code, v.Name, nullIfEmpty(v.Barcode), def); err != nil {
			return err
		}
	}
	// Absent codes archive instead of delete: IDs stay resolvable forever.
	for code, id := range existing {
		if !keep[code] {
			if _, err := tx.ExecContext(ctx,
				"UPDATE product_variants SET status = 'archived' WHERE id = ?", id); err != nil {
				return err
			}
		}
	}
	return nil
}

// SetProductStatus flips active/archived.
func (r *Repository) SetProductStatus(ctx context.Context, id int64, status string, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE products SET status = ?, updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?",
		status, actorID, id)
	return err
}

// ListProducts searches code/name (per word) with optional filters.
func (r *Repository) ListProducts(ctx context.Context, search string, categoryID *int64, status string, limit, offset int) ([]*contracts.Product, error) {
	var conds []string
	var args []any
	if status != "" {
		conds = append(conds, "p.status = ?")
		args = append(args, status)
	}
	if categoryID != nil {
		conds = append(conds, "p.category_id = ?")
		args = append(args, *categoryID)
	}
	for _, word := range strings.Fields(search) {
		like := "%" + word + "%"
		conds = append(conds, "(p.code LIKE ? OR p.name LIKE ?)")
		args = append(args, like, like)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+qualifiedColumns()+
			" FROM products p LEFT JOIN product_categories c ON c.id = p.category_id "+
			where+" ORDER BY p.name ASC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Product
	for rows.Next() {
		p, err := scanProductRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func qualifiedColumns() string {
	return `p.id, p.code, p.name, p.category_id, p.type, p.stock_tracked,
		p.base_uom, p.purchase_uom, p.sales_uom, p.purchase_to_base_factor, p.sales_to_base_factor,
		p.purchase_price, p.selling_price, p.min_selling_price, p.min_stock_qty, p.status,
		c.name`
}

func scanProductRow(rows *sql.Rows) (*contracts.Product, error) {
	p := &contracts.Product{}
	var category sql.NullInt64
	var categoryName sql.NullString
	var tracked int
	err := rows.Scan(&p.ID, &p.Code, &p.Name, &category, &p.Type, &tracked,
		&p.BaseUOM, &p.PurchaseUOM, &p.SalesUOM, &p.PurchaseFactor, &p.SalesFactor,
		&p.PurchasePrice, &p.SellingPrice, &p.MinSellingPrice, &p.MinStock, &p.Status,
		&categoryName)
	if category.Valid {
		p.CategoryID = &category.Int64
	}
	p.Tracked = tracked == 1
	p.CategoryName = categoryName.String
	return p, err
}

// CountProducts counts the List filter set.
func (r *Repository) CountProducts(ctx context.Context, search string, categoryID *int64, status string) (int64, error) {
	var conds []string
	var args []any
	if status != "" {
		conds = append(conds, "status = ?")
		args = append(args, status)
	}
	if categoryID != nil {
		conds = append(conds, "category_id = ?")
		args = append(args, *categoryID)
	}
	for _, word := range strings.Fields(search) {
		like := "%" + word + "%"
		conds = append(conds, "(code LIKE ? OR name LIKE ?)")
		args = append(args, like, like)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	var total int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM products "+where, args...).Scan(&total)
	return total, err
}

// VariantsByProduct returns a product's variants, default first.
func (r *Repository) VariantsByProduct(ctx context.Context, productID int64) ([]*contracts.Variant, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, product_id, code, name, barcode, is_default, status
		 FROM product_variants WHERE product_id = ? AND status = 'active'
		 ORDER BY is_default DESC, code ASC`, productID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Variant
	for rows.Next() {
		v := &contracts.Variant{}
		var barcode sql.NullString
		var def int
		if err := rows.Scan(&v.ID, &v.ProductID, &v.Code, &v.Name, &barcode, &def, &v.Status); err != nil {
			return nil, err
		}
		v.Barcode = barcode.String
		v.IsDefault = def == 1
		out = append(out, v)
	}
	return out, rows.Err()
}

// VariantByBarcode resolves a barcode globally (for scan flows).
func (r *Repository) VariantByBarcode(ctx context.Context, barcode string) (*contracts.Variant, error) {
	v := &contracts.Variant{}
	var bc sql.NullString
	var def int
	err := r.db.QueryRowContext(ctx,
		`SELECT id, product_id, code, name, barcode, is_default, status
		 FROM product_variants WHERE barcode = ? AND status = 'active'`, barcode).
		Scan(&v.ID, &v.ProductID, &v.Code, &v.Name, &bc, &def, &v.Status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	v.Barcode = bc.String
	v.IsDefault = def == 1
	return v, nil
}

// ListStocked lists tracked, active products with minimum stock
// (assistant critical-stock tool). Code-ordered, capped.
func (r *Repository) ListStocked(ctx context.Context) ([]*contracts.StockedProduct, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, code, name, min_stock_qty FROM products
		 WHERE stock_tracked = 1 AND status = 'active' ORDER BY code ASC LIMIT 5000`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []*contracts.StockedProduct{}
	for rows.Next() {
		var p contracts.StockedProduct
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.MinStock); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

// QRSource is one flat catalog row for bulk QR export: a product joined to
// one of its active variants (variant columns zero when the product has
// none). Intra-module join — products, variants, and categories are all
// owned by this module.
type QRSource struct {
	ProductID   int64
	ProductCode string
	ProductName string
	Category    string
	VariantID   int64
	VariantCode string
	VariantName string
	Barcode     string
}

// QRSources lists the whole printable catalog in one grouped read (P3):
// active barang products with their active variants. Jasa has no shelf to
// label and stays out.
func (r *Repository) QRSources(ctx context.Context) ([]QRSource, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT p.id, p.code, p.name, COALESCE(c.name, ''),
			COALESCE(v.id, 0), COALESCE(v.code, ''), COALESCE(v.name, ''),
			COALESCE(v.barcode, '')
		 FROM products p
		 LEFT JOIN product_variants v ON v.product_id = p.id AND v.status = 'active'
		 LEFT JOIN product_categories c ON c.id = p.category_id
		 WHERE p.status = 'active' AND p.type = 'barang'
		 ORDER BY p.code ASC, v.id ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []QRSource{}
	for rows.Next() {
		var s QRSource
		if err := rows.Scan(&s.ProductID, &s.ProductCode, &s.ProductName,
			&s.Category, &s.VariantID, &s.VariantCode, &s.VariantName,
			&s.Barcode); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func nullInt64(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
