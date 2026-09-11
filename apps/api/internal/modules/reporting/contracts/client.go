package contracts

import (
	"context"

	financecontracts "mini-erp/internal/modules/finance/contracts"
	usercontracts "mini-erp/internal/modules/user/contracts"
)

// TrendPoint is one day of revenue/expense/profit. It aliases the finance
// DailyPoint so the trend series has a single source of truth: the grouped
// DailyTrend query (F8), not a per-day NetProfit loop.
type TrendPoint = financecontracts.DailyPoint

// InventoryReport is the valued-stock report (positions + total).
type InventoryReport struct {
	Positions []financecontracts.InventoryPosition `json:"positions"`
	Total     int64                                `json:"total"`
}

// ReportingClient is the public surface of the reporting module.
type ReportingClient interface {
	// SalesTrend returns per-day figures for an inclusive YYYY-MM-DD range.
	SalesTrend(ctx context.Context, branchID int64, from, to string) ([]TrendPoint, error)
	// Inventory returns valued stock positions for one branch.
	Inventory(ctx context.Context, branchID int64) (*InventoryReport, error)
}

// PermissionCatalog lists this module's permission codes for the seed.
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "reporting.view", Name: "Laporan: Lihat"},
	}
}
