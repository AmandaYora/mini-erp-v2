package application

import (
	"context"
	"fmt"
	"sort"

	"mini-erp/internal/shared/apperror"
)

// GrossTurnoverCeiling is the PP23 small-business gross-turnover ceiling
// (peredaran bruto) in integer rupiah: Rp4.800.000.000 per FISCAL YEAR.
//
// It is a per-TAXPAYER figure, so the basis below is computed company-wide
// and year-to-date. Applying it per branch, or to a single month, would let
// real turnover pass the ceiling without anything noticing — the legacy
// system applied it to whatever date range was asked for, which made a
// monthly export silently identical to the uncapped one.
const GrossTurnoverCeiling int64 = 4_800_000_000

// TurnoverOrder is one sales order's position against the ceiling.
type TurnoverOrder struct {
	OrderID    int64  `json:"orderId"`
	Number     string `json:"number"`
	PartyName  string `json:"partyName"`
	FirstDate  string `json:"firstDate"`
	Revenue    int64  `json:"revenue"`
	Cumulative int64  `json:"cumulative"`
	Included   bool   `json:"included"`
}

// TurnoverBasis is the computed ceiling position for one fiscal year up to a
// cutoff date, plus the audit trail that explains it.
//
// Nothing here is stored. The basis is derived from the books every time it
// is asked for, so it can never go stale against a late posting or a
// reversal — the same reason the posting queue has no table.
type TurnoverBasis struct {
	Year             int             `json:"year"`
	CutoffDate       string          `json:"cutoffDate"`
	Ceiling          int64           `json:"ceiling"`
	IncludedTurnover int64           `json:"includedTurnover"`
	ExcludedTurnover int64           `json:"excludedTurnover"`
	Orders           []TurnoverOrder `json:"orders"`
	// StraddlingPayments lists payment documents that settle both an
	// included and an excluded order. They are KEPT (see excludedEntries),
	// and surfaced here because they are the one place the capped view
	// cannot be made exact — a reviewer should see them, not discover them.
	StraddlingPayments []string `json:"straddlingPayments"`

	excludedEntries map[int64]bool
}

// IsExcluded reports whether a journal entry falls outside the ceiling.
func (b *TurnoverBasis) IsExcluded(entryID int64) bool { return b.excludedEntries[entryID] }

// ExcludedCount returns how many journal entries the ceiling removes.
func (b *TurnoverBasis) ExcludedCount() int { return len(b.excludedEntries) }

// ExcludedEntryIDs lists the removed journal entries. Every report in the
// limited working paper is built from THIS one list, so a dropped order
// cannot survive in one sheet while vanishing from another.
func (b *TurnoverBasis) ExcludedEntryIDs() []int64 {
	out := make([]int64, 0, len(b.excludedEntries))
	for id := range b.excludedEntries {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// entryOrders is the attribution of one journal entry to the sales orders
// whose economics it carries.
type entryOrders struct {
	entryID int64
	date    string
	revenue int64
	orders  []int64
}

// GrossTurnoverBasis computes the ceiling position for the fiscal year of
// (year, month), accumulated from 1 January through the end of that month.
//
// Query cost is constant in the size of the range: one journal read plus at
// most four grouped contract reads (deliveries, returns, payments, order
// labels). No per-order query — that N+1 is what the batch contracts exist
// to avoid.
func (s *Service) GrossTurnoverBasis(ctx context.Context, year, month int) (*TurnoverBasis, error) {
	if !validYearMonth(year, month) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	// Bounds carry the TIME component. entry_date is DATETIME, so comparing
	// it against a bare "YYYY-MM-DD" makes MySQL read the bound as midnight
	// and silently drop everything booked later on the last day.
	from := fmt.Sprintf("%04d-01-01 00:00:00", year)
	to := monthEnd(year, month)
	cutoff := to[:10]

	rows, err := s.repo.TurnoverEntries(ctx, from, to)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	// Bucket source ids per document type so each type resolves in ONE read.
	var deliveryIDs, returnIDs, paymentIDs []int64
	for _, r := range rows {
		switch r.SourceType {
		case SourceDelivery:
			deliveryIDs = append(deliveryIDs, r.SourceID)
		case SourceSalesReturn:
			returnIDs = append(returnIDs, r.SourceID)
		case SourcePayment:
			paymentIDs = append(paymentIDs, r.SourceID)
		}
	}
	byDelivery, err := s.delivery.OrderIDsByNotes(ctx, deliveryIDs)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	byReturn, err := s.salesReturns.OrderIDsByReturns(ctx, returnIDs)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	byPayment, err := s.payment.OrderIDsByPayments(ctx, paymentIDs)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	// Attribute every entry to the order(s) behind it, and foot revenue per
	// order. A sales return arrives as NEGATIVE revenue, so it reduces the
	// order's turnover exactly as peredaran bruto expects.
	attributed := make([]entryOrders, 0, len(rows))
	revenueByOrder := map[int64]int64{}
	firstDate := map[int64]string{}
	for _, r := range rows {
		var orders []int64
		switch r.SourceType {
		case SourceDelivery:
			if id := byDelivery[r.SourceID]; id != 0 {
				orders = []int64{id}
			}
		case SourceSalesReturn:
			if id := byReturn[r.SourceID]; id != 0 {
				orders = []int64{id}
			}
		case SourcePayment:
			orders = byPayment[r.SourceID]
		}
		if len(orders) == 0 {
			// Purchases, expenses, opening, manual entries: no sales order
			// behind them, so the ceiling never touches them.
			continue
		}
		attributed = append(attributed, entryOrders{
			entryID: r.EntryID, date: r.Date, revenue: r.Revenue, orders: orders,
		})
		// Revenue is attributed only when the entry belongs to ONE order.
		// A payment settling several orders carries no revenue anyway, so
		// there is nothing to split and no arbitrary allocation to invent.
		if len(orders) == 1 {
			id := orders[0]
			revenueByOrder[id] += r.Revenue
			if cur, ok := firstDate[id]; !ok || r.Date < cur {
				firstDate[id] = r.Date
			}
		}
	}

	// Chronological order, ties broken by id so the result is deterministic
	// and reproducible. Legacy shuffled by CRC32(order_id) instead; for a
	// YEAR-TO-DATE ceiling that is the wrong shape — turnover accumulates
	// through the year, so the orders that breach the ceiling are the LATER
	// ones, not a pseudo-random sample nobody can explain to an auditor.
	ids := make([]int64, 0, len(revenueByOrder))
	for id := range revenueByOrder {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		di, dj := firstDate[ids[i]], firstDate[ids[j]]
		if di != dj {
			return di < dj
		}
		return ids[i] < ids[j]
	})

	labels, err := s.sales.SummariesByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	basis := &TurnoverBasis{
		Year: year, CutoffDate: cutoff, Ceiling: GrossTurnoverCeiling,
		Orders: make([]TurnoverOrder, 0, len(ids)), excludedEntries: map[int64]bool{},
	}
	excludedOrders := map[int64]bool{}
	var running int64
	for _, id := range ids {
		rev := revenueByOrder[id]
		row := TurnoverOrder{
			OrderID: id, FirstDate: firstDate[id], Revenue: rev, Included: true,
		}
		if sum, ok := labels[id]; ok && sum != nil {
			row.Number, row.PartyName = sum.Number, sum.PartyName
		}
		switch {
		case rev <= 0:
			// Fully returned or zero-value: consumes none of the ceiling,
			// and dropping it would remove a return without its sale.
			row.Cumulative = running
		case running+rev <= GrossTurnoverCeiling:
			running += rev
			row.Cumulative = running
		default:
			row.Included = false
			row.Cumulative = running
			excludedOrders[id] = true
			basis.ExcludedTurnover += rev
		}
		basis.Orders = append(basis.Orders, row)
	}
	basis.IncludedTurnover = running

	// An entry leaves the capped view only when EVERY order it carries is
	// excluded. A payment that settles one excluded and one included order
	// stays: removing it would strip cash from an order that is still in
	// the picture. Those documents are reported instead of being silently
	// resolved either way.
	straddling := map[string]bool{}
	for _, a := range attributed {
		all, any := true, false
		for _, id := range a.orders {
			if excludedOrders[id] {
				any = true
			} else {
				all = false
			}
		}
		if all && any {
			basis.excludedEntries[a.entryID] = true
			continue
		}
		if any {
			straddling[fmt.Sprintf("entry:%d", a.entryID)] = true
		}
	}
	for k := range straddling {
		basis.StraddlingPayments = append(basis.StraddlingPayments, k)
	}
	sort.Strings(basis.StraddlingPayments)
	return basis, nil
}

// ceilingHeadroom reports what is still available under the ceiling.
func (b *TurnoverBasis) ceilingHeadroom() int64 {
	if b.IncludedTurnover >= b.Ceiling {
		return 0
	}
	return b.Ceiling - b.IncludedTurnover
}

// summaryRow is one labelled figure on the working paper's first sheet.
type summaryRow struct {
	Label string
	Value string
}
