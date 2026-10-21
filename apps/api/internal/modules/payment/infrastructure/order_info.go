package infrastructure

import (
	"context"

	"mini-erp/internal/modules/payment/contracts"
	"mini-erp/internal/modules/payment/infrastructure/sqlc"
)

// PaymentInfoByOrders groups the active cash payments of the given orders into
// distinct methods and recording users, in payment order (one grouped read).
func (r *Repository) PaymentInfoByOrders(ctx context.Context, orderType string, orderIDs []int64) (map[int64]contracts.OrderPaymentInfo, error) {
	out := map[int64]contracts.OrderPaymentInfo{}
	if len(orderIDs) == 0 {
		return out, nil
	}
	rows, err := r.q.OrderPaymentInfo(ctx, sqlc.OrderPaymentInfoParams{OrderType: orderType, OrderIds: orderIDs})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		info := out[row.OrderID]
		if !containsString(info.Methods, row.Method) {
			info.Methods = append(info.Methods, row.Method)
		}
		if row.CreatedBy.Valid && row.CreatedBy.Int64 > 0 {
			if !containsInt64(info.PayerIDs, row.CreatedBy.Int64) {
				info.PayerIDs = append(info.PayerIDs, row.CreatedBy.Int64)
			}
			info.LastPayerID = row.CreatedBy.Int64 // rows come in payment id order
		}
		out[row.OrderID] = info
	}
	return out, nil
}

func containsString(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func containsInt64(list []int64, v int64) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
