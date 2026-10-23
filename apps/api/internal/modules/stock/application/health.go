package application

import (
	"context"
	"strconv"

	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/health"
)

// NegativeSummary is data-health alert A6 (stock.negative, ADR-0025): stock
// positions below zero on hand. Meta.products counts the distinct products;
// the sample is named through one product label read.
func (s *Service) NegativeSummary(ctx context.Context, branchID int64, sample int) (health.Result, error) {
	rows, err := s.repo.NegativePositions(ctx, branchID)
	if err != nil {
		return health.Result{}, apperror.Internal(err)
	}
	products := map[int64]bool{}
	for _, r := range rows {
		products[r.ProductID] = true
	}
	res := health.Result{Count: int64(len(rows)), Meta: map[string]int64{"products": int64(len(products))}}
	if len(rows) == 0 {
		return res, nil
	}
	top := rows[:min(sample, len(rows))]
	ids := make([]int64, 0, len(top))
	for _, r := range top {
		ids = append(ids, r.ProductID)
	}
	labels, err := s.products.LabelsByIDs(ctx, ids)
	if err != nil {
		return health.Result{}, apperror.Internal(err)
	}
	for _, r := range top {
		l := labels[r.ProductID]
		label := l.Name
		if l.Code != "" {
			label = l.Code + " " + l.Name
		}
		res.Sample = append(res.Sample, health.Sample{Label: label, Value: "stok " + strconv.FormatFloat(r.OnHand, 'f', -1, 64)})
	}
	return res, nil
}
