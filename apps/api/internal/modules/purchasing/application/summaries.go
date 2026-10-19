package application

import (
	"context"
	"strings"

	"mini-erp/internal/modules/purchasing/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/listquery"
)

// SummariesByIDs implements contracts.PurchaseOrderClient: many orders to
// lightweight rows in one read (payment ledger labels documents with it).
func (s *Service) SummariesByIDs(ctx context.Context, ids []int64) (map[int64]*contracts.OrderSummary, error) {
	out, err := s.repo.SummariesByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}

// SearchOrderIDs implements contracts.PurchaseOrderClient (see there). The
// supplier half goes through the party contract, never a join.
func (s *Service) SearchOrderIDs(ctx context.Context, branchID int64, term string, limit int) ([]int64, error) {
	f := ListFilter{Search: strings.TrimSpace(term)}
	if f.Search == "" {
		return nil, nil
	}
	if err := s.prepareFilter(ctx, &f); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > listquery.MaxIn {
		limit = listquery.MaxIn
	}
	ids, err := s.repo.SearchIDs(ctx, branchID, f, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return ids, nil
}
