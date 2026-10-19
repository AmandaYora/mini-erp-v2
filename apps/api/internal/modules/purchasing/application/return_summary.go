package application

import (
	"context"

	"mini-erp/internal/modules/purchasing/contracts"
	"mini-erp/internal/shared/apperror"
)

// Return summary of an order for the list badge "Diretur" (PO-01).
const (
	ReturnNone    = "none"
	ReturnPartial = "partial"
	ReturnFull    = "full"
)

// qtyEpsilon absorbs the decimal(24,12) rounding of stored quantities.
const qtyEpsilon = 1e-9

// returnState classifies an order from what its posted receipts brought in and
// what its confirmed returns sent back, per "productId/variantId" position
// (legacy isOrderFullyReturned): nothing returned = none; every received
// position returned in full = full; anything between = partial.
func returnState(received, returned map[string]float64) string {
	any := false
	for _, q := range returned {
		if q > qtyEpsilon {
			any = true
			break
		}
	}
	if !any {
		return ReturnNone
	}
	for key, got := range received {
		if got <= qtyEpsilon {
			continue
		}
		if returned[key]+qtyEpsilon < got {
			return ReturnPartial
		}
	}
	return ReturnFull
}

// ReturnSummaries classifies a page of orders with TWO grouped reads (posted
// receipts, confirmed returns), never one per order.
func (s *Service) ReturnSummaries(ctx context.Context, orders []*contracts.PurchaseOrder) (map[int64]string, error) {
	out := make(map[int64]string, len(orders))
	if len(orders) == 0 {
		return out, nil
	}
	if s.dep.receipts == nil || s.dep.returns == nil {
		return nil, apperror.Internal(errDependentsNotWired)
	}
	ids := make([]int64, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, o.ID)
	}
	returned, err := s.dep.returns.ReturnedQtyByOrders(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	// Orders without a confirmed return need no receipt read at all.
	var withReturns []int64
	for _, id := range ids {
		if len(returned[id]) > 0 {
			withReturns = append(withReturns, id)
		}
	}
	var received map[int64]map[string]float64
	if len(withReturns) > 0 {
		if received, err = s.dep.receipts.ReceivedQtyByOrders(ctx, withReturns); err != nil {
			return nil, apperror.Internal(err)
		}
	}
	for _, id := range ids {
		out[id] = returnState(received[id], returned[id])
	}
	return out, nil
}
