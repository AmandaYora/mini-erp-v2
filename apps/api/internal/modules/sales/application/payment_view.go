package application

import (
	"context"

	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	"mini-erp/internal/modules/sales/contracts"
	salesreturncontracts "mini-erp/internal/modules/salesreturn/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
)

// Payment status values of an order (list filter + view).
const (
	PayUnpaid  = "unpaid"
	PayPartial = "partial"
	PayPaid    = "paid"
	// PayOverdue is a filter value only: a net order past its due date that
	// still has a balance. In the view it is the isOverdue flag.
	PayOverdue = "overdue"
	// PayReceived is a filter value only: money was received on the order
	// (partial or paid) — data-health A12 lists those not fully shipped.
	PayReceived = "received"
)

// PaymentSummary is the paid/outstanding picture of one order (F2a-10d),
// net of the confirmed returns (SL-03, legacy buildFinancialSummary):
//
//	netTotal   = grand total + return adjustment (a return subtracts, an
//	             exchange adds replacement - returned) — equals what the books
//	             carry for the order;
//	paidAmount = cash really kept: active allocations minus the automatic
//	             return offset (already inside the payment module's paid),
//	             minus refunds and value kept as customer balance;
//	outstanding = max(0, order total net of reductions - paid);
//	refundDue   = what is owed BACK to the customer (paid more than the net).
//
// The exchange surcharge is collected on the return document, so it is shown
// as exchangeChargeDue and kept out of the order's own outstanding (the order
// payment form can only take what the payment module accepts).
type PaymentSummary struct {
	PaidAmount        int64  `json:"paidAmount"`
	OutstandingAmount int64  `json:"outstandingAmount"`
	PaymentStatus     string `json:"paymentStatus"` // unpaid | partial | paid
	EffectiveDueDate  string `json:"effectiveDueDate"`
	IsOverdue         bool   `json:"isOverdue"`
	// Net-of-returns picture. For an order without returns netTotal equals
	// originalTotal and the rest are zero.
	OriginalTotal     int64 `json:"originalTotal"`
	ReturnAdjustment  int64 `json:"returnAdjustment"`
	NetTotal          int64 `json:"netTotal"`
	ReturnOffset      int64 `json:"returnOffset"`
	ReturnRefunded    int64 `json:"returnRefunded"`
	ReturnCredit      int64 `json:"returnCredit"`
	RefundDue         int64 `json:"refundDue"`
	ExchangeChargeDue int64 `json:"exchangeChargeDue"`
	// FinancialStatus: settled | overdue | partial | open (draft/cancelled: open).
	FinancialStatus string `json:"financialStatus"`
}

// PaymentSummaries computes the summaries of a page of orders with ONE grouped
// payment read and the two grouped return-effect reads (never one query per
// order).
func (s *Service) PaymentSummaries(ctx context.Context, orders []*contracts.SalesOrder) (map[int64]PaymentSummary, error) {
	out := make(map[int64]PaymentSummary, len(orders))
	if len(orders) == 0 {
		return out, nil
	}
	ids := make([]int64, 0, len(orders))
	live := make([]int64, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, o.ID)
		if o.Status == contracts.StatusConfirmed || o.Status == contracts.StatusCompleted {
			live = append(live, o.ID)
		}
	}
	var paid map[int64]int64
	if s.dep.payments != nil {
		var err error
		if paid, err = s.dep.payments.PaidForOrders(ctx, paymentcontracts.OrderSales, ids); err != nil {
			return nil, apperror.Internal(err)
		}
	}
	var effects map[int64]salesreturncontracts.OrderEffect
	if s.dep.returns != nil && len(live) > 0 {
		var err error
		if effects, err = s.dep.returns.OrderEffects(ctx, orders[0].BranchID, live); err != nil {
			return nil, apperror.Internal(err)
		}
	}
	today := timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	for _, o := range orders {
		out[o.ID] = summarize(o, paid[o.ID], effects[o.ID], today)
	}
	return out, nil
}

// summarize is the pure money picture of one order (see PaymentSummary).
func summarize(o *contracts.SalesOrder, paidAllocated int64, e salesreturncontracts.OrderEffect, today string) PaymentSummary {
	sum := PaymentSummary{OriginalTotal: o.GrandTotal, NetTotal: o.GrandTotal, PaidAmount: paidAllocated, FinancialStatus: "open"}
	if o.Status == contracts.StatusCancelled || o.Status == contracts.StatusDraft {
		sum.PaymentStatus = PayUnpaid
		sum.NetTotal = o.GrandTotal
		return sum
	}
	sum.ReturnAdjustment = e.Adjustment
	sum.NetTotal = o.GrandTotal + e.Adjustment
	if sum.NetTotal < 0 {
		sum.NetTotal = 0
	}
	sum.ReturnOffset, sum.ReturnRefunded, sum.ReturnCredit = e.Offset, e.Refunded, e.Credit
	// Cash really kept on the order: the offset leg lives inside the allocations
	// but moved no cash; refunds and balances kept for the customer left it.
	kept := paidAllocated - e.Offset - e.Refunded - e.Credit
	if kept < 0 {
		kept = 0
	}
	sum.PaidAmount = kept
	// Reductions only: a surcharge is collected on the return document.
	reduced := o.GrandTotal
	if e.Adjustment < 0 {
		reduced += e.Adjustment
	}
	if reduced < 0 {
		reduced = 0
	}
	sum.OutstandingAmount = reduced - kept
	if sum.OutstandingAmount < 0 {
		sum.RefundDue = -sum.OutstandingAmount
		sum.OutstandingAmount = 0
	}
	if e.Adjustment > 0 {
		if due := e.Adjustment - e.Collected; due > 0 {
			sum.ExchangeChargeDue = due
		}
	}
	switch {
	case sum.OutstandingAmount == 0:
		sum.PaymentStatus = PayPaid
	case kept > 0:
		sum.PaymentStatus = PayPartial
	default:
		sum.PaymentStatus = PayUnpaid
	}
	if o.PaymentTerms == "net" && o.DueDate != "" {
		sum.EffectiveDueDate = trimDate(o.DueDate)
		sum.IsOverdue = sum.OutstandingAmount > 0 && sum.EffectiveDueDate < today
	}
	switch {
	case sum.OutstandingAmount == 0:
		sum.FinancialStatus = "settled"
	case sum.IsOverdue:
		sum.FinancialStatus = "overdue"
	case kept > 0:
		sum.FinancialStatus = "partial"
	}
	return sum
}
