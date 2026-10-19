package application

import (
	"context"
	"strings"

	"mini-erp/internal/modules/salesreturn/contracts"
	"mini-erp/internal/modules/salesreturn/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/listquery"
)

// ListFilter is the list filter (see infrastructure.ListFilter).
type ListFilter = infrastructure.ListFilter

// ListFiltered lists the branch's documents with the full filter set; Count
// uses the identical where.
func (s *Service) ListFiltered(ctx context.Context, branchID int64, f ListFilter, page, limit int) (*ListResult, error) {
	// The order half of the search resolves through the sales contract (order
	// number or customer); the return's own text is matched in SQL (RT-01).
	f.SearchOrderIDs = nil
	if strings.TrimSpace(f.Search) != "" {
		ids, serr := s.sales.SearchOrderIDs(ctx, branchID, f.Search, listquery.MaxIn)
		if serr != nil {
			return nil, serr
		}
		f.SearchOrderIDs = ids
	}
	total, err := s.repo.CountFiltered(ctx, branchID, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	rows, err := s.repo.ListFiltered(ctx, branchID, f, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if rows == nil {
		rows = []*contracts.SalesReturn{}
	}
	if err := s.decorateOrders(ctx, rows); err != nil {
		return nil, err
	}
	return &ListResult{Returns: rows, Total: total}, nil
}

// decorateOrders fills OrderNumber of a page in one batch read, so the list's
// "Order jual" column is never empty (RT-01).
func (s *Service) decorateOrders(ctx context.Context, rows []*contracts.SalesReturn) error {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.SalesOrderID)
	}
	orders, err := s.sales.SummariesByIDs(ctx, ids)
	if err != nil {
		return apperror.Internal(err)
	}
	for _, r := range rows {
		if o := orders[r.SalesOrderID]; o != nil {
			r.OrderNumber = o.Number
		}
	}
	return nil
}
