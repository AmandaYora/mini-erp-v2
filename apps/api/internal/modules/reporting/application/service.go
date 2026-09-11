package application

import (
	"context"

	financecontracts "mini-erp/internal/modules/finance/contracts"
	reportingcontracts "mini-erp/internal/modules/reporting/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
)

// MaxTrendDays caps the sales-trend range (30 days … 12 months, F7).
const MaxTrendDays = 366

// Service composes operational reports from finance (posted books).
type Service struct {
	finance financecontracts.FinanceClient
}

// NewService wires reporting reads over the finance contract.
func NewService(finance financecontracts.FinanceClient) *Service {
	return &Service{finance: finance}
}

// SalesTrend returns per-day revenue/expense/profit for an inclusive range.
//
// Query count: 1 finance DailyTrend call (3 constant queries inside finance:
// accounts + COGS mapping + one GROUP BY DATE(entry_date) footing). The old
// code looped NetProfit once per day — a 366-day trend cost 366 sequential
// finance reads (P3, PLAN F8).
func (s *Service) SalesTrend(ctx context.Context, branchID int64, from, to string) ([]reportingcontracts.TrendPoint, error) {
	start, err := timeutil.ParseDateInput(from)
	if err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "from", Message: "format tanggal harus YYYY-MM-DD"}})
	}
	end, err := timeutil.ParseDateInput(to)
	if err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "to", Message: "format tanggal harus YYYY-MM-DD"}})
	}
	if end.Before(start) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "to", Message: "tanggal akhir sebelum tanggal awal"}})
	}
	days := int(end.Sub(start).Hours()/24) + 1
	if days > MaxTrendDays {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "to", Message: "rentang maksimal 366 hari"}})
	}
	points, err := s.finance.DailyTrend(ctx, branchID, from, to)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return points, nil
}

// Inventory returns valued stock positions for one branch.
func (s *Service) Inventory(ctx context.Context, branchID int64) (*reportingcontracts.InventoryReport, error) {
	positions, total, err := s.finance.InventoryValue(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &reportingcontracts.InventoryReport{Positions: positions, Total: total}, nil
}
