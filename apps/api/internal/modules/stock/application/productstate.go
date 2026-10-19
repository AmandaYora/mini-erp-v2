package application

import (
	"context"

	productcontracts "mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/shared/apperror"
)

// ProductStockState implements product's StockStateProvider: the facts the
// product guards (UOM/type lock, archive, untrack) are decided from.
func (s *Service) ProductStockState(ctx context.Context, productID int64) (productcontracts.StockState, error) {
	var st productcontracts.StockState
	var err error
	if st.HasHistory, err = s.repo.ProductHasHistory(ctx, productID); err != nil {
		return st, apperror.Internal(err)
	}
	if st.Available, err = s.repo.ProductOnHand(ctx, productID); err != nil {
		return st, apperror.Internal(err)
	}
	if st.HasHolds, err = s.repo.ProductHasHolds(ctx, productID); err != nil {
		return st, apperror.Internal(err)
	}
	return st, nil
}

// VariantStockStates implements the per-variant half of the provider.
func (s *Service) VariantStockStates(ctx context.Context, productID int64) (map[int64]productcontracts.VariantStockState, error) {
	rows, err := s.repo.VariantStockStates(ctx, productID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make(map[int64]productcontracts.VariantStockState, len(rows))
	for _, r := range rows {
		out[r.VariantID] = productcontracts.VariantStockState{OnHand: r.OnHand, HasHistory: r.HasHistory}
	}
	return out, nil
}

// AvailableByProducts implements the batch availability half of the provider.
func (s *Service) AvailableByProducts(ctx context.Context, branchID int64, productIDs []int64) (map[int64]float64, error) {
	m, err := s.repo.AvailableByProducts(ctx, branchID, productIDs)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return m, nil
}
