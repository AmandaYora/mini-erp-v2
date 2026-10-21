package application

import (
	"context"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/finance/infrastructure"
	"mini-erp/internal/shared/apperror"
)

// Ignore / restore a stuck source document (FN-01). A document the books can
// never journal (a legacy leftover, a duplicate, a test shipment) would
// otherwise block every close forever. Ignoring is a deliberate, audited,
// reversible act: the document leaves the blocking queue, shows up as a
// warning in the close checklist, and can be restored at any time.

// postableDocTypes are the document kinds the queue scans (and so can be
// ignored). Business expenses auto-post and manual/opening journals have no
// source document, so neither appears here.
func postableDocType(docType string) bool {
	switch docType {
	case SourceDelivery, SourceGoodsReceipt, SourcePayment, SourceSalesReturn, SourcePurchaseReturn,
		SourceSalesReturnSettlement, SourcePurchaseReturnSettlement, SourceStockMovement:
		return true
	}
	return false
}

func validateIgnoreInput(docType string, docID int64, reason string, needReason bool) (string, error) {
	if !postableDocType(docType) {
		return "", apperror.Validation("", []apperror.FieldError{{Field: "docType", Message: "jenis dokumen tidak dikenal"}})
	}
	if docID <= 0 {
		return "", apperror.Validation("", []apperror.FieldError{{Field: "docId", Message: "wajib diisi"}})
	}
	reason = strings.TrimSpace(reason)
	if needReason {
		if reason == "" {
			return "", apperror.Validation("", []apperror.FieldError{{Field: "reason", Message: "alasan wajib diisi"}})
		}
		if len([]rune(reason)) > 255 {
			return "", apperror.Validation("", []apperror.FieldError{{Field: "reason", Message: "alasan maksimal 255 karakter"}})
		}
	}
	return reason, nil
}

// IgnoreSource takes one unposted document out of the blocking queue. It must
// currently be in the queue: ignoring something already journalled (or that does
// not exist) is refused. Idempotent: ignoring twice keeps the first reason.
func (s *Service) IgnoreSource(ctx context.Context, actorID, branchID int64, docType string, docID int64, reason string) (*QueueItem, error) {
	reason, err := validateIgnoreInput(docType, docID, reason, true)
	if err != nil {
		return nil, err
	}
	queue, err := s.UnpostedSources(ctx, branchID)
	if err != nil {
		return nil, err
	}
	var target *QueueItem
	for i := range queue {
		if queue[i].DocType == docType && queue[i].DocID == docID {
			target = &queue[i]
			break
		}
	}
	if target == nil {
		return nil, apperror.Conflict("Dokumen tidak ada di antrian posting (sudah dijurnal, dibatalkan, atau tidak ditemukan)")
	}
	if target.Ignored {
		return target, nil
	}
	key := docType + ":" + strconv.FormatInt(docID, 10)
	created, err := s.repo.IgnoreSource(ctx, branchID, docType, docID, reason, actorID, func(ctx context.Context, tx *infrastructure.Tx) error {
		return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
			Action: "finance.posting.ignore", Entity: "posting_source", BranchID: branchID, ActorID: actorID,
			Note:   target.Number + ": " + reason,
			Before: map[string]any{"ignored": false},
			After:  map[string]any{"ignored": true, "reason": reason},
			Metadata: map[string]any{"docType": docType, "docId": docID, "key": key, "number": target.Number,
				"amount": target.Amount, "date": target.Date},
		})
	})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	_ = created // a concurrent twin may have won; either way the document is ignored
	target.Ignored, target.IgnoreReason, target.IgnoredByID = true, reason, actorID
	return target, nil
}

// RestoreSource puts an ignored document back into the blocking queue.
func (s *Service) RestoreSource(ctx context.Context, actorID, branchID int64, docType string, docID int64) error {
	if _, err := validateIgnoreInput(docType, docID, "", false); err != nil {
		return err
	}
	was, err := s.repo.RestoreSource(ctx, branchID, docType, docID, func(ctx context.Context, tx *infrastructure.Tx, was infrastructure.PostingIgnore) error {
		return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
			Action: "finance.posting.restore", Entity: "posting_source", BranchID: branchID, ActorID: actorID,
			Note:     docType + " #" + strconv.FormatInt(docID, 10),
			Before:   map[string]any{"ignored": true, "reason": was.Reason},
			After:    map[string]any{"ignored": false},
			Metadata: map[string]any{"docType": docType, "docId": docID},
		})
	})
	if err != nil {
		return apperror.Internal(err)
	}
	if was == nil {
		return apperror.Conflict("Dokumen tidak sedang diabaikan")
	}
	return nil
}
