package application

import (
	"context"
	"strings"

	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	"mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/shared/apperror"
)

// assignedUserErrors validates the person in charge (SL-09): 0 = none, else an
// active user.
func (s *Service) assignedUserErrors(ctx context.Context, userID int64) []apperror.FieldError {
	if userID == 0 {
		return nil
	}
	if u, err := s.users.GetByID(ctx, userID); err != nil || u == nil || u.Status != "active" {
		return []apperror.FieldError{{Field: "assignedUserId", Message: "penanggung jawab tidak ditemukan atau nonaktif"}}
	}
	return nil
}

// decorateUsers fills the created-by / assigned / credit-approved names of a
// page in ONE batch read (unknown users stay empty).
func (s *Service) decorateUsers(ctx context.Context, orders []*contracts.SalesOrder) error {
	if s.users == nil || len(orders) == 0 {
		return nil
	}
	seen := map[int64]bool{}
	var ids []int64
	add := func(id int64) {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, o := range orders {
		add(o.CreatedBy)
		add(o.AssignedUserID)
		add(o.CreditApprovedBy)
	}
	if len(ids) == 0 {
		return nil
	}
	names, err := s.users.NamesByIDs(ctx, ids)
	if err != nil {
		return apperror.Internal(err)
	}
	for _, o := range orders {
		o.CreatedByName = names[o.CreatedBy]
		o.AssignedUserName = names[o.AssignedUserID]
		o.CreditApprovedByName = names[o.CreditApprovedBy]
	}
	return nil
}

// paymentLabels is how the orders were paid and by whom, per order id.
type paymentLabels struct {
	methods map[int64]string // "Tunai, Transfer"
	payers  map[int64]string // names of the users who recorded the payments
}

var methodLabels = map[string]string{
	paymentcontracts.MethodCash:     "Tunai",
	paymentcontracts.MethodTransfer: "Transfer",
	paymentcontracts.MethodCheck:    "Cek/Giro",
}

// paymentLabelsFor resolves the payment methods and the recording users of the
// orders: one grouped payment read + one batch name read. Best-effort — a
// failure leaves the labels empty rather than failing the page.
func (s *Service) paymentLabelsFor(ctx context.Context, ids []int64) paymentLabels {
	out := paymentLabels{methods: map[int64]string{}, payers: map[int64]string{}}
	if s.dep.payments == nil || len(ids) == 0 {
		return out
	}
	info, err := s.dep.payments.PaymentInfoByOrders(ctx, paymentcontracts.OrderSales, ids)
	if err != nil || len(info) == 0 {
		return out
	}
	var userIDs []int64
	seen := map[int64]bool{}
	for _, in := range info {
		for _, u := range in.PayerIDs {
			if !seen[u] {
				seen[u] = true
				userIDs = append(userIDs, u)
			}
		}
	}
	var names map[int64]string
	if s.users != nil && len(userIDs) > 0 {
		names, _ = s.users.NamesByIDs(ctx, userIDs)
	}
	for id, in := range info {
		ms := make([]string, 0, len(in.Methods))
		for _, m := range in.Methods {
			if l, ok := methodLabels[m]; ok {
				m = l
			}
			ms = append(ms, m)
		}
		out.methods[id] = strings.Join(ms, ", ")
		var ps []string
		for _, u := range in.PayerIDs {
			if n := names[u]; n != "" {
				ps = append(ps, n)
			}
		}
		out.payers[id] = strings.Join(ps, ", ")
	}
	return out
}

// DecorateDetail adds the readable extras of a detail read: user names and the
// cashier (the user who recorded the payment, falling back to the order's
// creator for a POS sale not yet paid — SL-10). Two grouped reads at most.
func (s *Service) DecorateDetail(ctx context.Context, orders []*contracts.SalesOrder) error {
	if err := s.decorateUsers(ctx, orders); err != nil {
		return err
	}
	ids := make([]int64, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, o.ID)
	}
	labels := s.paymentLabelsFor(ctx, ids)
	for _, o := range orders {
		o.CashierName = labels.payers[o.ID]
		if o.CashierName == "" && o.Channel == contracts.ChannelPOS {
			o.CashierName = o.CreatedByName
		}
	}
	return nil
}
