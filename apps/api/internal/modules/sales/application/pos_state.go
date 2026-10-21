package application

import (
	"context"

	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	"mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/shared/apperror"
)

// Next steps of a POS checkout, in order. The client resumes from NextStep
// after a reload or a failed call instead of improvising "susulan" documents.
const (
	StepConfirm         = "confirm"
	StepCreateDelivery  = "create_delivery"
	StepConfirmDelivery = "confirm_delivery"
	StepPay             = "pay"
	StepDone            = "done"
	StepCancelled       = "cancelled"
	// StepFinishFromDeliveries: an order built from SJ Langsung (ADR-0022) that
	// is confirmed but not completed yet — repeat POST /sales-orders/
	// from-deliveries with the same clientRef to resume the orchestration.
	StepFinishFromDeliveries = "finish_from_deliveries"
)

// DeliveryRef is a shipment of the order as the state reader sees it.
type DeliveryRef struct {
	ID     int64  `json:"id"`
	Number string `json:"number"`
	Status string `json:"status"`
}

// OrderState is where a client cart stands on the server.
type OrderState struct {
	Order       *contracts.SalesOrder `json:"order"`
	Deliveries  []DeliveryRef         `json:"deliveries"`
	PaidAmount  int64                 `json:"paidAmount"`
	Outstanding int64                 `json:"outstanding"`
	NextStep    string                `json:"nextStep"`
}

// StateByClientRef reads the checkout state of the cart identified by ref.
// A missing order is NotFound (the cart never reached the server).
func (s *Service) StateByClientRef(ctx context.Context, branchID int64, ref string) (*OrderState, error) {
	o, err := s.repo.OrderByClientRef(ctx, branchID, ref)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if o == nil {
		return nil, apperror.NotFound("Order")
	}
	o, err = s.enrich(ctx, o)
	if err != nil {
		return nil, err
	}
	st := &OrderState{Order: o, Deliveries: []DeliveryRef{}}
	var notes []*deliverycontracts.DeliveryNote
	if s.dep.deliveries != nil {
		if notes, err = s.dep.deliveries.ListBySO(ctx, branchID, o.ID); err != nil {
			return nil, apperror.Internal(err)
		}
	}
	for _, n := range notes {
		if n.DocumentKind != "" && n.DocumentKind != deliverycontracts.DocumentKindOrder {
			continue
		}
		st.Deliveries = append(st.Deliveries, DeliveryRef{ID: n.ID, Number: n.Number, Status: n.Status})
	}
	if s.dep.payments != nil {
		paid, err := s.dep.payments.PaidForOrders(ctx, paymentcontracts.OrderSales, []int64{o.ID})
		if err != nil {
			return nil, apperror.Internal(err)
		}
		st.PaidAmount = paid[o.ID]
	}
	st.Outstanding = o.GrandTotal - st.PaidAmount
	if st.Outstanding < 0 {
		st.Outstanding = 0
	}
	st.NextStep = nextStep(o, st)
	return st, nil
}

func nextStep(o *contracts.SalesOrder, st *OrderState) string {
	switch o.Status {
	case contracts.StatusCancelled:
		return StepCancelled
	case contracts.StatusDraft:
		return StepConfirm
	case contracts.StatusConfirmed:
		if o.Origin == contracts.OriginDelivery {
			return StepFinishFromDeliveries
		}
		hasDraft := false
		for _, d := range st.Deliveries {
			if d.Status == deliverycontracts.StatusDraft {
				hasDraft = true
			}
		}
		if hasDraft {
			return StepConfirmDelivery
		}
		return StepCreateDelivery
	default: // completed
		if st.Outstanding > 0 && o.PaymentTerms != "net" {
			return StepPay
		}
		return StepDone
	}
}
