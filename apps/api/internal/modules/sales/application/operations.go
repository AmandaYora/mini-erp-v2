package application

import (
	"context"
	"strings"

	partycontracts "mini-erp/internal/modules/party/contracts"
	"mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/listquery"
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

// CountMissingTaxInvoiceBetween counts them within an order-date range.
func (s *Service) CountMissingTaxInvoiceBetween(ctx context.Context, branchID int64, from, to string) (int64, error) {
	n, err := s.repo.CountMissingTaxInvoiceBetween(ctx, branchID, from, to)
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
	return s.OpenShipmentsFiltered(ctx, branchID, contracts.OpenShipmentFilter{Limit: limit})
}

// OpenShipmentsFiltered is OpenShipments with the work queue's search and
// order-date filters applied in SQL before the page cut (DL-02). A search term
// matches the order number OR a customer resolved through the party contract.
func (s *Service) OpenShipmentsFiltered(ctx context.Context, branchID int64, f contracts.OpenShipmentFilter) ([]*contracts.OpenShipment, error) {
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var partyIDs []int64
	if strings.TrimSpace(f.Search) != "" {
		ids, err := s.parties.SearchIDs(ctx, partycontracts.PartyCustomer, f.Search, listquery.MaxIn)
		if err != nil {
			return nil, err
		}
		partyIDs = ids
	}
	open, err := s.repo.OpenHeadersFiltered(ctx, branchID, f.Search, f.From, f.To, partyIDs, limit, f.Offset)
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
	// One grouped read for every counterparty on the page. Memoizing per
	// distinct party still cost a query per new party — batching costs one
	// for all of them, the same way ListOrders already does it (A3).
	nameIDs := make([]int64, 0, len(open))
	for _, o := range open {
		nameIDs = append(nameIDs, o.PartyID)
	}
	names, err := s.parties.NamesByIDs(ctx, nameIDs)
	if err != nil {
		return nil, err
	}
	for _, o := range open {
		o.Items = lines[o.OrderID]
		if o.Items == nil {
			o.Items = []*contracts.Item{}
		}
		if o.PartyID == 0 {
			o.PartyName = "Tunai"
			continue
		}
		o.PartyName = names[o.PartyID]
	}
	return open, nil
}

// SearchOrderIDs implements OpenShipmentSearcher (see the contract).
func (s *Service) SearchOrderIDs(ctx context.Context, branchID int64, term string, limit int) ([]int64, error) {
	if strings.TrimSpace(term) == "" {
		return nil, nil
	}
	if limit <= 0 || limit > listquery.MaxIn {
		limit = listquery.MaxIn
	}
	partyIDs, err := s.parties.SearchIDs(ctx, partycontracts.PartyCustomer, term, listquery.MaxIn)
	if err != nil {
		return nil, err
	}
	ids, err := s.repo.SearchOrderIDs(ctx, branchID, term, partyIDs, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return ids, nil
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

// SummariesByIDs resolves many orders to lightweight rows in one read (A3
// batch shape). Finance labels the gross-turnover working paper with it.
func (s *Service) SummariesByIDs(ctx context.Context, ids []int64) (map[int64]*contracts.OrderSummary, error) {
	out, err := s.repo.SummariesByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}

// CompletedSales implements the contract (see there).
func (s *Service) CompletedSales(ctx context.Context, branchID int64, from, to string) (int64, int64, error) {
	t, n, err := s.repo.CompletedSales(ctx, branchID, from, to)
	if err != nil {
		return 0, 0, apperror.Internal(err)
	}
	return t, n, nil
}

// CountOrders implements the contract (see there).
func (s *Service) CountOrders(ctx context.Context, branchID int64, from, to string) (int64, error) {
	n, err := s.repo.CountOrders(ctx, branchID, from, to)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return n, nil
}

// PriorityOrders ranks live orders (legacy score) and names their parties in
// one batch read.
func (s *Service) PriorityOrders(ctx context.Context, branchID int64, limit int) ([]*contracts.PriorityOrder, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	today := timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	rows, err := s.repo.PriorityOrders(ctx, branchID, today, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	partyIDs := make([]int64, 0, len(rows))
	for _, p := range rows {
		partyIDs = append(partyIDs, p.PartyID)
	}
	names, err := s.parties.NamesByIDs(ctx, partyIDs)
	if err != nil {
		return nil, err
	}
	for _, p := range rows {
		if p.PartyID == 0 {
			p.PartyName = "Tunai"
		} else {
			p.PartyName = names[p.PartyID]
		}
	}
	if rows == nil {
		rows = []*contracts.PriorityOrder{}
	}
	return rows, nil
}

// OpenOrderTotals implements the contract (see there).
func (s *Service) OpenOrderTotals(ctx context.Context, branchID, afterID int64, limit int) ([]contracts.OrderTotal, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	out, err := s.repo.OpenOrderTotals(ctx, branchID, afterID, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}

// ReportOrders implements the contract (see there).
func (s *Service) ReportOrders(ctx context.Context, branchID int64, status, from, to string, page, limit int) (*contracts.OrderReport, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, total, sum, err := s.repo.ReportOrders(ctx, branchID, status, from, to, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.PartyID)
	}
	names, err := s.parties.NamesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.PartyID == 0 {
			r.PartyName = "Tunai"
			continue
		}
		r.PartyName = names[r.PartyID]
	}
	if rows == nil {
		rows = []*contracts.OrderSummary{}
	}
	return &contracts.OrderReport{Rows: rows, Total: total, GrandTotal: sum}, nil
}
