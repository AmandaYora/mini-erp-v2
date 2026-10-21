package application

import (
	"testing"

	paymentcontracts "mini-erp/internal/modules/payment/contracts"
)

// F6: the journal memo follows the money's direction on both sides — a
// customer receipt is not a refund, and a supplier refund is not a payment.
func TestClassifyPaymentMemoFollowsDirection(t *testing.T) {
	cases := []struct {
		name, direction, orderType, method string
		memo, cashKey, counterKey          string
	}{
		{"terima dari pelanggan", paymentcontracts.DirectionIn, paymentcontracts.OrderSales, paymentcontracts.MethodCash,
			"Penerimaan pelanggan", "cash", "receivable"},
		{"tagihan tukar retur (charge)", paymentcontracts.DirectionIn, paymentcontracts.OrderSalesReturn, paymentcontracts.MethodTransfer,
			"Penerimaan pelanggan", "bank", "receivable"},
		{"refund ke pelanggan", paymentcontracts.DirectionOut, paymentcontracts.OrderSalesReturn, paymentcontracts.MethodCash,
			"Refund pelanggan", "cash", "receivable"},
		{"bayar pemasok", paymentcontracts.DirectionOut, paymentcontracts.OrderPurchase, paymentcontracts.MethodTransfer,
			"Bayar", "bank", "payable"},
		{"refund dari pemasok", paymentcontracts.DirectionIn, paymentcontracts.OrderPurchaseReturn, paymentcontracts.MethodCash,
			"Refund pemasok", "cash", "payable"},
		{"pakai saldo pelanggan", paymentcontracts.DirectionIn, paymentcontracts.OrderSales, paymentcontracts.MethodAdvance,
			"Pakai saldo uang muka", "customer_advance", "receivable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &paymentcontracts.Payment{Direction: tc.direction, Method: tc.method,
				Allocations: []*paymentcontracts.Allocation{{OrderType: tc.orderType, OrderID: 1, Amount: 1000}}}
			k, err := classifyPayment(p)
			if err != nil {
				t.Fatalf("classify: %v", err)
			}
			if k.memo != tc.memo || k.cashKey != tc.cashKey || k.counterKey != tc.counterKey {
				t.Fatalf("= memo %q cash %q counter %q, mau %q %q %q", k.memo, k.cashKey, k.counterKey, tc.memo, tc.cashKey, tc.counterKey)
			}
		})
	}
}
