package infrastructure

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"mini-erp/internal/modules/payment/contracts"
	"mini-erp/internal/modules/payment/infrastructure/sqlc"
)

const timeLayout = "2006-01-02 15:04:05"

// toPayment maps a generated payment row to the public contracts type. Every
// payment projection shares one field set, so the other row types are
// converted to sqlc.GetPaymentRow first.
func toPayment(r sqlc.GetPaymentRow) *contracts.Payment {
	return &contracts.Payment{
		ID:               r.ID,
		Number:           r.Number,
		BranchID:         r.BranchID,
		PartyID:          r.PartyID,
		PartyType:        r.PartyType,
		Direction:        r.Direction,
		Amount:           r.Amount,
		Method:           r.Method,
		PaidAt:           r.PaidAt.Format(timeLayout),
		Notes:            r.Notes.String,
		Status:           r.Status,
		ClientRef:        r.ClientRef.String,
		AmountTendered:   r.AmountTendered.Int64,
		Reference:        r.ReferenceNumber.String,
		PairID:           r.PairID.Int64,
		CashAccountID:    r.CashAccountID.Int64,
		CreditSourceType: r.CreditSourceType.String,
		CreditSourceID:   r.CreditSourceID.Int64,

		CorrectsPaymentID: r.CorrectsPaymentID.Int64,
		CancelReason:      r.CancelReason.String,
		CancelledAt:       stamp(r.CancelledAt),
		CancelledBy:       r.CancelledBy.Int64,
	}
}

// stamp renders an optional system stamp as stored (UTC text); "" when unset.
func stamp(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.UTC().Format(timeLayout)
}

// parsePaidAt reads the contracts paid_at text ("2006-01-02" or with a time of
// day, optionally fractional) as a UTC instant, the way MySQL read the string.
func parsePaidAt(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"2006-01-02 15:04:05.999999999", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("payment: invalid paid_at %q", s)
}

func nullStr(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

func nullID(v int64) sql.NullInt64 { return sql.NullInt64{Int64: v, Valid: v != 0} }

// actor stores the acting user id as-is (0 = system, kept verbatim).
func actor(id int64) sql.NullInt64 { return sql.NullInt64{Int64: id, Valid: true} }

func toAllocation(orderType string, orderID, amount int64) *contracts.Allocation {
	return &contracts.Allocation{OrderType: orderType, OrderID: orderID, Amount: amount}
}
