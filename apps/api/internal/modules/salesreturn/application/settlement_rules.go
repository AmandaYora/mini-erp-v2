package application

import (
	"strconv"

	"mini-erp/internal/modules/salesreturn/contracts"
	"mini-erp/internal/shared/apperror"
)

// settlementPosition is the money picture of one return, the single source of
// the settlement rules (K-04c): AddSettlement validates against it, the detail
// view and the create-time preview (K-04d) show it, so the screens can never
// offer what the server would refuse.
//
// Direction of the difference decides what is allowed (legacy
// sales-return.service.ts resolveSettlement):
//   - a plain return, or an exchange that costs LESS than the returned goods,
//     leaves value owed TO the customer (Owed): it first reduces the open bill
//     (Reduction, automatic) and only the rest (external) can be refunded in
//     cash or kept as the customer's balance (customer_credit);
//   - an exchange that costs MORE leaves value the customer owes (Charge): it
//     can only be collected (collect_payment), never refunded.
type settlementPosition struct {
	Owed      int64 // value owed to the customer
	Charge    int64 // value the customer owes (exchange costing more)
	Reduction int64 // part already offset against the order's open bill
	// ExternalSettled sums refund + customer_credit rows; Collected sums
	// collect_payment rows. Automatic reduce_receivable rows are the offset
	// itself (Reduction) and never count as external.
	ExternalSettled int64
	Collected       int64
	HasParty        bool
}

// positionOf derives the position from a return header and its memo rows.
// reduction is the amount offset (or about to be, for a preview).
func positionOf(mode string, total, difference, reduction int64, settlements []*contracts.Settlement, hasParty bool) settlementPosition {
	p := settlementPosition{Reduction: reduction, HasParty: hasParty}
	switch {
	case mode != contracts.ReturnModeExchange:
		p.Owed = total
	case difference < 0:
		p.Owed = -difference
	case difference > 0:
		p.Charge = difference
	}
	for _, st := range settlements {
		switch st.Type {
		case contracts.SettleRefund, contracts.SettleCustomerCredit:
			p.ExternalSettled += st.Amount
		case contracts.SettleCollectPayment:
			p.Collected += st.Amount
		}
	}
	return p
}

// ExternalRemaining is what is still free to refund or keep as customer
// balance: the owed value minus the automatic offset minus what was settled.
func (p settlementPosition) ExternalRemaining() int64 {
	if r := p.Owed - p.Reduction - p.ExternalSettled; r > 0 {
		return r
	}
	return 0
}

// ChargeRemaining is the exchange surcharge not yet collected.
func (p settlementPosition) ChargeRemaining() int64 {
	if r := p.Charge - p.Collected; r > 0 {
		return r
	}
	return 0
}

// Allowed lists the settlement types an operator may still add.
func (p settlementPosition) Allowed() []string {
	out := []string{}
	if p.ChargeRemaining() > 0 {
		out = append(out, contracts.SettleCollectPayment)
	}
	if p.ExternalRemaining() > 0 {
		out = append(out, contracts.SettleRefund)
		if p.HasParty {
			out = append(out, contracts.SettleCustomerCredit)
		}
	}
	return out
}

// validate checks one manual settlement against the position. Pure: the
// caller has already checked the return is confirmed and the amount positive.
func (p settlementPosition) validate(typ string, amount int64) error {
	switch typ {
	case contracts.SettleCollectPayment:
		if p.Charge <= 0 {
			return apperror.Conflict("Tambah bayar hanya untuk retur tukar yang barang penggantinya lebih mahal dari barang retur")
		}
		if amount > p.ChargeRemaining() {
			return apperror.Conflict("Melebihi selisih yang harus dibayar customer (sisa " + strconv.FormatInt(p.ChargeRemaining(), 10) + ")")
		}
	case contracts.SettleRefund, contracts.SettleCustomerCredit:
		if p.Owed <= 0 {
			return apperror.Conflict("Retur ini tidak menyisakan nilai untuk dikembalikan ke customer (refund atau saldo customer)")
		}
		if p.ExternalRemaining() <= 0 {
			return apperror.Conflict("Nilai retur ini sepenuhnya mengurangi sisa piutang order; tidak ada nilai yang bisa direfund atau dijadikan saldo customer")
		}
		if typ == contracts.SettleCustomerCredit && !p.HasParty {
			return apperror.Conflict("Saldo customer hanya bisa dipakai jika order memiliki data customer")
		}
		if amount > p.ExternalRemaining() {
			return apperror.Conflict("Melebihi sisa yang belum diselesaikan (sisa " + strconv.FormatInt(p.ExternalRemaining(), 10) + ")")
		}
	case contracts.SettleReduceReceivable:
		return apperror.Conflict("Pengurangan piutang dilakukan otomatis saat retur dikonfirmasi; catat sisanya sebagai refund atau saldo customer")
	default:
		return apperror.Validation("", []apperror.FieldError{{Field: "type", Message: "jenis penyelesaian tidak dikenal"}})
	}
	return nil
}

// offsetAmount is how much of the owed value the automatic offset takes off
// the order's open bill: the smaller of the owed value and what the customer
// still owes on the order. Shared by Confirm (autoOffset) and Preview so the
// preview shows exactly what confirmation will do.
func offsetAmount(orderGrandTotal, paid, owed int64) int64 {
	open := orderGrandTotal - paid
	if open > owed {
		open = owed
	}
	if open < 0 {
		return 0
	}
	return open
}
