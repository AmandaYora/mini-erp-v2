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
	BranchID       int64    `json:"branchId"`
	Today          Period   `json:"today"`
	MonthToDate    Period   `json:"monthToDate"`
	Balances       Balances `json:"balances"`
	InventoryValue int64    `json:"inventoryValue"`
}

// DashboardClient is the public surface of the dashboard module.
type DashboardClient interface {
	// Summary returns the operational snapshot for one branch.
	Summary(ctx context.Context, branchID int64) (*Summary, error)
	// CriticalStock returns tracked products at/below minimum, scarcest
	// first — one grouped stock read (P3), never one query per product.
	CriticalStock(ctx context.Context, branchID int64, limit int) ([]*stockcontracts.CriticalItem, error)
	// PriorityOrders returns confirmed sales orders near/past due, most
	// overdue first — one sales read plus in-memory due filtering.
	PriorityOrders(ctx context.Context, branchID int64, limit int) ([]*salescontracts.OrderSummary, error)
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
	}
}
