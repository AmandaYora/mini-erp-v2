package application

import (
	"context"

	"mini-erp/internal/modules/finance/infrastructure"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/apperror"
)

// averageCost returns the current moving-average unit cost for a position
// (0 when no costed history — callers post zero and the uncosted report
// flags it for review).
func (s *Service) averageCost(ctx context.Context, branchID, productID, variantID int64) (float64, error) {
	positions, err := s.repo.CostPositions(ctx, branchID)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	if p, ok := positions[infrastructure.CostKeyOf(productID, variantID)]; ok {
		return p.Average(), nil
	}
	return 0, nil
}

// SyncResult is one costing run outcome.
type SyncResult struct {
	Ingested  int   `json:"ingested"`
	Estimated int   `json:"estimated"`
	Cursor    int64 `json:"cursor"`
}

// Sync ingests new stock movements into the cost ledger (idempotent: the
// unique movement key makes re-runs no-ops). Costing rules:
//   - goods_receipt in: PO pro-rata unit cost (exact economics).
//   - opening in: current catalog purchase price, always estimated.
//   - every other movement: current running average (transfers, adjustments,
//     returns, damaged, write-offs, rollbacks are all cost-neutral by
//     construction). Outs with no history cost zero, estimated.
func (s *Service) Sync(ctx context.Context, branchID int64) (*SyncResult, error) {
	cursor, err := s.repo.MaxCostMovementID(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	movements, err := s.stock.ListCostMovements(ctx, branchID, cursor, 500)
	if err != nil {
		return nil, err
	}
	positions, err := s.repo.CostPositions(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	res := &SyncResult{Cursor: cursor}
	for _, m := range movements {
		unit, estimated, err := s.costOf(ctx, branchID, m, positions)
		if err != nil {
			return nil, err
		}
		if err := s.repo.InsertCostRow(ctx, branchID, m.ProductID, m.VariantID,
			m.ID, m.Direction, m.QtyBase, unit, estimated, m.RefType, m.RefID); err != nil {
			return nil, apperror.Internal(err)
		}
		applyToPositions(positions, m, unit)
		if estimated {
			res.Estimated++
		}
		res.Ingested++
		res.Cursor = m.ID
	}
	return res, nil
}

// costOf prices one movement. All computation reads the running positions
// map (prior rows of this run included), so ordering by movement id is load
// bearing — ListCostMovements guarantees it.
func (s *Service) costOf(ctx context.Context, branchID int64, m *stockcontracts.CostMovement, positions map[string]*infrastructure.CostPosition) (float64, bool, error) {
	switch {
	case m.Direction == "in" && m.RefType == "goods_receipt":
		return s.receiptUnitCost(ctx, m)
	case m.Direction == "in" && m.RefType == "opening":
		p, err := s.products.GetByID(ctx, m.ProductID)
		if err != nil {
			return 0, false, apperror.Internal(err)
		}
		if p == nil {
			return 0, true, nil
		}
		return float64(p.PurchasePrice), true, nil
	default:
		// Transfers, adjustments, returns, damaged, write-offs, and
		// rollbacks all ride the running average (cost-neutral by
		// construction). Known limitation: a cross-branch transfer books
		// the destination branch's average, not the source branch's — the
		// movement carries no source cost, and consolidated value can drift
		// by the average gap. Same-branch transfers are exact.
		if p, ok := positions[infrastructure.CostKeyOf(m.ProductID, m.VariantID)]; ok && p.Qty > 0 {
			return p.Average(), false, nil
		}
		return 0, true, nil
	}
}

// receiptUnitCost pro-ratas the PO line net over received base qty — the
// same economics the GR journal posts, so ledger and books never diverge.
func (s *Service) receiptUnitCost(ctx context.Context, m *stockcontracts.CostMovement) (float64, bool, error) {
	gr, err := s.goodsreceipt.GetByID(ctx, m.RefID)
	if err != nil {
		return 0, false, apperror.Internal(err)
	}
	if gr == nil {
		return 0, true, nil
	}
	po, err := s.purchasing.GetByID(ctx, gr.PurchaseOrderID)
	if err != nil {
		return 0, false, apperror.Internal(err)
	}
	if po == nil {
		return 0, true, nil
	}
	for _, l := range po.Items {
		// First matching line wins: per-base economics should agree across
		// UOMs; inconsistent UOM pricing converges on the first line.
		//
		// Tax BASE, not net: PPN Masukan is recoverable, so it is never part
		// of what a unit costs. For a tax-inclusive PO the net still carries
		// the tax, which would inflate moving-average cost and, through it,
		// COGS and margin. This is the same basis buildGoodsReceipt debits to
		// Persediaan, so the cost ledger and the books cannot diverge.
		if l.ProductID == m.ProductID && l.VariantID == m.VariantID && l.QtyBase > 0 {
			_, base, _ := lineEconomics(l.Qty, l.UnitPrice, l.DiscountPct, l.DiscountNominal, po.TaxType, po.TaxRate)
			return float64(base) / l.QtyBase, false, nil
		}
	}
	return 0, true, nil
}

func applyToPositions(positions map[string]*infrastructure.CostPosition, m *stockcontracts.CostMovement, unit float64) {
	key := infrastructure.CostKeyOf(m.ProductID, m.VariantID)
	p, ok := positions[key]
	if !ok {
		p = &infrastructure.CostPosition{ProductID: m.ProductID, VariantID: m.VariantID}
		positions[key] = p
	}
	if m.Direction == "in" {
		p.Qty += m.QtyBase
		p.Total += m.QtyBase * unit
	} else {
		p.Qty -= m.QtyBase
		p.Total -= m.QtyBase * unit
	}
}

// UncostedItem is one (product, variant) holding estimated cost rows.
type UncostedItem struct {
	ProductID   int64  `json:"productId"`
	VariantID   int64  `json:"variantId"`
	ProductCode string `json:"productCode"`
	ProductName string `json:"productName"`
}

// Uncosted lists positions whose history includes estimated costs.
func (s *Service) Uncosted(ctx context.Context, branchID int64) ([]UncostedItem, error) {
	pairs, err := s.repo.EstimatedProducts(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]UncostedItem, 0, len(pairs))
	for _, pv := range pairs {
		item := UncostedItem{ProductID: pv[0], VariantID: pv[1]}
		if p, err := s.products.GetByID(ctx, pv[0]); err == nil && p != nil {
			item.ProductCode, item.ProductName = p.Code, p.Name
		}
		out = append(out, item)
	}
	return out, nil
}
