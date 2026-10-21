package infrastructure

import (
	"database/sql"
	"time"

	"mini-erp/internal/modules/purchasereturn/contracts"
	"mini-erp/internal/modules/purchasereturn/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
	"mini-erp/internal/shared/timeutil"
)

const wireDateTime = "2006-01-02 15:04:05"

// now is the single clock the repository stamps updated_at with; the queries
// take it as a parameter (ADR-0012: no UTC_TIMESTAMP in sqlc).
func now() time.Time { return timeutil.NowUTC() }

// toReturn maps the full header projection of GetReturn.
func toReturn(r sqlc.GetReturnRow) *contracts.PurchaseReturn {
	ret := &contracts.PurchaseReturn{
		ID:               r.ID,
		Number:           r.Number,
		BranchID:         r.BranchID,
		PurchaseOrderID:  r.PurchaseOrderID,
		ReturnDate:       r.ReturnDate.Format(wireDateTime),
		Subtotal:         r.Subtotal,
		DiscountTotal:    r.DiscountTotal,
		TaxTotal:         r.TaxTotal,
		TaxType:          r.TaxType,
		TaxRate:          r.TaxRate,
		Total:            r.Total,
		Status:           r.Status,
		Notes:            r.Notes.String,
		Reason:           r.Reason,
		PayableReduction: r.PayableReduction,
		CancelReason:     r.CancelReason.String,
		CancelledBy:      r.CancelledBy.Int64,
	}
	if r.CancelledAt.Valid {
		ret.CancelledAt = r.CancelledAt.Time.Format(wireDateTime)
	}
	return ret
}

func toItem(r sqlc.ListItemsByReturnRow) *contracts.Item {
	return &contracts.Item{
		ID:              r.ID,
		ProductID:       r.ProductID,
		VariantID:       r.VariantID,
		LocationID:      r.LocationID,
		UOM:             r.Uom,
		UOMFactor:       r.UomFactor,
		Qty:             r.Qty,
		QtyBase:         r.QtyBase,
		UnitPrice:       r.UnitPrice,
		DiscountPct:     r.DiscountPct,
		DiscountNominal: r.DiscountNominal,
		TaxBase:         r.TaxBase,
		TaxAmount:       r.TaxAmount,
		LineTotal:       r.LineTotal,
	}
}

// toConfirmed maps the narrow cursor projection of ListConfirmedAfter.
func toConfirmed(r sqlc.ListConfirmedAfterRow) *contracts.PurchaseReturn {
	return &contracts.PurchaseReturn{
		ID:         r.ID,
		Number:     r.Number,
		BranchID:   r.BranchID,
		ReturnDate: r.ReturnDate.Format(wireDateTime),
		Total:      r.Total,
		Status:     r.Status,
	}
}

// toReturnProjection maps the payment-facing projection of ConfirmedByOrders.
func toReturnProjection(r sqlc.ConfirmedByOrdersRow) *contracts.Return {
	return &contracts.Return{
		ID:      r.ID,
		Number:  r.Number,
		Branch:  r.BranchID,
		OrderID: r.PurchaseOrderID,
		Total:   r.Total,
		Status:  r.Status,
	}
}

func toSettlement(r sqlc.ListSettlementsRow) *contracts.Settlement {
	return &contracts.Settlement{
		ID:              r.ID,
		Type:            normalizeSettlementType(r.SettlementType),
		Date:            r.SettlementDate.Format(wireDateTime),
		Amount:          r.Amount,
		PaymentMethod:   r.PaymentMethod.String,
		ReferenceNumber: r.ReferenceNumber.String,
		Notes:           r.Notes.String,
	}
}

// actor stores the acting user id as-is (0 = system/seed, kept verbatim).
func actor(id int64) sql.NullInt64 { return sql.NullInt64{Int64: id, Valid: true} }

// parseReturnDate reads the contracts date text ("YYYY-MM-DD" or with a time
// of day) as the wall-clock instant the DATETIME column stores (UTC, no
// conversion), the way MySQL read the string.
func parseReturnDate(s string) (time.Time, error) { return dbutil.ParseDateTime(s) }

// normalizeSettlementType maps the sales-side names this module once stored by
// mistake (K-04b) onto the purchase vocabulary, so old rows read correctly.
// collect_payment has no purchase counterpart and is kept as is (it can never
// be created any more).
func normalizeSettlementType(t string) string {
	switch t {
	case "reduce_receivable":
		return contracts.SettleReducePayable
	case "customer_credit":
		return contracts.SettleSupplierCredit
	default:
		return t
	}
}
