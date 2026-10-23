package application

import (
	"context"
	"fmt"

	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/health"
	"mini-erp/internal/shared/money"
)

// MinPriceAboveSelling is data-health alert A8 (master.min_price_above_selling,
// ADR-0025): active products the POS cannot sell at their catalog price.
func (s *Service) MinPriceAboveSelling(ctx context.Context, sample int) (health.Result, error) {
	rows, err := s.repo.MinPriceAboveSelling(ctx)
	if err != nil {
		return health.Result{}, apperror.Internal(err)
	}
	res := health.Result{Count: int64(len(rows))}
	for _, r := range rows[:min(sample, len(rows))] {
		res.Sample = append(res.Sample, health.Sample{
			Label: r.Code + " " + r.Name,
			Value: fmt.Sprintf("min %s / jual %s", money.Rupiah(r.MinSellingPrice), money.Rupiah(r.SellingPrice)),
		})
	}
	return res, nil
}

// DuplicateNames is data-health alert A9 (master.duplicate_product_name):
// groups of active products sharing a name (case and outer spaces ignored).
// Meta.rows is the number of products in those groups.
func (s *Service) DuplicateNames(ctx context.Context, sample int) (health.Result, error) {
	groups, err := s.repo.DuplicateNameGroups(ctx)
	if err != nil {
		return health.Result{}, apperror.Internal(err)
	}
	var rows int64
	res := health.Result{Count: int64(len(groups))}
	for i, g := range groups {
		rows += g.Rows
		if i < sample {
			res.Sample = append(res.Sample, health.Sample{Label: g.Name, Value: fmt.Sprintf("%d produk", g.Rows)})
		}
	}
	res.Meta = map[string]int64{"rows": rows}
	return res, nil
}
