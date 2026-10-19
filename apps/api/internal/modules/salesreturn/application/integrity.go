package application

import (
	"context"

	"mini-erp/internal/modules/salesreturn/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/integrity"
)

// IntegrityIssues finds returns whose stock effect disagrees with their status:
// confirmed without the stock movement, or cancelled with the movement not
// reversed. Repeating Confirm/Cancel repairs them (both are replay-safe).
func (s *Service) IntegrityIssues(ctx context.Context, branchID int64) ([]integrity.Issue, error) {
	var out []integrity.Issue
	confirmed, err := s.repo.List(ctx, branchID, 0, "confirmed", 500, 0)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for _, r := range confirmed {
		full, err := s.GetByID(ctx, r.ID)
		if err != nil || full == nil {
			continue
		}
		stocked := false
		for _, it := range full.Items {
			if it.LocationID != 0 {
				stocked = true
			}
		}
		if !stocked {
			continue
		}
		moved, err := s.stock.HasMovements(ctx, branchID, stockcontracts.RefSalesReturn, r.ID)
		if err != nil {
			return nil, err
		}
		if !moved {
			out = append(out, integrity.Issue{Kind: "confirmed_without_stock", DocType: "sales_return", DocID: r.ID, Number: r.Number,
				Detail: "Retur terkonfirmasi tetapi stok belum bergerak: konfirmasi ulang untuk melanjutkan"})
		}
	}
	cancelled, err := s.repo.List(ctx, branchID, 0, contracts.StatusCancelled, 500, 0)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for _, r := range cancelled {
		moved, err := s.stock.HasMovements(ctx, branchID, stockcontracts.RefSalesReturn, r.ID)
		if err != nil {
			return nil, err
		}
		if !moved {
			continue
		}
		undone, err := s.stock.HasMovements(ctx, branchID, stockcontracts.RefSalesReturn+"_cancel", r.ID)
		if err != nil {
			return nil, err
		}
		if !undone {
			out = append(out, integrity.Issue{Kind: "cancelled_stock_not_reversed", DocType: "sales_return", DocID: r.ID, Number: r.Number,
				Detail: "Retur dibatalkan tetapi gerakan stoknya belum dibalik: batalkan ulang untuk membalik"})
		}
	}
	return out, nil
}
