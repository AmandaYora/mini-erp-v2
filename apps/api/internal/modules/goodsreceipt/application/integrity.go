package application

import (
	"context"

	"mini-erp/internal/modules/goodsreceipt/contracts"
	"mini-erp/internal/modules/goodsreceipt/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/integrity"
)

// IntegrityIssues lists receipts left pending: recorded, stock not (fully) in.
// Complete re-runs the posting; Discard voids the receipt.
func (s *Service) IntegrityIssues(ctx context.Context, branchID int64) ([]integrity.Issue, error) {
	recs, err := s.repo.List(ctx, infrastructure.ListFilter{BranchID: branchID}, 500, 0)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var out []integrity.Issue
	for _, r := range recs {
		if r.Status == contracts.StatusPending {
			out = append(out, integrity.Issue{Kind: "pending_receipt", DocType: "goods_receipt", DocID: r.ID, Number: r.Number,
				Detail: "Penerimaan tersimpan tetapi belum diposting: pilih Posting ulang atau Batalkan"})
		}
	}
	return out, nil
}
