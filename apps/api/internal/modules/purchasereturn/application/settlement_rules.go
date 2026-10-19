package application

import (
	"strconv"

	"mini-erp/internal/modules/purchasereturn/contracts"
	"mini-erp/internal/shared/apperror"
)

// settlementPosition is the money picture of one purchase return, the single
// source of the settlement rules (K-04c, legacy purchase-return.service.ts
// resolveSettlement): AddSettlement validates against it, the detail view and
// the create-time preview (K-04d) show it.
//
// A purchase return always favours the buyer: its value first reduces what we
// owe the supplier (Reduction, the automatic offset) and only the excess over
// the open bill can be refunded in cash (refund) or kept as our balance with
// the supplier (supplier_credit). It NEVER adds to the payable: unlike sales
// there is no exchange surcharge, so collect_payment has no purchase twin.
type settlementPosition struct {
	Owed      int64 // return value (what the supplier owes us)
	Reduction int64 // part already offset against the order's open bill
	// ExternalSettled sums refund + supplier_credit rows.
	ExternalSettled int64
}

// positionOf derives the position from a return and its memo rows. reduction
// is the amount offset (or about to be, for a preview).
func positionOf(total, reduction int64, settlements []*contracts.Settlement) settlementPosition {
	p := settlementPosition{Owed: total, Reduction: reduction}
	for _, st := range settlements {
		if st.Type == contracts.SettleRefund || st.Type == contracts.SettleSupplierCredit {
			p.ExternalSettled += st.Amount
		}
	}
	return p
}

// ExternalRemaining is what is still free to refund or keep as supplier
// balance: the return value minus the automatic offset minus what was settled.
func (p settlementPosition) ExternalRemaining() int64 {
	if r := p.Owed - p.Reduction - p.ExternalSettled; r > 0 {
		return r
	}
	return 0
}

// Allowed lists the settlement types an operator may still add.
func (p settlementPosition) Allowed() []string {
	if p.ExternalRemaining() > 0 {
		return []string{contracts.SettleRefund, contracts.SettleSupplierCredit}
	}
	return []string{}
}

// validate checks one manual settlement against the position. Pure: the
// caller has already checked the return is confirmed and the amount positive.
func (p settlementPosition) validate(typ string, amount int64) error {
	switch typ {
	case contracts.SettleRefund, contracts.SettleSupplierCredit:
		if p.ExternalRemaining() <= 0 {
			return apperror.Conflict("Retur ini sepenuhnya mengurangi hutang ke supplier; tidak ada kelebihan untuk direfund atau dijadikan saldo supplier")
		}
		if amount > p.ExternalRemaining() {
			return apperror.Conflict("Melebihi sisa yang belum diselesaikan (sisa " + strconv.FormatInt(p.ExternalRemaining(), 10) + ")")
		}
	case contracts.SettleReducePayable:
		return apperror.Conflict("Pengurangan hutang dilakukan otomatis saat retur dikonfirmasi; catat kelebihannya sebagai refund atau saldo supplier")
	default:
		return apperror.Validation("", []apperror.FieldError{{Field: "type", Message: "jenis penyelesaian tidak dikenal (pilih refund atau supplier_credit)"}})
	}
	return nil
}

// offsetAmount is how much of the return value the automatic offset takes off
// the order's open bill: the smaller of the return value and what we still owe
// on the order. Shared by Confirm (autoOffset) and Preview.
func offsetAmount(orderGrandTotal, paid, total int64) int64 {
	open := orderGrandTotal - paid
	if open > total {
		open = total
	}
	if open < 0 {
		return 0
	}
	return open
}
