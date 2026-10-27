package infrastructure

import (
	"database/sql"
	"strings"
	"time"

	"mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/modules/sales/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
	"mini-erp/internal/shared/timeutil"
)

// Generated row types that carry the same columns are converted to one of them
// (Go allows struct conversion between identical field sets), so each mapper
// below serves every query that projects the same shape.

const (
	wireDateTime = "2006-01-02 15:04:05"
	wireDate     = "2006-01-02"
)

// now is the single clock the repository stamps updated_at/completed_at with;
// the queries take it as a parameter (ADR-0012: no UTC_TIMESTAMP in sqlc).
func now() time.Time { return timeutil.NowUTC() }

// toOrder maps the header projection shared by GetOrder, GetOrderByNumber,
// GetOrderByClientRef and the raw list/export path.
func toOrder(r sqlc.GetOrderRow) *contracts.SalesOrder {
	return &contracts.SalesOrder{
		ID:               r.ID,
		Number:           r.Number,
		BranchID:         r.BranchID,
		PartyID:          r.PartyID,
		Channel:          r.Channel,
		MemberCode:       r.MemberCode.String,
		OrderDate:        r.OrderDate.Format(wireDateTime),
		DueDate:          dbutil.FormatNullTime(r.DueDate),
		PaymentTerms:     r.PaymentTerms,
		TaxType:          r.TaxType,
		TaxRate:          r.TaxRate,
		Subtotal:         r.Subtotal,
		DiscountTotal:    r.DiscountTotal,
		TaxTotal:         r.TaxTotal,
		GrandTotal:       r.GrandTotal,
		Status:           r.Status,
		Notes:            r.Notes.String,
		ShipToAddressID:  r.ShipToAddressID.Int64,
		ShipToLabel:      r.ShipToLabel.String,
		ShipToRecipient:  r.ShipToRecipient.String,
		ShipToPhone:      r.ShipToPhone.String,
		ShipToAddress:    r.ShipToAddress.String,
		TaxInvoiceNumber: r.TaxInvoiceNumber.String,
		TaxInvoiceDate:   dbutil.FormatNullTime(r.TaxInvoiceDate),
		ClientRef:        r.ClientRef.String,
		CreatedAt:        r.CreatedAt.Format(wireDateTime),
		CreatedBy:        r.CreatedBy.Int64,
		AssignedUserID:   r.AssignedUserID.Int64,
		CreditApprovedBy: r.CreditApprovedBy.Int64,
		CreditApprovedAt: dbutil.FormatNullTime(r.CreditApprovedAt),
		Origin:           r.Origin,
	}
}

func toItem(r sqlc.ListItemsByOrderRow) *contracts.Item {
	return &contracts.Item{
		ID:              r.ID,
		ProductID:       r.ProductID,
		VariantID:       r.VariantID,
		ProductCode:     r.ProductCode,
		ProductName:     r.ProductName,
		UOM:             r.Uom,
		UOMFactor:       r.UomFactor,
		Qty:             r.Qty,
		QtyBase:         r.QtyBase,
		UnitPrice:       r.UnitPrice,
		DiscountPct:     r.DiscountPct,
		DiscountNominal: r.DiscountNominal,
		LineTotal:       r.LineTotal,
		StockTracked:    r.StockTracked,
		ItemType:        r.ItemType,
		VariantName:     r.VariantName.String,
		VariantCode:     r.VariantCode.String,
		BaseUOM:         r.BaseUom.String,
		PricingSource:   r.PricingSource,
		BasePrice:       r.BasePrice.Int64,
		DiscountReason:  r.DiscountReason.String,
		Note:            r.LineNote.String,
		Taxable:         r.Taxable,
		TaxBase:         r.TaxBase,
		TaxAmount:       r.TaxAmount,
	}
}

// toSummary maps the lightweight document projection (id, number, party, dates,
// total, status) shared by ListByParty, ListSummaries and ReportOrders.
func toSummary(r sqlc.ListOrdersByPartyRow) *contracts.OrderSummary {
	return &contracts.OrderSummary{
		ID:         r.ID,
		Number:     r.Number,
		PartyID:    r.PartyID,
		OrderDate:  r.OrderDate.Format(wireDateTime),
		DueDate:    dbutil.FormatNullTime(r.DueDate),
		GrandTotal: r.GrandTotal,
		Status:     r.Status,
	}
}

func toOrderTotal(r sqlc.ListOpenOrderTotalsRow) contracts.OrderTotal {
	t := contracts.OrderTotal{
		ID: r.ID, Number: r.Number, PartyID: r.PartyID, GrandTotal: r.GrandTotal,
		OrderDate: r.OrderDate.Format(wireDate),
	}
	if r.DueDate.Valid {
		t.DueDate = r.DueDate.Time.Format(wireDate)
	}
	return t
}

// actor stores the acting user id as-is (0 = system/seed, kept verbatim).
func actor(id int64) sql.NullInt64 { return sql.NullInt64{Int64: id, Valid: true} }

// boundFrom / boundTo turn the caller's optional YYYY-MM-DD day into the
// inclusive datetime bound (00:00:00 / 23:59:59); blank = unbounded (NULL).
func boundFrom(day string) (sql.NullTime, error) {
	day = strings.TrimSpace(day)
	if day == "" {
		return sql.NullTime{}, nil
	}
	return dbutil.NullDateTime(day + " 00:00:00")
}

func boundTo(day string) (sql.NullTime, error) {
	day = strings.TrimSpace(day)
	if day == "" {
		return sql.NullTime{}, nil
	}
	return dbutil.NullDateTime(day + " 23:59:59")
}

// dayRange is boundFrom/boundTo for queries whose range is mandatory.
func dayRange(from, to string) (time.Time, time.Time, error) {
	a, err := dbutil.ParseDateTime(strings.TrimSpace(from) + " 00:00:00")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	b, err := dbutil.ParseDateTime(strings.TrimSpace(to) + " 23:59:59")
	return a, b, err
}

// dayBounds turns an inclusive YYYY-MM-DD range (Jakarta days) into the UTC
// instants the completed_at column stores.
func dayBounds(from, to string) (time.Time, time.Time) {
	loc := timeutil.Jakarta
	start, err1 := time.ParseInLocation(wireDate, strings.TrimSpace(from), loc)
	end, err2 := time.ParseInLocation(wireDate, strings.TrimSpace(to), loc)
	if err1 != nil || err2 != nil {
		return time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2999, 12, 31, 23, 59, 59, 0, time.UTC)
	}
	// Last microsecond of the WIB day: completed_at is DATETIME(6).
	return start.UTC(), end.AddDate(0, 0, 1).Add(-time.Microsecond).UTC()
}

// headerParams are the nullable columns CreateOrder/UpdateOrder write.
type headerParams struct {
	orderDate time.Time
	dueDate   sql.NullTime
	invDate   sql.NullTime
}

func toHeaderParams(o *contracts.SalesOrder, withOrderDate bool) (headerParams, error) {
	var h headerParams
	var err error
	if withOrderDate {
		if h.orderDate, err = dbutil.ParseDateTime(o.OrderDate); err != nil {
			return h, err
		}
	}
	if h.dueDate, err = dbutil.NullDateTime(o.DueDate); err != nil {
		return h, err
	}
	h.invDate, err = dbutil.NullDateTime(o.TaxInvoiceDate)
	return h, err
}

func toInsertItem(orderID int64, it *contracts.Item) sqlc.InsertItemParams {
	itemType := it.ItemType
	if itemType == "" {
		itemType = "barang"
	}
	src := it.PricingSource
	if src == "" {
		src = contracts.PricingCatalog
	}
	return sqlc.InsertItemParams{
		OrderID: orderID, ProductID: it.ProductID, VariantID: it.VariantID,
		ProductCode: it.ProductCode, ProductName: it.ProductName, Uom: it.UOM,
		UomFactor: it.UOMFactor, Qty: it.Qty, QtyBase: it.QtyBase, UnitPrice: it.UnitPrice,
		DiscountPct: it.DiscountPct, DiscountNominal: it.DiscountNominal, LineTotal: it.LineTotal,
		StockTracked: it.StockTracked, ItemType: itemType,
		VariantName: dbutil.NullString(it.VariantName), VariantCode: dbutil.NullString(it.VariantCode),
		BaseUom:       dbutil.NullString(it.BaseUOM),
		PricingSource: src, BasePrice: sql.NullInt64{Int64: it.BasePrice, Valid: true},
		DiscountReason: dbutil.NullString(it.DiscountReason), LineNote: dbutil.NullString(it.Note),
		Taxable: it.Taxable, TaxBase: it.TaxBase, TaxAmount: it.TaxAmount,
	}
}
