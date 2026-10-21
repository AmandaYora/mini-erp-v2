package application

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/modules/finance/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/money"
)

// Inventory revaluation ("Revaluasi harga modal"): the web correction for a
// WRONG NON-ZERO cost basis — an opening cost or an inflow priced wrong —
// which neither "lengkapi harga modal" (zero-valued positions only) nor the
// cost restatement (outflows only; inflows are evidence) can fix.
//
// A revaluation sets a held position to qty × newUnitCost. The difference is
// written as ONE cost-only ledger row per position (qty 0, signed value,
// synthetic movement id — the same mechanism as the opening supplement), so
// every later outflow is priced at the new average, and booked by ONE journal
// in the same transaction: Persediaan (product/variant dims) against Selisih
// Stok (mapping `adjustment`), swapped when the value goes down. The books
// therefore move by exactly the ledger's change and the K-05 reconciliation
// keeps its `unexplained` (ADR-0020). Everything runs under the branch cost
// lock (ADR-0021), after the ledger is synced.

// SourceInventoryRevaluation marks the revaluation journal (source id = the
// finance_inventory_revaluations header).
const SourceInventoryRevaluation = "inventory_revaluation"

// maxRevaluationItems bounds one request (one journal).
const maxRevaluationItems = 200

// RevaluationItem asks to set one position to a new unit cost.
type RevaluationItem struct {
	ProductID int64
	VariantID int64
	// NewUnitCost per base unit, whole rupiah (≥ 0).
	NewUnitCost int64
}

// RevaluationInput drives PreviewRevaluation and CommitRevaluation.
type RevaluationInput struct {
	BranchID  int64
	ActorID   int64
	Date      string // journal date, YYYY-MM-DD; empty = today (WIB)
	Reason    string
	ClientRef string // optional idempotency key (replay returns the first commit)
	Items     []RevaluationItem
}

// RevaluationLine is one position before/after the revaluation.
type RevaluationLine struct {
	ProductID   int64   `json:"productId"`
	VariantID   int64   `json:"variantId"`
	ProductCode string  `json:"productCode"`
	ProductName string  `json:"productName"`
	Qty         float64 `json:"qty"`
	// UnitCostBefore is the position's moving average before; UnitCostAfter
	// the requested cost (per base unit).
	UnitCostBefore float64 `json:"unitCostBefore"`
	UnitCostAfter  float64 `json:"unitCostAfter"`
	ValueBefore    int64   `json:"valueBefore"`
	ValueAfter     int64   `json:"valueAfter"`
	// Delta = ValueAfter − ValueBefore: positive raises Persediaan (Cr Selisih
	// Stok), negative lowers it (Dr Selisih Stok).
	Delta int64 `json:"delta"`
}

// RevaluationResult is a preview or a committed revaluation.
type RevaluationResult struct {
	ID          int64             `json:"id,omitempty"`
	BranchID    int64             `json:"branchId"`
	Date        string            `json:"date"`
	Reason      string            `json:"reason"`
	TotalDelta  int64             `json:"totalDelta"`
	Lines       []RevaluationLine `json:"lines"`
	Applied     bool              `json:"applied"`
	Replayed    bool              `json:"replayed,omitempty"`
	EntryID     int64             `json:"entryId,omitempty"`
	EntryNumber string            `json:"entryNumber,omitempty"`
	CreatedBy   int64             `json:"createdBy,omitempty"`
	CreatedAt   string            `json:"createdAt,omitempty"`
	ReconBefore *InventoryRecon   `json:"reconBefore,omitempty"`
	ReconAfter  *InventoryRecon   `json:"reconAfter,omitempty"`
}

// RevaluationPosition is what the page shows for one variant of a product
// before a new cost is typed.
type RevaluationPosition struct {
	ProductID   int64   `json:"productId"`
	VariantID   int64   `json:"variantId"`
	ProductCode string  `json:"productCode"`
	ProductName string  `json:"productName"`
	VariantName string  `json:"variantName"`
	BaseUOM     string  `json:"baseUom"`
	Qty         float64 `json:"qty"`
	Value       int64   `json:"value"`
	AverageCost float64 `json:"averageCost"`
	// LastReceiptCost is the unit cost of the latest goods receipt in the
	// ledger (0 = none); CatalogCost the master purchase price per base unit.
	LastReceiptCost float64 `json:"lastReceiptCost"`
	CatalogCost     float64 `json:"catalogCost"`
}

// RevaluationPositions lists the ledger positions of one product (one per
// variant), synced first, with the reference prices the page offers.
func (s *Service) RevaluationPositions(ctx context.Context, branchID, productID int64) ([]RevaluationPosition, error) {
	if productID <= 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "productId", Message: "wajib diisi"}})
	}
	if _, err := s.Sync(ctx, branchID); err != nil {
		return nil, err
	}
	positions, err := s.repo.PositionsOfProduct(ctx, branchID, productID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var code, name, baseUOM string
	variantNames := map[int64]string{}
	if s.products != nil {
		p, err := s.products.GetByID(ctx, productID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if p == nil {
			return nil, apperror.NotFound("Produk")
		}
		code, name, baseUOM = p.Code, p.Name, p.BaseUOM
		for _, v := range p.Variants {
			if !v.IsHidden {
				variantNames[v.ID] = v.Name
			}
		}
	}
	catalog := map[int64]float64{}
	if s.products != nil {
		if c, err := s.products.PurchaseBaseCostsByIDs(ctx, []int64{productID}); err == nil && c != nil {
			catalog = c
		}
	}
	out := make([]RevaluationPosition, 0, len(positions))
	for _, p := range positions {
		last, err := s.repo.LastReceiptUnitCost(ctx, branchID, productID, p.VariantID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		out = append(out, RevaluationPosition{
			ProductID: productID, VariantID: p.VariantID, ProductCode: code, ProductName: name,
			VariantName: variantNames[p.VariantID], BaseUOM: baseUOM,
			Qty: p.Qty, Value: money.Round(p.Total), AverageCost: math.Round(p.Average()*100) / 100,
			LastReceiptCost: last, CatalogCost: math.Round(catalog[productID]*100) / 100,
		})
	}
	return out, nil
}

// PreviewRevaluation computes the revaluation without writing anything. It
// fails exactly where a commit would.
func (s *Service) PreviewRevaluation(ctx context.Context, in RevaluationInput) (*RevaluationResult, error) {
	date, reason, err := s.validateRevaluation(ctx, in, false)
	if err != nil {
		return nil, err
	}
	if _, err := s.Sync(ctx, in.BranchID); err != nil {
		return nil, err
	}
	lines, err := s.revaluationLines(ctx, in)
	if err != nil {
		return nil, err
	}
	return s.revaluationResult(ctx, in.BranchID, date, reason, lines), nil
}

// CommitRevaluation applies the revaluation: ledger rows, one journal, the
// record and the audit entry, all in one transaction under the cost lock. A
// replay with the same clientRef returns the first commit unchanged.
func (s *Service) CommitRevaluation(ctx context.Context, in RevaluationInput) (*RevaluationResult, error) {
	date, reason, err := s.validateRevaluation(ctx, in, true)
	if err != nil {
		return nil, err
	}
	in.ClientRef = strings.TrimSpace(in.ClientRef)
	if in.ClientRef != "" {
		if prev, err := s.revaluationByRef(ctx, in.BranchID, in.ClientRef); err != nil || prev != nil {
			return prev, err
		}
	}
	maps, err := s.repo.ListMappings(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	inventory, offset := maps["inventory"], maps["adjustment"]
	for key, a := range map[string]*contracts.Account{"inventory": inventory, "adjustment": offset} {
		if a == nil || a.Status != "active" {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "mapping",
				Message: fmt.Sprintf("pemetaan akun '%s' wajib aktif untuk revaluasi", key)}})
		}
	}
	b, err := s.branches.GetByID(ctx, in.BranchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if b == nil {
		return nil, apperror.NotFound("Cabang")
	}
	// Reconcile before taking the costing lock (it syncs the ledger itself).
	before, err := s.ReconcileInventory(ctx, in.BranchID)
	if err != nil {
		return nil, err
	}
	var res *RevaluationResult
	err = s.costLock(ctx, in.BranchID, costLockWaitRestate, func() error {
		if _, err := s.syncLocked(ctx, in.BranchID); err != nil {
			return err
		}
		lines, err := s.revaluationLines(ctx, in)
		if err != nil {
			return err
		}
		res = s.revaluationResult(ctx, in.BranchID, date, reason, lines)
		ids, err := s.repo.NextSyntheticCostIDs(ctx, len(lines))
		if err != nil {
			return apperror.Internal(err)
		}
		rev := &infrastructure.Revaluation{BranchID: in.BranchID, Date: date, Reason: reason,
			TotalDelta: res.TotalDelta, ClientRef: in.ClientRef}
		legs := make([]*contracts.JournalLine, 0, 2*len(lines))
		for i, l := range lines {
			rev.Lines = append(rev.Lines, infrastructure.RevaluationLine{ProductID: l.ProductID, VariantID: l.VariantID,
				Qty: l.Qty, UnitCostBefore: l.UnitCostBefore, UnitCostAfter: l.UnitCostAfter,
				ValueBefore: l.ValueBefore, ValueAfter: l.ValueAfter, Delta: l.Delta, CostMovementID: ids[i]})
			amt := l.Delta
			if amt < 0 {
				amt = -amt
			}
			inv := &contracts.JournalLine{AccountID: inventory.ID, ProductID: l.ProductID, VariantID: l.VariantID,
				Description: "Revaluasi harga modal persediaan"}
			off := &contracts.JournalLine{AccountID: offset.ID, ProductID: l.ProductID, VariantID: l.VariantID,
				Description: "Revaluasi harga modal persediaan"}
			if l.Delta > 0 {
				inv.Debit, off.Credit = amt, amt
			} else {
				inv.Credit, off.Debit = amt, amt
			}
			legs = append(legs, inv, off)
		}
		entry := &contracts.JournalEntry{
			BranchID: in.BranchID, Date: date,
			Memo:       fmt.Sprintf("Revaluasi harga modal (%d produk): %s", len(lines), truncateRunes(reason, 200)),
			Numbering:  &contracts.Numbering{Prefix: "JV", BranchCode: b.Code, Year: date[:4]},
			SourceType: SourceInventoryRevaluation, Lines: legs,
		}
		detail := make([]map[string]any, 0, len(lines))
		for i, l := range lines {
			if i == 50 {
				break
			}
			detail = append(detail, map[string]any{"productId": l.ProductID, "variantId": l.VariantID, "qty": l.Qty,
				"unitBefore": l.UnitCostBefore, "unitAfter": l.UnitCostAfter,
				"before": l.ValueBefore, "after": l.ValueAfter, "delta": l.Delta})
		}
		audit := func(ctx context.Context, tx auditcontracts.AuditTx, id, entryID int64, number string) error {
			return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
				Action: "finance.inventory_revaluation", Entity: "inventory_revaluation", EntityID: id, BranchID: in.BranchID,
				ActorID: in.ActorID, ActorType: actorType(in.ActorID), Note: number,
				Metadata: map[string]any{"reason": reason, "date": date, "totalDelta": res.TotalDelta,
					"positions": len(lines), "entryId": entryID, "lines": detail},
			})
		}
		id, entryID, err := s.repo.CommitRevaluation(ctx, rev, entry, in.ActorID, audit)
		if err != nil {
			if errors.Is(err, infrastructure.ErrRevaluationClientRef) {
				prev, perr := s.revaluationByRef(ctx, in.BranchID, in.ClientRef)
				if perr != nil {
					return perr
				}
				res = prev
				return nil
			}
			if perr := periodError(err); perr != nil {
				return perr
			}
			return dberr.Map(err)
		}
		res.ID, res.EntryID, res.EntryNumber, res.Applied = id, entryID, entry.Number, true
		res.CreatedBy = in.ActorID
		return nil
	})
	if err != nil {
		return nil, err
	}
	if res.Replayed {
		return res, nil
	}
	res.ReconBefore = before
	after, err := s.ReconcileInventory(ctx, in.BranchID)
	if err != nil {
		return res, err
	}
	res.ReconAfter = after
	return res, nil
}

// validateRevaluation checks the request shape, the date (not in the future,
// in an open period) and that the finance opening is posted: before it, the
// cutover journal itself prices the ledger, and a revaluation journal on top
// would book the difference twice.
func (s *Service) validateRevaluation(ctx context.Context, in RevaluationInput, commit bool) (string, string, error) {
	var fails []apperror.FieldError
	reason := strings.TrimSpace(in.Reason)
	if commit && reason == "" {
		fails = append(fails, apperror.FieldError{Field: "reason", Message: "alasan wajib diisi"})
	}
	if len([]rune(reason)) > 500 {
		fails = append(fails, apperror.FieldError{Field: "reason", Message: "maksimal 500 karakter"})
	}
	if len(in.Items) == 0 {
		fails = append(fails, apperror.FieldError{Field: "items", Message: "pilih minimal satu produk"})
	}
	if len(in.Items) > maxRevaluationItems {
		fails = append(fails, apperror.FieldError{Field: "items", Message: fmt.Sprintf("maksimal %d produk per revaluasi", maxRevaluationItems)})
	}
	seen := map[string]bool{}
	for i, it := range in.Items {
		bad := func(msg string) {
			fails = append(fails, apperror.FieldError{Field: "items", Message: fmt.Sprintf("baris %d: %s", i+1, msg)})
		}
		if it.ProductID <= 0 {
			bad("produk wajib dipilih")
			continue
		}
		if it.NewUnitCost < 0 {
			bad("harga modal baru tidak boleh negatif")
		}
		key := infrastructure.CostKeyOf(it.ProductID, it.VariantID)
		if seen[key] {
			bad("produk muncul dua kali")
		}
		seen[key] = true
	}
	if len(fails) > 0 {
		return "", "", apperror.Validation("", fails)
	}
	date, err := normalizeEntryDate(in.Date, "date")
	if err != nil {
		return "", "", err
	}
	if err := s.requireOpenPeriod(ctx, in.BranchID, date); err != nil {
		return "", "", err
	}
	if posted, err := s.OpeningPosted(ctx, in.BranchID); err != nil {
		return "", "", err
	} else if !posted {
		return "", "", apperror.Conflict("Saldo awal belum diposting; perbaiki harga modal lewat saldo awal stok, bukan revaluasi")
	}
	return date, reason, nil
}

// revaluationLines reads each position (the ledger must be synced) and prices
// it at the requested cost. Refused: a position without stock (nothing to
// value: a cost-only row there would only leave value on an empty position)
// and a request that changes nothing.
func (s *Service) revaluationLines(ctx context.Context, in RevaluationInput) ([]RevaluationLine, error) {
	items := append([]RevaluationItem(nil), in.Items...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].ProductID == items[j].ProductID {
			return items[i].VariantID < items[j].VariantID
		}
		return items[i].ProductID < items[j].ProductID
	})
	labels := s.productLabels(ctx, itemProductIDs(items))
	var fails []apperror.FieldError
	lines := make([]RevaluationLine, 0, len(items))
	for _, it := range items {
		p, err := s.repo.CostPositionOf(ctx, in.BranchID, it.ProductID, it.VariantID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		label := fmt.Sprintf("produk #%d", it.ProductID)
		if l, ok := labels[it.ProductID]; ok {
			label = l.Code + " " + l.Name
		}
		if p.Qty <= positionEpsilon {
			fails = append(fails, apperror.FieldError{Field: "items", Message: label + ": tidak punya stok di cabang ini"})
			continue
		}
		before := money.Round(p.Total)
		after := money.Round(p.Qty * float64(it.NewUnitCost))
		if after == before {
			fails = append(fails, apperror.FieldError{Field: "items", Message: label + ": nilai persediaan sudah sesuai harga modal baru"})
			continue
		}
		lines = append(lines, RevaluationLine{ProductID: it.ProductID, VariantID: it.VariantID, Qty: p.Qty,
			UnitCostBefore: math.Round(p.Average()*10000) / 10000, UnitCostAfter: float64(it.NewUnitCost),
			ValueBefore: before, ValueAfter: after, Delta: after - before})
	}
	if len(fails) > 0 {
		return nil, apperror.Validation("", fails)
	}
	return lines, nil
}

func itemProductIDs(items []RevaluationItem) []int64 {
	ids := make([]int64, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ProductID)
	}
	return ids
}

func (s *Service) revaluationResult(ctx context.Context, branchID int64, date, reason string, lines []RevaluationLine) *RevaluationResult {
	res := &RevaluationResult{BranchID: branchID, Date: date, Reason: reason, Lines: lines}
	ids := make([]int64, 0, len(lines))
	for _, l := range lines {
		res.TotalDelta += l.Delta
		ids = append(ids, l.ProductID)
	}
	labels := s.productLabels(ctx, ids)
	for i := range res.Lines {
		if l, ok := labels[res.Lines[i].ProductID]; ok {
			res.Lines[i].ProductCode, res.Lines[i].ProductName = l.Code, l.Name
		}
	}
	return res
}

func (s *Service) revaluationByRef(ctx context.Context, branchID int64, ref string) (*RevaluationResult, error) {
	v, err := s.repo.RevaluationByClientRef(ctx, branchID, ref)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if v == nil {
		return nil, nil
	}
	out := s.revaluationView(ctx, []*infrastructure.Revaluation{v})[0]
	out.Replayed = true
	return out, nil
}

// ListRevaluations pages the branch's past revaluations, newest first.
func (s *Service) ListRevaluations(ctx context.Context, branchID int64, page, limit int) ([]*RevaluationResult, int64, error) {
	rows, total, err := s.repo.ListRevaluations(ctx, branchID, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	return s.revaluationView(ctx, rows), total, nil
}

func (s *Service) revaluationView(ctx context.Context, rows []*infrastructure.Revaluation) []*RevaluationResult {
	var ids []int64
	for _, v := range rows {
		for _, l := range v.Lines {
			ids = append(ids, l.ProductID)
		}
	}
	labels := s.productLabels(ctx, ids)
	out := make([]*RevaluationResult, 0, len(rows))
	for _, v := range rows {
		r := &RevaluationResult{ID: v.ID, BranchID: v.BranchID, Date: v.Date, Reason: v.Reason, TotalDelta: v.TotalDelta,
			Applied: true, EntryID: v.JournalID, EntryNumber: v.JournalNo, CreatedBy: v.CreatedBy, CreatedAt: v.CreatedAt,
			Lines: make([]RevaluationLine, 0, len(v.Lines))}
		for _, l := range v.Lines {
			line := RevaluationLine{ProductID: l.ProductID, VariantID: l.VariantID, Qty: l.Qty,
				UnitCostBefore: l.UnitCostBefore, UnitCostAfter: l.UnitCostAfter,
				ValueBefore: l.ValueBefore, ValueAfter: l.ValueAfter, Delta: l.Delta}
			if lb, ok := labels[l.ProductID]; ok {
				line.ProductCode, line.ProductName = lb.Code, lb.Name
			}
			r.Lines = append(r.Lines, line)
		}
		out = append(out, r)
	}
	return out
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
