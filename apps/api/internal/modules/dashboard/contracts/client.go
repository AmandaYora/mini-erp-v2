package contracts

import (
	"context"

	financecontracts "mini-erp/internal/modules/finance/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	usercontracts "mini-erp/internal/modules/user/contracts"
)

// Period figures for a date range.
type Period struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Revenue int64  `json:"revenue"`
	Expense int64  `json:"expense"`
	Profit  int64  `json:"profit"`
}

// Balances are key book balances (cash basis footing, not physical count).
type Balances struct {
	Cash       int64 `json:"cash"`
	Receivable int64 `json:"receivable"`
	Payable    int64 `json:"payable"`
	Inventory  int64 `json:"inventory"`
}

// Summary is the operational snapshot for one branch: today + month-to-date
// P&L plus key balances. All figures come from finance (posted books), so the
// dashboard never duplicates accounting logic.
type Summary struct {
	BranchID       int64      `json:"branchId"`
	Today          Period     `json:"today"`
	MonthToDate    Period     `json:"monthToDate"`
	Balances       Balances   `json:"balances"`
	InventoryValue int64      `json:"inventoryValue"`
	Operations     Operations `json:"operations"`
}

// Operations are the figures read from the live documents (no journal posting
// needed), labelled apart from the booked Today/MonthToDate figures.
//
//   - Sales are the grand totals (tax included) of orders COMPLETED in the
//     period, net of confirmed returns (return-only: minus the total; exchange:
//     replacement total minus returned total) — the legacy definition.
//   - Orders* count the non-cancelled SALES orders dated in the period.
//   - Purchases* are rupiah: the summed grand totals of the non-cancelled
//     purchase orders dated in the period (drafts included);
//     PurchaseOrders* count those same orders.
//   - Outstanding receivable/payable are per order, never below zero: an
//     overpaid order does not offset another order's debt.
type Operations struct {
	SalesToday            int64 `json:"salesToday"`
	SalesMonth            int64 `json:"salesMonth"`
	SalesOrdersToday      int64 `json:"salesOrdersToday"`
	SalesOrdersMonth      int64 `json:"salesOrdersMonth"`
	OrdersToday           int64 `json:"ordersToday"`
	OrdersMonth           int64 `json:"ordersMonth"`
	PurchasesToday        int64 `json:"purchasesToday"`
	PurchasesMonth        int64 `json:"purchasesMonth"`
	PurchaseOrdersToday   int64 `json:"purchaseOrdersToday"`
	PurchaseOrdersMonth   int64 `json:"purchaseOrdersMonth"`
	OutstandingReceivable int64 `json:"outstandingReceivable"`
	OutstandingPayable    int64 `json:"outstandingPayable"`
	CriticalCount         int64 `json:"criticalCount"`
	TrackedCount          int64 `json:"trackedCount"`
	// MissingCostCount is how many products still carry an estimated cost
	// (DB-01); CostBasis is "complete" when none do, "estimated" otherwise: the
	// margin figures are then indicative only.
	MissingCostCount int64  `json:"missingCostCount"`
	CostBasis        string `json:"costBasis"`
}

// Reasons a priority order is on the list (DB-01), legacy wording.
const (
	ReasonOverdue          = "Lewat jatuh tempo"
	ReasonAwaitingDecision = "Menunggu keputusan"
	ReasonDueSoon          = "Segera jatuh tempo"
	ReasonOpen             = "Perlu diproses"
)

// PriorityItem is one order on the "needs action" list: a sales order or a
// purchase order, with why it is there and how old it is.
type PriorityItem struct {
	ID         int64  `json:"id"`
	Kind       string `json:"kind"` // sales | purchase
	Number     string `json:"number"`
	PartyID    int64  `json:"partyId"`
	PartyName  string `json:"partyName"`
	GrandTotal int64  `json:"grandTotal"`
	Status     string `json:"status"`
	OrderDate  string `json:"orderDate"`
	DueDate    string `json:"dueDate"`
	Score      int    `json:"score"`
	Reason     string `json:"reason"`
	AgeDays    int    `json:"ageDays"`
}

// DashboardClient is the public surface of the dashboard module.
type DashboardClient interface {
	// Summary returns the operational snapshot for one branch.
	Summary(ctx context.Context, branchID int64) (*Summary, error)
	// CriticalStock returns tracked products at/below minimum, scarcest
	// first — one grouped stock read (P3), never one query per product.
	CriticalStock(ctx context.Context, branchID int64, limit int) ([]*stockcontracts.CriticalItem, error)
	// PriorityOrders returns the orders that need action, most urgent first:
	// sales orders near/past due or awaiting a decision, plus purchase orders
	// past due (unpaid) or awaiting a decision, each with its reason and age.
	PriorityOrders(ctx context.Context, branchID int64, limit int) ([]*PriorityItem, error)
	// OrderStatus returns sales + purchasing counts per derived status
	// (draft/confirmed/completed/cancelled) — two GROUP BY reads, no
	// status-config tables (D4).
	OrderStatus(ctx context.Context, branchID int64) (*OrderStatus, error)
	// TopSellers returns this month's best sellers by base qty — one GROUP BY
	// read over sales lines.
	TopSellers(ctx context.Context, branchID int64, limit int) ([]*salescontracts.TopProduct, error)
	// TopMargin returns this month's highest-margin products from the books —
	// one grouped footing read plus one product lookup per ranked row.
	TopMargin(ctx context.Context, branchID int64, limit int) ([]financecontracts.ProductMargin, error)
}

// OrderStatus counts orders per derived status for both order flows.
type OrderStatus struct {
	Sales      map[string]int64 `json:"sales"`
	Purchasing map[string]int64 `json:"purchasing"`
}

// PermissionCatalog lists this module's permission codes for the seed.
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "dashboard.view", Name: "Dasbor: Lihat"},
		{Code: "dashboard.alerts.view_all", Name: "Dasbor: Lihat & tunda alert kesehatan data"},
		{Code: "dashboard.alerts.security", Name: "Dasbor: Lihat alert keamanan"},
	}
}
