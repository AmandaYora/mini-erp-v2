package infrastructure

import (
	"database/sql"
	"time"

	"mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/modules/delivery/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
	"mini-erp/internal/shared/timeutil"
)

const wireDateTime = "2006-01-02 15:04:05"

// now is the single clock the repository stamps updated_at/dispatched_at/
// confirmed_at with; the queries take it as a parameter (ADR-0012: no
// UTC_TIMESTAMP in sqlc).
func now() time.Time { return timeutil.NowUTC() }

// toNote maps the 23-column note projection shared by every note query
// (sqlc rows and the raw list/waiting paths select the same columns in the
// same order, so their row types convert to sqlc.GetNoteRow).
func toNote(r sqlc.GetNoteRow) *contracts.DeliveryNote {
	n := &contracts.DeliveryNote{
		ID:                              r.ID,
		Number:                          r.Number,
		BranchID:                        r.BranchID,
		SalesOrderID:                    r.SalesOrderID,
		DeliveryDate:                    r.DeliveryDate.Format(wireDateTime),
		DriverName:                      r.DriverName.String,
		VehiclePlate:                    r.VehiclePlate.String,
		WarehouseStaffName:              r.WarehouseStaffName.String,
		RecipientName:                   r.RecipientName.String,
		RecipientSignatureStatus:        r.RecipientSignatureStatus.String,
		RecipientSignatureMissingReason: r.RecipientSignatureMissingReason.String,
		DropLocationNote:                r.DropLocationNote.String,
		DispatchedBy:                    r.DispatchedBy.Int64,
		ConfirmedBy:                     r.ConfirmedBy.Int64,
		SalesReturnID:                   r.SalesReturnID.Int64,
		Status:                          r.Status,
		Notes:                           r.Notes.String,
		CancelReason:                    r.CancelReason.String,
		CancelledBy:                     r.CancelledBy.Int64,
	}
	if r.DispatchedAt.Valid {
		n.DispatchedAt = r.DispatchedAt.Time.Format(wireDateTime)
	}
	if r.ConfirmedAt.Valid {
		n.ConfirmedAt = r.ConfirmedAt.Time.Format(wireDateTime)
	}
	if r.CancelledAt.Valid {
		n.CancelledAt = r.CancelledAt.Time.Format(wireDateTime)
	}
	n.DocumentKind = r.DocumentKind
	if n.DocumentKind == "" {
		n.DocumentKind = contracts.DocumentKindOrder
	}
	return n
}

func toItem(r sqlc.ListItemsByDeliveryRow) *contracts.Item {
	return &contracts.Item{
		ID:             r.ID,
		ProductID:      r.ProductID,
		VariantID:      r.VariantID,
		LocationID:     r.LocationID,
		UOM:            r.Uom,
		UOMFactor:      r.UomFactor,
		Qty:            r.Qty,
		QtyBase:        r.QtyBase,
		ReservationKey: r.ReservationKey.String,
	}
}

// actor stores the acting user id as-is (0 = system/seed, kept verbatim).
func actor(id int64) sql.NullInt64 { return sql.NullInt64{Int64: id, Valid: true} }

// parseDeliveryDate reads the contracts delivery_date text ("YYYY-MM-DD" or
// with a time of day) as the wall-clock instant the DATETIME column stores
// (UTC, no conversion), the way MySQL read the string.
func parseDeliveryDate(s string) (time.Time, error) { return dbutil.ParseDateTime(s) }
