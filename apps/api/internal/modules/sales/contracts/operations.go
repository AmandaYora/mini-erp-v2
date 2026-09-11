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

// OpenShipment is one confirmed, shippable order with its lines for the
// delivery work queue (J3). PartyName resolves best-effort like ListOrders;
// walk-in reads "Tunai".
type OpenShipment struct {
	OrderID      int64   `json:"orderId"`
	Number       string  `json:"number"`
	PartyID      int64   `json:"partyId"`
	PartyName    string  `json:"partyName"`
	OrderDate    string  `json:"orderDate"`
	DueDate      string  `json:"dueDate"`
	PaymentTerms string  `json:"paymentTerms"`
	GrandTotal   int64   `json:"grandTotal"`
	Items        []*Item `json:"items"`
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
	// OpenShipments lists confirmed non-POS orders oldest-first with lines
	// and party names, capped at limit — the delivery work queue's
	// "create_sj" source (J3). POS orders are excluded: the cashier hands
	// them over immediately, they never wait in the warehouse queue.
	// Query count: 1 headers + 1 grouped items read + one party read per
	// distinct party (P3).
	OpenShipments(ctx context.Context, branchID int64, limit int) ([]*OpenShipment, error)
}
