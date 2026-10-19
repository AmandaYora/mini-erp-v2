package application

import (
	"context"
	"errors"

	financecontracts "mini-erp/internal/modules/finance/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	reportingcontracts "mini-erp/internal/modules/reporting/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
)

// MaxTrendDays caps the sales-trend range (30 days … 12 months, F7).
const MaxTrendDays = 366

// Service composes operational reports from finance (posted books).
type Service struct {
	finance    financecontracts.FinanceClient
	sales      salescontracts.SalesOpsClient
	purchasing purchasingcontracts.PurchasingOpsClient
	stock      stockcontracts.StockClient
}

// SetSources wires the operational readers (orders, stock) the order and stock
// reports read. Without them those reports refuse instead of inventing data.
func (s *Service) SetSources(sales salescontracts.SalesOpsClient, purchasing purchasingcontracts.PurchasingOpsClient, stock stockcontracts.StockClient) {
	s.sales, s.purchasing, s.stock = sales, purchasing, stock
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
//
// It is a deliberate thin alias of finance.InventoryValue (F8-11): the
// valuation has exactly ONE implementation, in finance. This entry exists only
// so the "reporting.view" audience (menu "Tren & Stok") can read the same
// numbers that finance.report.view readers get at
// GET /finance/reports/inventory-value. Do not add valuation logic here.
func (s *Service) Inventory(ctx context.Context, branchID int64) (*reportingcontracts.InventoryReport, error) {
	positions, total, err := s.finance.InventoryValue(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &reportingcontracts.InventoryReport{Positions: positions, Total: total}, nil
}

// OrderReportResult is the orders report: one flow, one page, exact totals.
type OrderReportResult struct {
	Type       string `json:"type"`
	Rows       any    `json:"rows"`
	Total      int64  `json:"total"`
	GrandTotal int64  `json:"grandTotal"`
}

// Orders reports the sales or purchasing orders of a date range / status.
func (s *Service) Orders(ctx context.Context, branchID int64, kind, status, from, to string, page, limit int) (*OrderReportResult, error) {
	if s.sales == nil || s.purchasing == nil {
		return nil, apperror.Internal(errSourcesUnwired)
	}
	switch kind {
	case "", "sales":
		rep, err := s.sales.ReportOrders(ctx, branchID, status, from, to, page, limit)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		return &OrderReportResult{Type: "sales", Rows: rep.Rows, Total: rep.Total, GrandTotal: rep.GrandTotal}, nil
	case "purchasing":
		rep, err := s.purchasing.ReportOrders(ctx, branchID, status, from, to, page, limit)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		return &OrderReportResult{Type: "purchasing", Rows: rep.Rows, Total: rep.Total, GrandTotal: rep.GrandTotal}, nil
	}
	return nil, apperror.Validation("", []apperror.FieldError{{Field: "type", Message: "harus sales atau purchasing"}})
}

// Stock reports the tracked products' totals, optionally critical only.
func (s *Service) Stock(ctx context.Context, branchID int64, criticalOnly bool, page, limit int) (*stockcontracts.StockReport, error) {
	if s.stock == nil {
		return nil, apperror.Internal(errSourcesUnwired)
	}
	rep, err := s.stock.StockReport(ctx, branchID, criticalOnly, page, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return rep, nil
}

var errSourcesUnwired = errors.New("reporting sources not wired")
