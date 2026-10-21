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

// toNote maps the 31-column note projection shared by every note query
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
		Origin:                          r.Origin,
		PartyID:                         r.PartyID.Int64,
		ShipToLabel:                     r.ShipToLabel.String,
		ShipToRecipient:                 r.ShipToRecipient.String,
		ShipToPhone:                     r.ShipToPhone.String,
		ShipToAddress:                   r.ShipToAddress.String,
		LinkedBy:                        r.LinkedBy.Int64,
	}
	if n.Origin == "" {
		n.Origin = contracts.OriginOrder
	}
	if r.LinkedAt.Valid {
		n.LinkedAt = r.LinkedAt.Time.Format(wireDateTime)
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

// toItemOfMany maps the per-note line projection (ListItemsByDeliveries) —
// the same columns as ListItemsByDelivery plus the owning note id.
func toItemOfMany(r sqlc.ListItemsByDeliveriesRow) *contracts.Item {
	return toItem(sqlc.ListItemsByDeliveryRow{
		ID: r.ID, ProductID: r.ProductID, VariantID: r.VariantID, LocationID: r.LocationID,
		Uom: r.Uom, UomFactor: r.UomFactor, Qty: r.Qty, QtyBase: r.QtyBase, ReservationKey: r.ReservationKey,
	})
}

// oldestDay reads the MIN(delivery_date) aggregate (untyped in the generated
// row: time.Time with parseTime, []byte otherwise, nil when no row) as a
// YYYY-MM-DD day.
func oldestDay(v any) string {
	switch t := v.(type) {
	case time.Time:
		return t.Format("2006-01-02")
	case []byte:
		if len(t) >= 10 {
			return string(t[:10])
		}
	case string:
		if len(t) >= 10 {
			return t[:10]
		}
	}
	return ""
}

// actor stores the acting user id as-is (0 = system/seed, kept verbatim).
func actor(id int64) sql.NullInt64 { return sql.NullInt64{Int64: id, Valid: true} }

// parseDeliveryDate reads the contracts delivery_date text ("YYYY-MM-DD" or
// with a time of day) as the wall-clock instant the DATETIME column stores
// (UTC, no conversion), the way MySQL read the string.
func parseDeliveryDate(s string) (time.Time, error) { return dbutil.ParseDateTime(s) }
