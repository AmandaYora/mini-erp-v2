package application

import (
	"context"
	"math"
	"strconv"
	"strings"

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
// unique movement key makes re-runs no-ops). The ledger keeps one invariant:
// every movement either changes the books at exactly its recorded value (a
// receipt, a shipment, a return, an adjustment, a write-off, a damaged
// arrival — each journaled from its own rows), or it is value-neutral (an
// internal move or a cancellation, whose two legs carry the same value). See
// costOf for how each kind is priced.
//
// Serialized per branch (one container), and the unique movement key backs it
// at the database. Positions are read only for the products of the batch, and
// a short look-back window picks up movements that committed after a higher id
// was already ingested. A transfer received from another branch is priced at
// what the sending branch's ledger relieved, so that branch is synced first.
func (s *Service) Sync(ctx context.Context, branchID int64) (*SyncResult, error) {
	if err := s.syncTransferSources(ctx, branchID); err != nil {
		return nil, err
	}
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
		c, err := s.costOf(ctx, m, positions, lastIn, catalog)
		if err != nil {
			return nil, err
		}
		unit := 0.0
		if m.QtyBase > 0 {
			unit = c.value / m.QtyBase
		}
		if err := s.repo.InsertCostRow(ctx, branchID, m.ProductID, m.VariantID,
			m.ID, m.Direction, m.QtyBase, unit, c.value, c.estimated, m.RefType, m.RefID, m.MovedAt); err != nil {
			return nil, apperror.Internal(err)
		}
		applyToPositions(positions, m, c.value)
		if m.Direction == "in" && unit > 0 {
			lastIn[infrastructure.CostKeyOf(m.ProductID, m.VariantID)] = unit
		}
		if c.estimated {
			res.Estimated++
		}
		res.Ingested++
		if m.ID > res.Cursor {
			res.Cursor = m.ID
		}
	}
	return res, nil
}

// movementCost is the priced value (whole rupiah, a magnitude) of one movement.
type movementCost struct {
	value     float64
	estimated bool
}

// costOf prices one movement (F2b-11, ADR-0020). Values are whole rupiah —
// the unit the journals post in — so the ledger and the books add up to the
// same figure row by row.
//
// Value-neutral pairs first, priced from the leg they mirror:
//
//  1. a cancellation (`<ref>_cancel`): the original document's own rows for
//     that position, pro rata — never today's average, which would leave the
//     difference in the ledger with no journal behind it;
//  2. a transfer in-leg: what the out-leg relieved (the other branch's ledger
//     for a cross-branch transfer);
//  3. a damaged-location move in-leg: the out-leg written just before it.
//
// Everything else runs the cost chain:
//
//  4. a goods receipt: the PO line's tax base per base unit (exact);
//  5. an explicit cost hint on the movement (opening balances with a cost): exact;
//  6. the position's running average while it holds stock — an out that
//     empties the position takes exactly its remaining value, so a position
//     at zero quantity is always at zero value;
//  7. the last known inflow cost (a return into an emptied position);
//  8. the catalog purchase price per base unit — marked ESTIMATED;
//  9. nothing: 0 and estimated. Documents reading such a movement are blocked
//     by the builders ("harga modal belum tersedia"), never booked at zero.
//
// All computation reads the running positions map (prior rows of this run
// included), so ordering by movement id is load bearing.
func (s *Service) costOf(ctx context.Context, m *stockcontracts.CostMovement, positions map[string]*infrastructure.CostPosition, lastIn map[string]float64, catalog map[int64]float64) (movementCost, error) {
	if c, ok, err := s.mirroredCost(ctx, m); err != nil || ok {
		return c, err
	}
	if m.Direction == "in" && m.RefType == stockcontracts.RefGoodsReceipt {
		unit, estimated, err := s.receiptUnitCost(ctx, m)
		return movementCost{value: wholeRupiah(m.QtyBase * unit), estimated: estimated}, err
	}
	if m.Direction == "in" && m.UnitCost > 0 {
		return movementCost{value: wholeRupiah(m.QtyBase * m.UnitCost)}, nil
	}
	key := infrastructure.CostKeyOf(m.ProductID, m.VariantID)
	if m.RefType != stockcontracts.RefOpening {
		if p, ok := positions[key]; ok && p.Qty > positionEpsilon {
			if m.Direction == "out" && math.Abs(p.Qty-m.QtyBase) <= positionEpsilon {
				return movementCost{value: p.Total}, nil
			}
			return movementCost{value: wholeRupiah(m.QtyBase * p.Average())}, nil
		}
		if unit := lastIn[key]; unit > 0 {
			return movementCost{value: wholeRupiah(m.QtyBase * unit), estimated: true}, nil
		}
	}
	if unit := catalog[m.ProductID]; unit > 0 {
		return movementCost{value: wholeRupiah(m.QtyBase * unit), estimated: true}, nil
	}
	return movementCost{estimated: true}, nil
}

// positionEpsilon is the quantity below which a position counts as empty
// (qty_base is decimal(24,12); fractional UOM conversions leave dust).
const positionEpsilon = 1e-9

func wholeRupiah(v float64) float64 {
	return float64(money.Round(v))
}

// mirroredCost prices a movement that mirrors another leg (rules 1-3 of
// costOf). ok is false when the movement is not a mirror, or when the leg it
// mirrors is not in any ledger — then the cost chain prices it and the row is
// marked estimated so it surfaces in the cost-basis list.
func (s *Service) mirroredCost(ctx context.Context, m *stockcontracts.CostMovement) (movementCost, bool, error) {
	opposite := "in"
	if m.Direction == "in" {
		opposite = "out"
	}
	switch {
	case strings.HasSuffix(m.RefType, stockcontracts.CancelSuffix) && m.RefID > 0:
		base := strings.TrimSuffix(m.RefType, stockcontracts.CancelSuffix)
		return s.proRata(ctx, m, m.BranchID, base, opposite, m.BranchID, m.RefType)
	case m.RefType == stockcontracts.RefTransfer && m.Direction == "in" && m.RefID > 0:
		return s.proRata(ctx, m, 0, stockcontracts.RefTransfer, "out", 0, stockcontracts.RefTransfer)
	case m.RefType == stockcontracts.RefDamaged && m.Direction == "in":
		prev, err := s.repo.PrecedingRefRow(ctx, m.BranchID, m.ProductID, m.VariantID, stockcontracts.RefDamaged, m.ID)
		if err != nil {
			return movementCost{}, false, apperror.Internal(err)
		}
		if prev == nil || prev.Direction != "out" || prev.Qty <= 0 {
			return movementCost{}, false, nil
		}
		v := math.Abs(prev.Total)
		if math.Abs(prev.Qty-m.QtyBase) > positionEpsilon {
			v = wholeRupiah(v * m.QtyBase / prev.Qty)
		}
		return movementCost{value: v, estimated: prev.Estimated}, true, nil
	}
	return movementCost{}, false, nil
}

// proRata values a mirror leg as its share of the mirrored legs' total value.
// The share is cumulative — (already mirrored + this qty) of the total, minus
// what the earlier mirror rows already took — so the legs of one document net
// to exactly zero once fully mirrored, rounding included.
func (s *Service) proRata(ctx context.Context, m *stockcontracts.CostMovement, srcBranch int64, srcRef, srcDir string, doneBranch int64, doneRef string) (movementCost, bool, error) {
	srcQty, srcValue, srcEstimated, err := s.repo.RefCostSum(ctx, srcBranch, srcRef, m.RefID, m.ProductID, m.VariantID, srcDir)
	if err != nil {
		return movementCost{}, false, apperror.Internal(err)
	}
	if srcQty <= positionEpsilon {
		return movementCost{}, false, nil
	}
	doneQty, doneValue, _, err := s.repo.RefCostSum(ctx, doneBranch, doneRef, m.RefID, m.ProductID, m.VariantID, m.Direction)
	if err != nil {
		return movementCost{}, false, apperror.Internal(err)
	}
	share := math.Min(doneQty+m.QtyBase, srcQty) / srcQty
	v := wholeRupiah(srcValue*share) - doneValue
	if v < 0 {
		v = 0
	}
	return movementCost{value: v, estimated: srcEstimated}, true, nil
}

// syncTransferSources syncs every branch that sent a transfer this branch is
// about to ingest, so the in-leg can mirror the out-leg's recorded value. It
// runs before the branch's own costing lock is taken: no lock is ever held
// while waiting for another branch's, so two branches sending to each other
// cannot deadlock.
func (s *Service) syncTransferSources(ctx context.Context, branchID int64) error {
	if ctx.Value(transferSyncKey{}) != nil {
		return nil // one level: a source's own sources are not chased further
	}
	cursor, err := s.repo.MaxCostMovementID(ctx, branchID)
	if err != nil {
		return apperror.Internal(err)
	}
	pending, err := s.stock.ListCostMovements(ctx, branchID, cursor, 500)
	if err != nil {
		return err
	}
	seen := map[int64]bool{}
	for _, m := range pending {
		if m.RefType != stockcontracts.RefTransfer || m.Direction != "in" || m.RefID <= 0 || seen[m.RefID] {
			continue
		}
		seen[m.RefID] = true
		from, err := s.stock.TransferSourceBranch(ctx, m.RefID)
		if err != nil {
			return err
		}
		if from <= 0 || from == branchID {
			continue
		}
		if err := s.drainCosting(context.WithValue(ctx, transferSyncKey{}, true), from); err != nil {
			return err
		}
	}
	return nil
}

type transferSyncKey struct{}

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

func applyToPositions(positions map[string]*infrastructure.CostPosition, m *stockcontracts.CostMovement, value float64) {
	key := infrastructure.CostKeyOf(m.ProductID, m.VariantID)
	p, ok := positions[key]
	if !ok {
		p = &infrastructure.CostPosition{ProductID: m.ProductID, VariantID: m.VariantID}
		positions[key] = p
	}
	if m.Direction == "in" {
		p.Qty += m.QtyBase
		p.Total += value
	} else {
		p.Qty -= m.QtyBase
		p.Total -= value
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
