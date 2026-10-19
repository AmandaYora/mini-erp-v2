package application

import (
	"context"
	"strings"

	"mini-erp/internal/modules/payment/contracts"
	"mini-erp/internal/modules/payment/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/listquery"
)

// ListFilter is the payment list filter (see infrastructure.ListFilter).
type ListFilter = infrastructure.ListFilter

// ListFiltered lists payments with the full F3a-5 filter set (search by
// number/notes/reference/party, method, direction, paid-date range,
// order allocation, party, status) and decorates every row with the party name
// from one batch read. Count uses the identical where.
func (s *Service) ListFiltered(ctx context.Context, branchID int64, f ListFilter, page, limit int) (*ListResult, error) {
	if err := listquery.ValidateEnum("direction", f.Direction, "in", "out"); err != nil {
		return nil, err
	}
	if err := listquery.ValidateEnum("orderType", f.OrderType, contracts.OrderSales, contracts.OrderPurchase, contracts.OrderSalesReturn, contracts.OrderPurchaseReturn); err != nil {
		return nil, err
	}
	if f.OrderID != 0 && f.OrderType == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "orderType", Message: "wajib diisi bersama orderId"}})
	}
	if strings.TrimSpace(f.Search) != "" {
		ids, err := s.parties.SearchIDs(ctx, "", f.Search, listquery.MaxIn)
		if err != nil {
			return nil, err
		}
		f.SearchPartyIDs = ids
	}
	total, err := s.repo.CountFiltered(ctx, branchID, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	payments, err := s.repo.ListFiltered(ctx, branchID, f, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if payments == nil {
		payments = []*contracts.Payment{}
	}
	ids := make([]int64, 0, len(payments))
	for _, p := range payments {
		ids = append(ids, p.PartyID)
	}
	names, err := s.parties.NamesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, p := range payments {
		if p.PartyID == 0 {
			p.PartyName = "Tunai"
		} else {
			p.PartyName = names[p.PartyID]
		}
	}
	return &ListResult{Payments: payments, Total: total}, nil
}
