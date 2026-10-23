package application

import (
	"context"
	"fmt"

	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/health"
)

// DuplicateNames is data-health alert A10 (master.duplicate_customer_name,
// ADR-0025) for partyType "customer": groups of active parties sharing a name,
// case and outer spaces ignored. Meta.rows is the parties in those groups.
func (s *Service) DuplicateNames(ctx context.Context, partyType string, sample int) (health.Result, error) {
	groups, err := s.repo.DuplicateNameGroups(ctx, partyType)
	if err != nil {
		return health.Result{}, apperror.Internal(err)
	}
	var rows int64
	res := health.Result{Count: int64(len(groups))}
	for i, g := range groups {
		rows += g.RowsCount
		if i < sample {
			res.Sample = append(res.Sample, health.Sample{Label: g.NormName, Value: fmt.Sprintf("%d data", g.RowsCount)})
		}
	}
	res.Meta = map[string]int64{"rows": rows}
	return res, nil
}
