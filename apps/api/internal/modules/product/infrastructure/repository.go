package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"

	"mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/modules/product/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
	"mini-erp/internal/shared/timeutil"
)

// Repository owns the product module tables.
type Repository struct {
	db *sql.DB
	q  *sqlc.Queries
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: sqlc.New(db)}
}

// actor binds an actor id as stored historically: always a value (0 included).
func actor(id int64) sql.NullInt64 { return sql.NullInt64{Int64: id, Valid: true} }

// inChunks runs fn over consecutive slices of at most size elements.
func inChunks[T any](list []T, size int, fn func([]T) error) error {
	for start := 0; start < len(list); start += size {
		if err := fn(list[start:min(start+size, len(list))]); err != nil {
			return err
		}
	}
	return nil
}

// --- categories -------------------------------------------------------------

// CategoryByID returns nil, nil when missing.
func (r *Repository) CategoryByID(ctx context.Context, id int64) (*contracts.Category, error) {
	row, err := r.q.GetCategoryByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toCategory(row.ID, row.Code, row.Name, row.ParentID, row.SortOrder, row.Status), nil
}

// CategoryByCode returns nil, nil when missing.
func (r *Repository) CategoryByCode(ctx context.Context, code string) (*contracts.Category, error) {
	row, err := r.q.GetCategoryByCode(ctx, code)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toCategory(row.ID, row.Code, row.Name, row.ParentID, row.SortOrder, row.Status), nil
}

// ListCategories returns all categories ordered by sort_order, then name.
func (r *Repository) ListCategories(ctx context.Context) ([]*contracts.Category, error) {
	rows, err := r.q.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	var out []*contracts.Category
	for _, row := range rows {
		out = append(out, toCategory(row.ID, row.Code, row.Name, row.ParentID, row.SortOrder, row.Status))
	}
	return out, nil
}

// CreateCategory inserts a category row.
func (r *Repository) CreateCategory(ctx context.Context, c *contracts.Category, actorID int64) (int64, error) {
	res, err := r.q.CreateCategory(ctx, sqlc.CreateCategoryParams{
		Code: c.Code, Name: c.Name, ParentID: nullPtr(c.ParentID), SortOrder: int32(c.SortOrder), CreatedBy: actor(actorID), UpdatedBy: actor(actorID),
	})
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateCategory rewrites name/parent/sort order (code is immutable).
func (r *Repository) UpdateCategory(ctx context.Context, c *contracts.Category, actorID int64) error {
	return r.q.UpdateCategory(ctx, sqlc.UpdateCategoryParams{
		Name: c.Name, ParentID: nullPtr(c.ParentID), SortOrder: int32(c.SortOrder), Now: timeutil.NowUTC(), UpdatedBy: actor(actorID), ID: c.ID,
	})
}

// SetCategoryStatus flips active/archived.
func (r *Repository) SetCategoryStatus(ctx context.Context, id int64, status string, actorID int64) error {
	return r.q.SetCategoryStatus(ctx, sqlc.SetCategoryStatusParams{
		Status: status, Now: timeutil.NowUTC(), UpdatedBy: actor(actorID), ID: id,
	})
}

// HasAncestor walks the parent chain of startID looking for targetID.
// It bounds the walk (corrupt chains terminate) to detect cycles.
func (r *Repository) HasAncestor(ctx context.Context, startID, targetID int64) (bool, error) {
	current := startID
	for i := 0; i < 20; i++ {
		parent, err := r.q.GetCategoryParent(ctx, current)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && !parent.Valid) {
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
	return r.q.CountActiveSubcategories(ctx, nullPtr(&categoryID))
}

// CountActiveProductsByCategory counts non-archived products in a category.
func (r *Repository) CountActiveProductsByCategory(ctx context.Context, categoryID int64) (int64, error) {
	return r.q.CountActiveProductsByCategory(ctx, nullPtr(&categoryID))
}

// --- products ---------------------------------------------------------------

// GetProduct returns nil, nil when missing.
func (r *Repository) GetProduct(ctx context.Context, id int64) (*contracts.Product, error) {
	row, err := r.q.GetProduct(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toProduct(sqlc.GetProductRow(row)), nil
}

// ProductsByIDs loads many products (no variants/category) in ONE read.
func (r *Repository) ProductsByIDs(ctx context.Context, ids []int64) (map[int64]*contracts.Product, error) {
	out := map[int64]*contracts.Product{}
	list := distinctIDs(ids)
	if len(list) == 0 {
		return out, nil
	}
	rows, err := r.q.ListProductsByIDs(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		p := toProduct(sqlc.GetProductRow(row))
		out[p.ID] = p
	}
	return out, nil
}

// CategoryNamesByIDs resolves category names in ONE read.
func (r *Repository) CategoryNamesByIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	out := map[int64]string{}
	list := distinctIDs(ids)
	if len(list) == 0 {
		return out, nil
	}
	rows, err := r.q.ListCategoryNamesByIDs(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.Name
	}
	return out, nil
}

// ProductByCode returns nil, nil when missing.
func (r *Repository) ProductByCode(ctx context.Context, code string) (*contracts.Product, error) {
	row, err := r.q.GetProductByCode(ctx, code)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toProduct(sqlc.GetProductRow(row)), nil
}

// CreateProduct inserts the product plus its variants atomically.
func (r *Repository) CreateProduct(ctx context.Context, p *contracts.Product, actorID int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	id, err := createProductTx(ctx, r.q.WithTx(tx), p, actorID)
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func createProductTx(ctx context.Context, q *sqlc.Queries, p *contracts.Product, actorID int64) (int64, error) {
	res, err := q.InsertProduct(ctx, sqlc.InsertProductParams{
		Code: p.Code, Name: p.Name, CategoryID: nullPtr(p.CategoryID), Type: p.Type, StockTracked: p.Tracked,
		BaseUom: p.BaseUOM, PurchaseUom: p.PurchaseUOM, SalesUom: p.SalesUOM,
		PurchaseToBaseFactor: p.PurchaseFactor, SalesToBaseFactor: p.SalesFactor,
		PurchasePrice: p.PurchasePrice, SellingPrice: p.SellingPrice, MinSellingPrice: p.MinSellingPrice,
		MinStockQty: p.MinStock, Status: statusOrActive(p.Status), Summary: dbutil.NullString(p.Summary),
		Attributes: encodeAttrs(p.Attributes), CreatedBy: actor(actorID), UpdatedBy: actor(actorID),
	})
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := writeVariants(ctx, q, id, p.Variants); err != nil {
		return 0, err
	}
	return id, nil
}

// UpdateProduct rewrites the product plus its variants atomically.
func (r *Repository) UpdateProduct(ctx context.Context, p *contracts.Product, actorID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := updateProductTx(ctx, r.q.WithTx(tx), p, actorID); err != nil {
		return err
	}
	return tx.Commit()
}

func updateProductTx(ctx context.Context, q *sqlc.Queries, p *contracts.Product, actorID int64) error {
	if err := q.UpdateProduct(ctx, sqlc.UpdateProductParams{
		Code: p.Code, Name: p.Name, CategoryID: nullPtr(p.CategoryID), Type: p.Type, StockTracked: p.Tracked,
		BaseUom: p.BaseUOM, PurchaseUom: p.PurchaseUOM, SalesUom: p.SalesUOM,
		PurchaseToBaseFactor: p.PurchaseFactor, SalesToBaseFactor: p.SalesFactor,
		PurchasePrice: p.PurchasePrice, SellingPrice: p.SellingPrice, MinSellingPrice: p.MinSellingPrice,
		MinStockQty: p.MinStock, Status: statusOrActive(p.Status), Summary: dbutil.NullString(p.Summary),
		Attributes: encodeAttrs(p.Attributes), Now: timeutil.NowUTC(), UpdatedBy: actor(actorID), ID: p.ID,
	}); err != nil {
		return err
	}
	return writeVariants(ctx, q, p.ID, p.Variants)
}

// ProductWrite is one create (Product.ID 0) or update of a bulk import.
type ProductWrite struct {
	Product *contracts.Product
	// Row is the caller's own label for error reporting (spreadsheet row).
	Row int
}

// WriteProducts applies the writes in ONE transaction (a chunk of a bulk
// import): all of them or none.
func (r *Repository) WriteProducts(ctx context.Context, writes []ProductWrite, actorID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.q.WithTx(tx)
	for _, w := range writes {
		if w.Product.ID > 0 {
			err = updateProductTx(ctx, q, w.Product, actorID)
		} else {
			_, err = createProductTx(ctx, q, w.Product, actorID)
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ProductsByCodes loads products by (upper-cased) code in chunked IN reads.
func (r *Repository) ProductsByCodes(ctx context.Context, codes []string) (map[string]*contracts.Product, error) {
	out := map[string]*contracts.Product{}
	uniq := map[string]bool{}
	var list []string
	for _, c := range codes {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c != "" && !uniq[c] {
			uniq[c] = true
			list = append(list, c)
		}
	}
	err := inChunks(list, 1000, func(chunk []string) error {
		rows, err := r.q.ListProductsByCodes(ctx, chunk)
		if err != nil {
			return err
		}
		for _, row := range rows {
			p := toProduct(sqlc.GetProductRow(row))
			out[strings.ToUpper(p.Code)] = p
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// AllVariantsByProducts returns every variant (any status) of many products
// grouped by product id, in chunked reads.
func (r *Repository) AllVariantsByProducts(ctx context.Context, ids []int64) (map[int64][]*contracts.Variant, error) {
	out := map[int64][]*contracts.Variant{}
	err := inChunks(ids, 1000, func(chunk []int64) error {
		rows, err := r.q.ListAllVariantsByProducts(ctx, chunk)
		if err != nil {
			return err
		}
		for _, row := range rows {
			v := toVariant(sqlc.GetDefaultVariantRow(row))
			out[v.ProductID] = append(out[v.ProductID], v)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CategoryWrite is one category upsert of a bulk import.
type CategoryWrite struct {
	ID         int64 // > 0 updates (and re-activates) an existing category
	Code, Name string
	ParentCode string // resolved to an id inside the transaction
	SortOrder  int
	Status     string
}

// ApplyCategories upserts an import's categories in ONE transaction, parents
// before children (the caller orders them).
func (r *Repository) ApplyCategories(ctx context.Context, writes []CategoryWrite, actorID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.q.WithTx(tx)
	ids := map[string]int64{}
	for _, w := range writes {
		var parent sql.NullInt64
		if w.ParentCode != "" {
			pid, ok := ids[strings.ToUpper(w.ParentCode)]
			if !ok {
				if pid, err = q.GetCategoryIDByCode(ctx, w.ParentCode); err != nil {
					return err
				}
			}
			parent = sql.NullInt64{Int64: pid, Valid: true}
		}
		status := statusOrActive(w.Status)
		if w.ID > 0 {
			if err := q.UpdateImportedCategory(ctx, sqlc.UpdateImportedCategoryParams{
				Name: w.Name, ParentID: parent, SortOrder: int32(w.SortOrder), Status: status,
				Now: timeutil.NowUTC(), UpdatedBy: actor(actorID), ID: w.ID,
			}); err != nil {
				return err
			}
			ids[strings.ToUpper(w.Code)] = w.ID
			continue
		}
		res, err := q.InsertImportedCategory(ctx, sqlc.InsertImportedCategoryParams{
			Code: w.Code, Name: w.Name, ParentID: parent, SortOrder: int32(w.SortOrder), Status: status,
			CreatedBy: actor(actorID), UpdatedBy: actor(actorID),
		})
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		ids[strings.ToUpper(w.Code)] = id
	}
	return tx.Commit()
}

// writeVariants executes a variant plan computed by the service: rows with
// ID 0 are inserted, the rest are updated in place (code included — the
// service has already refused renames of variants with stock history). A row
// is never deleted: retiring a variant sets its status, so IDs that stock and
// documents reference stay resolvable forever.
func writeVariants(ctx context.Context, q *sqlc.Queries, productID int64, variants []*contracts.Variant) error {
	for _, v := range variants {
		status := statusOrActive(v.Status)
		if v.ID > 0 {
			if err := q.UpdateVariant(ctx, sqlc.UpdateVariantParams{
				Code: v.Code, Name: v.Name, Barcode: dbutil.NullString(v.Barcode),
				IsDefault: v.IsDefault, IsHidden: v.IsHidden, SortOrder: int32(v.SortOrder),
				Attributes: encodeAttrs(v.Attributes), Status: status, Now: timeutil.NowUTC(),
				ID: v.ID, ProductID: productID,
			}); err != nil {
				return err
			}
			continue
		}
		if err := q.InsertVariant(ctx, sqlc.InsertVariantParams{
			ProductID: productID, Code: v.Code, Name: v.Name, Barcode: dbutil.NullString(v.Barcode),
			IsDefault: v.IsDefault, IsHidden: v.IsHidden, SortOrder: int32(v.SortOrder),
			Attributes: encodeAttrs(v.Attributes), Status: status,
		}); err != nil {
			return err
		}
	}
	return nil
}

// SetProductStatus flips active/archived.
func (r *Repository) SetProductStatus(ctx context.Context, id int64, status string, actorID int64) error {
	return r.q.SetProductStatus(ctx, sqlc.SetProductStatusParams{
		Status: status, Now: timeutil.NowUTC(), UpdatedBy: actor(actorID), ID: id,
	})
}

// --- variants ---------------------------------------------------------------

// VariantsByProduct returns a product's ACTIVE variants, default first
// (hidden default included: documents may still reference it).
func (r *Repository) VariantsByProduct(ctx context.Context, productID int64) ([]*contracts.Variant, error) {
	rows, err := r.q.ListActiveVariantsByProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	var out []*contracts.Variant
	for _, row := range rows {
		out = append(out, toVariant(sqlc.GetDefaultVariantRow(row)))
	}
	return out, nil
}

// VariantsByProducts returns the active variants of many products grouped by
// product id, in ONE read.
func (r *Repository) VariantsByProducts(ctx context.Context, ids []int64) (map[int64][]*contracts.Variant, error) {
	out := map[int64][]*contracts.Variant{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.q.ListActiveVariantsByProducts(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		v := toVariant(sqlc.GetDefaultVariantRow(row))
		out[v.ProductID] = append(out[v.ProductID], v)
	}
	return out, nil
}

// AllVariantsByProduct returns every variant of a product, any status, by id
// (the starting point of a variant plan).
func (r *Repository) AllVariantsByProduct(ctx context.Context, productID int64) ([]*contracts.Variant, error) {
	rows, err := r.q.ListAllVariantsByProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	var out []*contracts.Variant
	for _, row := range rows {
		out = append(out, toVariant(sqlc.GetDefaultVariantRow(row)))
	}
	return out, nil
}

// VariantByBarcode resolves an active variant by barcode (barcodes are
// globally unique).
func (r *Repository) VariantByBarcode(ctx context.Context, barcode string) (*contracts.Variant, error) {
	row, err := r.q.GetActiveVariantByBarcode(ctx, sql.NullString{String: barcode, Valid: true})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toVariant(sqlc.GetDefaultVariantRow(row)), nil
}

// ActiveVariantsByCode lists active variants carrying the code (variant codes
// are unique per product only, so several products may answer).
func (r *Repository) ActiveVariantsByCode(ctx context.Context, code string, limit int) ([]*contracts.Variant, error) {
	rows, err := r.q.ListActiveVariantsByCode(ctx, sqlc.ListActiveVariantsByCodeParams{Code: code, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	var out []*contracts.Variant
	for _, row := range rows {
		out = append(out, toVariant(sqlc.GetDefaultVariantRow(row)))
	}
	return out, nil
}

// DefaultVariant returns the product's default active variant, else its first.
func (r *Repository) DefaultVariant(ctx context.Context, productID int64) (*contracts.Variant, error) {
	row, err := r.q.GetDefaultVariant(ctx, productID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toVariant(sqlc.GetDefaultVariantRow(row)), nil
}

// ScanKeyOwner is a variant that already uses a scan key.
type ScanKeyOwner struct {
	ProductID int64
	Code      string
	Barcode   string
}

// OtherVariantsUsingKeys finds variants of OTHER products whose code or
// barcode is one of keys, in one query. excludeProductID 0 excludes none.
func (r *Repository) OtherVariantsUsingKeys(ctx context.Context, keys []string, excludeProductID int64) ([]ScanKeyOwner, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	barcodes := make([]sql.NullString, 0, len(keys))
	for _, k := range keys {
		barcodes = append(barcodes, sql.NullString{String: k, Valid: true})
	}
	rows, err := r.q.ListOtherVariantsUsingKeys(ctx, sqlc.ListOtherVariantsUsingKeysParams{
		Codes: keys, Barcodes: barcodes, ExcludeProductID: excludeProductID,
	})
	if err != nil {
		return nil, err
	}
	var out []ScanKeyOwner
	for _, row := range rows {
		out = append(out, ScanKeyOwner{ProductID: row.ProductID, Code: row.Code, Barcode: row.Barcode.String})
	}
	return out, nil
}

// ProductCodesIn returns which of keys are product codes of OTHER products.
func (r *Repository) ProductCodesIn(ctx context.Context, keys []string, excludeProductID int64) (map[string]bool, error) {
	out := map[string]bool{}
	if len(keys) == 0 {
		return out, nil
	}
	codes, err := r.q.ListProductCodesIn(ctx, sqlc.ListProductCodesInParams{Codes: keys, ExcludeID: excludeProductID})
	if err != nil {
		return nil, err
	}
	for _, c := range codes {
		out[strings.ToUpper(c)] = true
	}
	return out, nil
}

// --- price / label lookups ----------------------------------------------------

// PricingByIDs resolves the member-pricing slice of many products in ONE read.
func (r *Repository) PricingByIDs(ctx context.Context, ids []int64) (map[int64]contracts.PricingInfo, error) {
	out := map[int64]contracts.PricingInfo{}
	list := distinctIDs(ids)
	if len(list) == 0 {
		return out, nil
	}
	rows, err := r.q.ListPricingByIDs(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = contracts.PricingInfo{
			ID: row.ID, Code: row.Code, Name: row.Name, Status: row.Status,
			SellingPrice: row.SellingPrice, MinSellingPrice: row.MinSellingPrice, PurchasePrice: row.PurchasePrice,
			PurchaseFactor: row.PurchaseToBaseFactor, SalesFactor: row.SalesToBaseFactor,
		}
	}
	return out, nil
}

// SearchProductIDs finds active products by code/name fragment.
func (r *Repository) SearchProductIDs(ctx context.Context, q string, limit int) ([]int64, error) {
	ids, err := r.q.SearchProductIDs(ctx, sqlc.SearchProductIDsParams{Pattern: "%" + q + "%", Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	var out []int64
	out = append(out, ids...)
	return out, nil
}

// ListStocked lists tracked, active products with minimum stock
// (assistant critical-stock tool). Code-ordered, capped.
func (r *Repository) ListStocked(ctx context.Context) ([]*contracts.StockedProduct, error) {
	var all []*contracts.StockedProduct
	var after int64
	for {
		page, err := r.ListStockedAfter(ctx, after, 2000)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < 2000 {
			break
		}
		after = page[len(page)-1].ID
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Code < all[j].Code })
	if all == nil {
		all = []*contracts.StockedProduct{}
	}
	return all, nil
}

// ListStockedAfter pages tracked, active products by ascending id.
func (r *Repository) ListStockedAfter(ctx context.Context, afterID int64, limit int) ([]*contracts.StockedProduct, error) {
	rows, err := r.q.ListStockedAfter(ctx, sqlc.ListStockedAfterParams{ID: afterID, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]*contracts.StockedProduct, 0, len(rows))
	for _, row := range rows {
		out = append(out, &contracts.StockedProduct{ID: row.ID, Code: row.Code, Name: row.Name, MinStock: row.MinStockQty})
	}
	return out, nil
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
	// VariantHidden marks the implicit default variant: it never becomes a
	// label of its own (the product-level label scans through the same code).
	VariantHidden bool
}

// QRSources lists the whole printable catalog in one grouped read (P3):
// active barang products with their active variants. Jasa has no shelf to
// label and stays out.
func (r *Repository) QRSources(ctx context.Context) ([]QRSource, error) {
	rows, err := r.q.ListQRSources(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]QRSource, 0, len(rows))
	for _, row := range rows {
		out = append(out, QRSource{
			ProductID: row.ProductID, ProductCode: row.ProductCode, ProductName: row.ProductName,
			Category: row.Category.String, VariantID: row.VariantID.Int64, VariantCode: row.VariantCode.String,
			VariantName: row.VariantName.String, Barcode: row.Barcode.String, VariantHidden: row.VariantHidden.Bool,
		})
	}
	return out, nil
}

// LabelsByIDs resolves code+name for many products in one grouped read.
// Duplicate and zero ids collapse before the query, so a report iterating
// 500 rows over 40 distinct products asks for 40.
func (r *Repository) LabelsByIDs(ctx context.Context, ids []int64) (map[int64]contracts.Label, error) {
	out := map[int64]contracts.Label{}
	list := distinctIDs(ids)
	if len(list) == 0 {
		return out, nil
	}
	rows, err := r.q.ListLabelsByIDs(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = contracts.Label{Code: row.Code, Name: row.Name}
	}
	return out, nil
}

// ExistingCodes reports which of the given codes already exist, in one
// grouped read.
//
// Spreadsheet import asks "is this code already taken?" for every row. Doing
// that one row at a time made a 5000-row file cost 5000 queries before a
// single product was written. Codes are compared upper-cased, matching how
// import normalises them.
func (r *Repository) ExistingCodes(ctx context.Context, codes []string) (map[string]bool, error) {
	out := map[string]bool{}
	seen := map[string]bool{}
	list := make([]string, 0, len(codes))
	for _, c := range codes {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		list = append(list, c)
	}
	if len(list) == 0 {
		return out, nil
	}
	rows, err := r.q.ListExistingCodes(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, code := range rows {
		out[strings.ToUpper(code)] = true
	}
	return out, nil
}

// PurchaseBaseCostsByIDs resolves the purchase price per BASE unit (price per
// purchase unit divided by the purchase→base factor).
func (r *Repository) PurchaseBaseCostsByIDs(ctx context.Context, ids []int64) (map[int64]float64, error) {
	out := map[int64]float64{}
	list := distinctIDs(ids)
	if len(list) == 0 {
		return out, nil
	}
	rows, err := r.q.ListPurchaseBaseCostsByIDs(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		f := row.PurchaseToBaseFactor
		if f <= 0 {
			f = 1
		}
		out[row.ID] = float64(row.PurchasePrice) / f
	}
	return out, nil
}

// PurchasePricesByIDs resolves catalog purchase price for many products in
// ONE grouped read.
func (r *Repository) PurchasePricesByIDs(ctx context.Context, ids []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	list := distinctIDs(ids)
	if len(list) == 0 {
		return out, nil
	}
	rows, err := r.q.ListPurchasePricesByIDs(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.PurchasePrice
	}
	return out, nil
}
