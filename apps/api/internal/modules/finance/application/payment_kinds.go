package application

import (
	"mini-erp/internal/modules/finance/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	"mini-erp/internal/shared/apperror"
)

// paymentKind says which accounts one payment touches (F2b-1).
//
// The cash-side account follows the method; the counter-account follows what
// the payment SETTLES, not just its direction:
//
//	sales / sales_return       -> receivable  (customer side)
//	purchase / purchase_return -> payable     (supplier side)
//
// So a refund to a customer (out + sales_return) debits receivable and credits
// cash, a refund from a supplier (in + purchase_return) debits cash and credits
// payable, and an exchange top-up (in + sales_return, a "charge") debits cash
// and credits receivable. Offsets (method "offset") never reach the books: the
// return document already credited receivable / debited payable.
type paymentKind struct {
	cashKey    string
	counterKey string
	memo       string
	skip       bool
	// advance: the party's saldo account pays the bill (method "advance",
	// N-41); the saldo leg then carries the party like the receivable leg.
	advance bool
}

func classifyPayment(p *paymentcontracts.Payment) (paymentKind, error) {
	if p.Method == paymentcontracts.MethodOffset {
		return paymentKind{skip: true}, nil
	}
	var k paymentKind
	switch p.Method {
	case paymentcontracts.MethodCash:
		k.cashKey = "cash"
	case paymentcontracts.MethodTransfer, paymentcontracts.MethodCheck:
		k.cashKey = "bank"
	case paymentcontracts.MethodAdvance:
		k.advance = true // the saldo account is chosen by the side below
	default:
		return k, apperror.Conflict("Metode pembayaran tak dikenal")
	}
	if len(p.Allocations) == 0 {
		return k, apperror.Conflict("Pembayaran tanpa alokasi tidak dapat dijurnal")
	}
	family := ""
	for _, a := range p.Allocations {
		f := ""
		switch a.OrderType {
		case paymentcontracts.OrderSales, paymentcontracts.OrderSalesReturn:
			f = "receivable"
		case paymentcontracts.OrderPurchase, paymentcontracts.OrderPurchaseReturn:
			f = "payable"
		default:
			return k, apperror.Conflict("Jenis alokasi pembayaran tak dikenal")
		}
		if family != "" && family != f {
			return k, apperror.Conflict("Pembayaran mencampur sisi pelanggan dan pemasok")
		}
		family = f
	}
	k.counterKey = family
	if k.advance {
		k.cashKey = "customer_advance"
		if family == "payable" {
			k.cashKey = "supplier_advance"
		}
	}
	// Direction must make sense for the family: customers pay in / are refunded
	// out; suppliers are paid out / refund in. Either direction is allowed per
	// family (that is exactly refund vs normal), so only the memo differs —
	// and it must follow the direction on BOTH sides (F6: every customer
	// receipt used to read "Refund pelanggan").
	out := p.Direction == paymentcontracts.DirectionOut
	switch {
	case k.advance:
		k.memo = "Pakai saldo uang muka"
	case family == "receivable" && out:
		k.memo = "Refund pelanggan"
	case family == "receivable":
		k.memo = "Penerimaan pelanggan"
	case out:
		k.memo = "Bayar"
	default:
		k.memo = "Refund pemasok"
	}
	_ = contracts.AccountAsset
	return k, nil
}
