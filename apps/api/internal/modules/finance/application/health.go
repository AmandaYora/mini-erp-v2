package application

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	productcontracts "mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/health"
	"mini-erp/internal/shared/money"
)

// Data-health checks (ADR-0025). Pure reads over the cost ledger, periods and
// journals: unlike Readiness/UnpostedSources they never drain costing, take the
// cost lock or write a row, so the dashboard may run them on a timer.

// pricingBatch bounds one PricingByIDs read.
const pricingBatch = 1000

// costAbove is one stocked product whose ledger unit cost exceeds ratio × its
// selling price per base unit.
type costAbove struct {
	pricing productcontracts.PricingInfo
	qty     float64
	value   float64
	unit    float64 // ledger cost per base unit
	sell    float64 // selling price per base unit
}

func (c costAbove) ratio() float64 { return c.unit / c.sell }

// costAbovePrice lists, highest ledger value first, the stocked products whose
// moving-average cost per base unit is above ratio × selling price per base
// unit. Products without a selling price are skipped (nothing to compare).
//
// Query count: 1 grouped ledger read + 1 pricing read per 1.000 stocked products.
func (s *Service) costAbovePrice(ctx context.Context, branchID int64, ratio float64) ([]costAbove, error) {
	positions, err := s.repo.StockedProductCosts(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var out []costAbove
	for start := 0; start < len(positions); start += pricingBatch {
		end := min(start+pricingBatch, len(positions))
		ids := make([]int64, 0, end-start)
		for _, p := range positions[start:end] {
			ids = append(ids, p.ProductID)
		}
		prices, err := s.products.PricingByIDs(ctx, ids)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		for _, p := range positions[start:end] {
			pr, ok := prices[p.ProductID]
			if !ok || pr.SellingPrice <= 0 || p.Qty <= 0 {
				continue
			}
			factor := pr.SalesFactor
			if factor <= 0 {
				factor = 1
			}
			c := costAbove{pricing: pr, qty: p.Qty, value: p.Value, unit: p.Value / p.Qty, sell: float64(pr.SellingPrice) / factor}
			// The epsilon keeps an exact 1.2× (float noise) out.
			if c.unit > ratio*c.sell+1e-6 {
				out = append(out, c)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].value != out[j].value {
			return out[i].value > out[j].value
		}
		return out[i].pricing.ID < out[j].pricing.ID
	})
	return out, nil
}

// salesUnitCost is the ledger cost of one SALES unit, comparable with the
// catalog selling price.
func (c costAbove) salesUnitCost() int64 {
	factor := c.pricing.SalesFactor
	if factor <= 0 {
		factor = 1
	}
	return money.Round(c.unit * factor)
}

// CostAbovePrice is alert A2 (inventory.cost_above_price): count, ledger value,
// archived split and the five largest by value.
func (s *Service) CostAbovePrice(ctx context.Context, branchID int64, ratio float64, sample int) (health.Result, error) {
	rows, err := s.costAbovePrice(ctx, branchID, ratio)
	if err != nil {
		return health.Result{}, err
	}
	var total float64
	var archived int64
	res := health.Result{Count: int64(len(rows))}
	for i, r := range rows {
		total += r.value
		if r.pricing.Status == "archived" {
			archived++
		}
		if i < sample {
			res.Sample = append(res.Sample, health.Sample{
				Label: productLabel(r.pricing),
				Value: fmt.Sprintf("modal %s / jual %s", money.Rupiah(r.salesUnitCost()), money.Rupiah(r.pricing.SellingPrice)),
			})
		}
	}
	res.AmountRp = health.Amount(money.Round(total))
	res.Meta = map[string]int64{"archivedCount": archived}
	return res, nil
}

// CostAbovePriceItems is the paginated full list of A2.
func (s *Service) CostAbovePriceItems(ctx context.Context, branchID int64, ratio float64, page, limit int) ([]health.Item, int64, error) {
	rows, err := s.costAbovePrice(ctx, branchID, ratio)
	if err != nil {
		return nil, 0, err
	}
	from, to := pageBounds(len(rows), page, limit)
	items := make([]health.Item, 0, to-from)
	for _, r := range rows[from:to] {
		id := strconv.FormatInt(r.pricing.ID, 10)
		items = append(items, health.Item{
			Key:   id,
			Label: productLabel(r.pricing),
			Columns: map[string]any{
				"status": r.pricing.Status, "qtyBase": r.qty, "valueRp": money.Round(r.value),
				"unitCostRp": r.salesUnitCost(), "sellingPriceRp": r.pricing.SellingPrice,
				"ratio": float64(money.Round(r.ratio()*100)) / 100,
			},
			Links: []health.Link{
				{Label: "Harga produk", To: "/products/" + id + "?tab=harga", Perm: "products.view"},
				{Label: "Revaluasi", To: "/finance/inventory-revaluations?productId=" + id, Perm: "finance.manage"},
			},
		})
	}
	return items, int64(len(rows)), nil
}

// openingProduct aggregates the opening-only positions of one product.
type openingProduct struct {
	pricing   productcontracts.PricingInfo
	productID int64
	positions int64
	qty       float64
	value     float64
}

// openingUnverified groups the opening-only ledger positions per product,
// highest value first, named through one pricing read per 1.000 products.
func (s *Service) openingUnverified(ctx context.Context, branchID int64) ([]openingProduct, int64, error) {
	positions, err := s.repo.OpeningOnlyPositions(ctx, branchID)
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	byID := map[int64]*openingProduct{}
	var ordered []*openingProduct
	for _, p := range positions {
		op := byID[p.ProductID]
		if op == nil {
			op = &openingProduct{productID: p.ProductID}
			byID[p.ProductID] = op
			ordered = append(ordered, op)
		}
		op.positions++
		op.qty += p.Qty
		op.value += p.Total
	}
	for start := 0; start < len(ordered); start += pricingBatch {
		end := min(start+pricingBatch, len(ordered))
		ids := make([]int64, 0, end-start)
		for _, op := range ordered[start:end] {
			ids = append(ids, op.productID)
		}
		prices, err := s.products.PricingByIDs(ctx, ids)
		if err != nil {
			return nil, 0, apperror.Internal(err)
		}
		for _, op := range ordered[start:end] {
			op.pricing = prices[op.productID]
			op.pricing.ID = op.productID
		}
	}
	out := make([]openingProduct, 0, len(ordered))
	for _, op := range ordered {
		out = append(out, *op)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].value != out[j].value {
			return out[i].value > out[j].value
		}
		return out[i].productID < out[j].productID
	})
	return out, int64(len(positions)), nil
}

// OpeningUnverified is alert A7 (inventory.opening_unverified): products whose
// stock is still exactly the migrated opening balance.
func (s *Service) OpeningUnverified(ctx context.Context, branchID int64, sample int) (health.Result, error) {
	rows, positions, err := s.openingUnverified(ctx, branchID)
	if err != nil {
		return health.Result{}, err
	}
	var total, archivedValue float64
	var archived int64
	res := health.Result{Count: int64(len(rows))}
	for i, r := range rows {
		total += r.value
		if r.pricing.Status == "archived" {
			archived++
			archivedValue += r.value
		}
		if i < sample {
			res.Sample = append(res.Sample, health.Sample{Label: productLabel(r.pricing), Value: money.Rupiah(money.Round(r.value)).String()})
		}
	}
	res.AmountRp = health.Amount(money.Round(total))
	res.Meta = map[string]int64{"positions": positions, "archivedCount": archived, "archivedAmountRp": money.Round(archivedValue)}
	return res, nil
}

// OpeningUnverifiedItems is the paginated full list of A7.
func (s *Service) OpeningUnverifiedItems(ctx context.Context, branchID int64, page, limit int) ([]health.Item, int64, error) {
	rows, _, err := s.openingUnverified(ctx, branchID)
	if err != nil {
		return nil, 0, err
	}
	from, to := pageBounds(len(rows), page, limit)
	items := make([]health.Item, 0, to-from)
	for _, r := range rows[from:to] {
		id := strconv.FormatInt(r.productID, 10)
		items = append(items, health.Item{
			Key:   id,
			Label: productLabel(r.pricing),
			Columns: map[string]any{
				"status": r.pricing.Status, "qtyBase": r.qty, "valueRp": money.Round(r.value), "positions": r.positions,
			},
			Links: []health.Link{{Label: "Kartu stok", To: "/stock/items/" + id, Perm: "stock.view"}},
		})
	}
	return items, int64(len(rows)), nil
}

// UnclosedPastPeriods is alert A5 (finance.period_unclosed): the months before
// currentYM (YYYYMM) with booked activity or an open period row that are not
// closed, ascending.
func (s *Service) UnclosedPastPeriods(ctx context.Context, branchID int64, currentYM int) ([]int, error) {
	months, err := s.repo.ActiveMonths(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var out []int
	for ym, closed := range months {
		if ym < currentYM && !closed {
			out = append(out, ym)
		}
	}
	sort.Ints(out)
	return out, nil
}

// UnbalancedJournals is alert A4 (finance.unbalanced_journal) over journals
// dated on/after from (the earliest open month: closed books cannot change).
func (s *Service) UnbalancedJournals(ctx context.Context, branchID int64, from string, sample int) (health.Result, error) {
	n, err := s.repo.CountUnbalanced(ctx, branchID, from, "")
	if err != nil {
		return health.Result{}, apperror.Internal(err)
	}
	res := health.Result{Count: n}
	if n == 0 {
		return res, nil
	}
	numbers, err := s.repo.UnbalancedNumbers(ctx, branchID, from, sample)
	if err != nil {
		return health.Result{}, apperror.Internal(err)
	}
	for _, num := range numbers {
		res.Sample = append(res.Sample, health.Sample{Label: num, Value: "debit ≠ kredit"})
	}
	return res, nil
}

func productLabel(p productcontracts.PricingInfo) string {
	if p.Code == "" {
		return p.Name
	}
	return p.Code + " " + p.Name
}

// pageBounds slices n rows for a 1-based page.
func pageBounds(n, page, limit int) (from, to int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	from = min((page-1)*limit, n)
	to = min(from+limit, n)
	return from, to
}
