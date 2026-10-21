package application

import (
	"context"
	"strconv"

	"mini-erp/internal/modules/finance/infrastructure"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/doclock"
	"mini-erp/internal/shared/money"
)

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
//
// Serialized per branch (one container), and the unique movement key backs it
// at the database. Positions are read only for the products of the batch, and
// a short look-back window picks up movements that committed after a higher id
// was already ingested.
func (s *Service) Sync(ctx context.Context, branchID int64) (*SyncResult, error) {
	var res *SyncResult
	err := doclock.Lock("fincost:"+strconv.FormatInt(branchID, 10), func() error {
		r, err := s.syncLocked(ctx, branchID)
		res = r
		return err
	})
	return res, err
}

// costLookBackMinutes bounds how far back a late-committing lower id is sought.
const costLookBackMinutes = 10

func (s *Service) syncLocked(ctx context.Context, branchID int64) (*SyncResult, error) {
	cursor, err := s.repo.MaxCostMovementID(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	movements, err := s.stock.ListCostMovements(ctx, branchID, cursor, 500)
	if err != nil {
		return nil, err
	}
	// Late committers: recent movements at or below the cursor that never made
	// it into the ledger.
	recent, err := s.stock.ListRecentCostMovements(ctx, branchID, costLookBackMinutes, 2000)
	if err != nil {
		return nil, err
	}
	var late []*stockcontracts.CostMovement
	if len(recent) > 0 {
		ids := make([]int64, 0, len(recent))
		for _, m := range recent {
			if m.ID <= cursor {
				ids = append(ids, m.ID)
			}
		}
		present, err := s.repo.CostMovementIDsPresent(ctx, branchID, ids)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		for _, m := range recent {
			if m.ID <= cursor && !present[m.ID] {
				late = append(late, m)
			}
		}
	}
	movements = append(late, movements...)
	if len(movements) == 0 {
		return &SyncResult{Cursor: cursor}, nil
	}
	seen := map[string]bool{}
	var keys [][2]int64
	for _, m := range movements {
		k := infrastructure.CostKeyOf(m.ProductID, m.VariantID)
		if !seen[k] {
			seen[k] = true
			keys = append(keys, [2]int64{m.ProductID, m.VariantID})
		}
	}
	positions, err := s.repo.CostPositionsFor(ctx, branchID, keys)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	lastIn, err := s.repo.LastInCostsFor(ctx, branchID, keys)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	// The catalog is the last resort for a movement nothing else can price.
	// Resolve every product of the batch once, never per movement.
	needCatalog := make([]int64, 0, len(movements))
	for _, m := range movements {
		if m.RefType != "goods_receipt" && m.UnitCost <= 0 {
			needCatalog = append(needCatalog, m.ProductID)
		}
	}
	catalog := map[int64]float64{}
	if len(needCatalog) > 0 {
		catalog, err = s.products.PurchaseBaseCostsByIDs(ctx, needCatalog)
		if err != nil {
			return nil, err
		}
	}
	res := &SyncResult{Cursor: cursor}
	for _, m := range movements {
		unit, estimated, err := s.costOf(ctx, m, positions, lastIn, catalog)
		if err != nil {
			return nil, err
		}
		if err := s.repo.InsertCostRow(ctx, branchID, m.ProductID, m.VariantID,
			m.ID, m.Direction, m.QtyBase, unit, estimated, m.RefType, m.RefID, m.MovedAt); err != nil {
			return nil, apperror.Internal(err)
		}
		applyToPositions(positions, m, unit)
		if m.Direction == "in" && unit > 0 {
			lastIn[infrastructure.CostKeyOf(m.ProductID, m.VariantID)] = unit
		}
		if estimated {
			res.Estimated++
		}
		res.Ingested++
		if m.ID > res.Cursor {
			res.Cursor = m.ID
		}
	}
	return res, nil
}

// costOf prices one movement through the cost chain (F2b-11):
//
//  1. a goods receipt: the PO line's tax base per base unit (exact);
//  2. an explicit cost hint on the movement (opening balances with a cost): exact;
//  3. the position's running average while it holds stock;
//  4. the last known inflow cost (a return into an emptied position);
//  5. the catalog purchase price per base unit — marked ESTIMATED;
//  6. nothing: 0 and estimated. Documents reading such a movement are blocked
//     by the builders ("harga modal belum tersedia"), never booked at zero.
//
// All computation reads the running positions map (prior rows of this run
// included), so ordering by movement id is load bearing.
func (s *Service) costOf(ctx context.Context, m *stockcontracts.CostMovement, positions map[string]*infrastructure.CostPosition, lastIn map[string]float64, catalog map[int64]float64) (float64, bool, error) {
	if m.Direction == "in" && m.RefType == "goods_receipt" {
		return s.receiptUnitCost(ctx, m)
	}
	if m.Direction == "in" && m.UnitCost > 0 {
		return m.UnitCost, false, nil
	}
	key := infrastructure.CostKeyOf(m.ProductID, m.VariantID)
	if m.RefType != "opening" {
		// Transfers, adjustments, returns, damaged, write-offs and rollbacks
		// ride the running average (cost-neutral by construction). Known limit:
		// a cross-branch transfer books the destination average.
		if p, ok := positions[key]; ok && p.Qty > 0 {
			return p.Average(), false, nil
		}
		if unit := lastIn[key]; unit > 0 {
			return unit, true, nil
		}
	}
	if unit := catalog[m.ProductID]; unit > 0 {
		return unit, true, nil
	}
	return 0, true, nil
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
			_, base, _, _ := money.LineAmounts(l.Qty, l.UnitPrice, l.DiscountPct, l.DiscountNominal, po.TaxType, po.TaxRate).Split()
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

// UncostedCount counts the products with estimated cost rows (the cost-basis
// warning of the dashboard and margin reports).
func (s *Service) UncostedCount(ctx context.Context, branchID int64) (int64, error) {
	pairs, err := s.repo.EstimatedProducts(ctx, branchID)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	seen := map[int64]bool{}
	for _, pv := range pairs {
		seen[pv[0]] = true
	}
	return int64(len(seen)), nil
}

// Uncosted lists positions whose history includes estimated costs.
func (s *Service) Uncosted(ctx context.Context, branchID int64) ([]UncostedItem, error) {
	pairs, err := s.repo.EstimatedProducts(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]UncostedItem, 0, len(pairs))
	ids := make([]int64, 0, len(pairs))
	for _, pv := range pairs {
		out = append(out, UncostedItem{ProductID: pv[0], VariantID: pv[1]})
		ids = append(ids, pv[0])
	}
	labels := s.productLabels(ctx, ids)
	for i := range out {
		if l, ok := labels[out[i].ProductID]; ok {
			out[i].ProductCode, out[i].ProductName = l.Code, l.Name
		}
	}
	return out, nil
}
