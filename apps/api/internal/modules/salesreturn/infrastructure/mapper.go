package infrastructure

import (
	"database/sql"
	"strings"
	"time"

	"mini-erp/internal/modules/salesreturn/contracts"
	"mini-erp/internal/modules/salesreturn/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
	"mini-erp/internal/shared/timeutil"
)

const wireDateTime = "2006-01-02 15:04:05"

// now is the single clock the repository stamps updated_at with; the queries
// take it as a parameter (ADR-0012: no UTC_TIMESTAMP in sqlc).
func now() time.Time { return timeutil.NowUTC() }

// toReturn maps the full header projection of GetReturn.
func toReturn(r sqlc.GetReturnRow) *contracts.SalesReturn {
	return &contracts.SalesReturn{
		ID:                        r.ID,
		Number:                    r.Number,
		BranchID:                  r.BranchID,
		SalesOrderID:              r.SalesOrderID,
		ReturnDate:                r.ReturnDate.Format(wireDateTime),
		Subtotal:                  r.Subtotal,
		DiscountTotal:             r.DiscountTotal,
		TaxTotal:                  r.TaxTotal,
		TaxType:                   r.TaxType,
		TaxRate:                   r.TaxRate,
		Total:                     r.Total,
		Status:                    r.Status,
		Notes:                     r.Notes.String,
		ReturnMode:                r.ReturnMode,
		ReplacementDeliveryStatus: r.ReplacementDeliveryStatus,
		Reason:                    r.Reason,
		ReplacementSubtotal:       r.ReplacementSubtotal,
		ReplacementTaxTotal:       r.ReplacementTaxTotal,
		ReplacementTotal:          r.ReplacementTotal,
		DifferenceAmount:          r.DifferenceAmount,
		ReceivableReduction:       r.ReceivableReduction,
	}
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
		Condition:       r.ReturnCondition,
	}
}

func toReplacement(r sqlc.ListReplacementItemsRow) *contracts.ReplacementItem {
	return &contracts.ReplacementItem{
		ID:         r.ID,
		ProductID:  r.ProductID,
		VariantID:  r.VariantID,
		LocationID: r.LocationID,
		UOM:        r.Uom,
		UOMFactor:  r.UomFactor,
		Qty:        r.Qty,
		QtyBase:    r.QtyBase,
		UnitPrice:  r.UnitPrice,
		TaxBase:    r.TaxBase,
		TaxAmount:  r.TaxAmount,
		LineTotal:  r.LineTotal,
	}
}

// toConfirmed maps the narrow cursor projection of ListConfirmedAfter.
func toConfirmed(r sqlc.ListConfirmedAfterRow) *contracts.SalesReturn {
	return &contracts.SalesReturn{
		ID:         r.ID,
		Number:     r.Number,
		BranchID:   r.BranchID,
		ReturnDate: r.ReturnDate.Format(wireDateTime),
		Total:      r.Total,
		Status:     r.Status,
	}
}

// toReturnProjection maps the payment-facing projection of ConfirmedByOrders:
// Total is the return's money effect, not its goods value (an even exchange
// owes nothing), with the same rule as GetReturn.
func toReturnProjection(r sqlc.ConfirmedByOrdersRow) *contracts.Return {
	total, kind := contracts.MoneyEffect(r.ReturnMode, r.Total, r.DifferenceAmount)
	return &contracts.Return{
		ID:      r.ID,
		Number:  r.Number,
		Branch:  r.BranchID,
		OrderID: r.SalesOrderID,
		Total:   total,
		Kind:    kind,
		Status:  r.Status,
	}
}

func toSettlement(r sqlc.ListSettlementsRow) *contracts.Settlement {
	return &contracts.Settlement{
		ID:              r.ID,
		Type:            r.SettlementType,
		Date:            r.SettlementDate.Format(wireDateTime),
		Amount:          r.Amount,
		PaymentMethod:   r.PaymentMethod.String,
		ReferenceNumber: r.ReferenceNumber.String,
		Notes:           r.Notes.String,
		Auto:            r.Auto,
	}
}

// actor stores the acting user id as-is (0 = system/seed, kept verbatim).
func actor(id int64) sql.NullInt64 { return sql.NullInt64{Int64: id, Valid: true} }

// parseReturnDate reads the contracts date text ("YYYY-MM-DD" or with a time
// of day) as the wall-clock instant the DATETIME column stores (UTC, no
// conversion), the way MySQL read the string.
func parseReturnDate(s string) (time.Time, error) { return dbutil.ParseDateTime(s) }

// dayBounds turns the inclusive YYYY-MM-DD range the reports take into the
// day-start/day-end instants the old SQL compared against
// (from 00:00:00, to 23:59:59).
func dayBounds(from, to string) (time.Time, time.Time, error) {
	a, err := dbutil.ParseDateTime(strings.TrimSpace(from) + " 00:00:00")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	b, err := dbutil.ParseDateTime(strings.TrimSpace(to) + " 23:59:59")
	return a, b, err
}

func conditionOrDefault(c string) string {
	switch c {
	case contracts.ConditionDamaged, contracts.ConditionNotReturned:
		return c
	default:
		return contracts.ConditionNormal
	}
}

func returnModeOrDefault(m string) string {
	if m == contracts.ReturnModeExchange {
		return contracts.ReturnModeExchange
	}
	return contracts.ReturnModeReturnOnly
}

func replacementStatusOrDefault(s string) string {
	switch s {
	case contracts.ReplacementPending, contracts.ReplacementDispatched, contracts.ReplacementConfirmed:
		return s
	default:
		return contracts.ReplacementNotRequired
	}
}
