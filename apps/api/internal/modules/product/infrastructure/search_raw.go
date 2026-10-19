package infrastructure

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/modules/product/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
)

// ProductFilter is the single source of the product list filter, shared by
// ListProducts and CountProducts so the two can never drift apart.
type ProductFilter struct {
	Search     string
	CategoryID *int64
	Status     string
	Type       string
	Tracked    *bool
	// ID narrows to one product (hydrating a selected option without searching again).
	ID *int64
	// Ranked orders a non-empty search by relevance (exact code, code prefix,
	// name prefix, then the rest, each by name) instead of plain name order.
	Ranked bool
}

// likeEscape neutralises LIKE wildcards in a user-typed prefix.
func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// orderBy is the list ordering: relevance rank first when Ranked and a search
// term exists (F3b/PR-03, legacy product.service.ts search_rank), then name.
func (f ProductFilter) orderBy() (string, []any) {
	term := strings.TrimSpace(f.Search)
	if !f.Ranked || term == "" {
		return "ORDER BY p.name ASC, p.id ASC", nil
	}
	prefix := likeEscape(term) + "%"
	return `ORDER BY CASE WHEN p.code = ? THEN 0 WHEN p.code LIKE ? THEN 1 WHEN p.name LIKE ? THEN 2 ELSE 3 END,
		p.name ASC, p.id ASC`, []any{term, prefix, prefix}
}

// where builds the filter: fixed optional columns plus one (code|name LIKE)
// condition per whitespace-separated search word, all ANDed. The number of
// conditions follows the number of words.
func (f ProductFilter) where(prefix string) (string, []any) {
	var conds []string
	var args []any
	if f.ID != nil {
		conds = append(conds, prefix+"id = ?")
		args = append(args, *f.ID)
	}
	if f.Status != "" {
		conds = append(conds, prefix+"status = ?")
		args = append(args, f.Status)
	}
	if f.Type != "" {
		conds = append(conds, prefix+"type = ?")
		args = append(args, f.Type)
	}
	if f.Tracked != nil {
		conds = append(conds, prefix+"stock_tracked = ?")
		args = append(args, dbutil.Bool(*f.Tracked))
	}
	if f.CategoryID != nil {
		conds = append(conds, prefix+"category_id = ?")
		args = append(args, *f.CategoryID)
	}
	for _, word := range strings.Fields(f.Search) {
		like := "%" + word + "%"
		conds = append(conds, "("+prefix+"code LIKE ? OR "+prefix+"name LIKE ?)")
		args = append(args, like, like)
	}
	if len(conds) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}

// ListProducts searches code/name (per word) with the filter set.
func (r *Repository) ListProducts(ctx context.Context, f ProductFilter, limit, offset int) ([]*contracts.Product, error) {
	where, args := f.where("p.")
	order, orderArgs := f.orderBy()
	args = append(args, orderArgs...)
	args = append(args, limit, offset)
	// raw-sql: variable-arity AND of one LIKE group per search word; sqlc.narg/slice cannot express a per-word condition list
	rows, err := r.db.QueryContext(ctx,
		`SELECT p.id, p.code, p.name, p.category_id, p.type, p.stock_tracked,
			p.base_uom, p.purchase_uom, p.sales_uom, p.purchase_to_base_factor, p.sales_to_base_factor,
			p.purchase_price, p.selling_price, p.min_selling_price, p.min_stock_qty, p.status,
			p.summary, CAST(p.attributes AS CHAR), c.name, p.updated_at
		 FROM products p LEFT JOIN product_categories c ON c.id = p.category_id `+
			where+" "+order+" LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Product
	for rows.Next() {
		var row listRow
		var updatedAt time.Time
		if err := rows.Scan(&row.p.ID, &row.p.Code, &row.p.Name, &row.p.CategoryID, &row.p.Type, &row.p.StockTracked,
			&row.p.BaseUom, &row.p.PurchaseUom, &row.p.SalesUom, &row.p.PurchaseToBaseFactor, &row.p.SalesToBaseFactor,
			&row.p.PurchasePrice, &row.p.SellingPrice, &row.p.MinSellingPrice, &row.p.MinStockQty, &row.p.Status,
			&row.p.Summary, &row.p.Attributes, &row.categoryName, &updatedAt); err != nil {
			return nil, err
		}
		p := toProduct(row.p)
		p.CategoryName = row.categoryName.String
		if p.Status == "archived" {
			at := updatedAt.UTC()
			p.ArchivedAt = &at
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CountProducts counts the List filter set.
func (r *Repository) CountProducts(ctx context.Context, f ProductFilter) (int64, error) {
	where, args := f.where("")
	var total int64
	// raw-sql: same variable-arity per-word filter as ListProducts (kept in one place so list and count cannot drift)
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM products "+where, args...).Scan(&total)
	return total, err
}

// listRow is one raw list row: the product columns plus the joined category name.
type listRow struct {
	p            sqlc.GetProductRow
	categoryName sql.NullString
}
