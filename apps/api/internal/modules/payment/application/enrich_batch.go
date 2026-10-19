package application

import (
	"context"

	"mini-erp/internal/modules/payment/contracts"
	"mini-erp/internal/shared/apperror"
)

// enrichBatch decorates a page of payments with allocations, order numbers and
// party names in a constant number of reads (F4-7): one IN query for the
// allocations, one batch per document kind that appears, one names batch —
// instead of ~5 reads per payment. Returns are labelled through a per-call
// cache (their contracts have no batch read; they are rare in allocations).
func (s *Service) enrichBatch(ctx context.Context, payments []*contracts.Payment) ([]*contracts.Payment, error) {
	if len(payments) == 0 {
		return payments, nil
	}
	ids := make([]int64, len(payments))
	partyIDs := make([]int64, len(payments))
	for i, p := range payments {
		ids[i], partyIDs[i] = p.ID, p.PartyID
	}
	byPayment, err := s.repo.AllocationsByPayments(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var salesIDs, purchaseIDs []int64
	for _, allocs := range byPayment {
		for _, a := range allocs {
			switch a.OrderType {
			case contracts.OrderSales:
				salesIDs = append(salesIDs, a.OrderID)
			case contracts.OrderPurchase:
				purchaseIDs = append(purchaseIDs, a.OrderID)
			}
		}
	}
	salesNums := map[int64]string{}
	if len(salesIDs) > 0 {
		m, err := s.sales.SummariesByIDs(ctx, salesIDs)
		if err != nil {
			return nil, err
		}
		for id, o := range m {
			salesNums[id] = o.Number
		}
	}
	purchaseNums := map[int64]string{}
	if len(purchaseIDs) > 0 {
		m, err := s.purchasing.SummariesByIDs(ctx, purchaseIDs)
		if err != nil {
			return nil, err
		}
		for id, o := range m {
			purchaseNums[id] = o.Number
		}
	}
	type retKey struct {
		t  string
		id int64
	}
	retNums := map[retKey]string{}
	names, err := s.parties.NamesByIDs(ctx, partyIDs)
	if err != nil {
		return nil, err
	}
	for _, p := range payments {
		allocs := byPayment[p.ID]
		if allocs == nil {
			allocs = []*contracts.Allocation{}
		}
		for _, a := range allocs {
			switch a.OrderType {
			case contracts.OrderSales:
				a.OrderNumber = salesNums[a.OrderID]
			case contracts.OrderPurchase:
				a.OrderNumber = purchaseNums[a.OrderID]
			default:
				k := retKey{a.OrderType, a.OrderID}
				num, ok := retNums[k]
				if !ok {
					num = s.orderNumber(ctx, a.OrderType, a.OrderID)
					retNums[k] = num
				}
				a.OrderNumber = num
			}
		}
		p.Allocations = allocs
		p.PartyName = names[p.PartyID]
	}
	return payments, nil
}
