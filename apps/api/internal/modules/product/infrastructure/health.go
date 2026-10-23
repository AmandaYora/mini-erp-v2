package infrastructure

import (
	"context"

	"mini-erp/internal/modules/product/infrastructure/sqlc"
)

// Data-health reads (ADR-0025): one read each over the catalog, no write.

// MinPriceAboveSelling lists active products whose minimum selling price is
// above their selling price (POS refuses the catalog price), name order.
func (r *Repository) MinPriceAboveSelling(ctx context.Context) ([]sqlc.ListMinPriceAboveSellingRow, error) {
	return r.q.ListMinPriceAboveSelling(ctx)
}

// DuplicateNameGroup is one case/space-insensitive name shared by several
// active products.
type DuplicateNameGroup struct {
	Name string
	Rows int64
}

// DuplicateNameGroups lists the duplicate active-product names, largest first.
func (r *Repository) DuplicateNameGroups(ctx context.Context) ([]DuplicateNameGroup, error) {
	rows, err := r.q.ListDuplicateNameGroups(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]DuplicateNameGroup, 0, len(rows))
	for _, row := range rows {
		name := row.NormName
		switch v := row.SampleName.(type) {
		case []byte:
			name = string(v)
		case string:
			name = v
		}
		out = append(out, DuplicateNameGroup{Name: name, Rows: row.RowsCount})
	}
	return out, nil
}
