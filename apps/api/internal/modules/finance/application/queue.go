package application

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"

	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	purchasereturncontracts "mini-erp/internal/modules/purchasereturn/contracts"
	salesreturncontracts "mini-erp/internal/modules/salesreturn/contracts"
	"mini-erp/internal/shared/apperror"
)

// errPostedIndex surfaces a journal-index read failure distinctly so a
// half-built queue is never presented as "nothing to post".
var errPostedIndex = errors.New("gagal membaca indeks jurnal")

// Derived posting queue without a posting table (G-revisi): "unposted" is
// confirmed-documents minus the journal index, computed on read. It cannot
// drift — there is no second store to sync — and posting itself stays
// idempotent through Post().
//
// Query count for N candidates: 5 candidate reads (one bounded list per
// document kind) + 5 posted-set reads (one grouped query per source type).
// Never one query per document (P3). Expenses are absent on purpose: they
// auto-post on create, so they can never be unposted.

// QueueCap bounds each candidate list. The queue is an operator worklist,
// not a report — beyond the cap the accountant posts in batches and the
// list shrinks.
const QueueCap = 200

// MaxBatch caps one batch-post call so a fat-fingered "post all" cannot
// lock the books for minutes.
const MaxBatch = 50

// QueueItem is one unposted document awaiting its journal.
type QueueItem struct {
	DocType string `json:"docType"`
	DocID   int64  `json:"docId"`
	Number  string `json:"number"`
	Date    string `json:"date"`
}

// UnpostedSources returns confirmed-but-unjournaled documents, newest first
// per kind. Replacement shipments are excluded (they never auto-post);
// cancelled documents are excluded by the candidate lists themselves.
func (s *Service) UnpostedSources(ctx context.Context, branchID int64) ([]QueueItem, error) {
	out := []QueueItem{}

	posted := func(docType string) map[int64]bool {
		set, err := s.repo.PostedSourceIDs(ctx, branchID, docType)
		if err != nil {
			return nil
		}
		return set
	}

	if notes, err := s.delivery.ListConfirmed(ctx, branchID, QueueCap); err != nil {
		return nil, apperror.Internal(err)
	} else {
		done := posted(SourceDelivery)
		if done == nil {
			return nil, apperror.Internal(errPostedIndex)
		}
		for _, n := range notes {
			if n.DocumentKind == deliverycontracts.DocumentKindReplacement {
				continue
			}
			if !done[n.ID] {
				out = append(out, QueueItem{DocType: SourceDelivery, DocID: n.ID, Number: n.Number, Date: n.DeliveryDate})
			}
		}
	}

	if receipts, err := s.goodsreceipt.ListRecent(ctx, branchID, QueueCap); err != nil {
		return nil, apperror.Internal(err)
	} else {
		done := posted(SourceGoodsReceipt)
		if done == nil {
			return nil, apperror.Internal(errPostedIndex)
		}
		for _, r := range receipts {
			if !done[r.ID] {
				out = append(out, QueueItem{DocType: SourceGoodsReceipt, DocID: r.ID,
					Number: "Penerimaan #" + itoa(r.ID) + " (" + r.OrderNumber + ")", Date: r.ReceivedAt})
			}
		}
	}

	if payments, err := s.payment.ListActive(ctx, branchID, QueueCap); err != nil {
		return nil, apperror.Internal(err)
	} else {
		done := posted(SourcePayment)
		if done == nil {
			return nil, apperror.Internal(err)
		}
		for _, p := range payments {
			if !done[p.ID] {
				out = append(out, QueueItem{DocType: SourcePayment, DocID: p.ID, Number: p.Number, Date: p.PaidAt})
			}
		}
	}

	if returns, err := s.salesReturns.ListByBranch(ctx, branchID); err != nil {
		return nil, apperror.Internal(err)
	} else {
		done := posted(SourceSalesReturn)
		if done == nil {
			return nil, apperror.Internal(err)
		}
		for _, r := range returns {
			if r.Status != salesreturncontracts.StatusConfirmed {
				continue
			}
			if !done[r.ID] {
				out = append(out, QueueItem{DocType: SourceSalesReturn, DocID: r.ID, Number: r.Number, Date: ""})
			}
		}
	}

	if returns, err := s.purchaseReturns.ListByBranch(ctx, branchID); err != nil {
		return nil, apperror.Internal(err)
	} else {
		done := posted(SourcePurchaseReturn)
		if done == nil {
			return nil, apperror.Internal(err)
		}
		for _, r := range returns {
			if r.Status != purchasereturncontracts.StatusConfirmed {
				continue
			}
			if !done[r.ID] {
				out = append(out, QueueItem{DocType: SourcePurchaseReturn, DocID: r.ID, Number: r.Number, Date: ""})
			}
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date > out[j].Date
		}
		if out[i].DocType != out[j].DocType {
			return out[i].DocType < out[j].DocType
		}
		return out[i].DocID > out[j].DocID
	})
	return out, nil
}

// BatchItem is one posting request inside a batch.
type BatchItem struct {
	DocType string `json:"docType"`
	DocID   int64  `json:"docId"`
}

// BatchResult is the per-item outcome. One item's failure never aborts the
// rest — the accountant fixes the failure and re-runs; idempotency makes
// re-runs safe.
type BatchResult struct {
	DocType     string `json:"docType"`
	DocID       int64  `json:"docId"`
	EntryID     int64  `json:"entryId,omitempty"`
	EntryNumber string `json:"entryNumber,omitempty"`
	Error       string `json:"error,omitempty"`
}

// PostBatch posts up to MaxBatch documents, collecting per-item outcomes.
func (s *Service) PostBatch(ctx context.Context, actorID, branchID int64, items []BatchItem) ([]BatchResult, error) {
	if len(items) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: "wajib diisi"}})
	}
	if len(items) > MaxBatch {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: "maksimal 50 dokumen per batch"}})
	}
	out := make([]BatchResult, 0, len(items))
	for _, it := range items {
		res := BatchResult{DocType: it.DocType, DocID: it.DocID}
		entry, err := s.Post(ctx, actorID, branchID, it.DocType, it.DocID)
		if err != nil {
			if appErr, ok := err.(*apperror.AppError); ok {
				res.Error = appErr.Message
			} else {
				res.Error = err.Error()
			}
		} else {
			res.EntryID, res.EntryNumber = entry.ID, entry.Number
		}
		out = append(out, res)
	}
	return out, nil
}

// CloseDay posts every queued document dated on or before day (YYYY-MM-DD)
// — the "tutup harian" that replaces the legacy close-day endpoint. Items
// without a comparable date ride along: leaving dateless documents behind
// would strand them outside every daily close.
func (s *Service) CloseDay(ctx context.Context, actorID, branchID int64, day string) ([]BatchResult, error) {
	queue, err := s.UnpostedSources(ctx, branchID)
	if err != nil {
		return nil, err
	}
	day = strings.TrimSpace(day)
	batch := make([]BatchItem, 0, len(queue))
	for _, q := range queue {
		if q.Date != "" && len(q.Date) >= 10 && q.Date[:10] > day {
			continue
		}
		batch = append(batch, BatchItem{DocType: q.DocType, DocID: q.DocID})
		if len(batch) == MaxBatch {
			break
		}
	}
	if len(batch) == 0 {
		return []BatchResult{}, nil
	}
	return s.PostBatch(ctx, actorID, branchID, batch)
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
