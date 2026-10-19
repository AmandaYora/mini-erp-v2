package contracts

import (
	"context"
)

// PurchasingOpsClient is the operational read surface of the purchasing
// module for the dashboard: grouped aggregates only, never per-row loops
// (P3). It lives next to PurchaseOrderClient so existing consumers keep
// compiling untouched.
type PurchasingOpsClient interface {
	// StatusCounts returns order counts keyed by derived status
	// (draft/confirmed/completed/cancelled) — one GROUP BY query straight off
	// the status column, no status-config tables (D4).
	StatusCounts(ctx context.Context, branchID int64) (map[string]int64, error)
	// CountMissingSupplierInvoice counts confirmed/completed taxed orders
	// without a supplier invoice number — one COUNT query. Finance reads it
	// for the period-readiness gate.
	CountMissingSupplierInvoice(ctx context.Context, branchID int64) (int64, error)
	// CountMissingSupplierInvoiceBetween is the same count for orders dated
	// within [from, to] (YYYY-MM-DD, inclusive).
	CountMissingSupplierInvoiceBetween(ctx context.Context, branchID int64, from, to string) (int64, error)
	// CountOrders counts the non-cancelled orders dated within a range.
	CountOrders(ctx context.Context, branchID int64, from, to string) (int64, error)
	// OpenOrderTotals pages confirmed/completed orders after a cursor id with
	// their grand total (the payable base).
	OpenOrderTotals(ctx context.Context, branchID, afterID int64, limit int) ([]OrderTotal, error)
	// ReportOrders pages the orders of a date range / status with an exact
	// total and grand-total sum (operational order report).
	ReportOrders(ctx context.Context, branchID int64, status, from, to string, page, limit int) (*OrderReport, error)
}

// OrderTotal is one order's grand total.
type OrderTotal struct {
	ID         int64
	Number     string
	PartyID    int64
	GrandTotal int64
	OrderDate  string // YYYY-MM-DD
	DueDate    string // YYYY-MM-DD, empty when none
}

// OrderReport is one page of the operational order report.
type OrderReport struct {
	Rows       []*OrderSummary `json:"rows"`
	Total      int64           `json:"total"`
	GrandTotal int64           `json:"grandTotal"`
}
