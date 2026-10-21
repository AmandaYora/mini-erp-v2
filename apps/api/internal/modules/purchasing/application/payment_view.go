package application

import (
	"context"

	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	"mini-erp/internal/modules/purchasing/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
)

// PaymentSummary is the paid/outstanding picture of one order (F2a-10d).
type PaymentSummary struct {
	// PaidAmount is money really paid to the supplier; ReturnOffset is the
	// part of the bill a confirmed return cleared without money moving (the
	// sales side's SL-03 split). Outstanding subtracts both.
	PaidAmount        int64  `json:"paidAmount"`
	ReturnOffset      int64  `json:"returnOffset"`
	OutstandingAmount int64  `json:"outstandingAmount"`
	PaymentStatus     string `json:"paymentStatus"` // unpaid | partial | paid
	EffectiveDueDate  string `json:"effectiveDueDate"`
	IsOverdue         bool   `json:"isOverdue"`
}

// PaymentSummaries computes the summaries of a page of purchase orders with ONE grouped
// payment read (never one query per order).
func (s *Service) PaymentSummaries(ctx context.Context, orders []*contracts.PurchaseOrder) (map[int64]PaymentSummary, error) {
	out := make(map[int64]PaymentSummary, len(orders))
	if len(orders) == 0 {
		return out, nil
	}
	ids := make([]int64, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, o.ID)
	}
	var paid, offset map[int64]int64
	if s.dep.payments != nil {
		var err error
		if paid, err = s.dep.payments.PaidForOrders(ctx, paymentcontracts.OrderPurchase, ids); err != nil {
			return nil, apperror.Internal(err)
		}
		if offset, err = s.dep.payments.OffsetForOrders(ctx, paymentcontracts.OrderPurchase, ids); err != nil {
			return nil, apperror.Internal(err)
		}
	}
	today := timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	for _, o := range orders {
		p := paid[o.ID]
		cash := max(0, p-offset[o.ID])
		sum := PaymentSummary{PaidAmount: cash, ReturnOffset: p - cash}
		switch {
		case o.Status == contracts.StatusCancelled || o.Status == contracts.StatusDraft:
			sum.PaymentStatus = "unpaid"
		default:
			sum.OutstandingAmount = o.GrandTotal - p
			if sum.OutstandingAmount < 0 {
				sum.OutstandingAmount = 0
			}
			switch {
			case sum.OutstandingAmount == 0:
				sum.PaymentStatus = "paid"
			case cash > 0:
				sum.PaymentStatus = "partial"
			default:
				sum.PaymentStatus = "unpaid"
			}
		}
		if o.PaymentTerms == "net" && o.DueDate != "" {
			sum.EffectiveDueDate = o.DueDate[:min(10, len(o.DueDate))]
			sum.IsOverdue = sum.OutstandingAmount > 0 && sum.EffectiveDueDate < today
		}
		out[o.ID] = sum
	}
	return out, nil
}
