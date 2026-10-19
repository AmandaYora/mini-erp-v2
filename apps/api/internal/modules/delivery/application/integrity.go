package application

import (
	"context"

	"mini-erp/internal/modules/delivery/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/integrity"
)

// IntegrityIssues lists draft shipments whose stock already went out: a confirm
// that failed between the posting and the status flip. Repeating the confirm
// (or cancelling the draft) resolves them.
func (s *Service) IntegrityIssues(ctx context.Context, branchID int64) ([]integrity.Issue, error) {
	drafts, err := s.repo.List(ctx, branchID, 0, contracts.StatusDraft, 500, 0)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var out []integrity.Issue
	for _, n := range drafts {
		moved, err := s.stock.HasMovements(ctx, branchID, stockcontracts.RefDelivery, n.ID)
		if err != nil {
			return nil, err
		}
		if moved {
			out = append(out, integrity.Issue{Kind: "draft_with_stock", DocType: "delivery", DocID: n.ID, Number: n.Number,
				Detail: "Surat jalan masih draf tetapi stok sudah keluar: konfirmasi ulang untuk melanjutkan, atau batalkan untuk membalik stok"})
		}
	}
	return out, nil
}
