package application

import (
	"context"

	"mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/modules/delivery/infrastructure"
	salescontracts "mini-erp/internal/modules/sales/contracts"
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
	if err := s.decorateList(ctx, rows); err != nil {
		return nil, err
	}
	return &ListResult{Notes: rows, Total: total}, nil
}

// decorateList completes a list page the way enrich completes one note —
// lines, order numbers, direct-note customers — in three grouped reads
// instead of three per note.
func (s *Service) decorateList(ctx context.Context, notes []*contracts.DeliveryNote) error {
	if len(notes) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(notes))
	var soIDs []int64
	seen := map[int64]bool{}
	for _, n := range notes {
		ids = append(ids, n.ID)
		if n.SalesOrderID != 0 && !seen[n.SalesOrderID] {
			seen[n.SalesOrderID] = true
			soIDs = append(soIDs, n.SalesOrderID)
		}
	}
	lines, err := s.repo.ItemsByDeliveries(ctx, ids)
	if err != nil {
		return apperror.Internal(err)
	}
	orders := map[int64]*salescontracts.OrderSummary{}
	if len(soIDs) > 0 {
		if orders, err = s.sales.SummariesByIDs(ctx, soIDs); err != nil {
			return apperror.Internal(err)
		}
	}
	for _, n := range notes {
		n.Items = lines[n.ID]
		if n.Items == nil {
			n.Items = []*contracts.Item{}
		}
		if so := orders[n.SalesOrderID]; so != nil {
			n.OrderNumber = so.Number
		}
	}
	return s.decorateParties(ctx, notes)
}
