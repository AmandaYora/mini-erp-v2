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
}
