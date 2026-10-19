package infrastructure

import (
	"database/sql"
	"encoding/json"

	"mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/modules/product/infrastructure/sqlc"
)

func ptrInt64(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}

func nullPtr(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

func toCategory(id int64, code, name string, parent sql.NullInt64, sortOrder int32, status string) *contracts.Category {
	return &contracts.Category{ID: id, Code: code, Name: name, ParentID: ptrInt64(parent), SortOrder: int(sortOrder), Status: status}
}

// toProduct maps the product column projection; every product query selects the
// same columns, so its row types convert to sqlc.GetProductRow.
func toProduct(m sqlc.GetProductRow) *contracts.Product {
	return &contracts.Product{
		ID: m.ID, Code: m.Code, Name: m.Name, CategoryID: ptrInt64(m.CategoryID), Type: m.Type,
		Tracked: m.StockTracked, BaseUOM: m.BaseUom, PurchaseUOM: m.PurchaseUom, SalesUOM: m.SalesUom,
		PurchaseFactor: m.PurchaseToBaseFactor, SalesFactor: m.SalesToBaseFactor,
		PurchasePrice: m.PurchasePrice, SellingPrice: m.SellingPrice, MinSellingPrice: m.MinSellingPrice,
		MinStock: m.MinStockQty, Status: m.Status, Summary: m.Summary.String,
		Attributes: decodeAttrs(m.Attributes),
	}
}

// toVariant maps the variant projection shared by every variant query.
func toVariant(m sqlc.GetDefaultVariantRow) *contracts.Variant {
	return &contracts.Variant{
		ID: m.ID, ProductID: m.ProductID, Code: m.Code, Name: m.Name, Barcode: m.Barcode.String,
		IsDefault: m.IsDefault, Status: m.Status, IsHidden: m.IsHidden, SortOrder: int(m.SortOrder),
		Attributes: decodeAttrs(m.Attributes),
	}
}

// decodeAttrs never returns nil (the API always shows an object) and reads a
// corrupt value as empty rather than failing a read.
func decodeAttrs(v any) map[string]string {
	out := map[string]string{}
	var raw []byte
	switch t := v.(type) {
	case []byte:
		raw = t
	case string:
		raw = []byte(t)
	}
	if len(raw) > 0 {
		var m map[string]any
		if json.Unmarshal(raw, &m) == nil {
			for k, v := range m {
				switch t := v.(type) {
				case string:
					out[k] = t
				case nil:
				default:
					b, _ := json.Marshal(t)
					out[k] = string(b)
				}
			}
		}
	}
	return out
}

// encodeAttrs maps an empty attribute set to NULL and otherwise to JSON text
// (bound as a string: a []byte argument would reach the JSON column as binary).
func encodeAttrs(m map[string]string) any {
	if len(m) == 0 {
		return nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	return string(b)
}

func statusOrActive(st string) string {
	if st == "" {
		return "active"
	}
	return st
}

// distinctIDs drops zero and repeated ids, keeping first-seen order.
func distinctIDs(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
