package application

import (
	"context"
	"strings"

	"mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
)

// StatusCounts returns one branch's order counts keyed by derived status.
//
// Query count: 1 grouped repo read for all statuses. The four fixed statuses
// are zero-filled so readers never nil-check; unknown values pass through
// instead of failing — a report must not break on data it does not know.
func (s *Service) StatusCounts(ctx context.Context, branchID int64) (map[string]int64, error) {
	counts, err := s.repo.StatusCounts(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := map[string]int64{
		contracts.StatusDraft:     0,
		contracts.StatusConfirmed: 0,
		contracts.StatusCompleted: 0,
		contracts.StatusCancelled: 0,
	}
	for status, n := range counts {
		out[status] = n
	}
	return out, nil
}

// TopProducts ranks products by base qty sold in an inclusive YYYY-MM-DD
// range, biggest first.
//
// Query count: 1 grouped repo read for the whole ranking (limit is a display
// cap applied in SQL, the ranking itself covers the full range).
func (s *Service) TopProducts(ctx context.Context, branchID int64, from, to string, limit int) ([]*contracts.TopProduct, error) {
	if _, err := timeutil.ParseDateInput(trimDate(from)); err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "from", Message: "format tanggal harus YYYY-MM-DD"}})
	}
	if _, err := timeutil.ParseDateInput(trimDate(to)); err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "to", Message: "format tanggal harus YYYY-MM-DD"}})
	}
	if trimDate(from) > trimDate(to) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "to", Message: "tidak boleh sebelum dari"}})
	}
	if limit <= 0 || limit > 20 {
		limit = 5
	}
	rows, err := s.repo.TopProducts(ctx, branchID, trimDate(from), trimDate(to), limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if rows == nil {
		rows = []*contracts.TopProduct{}
	}
	return rows, nil
}

// CountMissingTaxInvoice counts taxed orders past draft without a tax
// invoice number — one COUNT query for the readiness gate.
func (s *Service) CountMissingTaxInvoice(ctx context.Context, branchID int64) (int64, error) {
	n, err := s.repo.CountMissingTaxInvoice(ctx, branchID)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return n, nil
}

// OpenShipments lists confirmed non-POS orders oldest-first with lines and
// party names for the delivery work queue (J3).
//
// Query count: 1 headers + 1 grouped items read + one party read per
// distinct party (same best-effort pattern as ListOrders).
func (s *Service) OpenShipments(ctx context.Context, branchID int64, limit int) ([]*contracts.OpenShipment, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	open, err := s.repo.OpenHeaders(ctx, branchID, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if len(open) == 0 {
		return []*contracts.OpenShipment{}, nil
	}
	ids := make([]int64, 0, len(open))
	for _, o := range open {
		ids = append(ids, o.OrderID)
	}
	lines, err := s.repo.ItemsByOrders(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	names := map[int64]string{0: "Tunai"}
	for _, o := range open {
		o.Items = lines[o.OrderID]
		if o.Items == nil {
			o.Items = []*contracts.Item{}
		}
		name, ok := names[o.PartyID]
		if !ok {
			name = ""
			if p, err := s.parties.GetByID(ctx, o.PartyID); err == nil && p != nil {
				name = p.Name
			}
			names[o.PartyID] = name
		}
		o.PartyName = name
	}
	return open, nil
}

// OrderParties maps order ids to party ids in one grouped read (A3).
func (s *Service) OrderParties(ctx context.Context, ids []int64) (map[int64]int64, error) {
	parties, err := s.repo.OrderParties(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return parties, nil
}

func trimDate(s string) string {
	return strings.TrimSpace(s)
}
