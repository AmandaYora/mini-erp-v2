package application

import (
	"context"

	"mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/modules/delivery/infrastructure"
	"mini-erp/internal/shared/apperror"
)

// ListFilter is the list filter (see infrastructure.ListFilter).
type ListFilter = infrastructure.ListFilter

// ListFiltered lists the branch's documents with the full filter set; Count
// uses the identical where.
func (s *Service) ListFiltered(ctx context.Context, branchID int64, f ListFilter, page, limit int) (*ListResult, error) {
	total, err := s.repo.CountFiltered(ctx, branchID, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	rows, err := s.repo.ListFiltered(ctx, branchID, f, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if rows == nil {
		rows = []*contracts.DeliveryNote{}
	}
	return &ListResult{Notes: rows, Total: total}, nil
}
