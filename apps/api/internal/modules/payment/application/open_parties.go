package application

import (
	"context"
	"errors"
	"sort"
	"strings"

	partycontracts "mini-erp/internal/modules/party/contracts"
	"mini-erp/internal/modules/payment/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/listquery"
)

// errOpsNotWired is a programmer error: the order clients handed to NewService
// must also implement the sales / purchasing ops contracts (the real services do).
var errOpsNotWired = errors.New("payment: sales/purchasing ops contracts not wired")

// Sides of the open-parties list (PY-01).
const (
	SideReceivable = "receivable" // customers who owe us (sales orders)
	SidePayable    = "payable"    // suppliers we owe (purchase orders)
)

// openPartyScanPage is the page size of the order scan; the scan is bounded by
// openPartyMaxPages (50k documents), far above one branch's open book.
const (
	openPartyScanPage = 1000
	openPartyMaxPages = 50
)

// OpenParty is one party that still has an unpaid balance (legacy "daftar
// pihak berpiutang/berhutang"): how many unpaid documents it has, the oldest
// one and what it owes in total.
type OpenParty struct {
	PartyID     int64  `json:"partyId"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Side        string `json:"side"`
	OrderCount  int    `json:"orderCount"`
	OldestDate  string `json:"oldestDate"`
	Outstanding int64  `json:"outstanding"`
}

// OpenPartiesResult is one page plus the figures of the WHOLE filtered set.
type OpenPartiesResult struct {
	Items            []OpenParty `json:"items"`
	Total            int64       `json:"total"`
	TotalOutstanding int64       `json:"totalOutstanding"`
}

// OpenParties lists the branch's parties with unpaid documents, oldest first
// (due date, else order date). It is derived from the books of the owning
// modules through their contracts: confirmed/completed orders paged by cursor
// (sales / purchasing ops), minus active allocations read in ONE grouped query
// per page — never a query per order or per party. A text term is resolved to
// party ids first (party contract), so only matching parties are aggregated.
// Walk-in orders (party 0) carry no debtor and never appear.
//
// Query count: 1 party search (only with a term) + per scanned page of 1000
// orders (1 orders read + 1 allocation sum) + 1 party-label read for the page.
func (s *Service) OpenParties(ctx context.Context, branchID int64, side, search string, page, limit int) (*OpenPartiesResult, error) {
	var (
		orderType string
		partyType string
	)
	switch side {
	case SideReceivable:
		orderType, partyType = contracts.OrderSales, partycontracts.PartyCustomer
	case SidePayable:
		orderType, partyType = contracts.OrderPurchase, partycontracts.PartySupplier
	default:
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "side", Message: "harus receivable atau payable"}})
	}
	var only map[int64]bool
	if term := strings.TrimSpace(search); term != "" {
		ids, err := s.parties.SearchIDs(ctx, partyType, term, listquery.MaxIn)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		only = make(map[int64]bool, len(ids))
		for _, id := range ids {
			only[id] = true
		}
	}

	agg := map[int64]*OpenParty{}
	var after int64
	for p := 0; p < openPartyMaxPages; p++ {
		rows, err := s.openOrderPage(ctx, side, branchID, after)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			break
		}
		after = rows[len(rows)-1].id
		ids := make([]int64, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.id)
		}
		paid, err := s.repo.PaidForOrders(ctx, orderType, ids)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		for _, r := range rows {
			if r.partyID == 0 || (only != nil && !only[r.partyID]) {
				continue
			}
			out := r.total - paid[r.id]
			if out <= 0 {
				continue
			}
			op := agg[r.partyID]
			if op == nil {
				op = &OpenParty{PartyID: r.partyID, Side: side}
				agg[r.partyID] = op
			}
			op.OrderCount++
			op.Outstanding += out
			day := r.due
			if day == "" {
				day = r.date
			}
			if op.OldestDate == "" || (day != "" && day < op.OldestDate) {
				op.OldestDate = day
			}
		}
		if len(rows) < openPartyScanPage {
			break
		}
	}

	all := make([]*OpenParty, 0, len(agg))
	var totalOutstanding int64
	for _, op := range agg {
		all = append(all, op)
		totalOutstanding += op.Outstanding
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].OldestDate != all[j].OldestDate {
			return all[i].OldestDate < all[j].OldestDate
		}
		return all[i].PartyID < all[j].PartyID
	})

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	from := (page - 1) * limit
	if from > len(all) {
		from = len(all)
	}
	to := from + limit
	if to > len(all) {
		to = len(all)
	}
	pageRows := all[from:to]
	ids := make([]int64, 0, len(pageRows))
	for _, op := range pageRows {
		ids = append(ids, op.PartyID)
	}
	labels, err := s.parties.LabelsByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	items := make([]OpenParty, 0, len(pageRows))
	for _, op := range pageRows {
		if l, ok := labels[op.PartyID]; ok {
			op.Code, op.Name = l.Code, l.Name
		}
		items = append(items, *op)
	}
	return &OpenPartiesResult{Items: items, Total: int64(len(all)), TotalOutstanding: totalOutstanding}, nil
}

// openOrder is one billable document of the scan, reduced to what the list needs.
type openOrder struct {
	id, partyID, total int64
	date, due          string
}

// openOrderPage reads one cursor page of the owning module's billable orders.
// The concrete sales / purchasing services implement the ops contracts next to
// their order clients; a stub that does not is a wiring error, not a 500 loop.
func (s *Service) openOrderPage(ctx context.Context, side string, branchID, after int64) ([]openOrder, error) {
	if side == SideReceivable {
		ops, ok := s.sales.(salescontracts.SalesOpsClient)
		if !ok {
			return nil, apperror.Internal(errOpsNotWired)
		}
		page, err := ops.OpenOrderTotals(ctx, branchID, after, openPartyScanPage)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		out := make([]openOrder, 0, len(page))
		for _, r := range page {
			out = append(out, openOrder{r.ID, r.PartyID, r.GrandTotal, r.OrderDate, r.DueDate})
		}
		return out, nil
	}
	ops, ok := s.purchasing.(purchasingcontracts.PurchasingOpsClient)
	if !ok {
		return nil, apperror.Internal(errOpsNotWired)
	}
	page, err := ops.OpenOrderTotals(ctx, branchID, after, openPartyScanPage)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]openOrder, 0, len(page))
	for _, r := range page {
		out = append(out, openOrder{r.ID, r.PartyID, r.GrandTotal, r.OrderDate, r.DueDate})
	}
	return out, nil
}
