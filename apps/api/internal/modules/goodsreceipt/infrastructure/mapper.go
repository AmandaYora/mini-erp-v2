package infrastructure

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"mini-erp/internal/modules/goodsreceipt/contracts"
	"mini-erp/internal/modules/goodsreceipt/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
)

const wireDateTime = "2006-01-02 15:04:05"

// toReceipt maps the header projection shared by GetReceipt,
// GetReceiptByClientRef, ListPostedAfter and ListReceipts (identical field
// sets, so the generated row types convert to sqlc.GetReceiptRow).
func toReceipt(r sqlc.GetReceiptRow) *contracts.GoodsReceipt {
	rec := &contracts.GoodsReceipt{
		ID:                     r.ID,
		Number:                 r.Number.String,
		BranchID:               r.BranchID,
		PurchaseOrderID:        r.PurchaseOrderID,
		ReceivedAt:             r.ReceivedAt.Format(wireDateTime),
		Notes:                  r.Notes.String,
		SupplierDeliveryNumber: r.SupplierDeliveryNumber.String,
		Status:                 r.Status,
		ClientRef:              r.ClientRef.String,
		CreatedBy:              r.CreatedBy.Int64,
		CancelReason:           r.CancelReason.String,
		CancelledBy:            r.CancelledBy.Int64,
	}
	if r.CancelledAt.Valid {
		rec.CancelledAt = r.CancelledAt.Time.Format(wireDateTime)
	}
	return rec
}

func toItem(r sqlc.ListItemsByReceiptRow) *contracts.Item {
	return &contracts.Item{
		ID:         r.ID,
		ProductID:  r.ProductID,
		VariantID:  r.VariantID,
		LocationID: r.LocationID,
		UOM:        r.Uom,
		UOMFactor:  r.UomFactor,
		Qty:        r.Qty,
		QtyBase:    r.QtyBase,
	}
}

// actor stores the acting user id as-is (0 = system/seed, kept verbatim).
func actor(id int64) sql.NullInt64 { return sql.NullInt64{Int64: id, Valid: true} }

// parseReceivedAt reads the contracts received_at text the way MySQL read the
// string: a bare day, a day with time, or the ISO "T" separator without a
// zone (legacy callers send "2024-05-12T01:02:03", which time.RFC3339Nano
// rejects for lack of an offset).
func parseReceivedAt(s string) (time.Time, error) {
	if t, err := dbutil.ParseDateTime(s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02T15:04:05", strings.TrimSpace(s)); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("tanggal/waktu tidak valid: %q", s)
}
