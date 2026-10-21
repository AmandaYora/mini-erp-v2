package infrastructure

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"mini-erp/internal/modules/purchasing/contracts"
	"mini-erp/internal/modules/purchasing/infrastructure/sqlc"
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

// now is the single clock the repository stamps updated_at with; the queries
// take it as a parameter (ADR-0012: no UTC_TIMESTAMP in sqlc queries).
func now() time.Time { return timeutil.NowUTC() }

// toOrder maps the header projection shared by GetOrder, GetOrderByNumber and
// the list/export queries.
func toOrder(r sqlc.GetOrderRow) *contracts.PurchaseOrder {
	return &contracts.PurchaseOrder{
		ID:                    r.ID,
		Number:                r.Number,
		BranchID:              r.BranchID,
		PartyID:               r.PartyID,
		OrderDate:             r.OrderDate.Format(wireDateTime),
		DueDate:               dbutil.FormatNullTime(r.DueDate),
		PaymentTerms:          r.PaymentTerms,
		TaxType:               r.TaxType,
		TaxRate:               r.TaxRate,
		Subtotal:              r.Subtotal,
		DiscountTotal:         r.DiscountTotal,
		TaxTotal:              r.TaxTotal,
		GrandTotal:            r.GrandTotal,
		Status:                r.Status,
		Notes:                 r.Notes.String,
		SupplierInvoiceNumber: r.SupplierInvoiceNumber.String,
		SupplierInvoiceDate:   dbutil.FormatNullTime(r.SupplierInvoiceDate),
		TaxInvoiceNumber:      r.TaxInvoiceNumber.String,
		TaxInvoiceDate:        dbutil.FormatNullTime(r.TaxInvoiceDate),
		AssignedUserID:        r.AssignedUserID.Int64,
		CreatedBy:             r.CreatedBy.Int64,
		PaymentTermDays:       int(r.PaymentTermDays.Int16),
		CancelReason:          r.CancelReason.String,
		CancelledAt:           wibStamp(r.CancelledAt),
		CancelledBy:           r.CancelledBy.Int64,
		ShortCloseReason:      r.ShortCloseReason.String,
		ShortClosedAt:         wibStamp(r.ShortClosedAt),
		ShortClosedBy:         r.ShortClosedBy.Int64,
	}
}

// wibStamp renders a stored UTC instant as the Jakarta wall clock (the API's
// one datetime form); NULL stays empty.
func wibStamp(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.UTC().In(timeutil.Jakarta).Format(wireDateTime)
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
		DiscountReason:  r.DiscountReason.String,
		Note:            r.LineNote.String,
		Taxable:         r.Taxable,
		TaxRate:         r.TaxRate,
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

// termDays maps 0 to NULL (no credit period given).
func termDays(n int) (sql.NullInt16, error) {
	if n == 0 {
		return sql.NullInt16{}, nil
	}
	if n < 0 || n > math.MaxInt16 {
		return sql.NullInt16{}, fmt.Errorf("payment_term_days di luar rentang: %d", n)
	}
	return sql.NullInt16{Int16: int16(n), Valid: true}, nil
}

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

// headerParams are the columns CreateOrder/UpdateOrder write from a document.
type headerParams struct {
	orderDate  time.Time
	dueDate    sql.NullTime
	invDate    sql.NullTime
	taxInvDate sql.NullTime
	termDays   sql.NullInt16
}

func toHeaderParams(o *contracts.PurchaseOrder, withOrderDate bool) (headerParams, error) {
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
	if h.invDate, err = dbutil.NullDateTime(o.SupplierInvoiceDate); err != nil {
		return h, err
	}
	if h.taxInvDate, err = dbutil.NullDateTime(o.TaxInvoiceDate); err != nil {
		return h, err
	}
	h.termDays, err = termDays(o.PaymentTermDays)
	return h, err
}

func toInsertItem(orderID int64, it *contracts.Item) sqlc.InsertItemParams {
	itemType := it.ItemType
	if itemType == "" {
		itemType = "barang"
	}
	return sqlc.InsertItemParams{
		OrderID: orderID, ProductID: it.ProductID, VariantID: it.VariantID,
		ProductCode: it.ProductCode, ProductName: it.ProductName, Uom: it.UOM,
		UomFactor: it.UOMFactor, Qty: it.Qty, QtyBase: it.QtyBase, UnitPrice: it.UnitPrice,
		DiscountPct: it.DiscountPct, DiscountNominal: it.DiscountNominal, LineTotal: it.LineTotal,
		StockTracked: it.StockTracked, ItemType: itemType,
		VariantName: dbutil.NullString(it.VariantName), VariantCode: dbutil.NullString(it.VariantCode),
		BaseUom: dbutil.NullString(it.BaseUOM), DiscountReason: dbutil.NullString(it.DiscountReason),
		LineNote: dbutil.NullString(it.Note), Taxable: it.Taxable, TaxRate: it.TaxRate,
		TaxBase: it.TaxBase, TaxAmount: it.TaxAmount,
	}
}
