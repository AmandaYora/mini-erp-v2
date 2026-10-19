package application

import (
	"context"

	"mini-erp/internal/modules/finance/contracts"
	partycontracts "mini-erp/internal/modules/party/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/shared/apperror"
)

// entryLines resolves the legs of many entries in one grouped read.
//
// Reports iterate entries and need each one's lines. GetEntry per entry cost
// two queries per row and re-read the header the caller already holds; this
// asks once for all of them. Unlike decoration, this IS the report's data,
// so a failure propagates instead of degrading to blanks.
func (s *Service) entryLines(ctx context.Context, entries []*contracts.JournalEntry) (map[int64][]*contracts.JournalLine, error) {
	if len(entries) == 0 {
		return map[int64][]*contracts.JournalLine{}, nil
	}
	ids := make([]int64, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.ID)
	}
	lines, err := s.repo.LinesByEntryIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return lines, nil
}

// Report decoration: resolving the code and name a report row displays.
//
// Decoration is best-effort by design — a report is about money, and a
// product renamed or a party archived must never turn a balance into a 500.
// Both helpers therefore swallow the error and return what they have, which
// is exactly what the per-row `if x, err := ...; err == nil` calls they
// replace did. What changes is the cost: one grouped read for the whole
// report instead of one read per row.

// productLabels resolves code+name for the given product ids in one read.
// A nil client (dashboards wired without product) yields an empty map, and
// rows simply render without a name.
func (s *Service) productLabels(ctx context.Context, ids []int64) map[int64]productcontracts.Label {
	if s.products == nil || len(ids) == 0 {
		return map[int64]productcontracts.Label{}
	}
	labels, err := s.products.LabelsByIDs(ctx, ids)
	if err != nil || labels == nil {
		return map[int64]productcontracts.Label{}
	}
	return labels
}

// partyLabels resolves code+name for the given party ids in one read.
func (s *Service) partyLabels(ctx context.Context, ids []int64) map[int64]partycontracts.Label {
	if s.parties == nil || len(ids) == 0 {
		return map[int64]partycontracts.Label{}
	}
	labels, err := s.parties.LabelsByIDs(ctx, ids)
	if err != nil || labels == nil {
		return map[int64]partycontracts.Label{}
	}
	return labels
}
