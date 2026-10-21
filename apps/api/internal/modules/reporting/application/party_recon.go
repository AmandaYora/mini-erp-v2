package application

import (
	"context"
	"errors"
	"sort"

	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	"mini-erp/internal/shared/apperror"
)

// errPaymentsNotWired is a programmer error: the party reconciliation needs
// the payment module's party balance.
var errPaymentsNotWired = errors.New("reporting: payment client not wired")

// SetPayments wires the payment module's party balance (the documents' view
// of what each party owes) for the party reconciliation.
func (s *Service) SetPayments(payments paymentcontracts.PaymentClient) { s.payments = payments }

// PartyReconRow is one party whose books and documents disagree.
type PartyReconRow struct {
	PartyID   int64  `json:"partyId"`
	PartyName string `json:"partyName"`
	// Books is the control-account balance (Piutang + Uang Muka Pelanggan, or
	// Hutang + Uang Muka Supplier); Documents is the payment module's party
	// balance (orders − payments − returns + refunds). Difference = Books −
	// Documents.
	Books      int64 `json:"books"`
	Documents  int64 `json:"documents"`
	Difference int64 `json:"difference"`
	// OpenOrders > 0: the party still has confirmed orders. The books bill per
	// shipment/receipt while the documents bill the whole order, so the gap
	// may be timing (goods not yet shipped/received, a document dated ahead).
	OpenOrders int  `json:"openOrders"`
	Timing     bool `json:"timing"`
}

// PartyRecon is the reconciliation of one side.
type PartyRecon struct {
	Side            string          `json:"side"`
	PartiesChecked  int             `json:"partiesChecked"`
	Matched         int             `json:"matched"`
	Rows            []PartyReconRow `json:"rows"`
	TotalDifference int64           `json:"totalDifference"`
	// Unexplained counts rows of parties without open orders: there the books
	// and the documents must agree exactly, so every such row is an input or
	// posting error to investigate (and correct through the web).
	Unexplained int `json:"unexplained"`
}

// PartyReconciliation compares, party by party, what the books hold on the
// control accounts with what the documents say is owed (ADR-0021/0023). It is
// the standing check for wrong inputs that reach the books unevenly: a
// payment posted to the wrong party, a cancelled document whose journal was
// left behind, a manual journal on a control account.
//
// Query count: 2 finance footings per control account + 1 order scan per
// 1000 orders + one party-balance read per party (bounded by the branch's
// parties; an on-demand operator report, not a list page).
func (s *Service) PartyReconciliation(ctx context.Context, branchID int64, side string) (*PartyRecon, error) {
	if s.payments == nil {
		return nil, apperror.Internal(errPaymentsNotWired)
	}
	books, err := s.finance.PartyControlBalances(ctx, branchID, side)
	if err != nil {
		return nil, err
	}
	withOrders, err := s.payments.PartiesWithOrders(ctx, branchID, side)
	if err != nil {
		return nil, err
	}
	seen := map[int64]bool{}
	var parties []int64
	for _, id := range withOrders {
		if !seen[id] {
			seen[id] = true
			parties = append(parties, id)
		}
	}
	for id, v := range books {
		if v != 0 && id != 0 && !seen[id] {
			seen[id] = true
			parties = append(parties, id)
		}
	}
	sort.Slice(parties, func(i, j int) bool { return parties[i] < parties[j] })
	out := &PartyRecon{Side: side, Rows: []PartyReconRow{}}
	for _, id := range parties {
		bal, err := s.payments.GetPartyBalance(ctx, branchID, id)
		if err != nil {
			var ae *apperror.AppError
			if errors.As(err, &ae) && ae.Code == apperror.CodeNotFound {
				continue
			}
			return nil, err
		}
		out.PartiesChecked++
		diff := books[id] - bal.Outstanding
		if diff == 0 {
			out.Matched++
			continue
		}
		row := PartyReconRow{PartyID: id, PartyName: bal.PartyName, Books: books[id], Documents: bal.Outstanding,
			Difference: diff, OpenOrders: bal.OpenOrders, Timing: bal.OpenOrders > 0}
		if !row.Timing {
			out.Unexplained++
		}
		out.TotalDifference += diff
		out.Rows = append(out.Rows, row)
	}
	sort.SliceStable(out.Rows, func(i, j int) bool {
		if out.Rows[i].Timing != out.Rows[j].Timing {
			return !out.Rows[i].Timing
		}
		return abs64(out.Rows[i].Difference) > abs64(out.Rows[j].Difference)
	})
	return out, nil
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
