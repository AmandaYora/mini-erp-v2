package contracts

import (
	"context"
)

// TopProduct is one product's realised sales over a range, from order lines
// (operational — includes unposted drafts, excludes cancelled orders). Names
// are the line snapshots, so no product lookup is needed.
type TopProduct struct {
	ProductID   int64   `json:"productId"`
	VariantID   int64   `json:"variantId"`
	ProductCode string  `json:"productCode"`
	ProductName string  `json:"productName"`
	QtyBase     float64 `json:"qtyBase"`
	Revenue     int64   `json:"revenue"`
	Orders      int64   `json:"orders"`
}

// PriorityOrder is a live order with its urgency score.
type PriorityOrder struct {
	ID         int64  `json:"id"`
	Number     string `json:"number"`
	PartyID    int64  `json:"partyId"`
	PartyName  string `json:"partyName"`
	GrandTotal int64  `json:"grandTotal"`
	Status     string `json:"status"`
	OrderDate  string `json:"orderDate"`
	DueDate    string `json:"dueDate"`
	Score      int    `json:"score"`
}

// OrderTotal is one order's grand total (receivable base).
type OrderTotal struct {
	ID         int64
	Number     string
	PartyID    int64
	GrandTotal int64
	OrderDate  string // YYYY-MM-DD
	DueDate    string // YYYY-MM-DD, empty when none
}

// OpenShipment is one confirmed, shippable order with its lines for the
// delivery work queue (J3). PartyName resolves best-effort like ListOrders;
// walk-in reads "Tunai".
type OpenShipment struct {
	OrderID      int64  `json:"orderId"`
	Number       string `json:"number"`
	PartyID      int64  `json:"partyId"`
	PartyName    string `json:"partyName"`
	OrderDate    string `json:"orderDate"`
	DueDate      string `json:"dueDate"`
	PaymentTerms string `json:"paymentTerms"`
	GrandTotal   int64  `json:"grandTotal"`
	// OrphanPos marks a POS order left confirmed without a completed handover.
	OrphanPos bool    `json:"orphanPos"`
	Items     []*Item `json:"items"`
}

// OpenShipmentFilter narrows the delivery work queue's create-SJ source (DL-02).
// All filters run in SQL before the page cut, so an order beyond the first
// Limit rows is still reachable.
type OpenShipmentFilter struct {
	// Search matches the order number (substring) OR a customer name/code/phone
	// resolved through the party contract.
	Search string
	// From/To are an inclusive YYYY-MM-DD order-date range; empty = open.
	From, To      string
	Limit, Offset int
}

// OpenShipmentSearcher is the filtered/paged variant of OpenShipments. It is a
// separate interface so existing SalesOpsClient fakes keep compiling.
type OpenShipmentSearcher interface {
	// OpenShipmentsFiltered is OpenShipments with Search/date filters and an
	// offset. Query count: 1 party search (only with Search) + 1 headers + 1
	// grouped items + 1 grouped party names.
	OpenShipmentsFiltered(ctx context.Context, branchID int64, f OpenShipmentFilter) ([]*OpenShipment, error)
	// SearchOrderIDs returns up to limit ids of the branch's orders (any
	// status) whose number contains term or whose customer matches it.
	SearchOrderIDs(ctx context.Context, branchID int64, term string, limit int) ([]int64, error)
}

// SalesOpsClient is the operational read surface of the sales module for the
// dashboard: grouped aggregates only, never per-row loops (P3). It lives next
// to SalesOrderClient so existing consumers keep compiling untouched.
type SalesOpsClient interface {
	// StatusCounts returns order counts keyed by derived status
	// (draft/confirmed/completed/cancelled) — one GROUP BY query straight off
	// the status column, no status-config tables (D4).
	StatusCounts(ctx context.Context, branchID int64) (map[string]int64, error)
	// TopProducts ranks products by base qty sold in an inclusive YYYY-MM-DD
	// range, biggest first, capped at limit — one GROUP BY query.
	TopProducts(ctx context.Context, branchID int64, from, to string, limit int) ([]*TopProduct, error)
	// CountMissingTaxInvoice counts confirmed/completed taxed orders without
	// a tax invoice number — one COUNT query. Finance reads it for the
	// period-readiness gate (safe-close must not seal a month whose PPN
	// paperwork is incomplete).
	CountMissingTaxInvoice(ctx context.Context, branchID int64) (int64, error)
	// CountMissingTaxInvoiceBetween is the same count for orders dated within
	// [from, to] (YYYY-MM-DD, inclusive): the month-end gate counts the month,
	// not the company's whole history.
	CountMissingTaxInvoiceBetween(ctx context.Context, branchID int64, from, to string) (int64, error)
	// OpenShipments lists confirmed non-POS orders oldest-first with lines
	// and party names, capped at limit — the delivery work queue's
	// "create_sj" source (J3). POS orders are excluded: the cashier hands
	// them over immediately, they never wait in the warehouse queue.
	// Query count: 1 headers + 1 grouped items read + one party read per
	// distinct party (P3).
	OpenShipments(ctx context.Context, branchID int64, limit int) ([]*OpenShipment, error)
	// CompletedSales sums the grand total (tax included) and counts the orders
	// completed within an inclusive Jakarta-day range.
	CompletedSales(ctx context.Context, branchID int64, from, to string) (total, count int64, err error)
	// CountOrders counts the non-cancelled orders dated within a range.
	CountOrders(ctx context.Context, branchID int64, from, to string) (int64, error)
	// PriorityOrders ranks live orders by the legacy urgency score.
	PriorityOrders(ctx context.Context, branchID int64, limit int) ([]*PriorityOrder, error)
	// OpenOrderTotals pages confirmed/completed orders after a cursor id.
	OpenOrderTotals(ctx context.Context, branchID, afterID int64, limit int) ([]OrderTotal, error)
	// ReportOrders pages the orders of a date range / status with an exact
	// total and grand-total sum (operational order report).
	ReportOrders(ctx context.Context, branchID int64, status, from, to string, page, limit int) (*OrderReport, error)
}

// OrderReport is one page of the operational order report.
type OrderReport struct {
	Rows       []*OrderSummary `json:"rows"`
	Total      int64           `json:"total"`
	GrandTotal int64           `json:"grandTotal"`
}
