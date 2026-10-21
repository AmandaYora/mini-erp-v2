package application

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strconv"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/modules/finance/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/doclock"
	"mini-erp/internal/shared/money"
)

// K-05: the cutover journal's "Persediaan" legs are computed by the server from
// the cost ledger (the same positions the inventory-value report reads), never
// typed. The ledger is derived from stock movements — the stock opening import
// is its input — so stock opening → finance opening → inventory report all
// carry the same number. Finance reads only its own ledger here (no stock join).

// SourceOpeningSupplement marks a journal that books the value of positions
// completed after the opening was locked (source_doc_id = the first
// cost-only ledger row it created, so it is unique per completion).
const SourceOpeningSupplement = "opening_supplement"

// MissingCostItem is a held position (qty > 0) the ledger values at zero.
type MissingCostItem struct {
	ProductID   int64   `json:"productId"`
	VariantID   int64   `json:"variantId"`
	ProductCode string  `json:"productCode"`
	ProductName string  `json:"productName"`
	Qty         float64 `json:"qty"`
	// SuggestedCost is the catalog purchase price per base unit (0 = none).
	SuggestedCost int64 `json:"suggestedCost"`
}

// OpeningInventory is what the cutover journal will book as Persediaan.
type OpeningInventory struct {
	// Total is Σ round(qty × average cost) over held positions — exactly the
	// inventory-value report's total.
	Total     int64 `json:"total"`
	Positions int   `json:"positions"`
	// Estimated counts positions whose cost came from the catalog price rather
	// than an explicit/receipt cost.
	Estimated   int               `json:"estimated"`
	MissingCost []MissingCostItem `json:"missingCost"`
	// OpeningPosted tells the UI whether "lengkapi" (complete) is the next step.
	OpeningPosted bool `json:"openingPosted"`
}

type openingPosition struct {
	productID, variantID int64
	qty                  float64
	value                int64
}

// openingPositions syncs the cost ledger and returns the valued positions plus
// the held-but-unvalued ones.
func (s *Service) openingPositions(ctx context.Context, branchID int64) (valued []openingPosition, missing []openingPosition, err error) {
	if _, err := s.Sync(ctx, branchID); err != nil {
		return nil, nil, err
	}
	positions, err := s.repo.CostPositions(ctx, branchID)
	if err != nil {
		return nil, nil, apperror.Internal(err)
	}
	for _, p := range positions {
		if p.Qty <= 1e-9 {
			continue
		}
		v := money.Round(p.Qty * p.Average())
		op := openingPosition{productID: p.ProductID, variantID: p.VariantID, qty: p.Qty, value: v}
		if v > 0 {
			valued = append(valued, op)
		} else {
			missing = append(missing, op)
		}
	}
	less := func(a []openingPosition) {
		sort.Slice(a, func(i, j int) bool {
			if a[i].productID == a[j].productID {
				return a[i].variantID < a[j].variantID
			}
			return a[i].productID < a[j].productID
		})
	}
	less(valued)
	less(missing)
	return valued, missing, nil
}

// OpeningInventory reports the server-computed inventory for the cutover
// (GET /finance/opening/inventory): total, estimated count, and the products
// held without any cost.
func (s *Service) OpeningInventory(ctx context.Context, branchID int64) (*OpeningInventory, error) {
	valued, missing, err := s.openingPositions(ctx, branchID)
	if err != nil {
		return nil, err
	}
	out := &OpeningInventory{Positions: len(valued), MissingCost: []MissingCostItem{}}
	for _, p := range valued {
		out.Total += p.value
	}
	if pairs, err := s.repo.EstimatedProducts(ctx, branchID); err != nil {
		return nil, apperror.Internal(err)
	} else {
		held := map[[2]int64]bool{}
		for _, p := range valued {
			held[[2]int64{p.productID, p.variantID}] = true
		}
		for _, pv := range pairs {
			if held[pv] {
				out.Estimated++
			}
		}
	}
	ids := make([]int64, 0, len(missing))
	for _, p := range missing {
		ids = append(ids, p.productID)
	}
	labels := s.productLabels(ctx, ids)
	catalog := map[int64]float64{}
	if len(ids) > 0 && s.products != nil {
		if c, err := s.products.PurchaseBaseCostsByIDs(ctx, ids); err == nil && c != nil {
			catalog = c
		}
	}
	for _, p := range missing {
		it := MissingCostItem{ProductID: p.productID, VariantID: p.variantID, Qty: p.qty,
			SuggestedCost: int64(math.Round(catalog[p.productID]))}
		if l, ok := labels[p.productID]; ok {
			it.ProductCode, it.ProductName = l.Code, l.Name
		}
		out.MissingCost = append(out.MissingCost, it)
	}
	posted, err := s.OpeningPosted(ctx, branchID)
	if err != nil {
		return nil, err
	}
	out.OpeningPosted = posted
	return out, nil
}

// openingAccounts resolves the two mapped accounts the server-computed legs use.
func (s *Service) openingAccounts(ctx context.Context) (inventory, equity *contracts.Account, err error) {
	maps, err := s.repo.ListMappings(ctx)
	if err != nil {
		return nil, nil, apperror.Internal(err)
	}
	inventory, equity = maps["inventory"], maps["opening_equity"]
	for key, a := range map[string]*contracts.Account{"inventory": inventory, "opening_equity": equity} {
		if a == nil || a.Status != "active" {
			return nil, nil, apperror.Validation("", []apperror.FieldError{{Field: "lines",
				Message: fmt.Sprintf("pemetaan akun '%s' belum diatur", key)}})
		}
	}
	return inventory, equity, nil
}

// serverOpeningLegs builds the computed legs: one Dr Persediaan per held,
// costed position (product dimension kept for per-product reports) and one
// Cr Modal Awal for the whole amount.
func serverOpeningLegs(inventory, equity *contracts.Account, valued []openingPosition) ([]*contracts.JournalLine, int64) {
	var total int64
	legs := make([]*contracts.JournalLine, 0, len(valued)+1)
	for _, p := range valued {
		legs = append(legs, &contracts.JournalLine{AccountID: inventory.ID, Debit: p.value,
			ProductID: p.productID, VariantID: p.variantID, Description: "Saldo awal persediaan"})
		total += p.value
	}
	if total > 0 {
		legs = append(legs, &contracts.JournalLine{AccountID: equity.ID, Credit: total,
			Description: "Modal awal untuk persediaan"})
	}
	return legs, total
}

// CompleteItem asks to price one held-but-uncosted position.
type CompleteItem struct {
	ProductID int64
	VariantID int64
	// UnitCost per base unit in whole rupiah; 0 = the catalog purchase price.
	UnitCost int64
}

// CompleteResult is the outcome of CompleteOpening.
type CompleteResult struct {
	Entry *contracts.JournalEntry
	Total int64
}

// CompleteOpening is the post-lock path of "Lengkapi Stok Awal" (legacy
// supplementOpening): after the cutover journal is final, positions that were
// held without a cost get one. Additive and safe — only positions valued at
// zero are touched, never one that already carries value. It writes a
// cost-only ledger row per position and ONE supplement journal
// (Dr Persediaan per product / Cr Modal Awal) in a single transaction, so
// books and ledger move together and the reconciliation stays at zero.
func (s *Service) CompleteOpening(ctx context.Context, actorID, branchID int64, items []CompleteItem) (*CompleteResult, error) {
	if len(items) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: "wajib diisi"}})
	}
	if posted, err := s.OpeningPosted(ctx, branchID); err != nil {
		return nil, err
	} else if !posted {
		return nil, apperror.Conflict("Saldo awal belum diposting; lengkapi harga beli produk lalu pratinjau saldo awal")
	}
	inventory, equity, err := s.openingAccounts(ctx)
	if err != nil {
		return nil, err
	}
	var res *CompleteResult
	err = doclock.Lock("fincost:"+strconv.FormatInt(branchID, 10), func() error {
		r, err := s.completeLocked(ctx, actorID, branchID, items, inventory, equity)
		res = r
		return err
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *Service) completeLocked(ctx context.Context, actorID, branchID int64, items []CompleteItem, inventory, equity *contracts.Account) (*CompleteResult, error) {
	// Bring the ledger up to date before judging positions (Sync takes the same
	// lock, so it runs through syncLocked here).
	if _, err := s.syncLocked(ctx, branchID); err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ProductID)
	}
	catalog := map[int64]float64{}
	if s.products != nil {
		c, err := s.products.PurchaseBaseCostsByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		if c != nil {
			catalog = c
		}
	}
	type priced struct {
		item  CompleteItem
		unit  float64
		value int64
	}
	var rows []priced
	seen := map[string]bool{}
	var failures []apperror.FieldError
	for i, it := range items {
		key := infrastructure.CostKeyOf(it.ProductID, it.VariantID)
		bad := func(msg string) {
			failures = append(failures, apperror.FieldError{Field: "items", Message: fmt.Sprintf("baris %d: %s", i+1, msg)})
		}
		if seen[key] {
			bad("produk muncul dua kali")
			continue
		}
		seen[key] = true
		p, err := s.repo.CostPositionOf(ctx, branchID, it.ProductID, it.VariantID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if p.Qty <= 1e-9 {
			bad("produk tidak punya stok")
			continue
		}
		if money.Round(p.Total) > 0 {
			bad("produk sudah punya harga modal")
			continue
		}
		unit := float64(it.UnitCost)
		if unit <= 0 {
			unit = catalog[it.ProductID]
		}
		if unit <= 0 {
			bad("harga modal belum tersedia (isi harga beli di master produk atau kirim unitCost)")
			continue
		}
		v := money.Round(p.Qty * unit)
		if v <= 0 {
			bad("nilai persediaan nol")
			continue
		}
		rows = append(rows, priced{item: it, unit: unit, value: v})
	}
	if len(failures) > 0 {
		return nil, apperror.Validation("", failures)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].item.ProductID == rows[j].item.ProductID {
			return rows[i].item.VariantID < rows[j].item.VariantID
		}
		return rows[i].item.ProductID < rows[j].item.ProductID
	})
	ledgerIDs, err := s.repo.NextSyntheticCostIDs(ctx, len(rows))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var total int64
	lines := make([]*contracts.JournalLine, 0, len(rows)+1)
	for _, r := range rows {
		lines = append(lines, &contracts.JournalLine{AccountID: inventory.ID, Debit: r.value,
			ProductID: r.item.ProductID, VariantID: r.item.VariantID, Description: "Lengkapi harga modal saldo awal"})
		total += r.value
	}
	lines = append(lines, &contracts.JournalLine{AccountID: equity.ID, Credit: total, Description: "Modal awal untuk persediaan"})

	date, err := normalizeEntryDate("", "date")
	if err != nil {
		return nil, err
	}
	if err := s.requireOpenPeriod(ctx, branchID, date); err != nil {
		return nil, err
	}
	b, err := s.branches.GetByID(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if b == nil {
		return nil, apperror.NotFound("Cabang")
	}
	entryID, err := s.repo.CreateEntry(ctx, &contracts.JournalEntry{
		BranchID: branchID, Date: date, Memo: "Lengkapi harga modal saldo awal",
		Numbering:  &contracts.Numbering{Prefix: "JV", BranchCode: b.Code, Year: date[:4]},
		SourceType: SourceOpeningSupplement, SourceID: ledgerIDs[0], Lines: lines,
	}, actorID, func(ctx context.Context, tx *sql.Tx, id int64, number string) error {
		for i, r := range rows {
			if err := s.repo.InsertCostValueTx(ctx, tx, branchID, r.item.ProductID, r.item.VariantID,
				ledgerIDs[i], r.unit, r.value, date); err != nil {
				return err
			}
		}
		return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
			Action: "journals.opening_complete", Entity: "journal", EntityID: id, BranchID: branchID, ActorID: actorID, Note: number,
			Metadata: map[string]any{"positions": len(rows), "total": total},
		})
	})
	if err != nil {
		return nil, dberr.Map(err)
	}
	entry, err := s.repo.GetEntry(ctx, entryID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &CompleteResult{Entry: entry, Total: total}, nil
}

// InventoryRecon compares the books' Persediaan balance with the cost
// ledger's value (K-05 / F-05). A drift means a journal was reversed by hand,
// stock moved without being journalled, or an imported opening disagrees with
// the imported stock.
type InventoryRecon struct {
	Book   int64 `json:"book"`
	Ledger int64 `json:"ledger"`
	Diff   int64 `json:"diff"` // book − ledger
	// Tolerance is the rounding allowance: journals round each leg to a whole
	// rupiah while the ledger keeps fractions.
	Tolerance int64 `json:"tolerance"`
	// Pending counts documents not yet journalled; while any exist the gap can
	// be explained by them, so the check degrades from blocker to warning.
	Pending     int `json:"pending"`
	MissingCost int `json:"missingCost"`
	// TransferNet is the ledger value cross-branch transfers moved into (+) or
	// out of (-) this branch. Transfers carry no journal by design, so it is
	// excluded from Ledger before comparing.
	TransferNet int64 `json:"transferNet"`
	// OpeningUnposted is the opening-stock value awaiting the finance opening
	// journal; excluded from Ledger until that journal exists.
	OpeningUnposted int64 `json:"openingUnposted"`
	Consistent      bool  `json:"consistent"`
}

// ReconcileInventory computes the book-vs-ledger comparison for a branch.
func (s *Service) ReconcileInventory(ctx context.Context, branchID int64) (*InventoryRecon, error) {
	valued, missing, err := s.openingPositions(ctx, branchID)
	if err != nil {
		return nil, err
	}
	var ledger int64
	for _, p := range valued {
		ledger += p.value
	}
	transfer, opening, err := s.repo.UnjournaledCostValue(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	transferNet := int64(math.Round(transfer))
	var openingUnposted int64
	if posted, err := s.OpeningPosted(ctx, branchID); err != nil {
		return nil, err
	} else if !posted {
		openingUnposted = int64(math.Round(opening))
	}
	// What the books are expected to hold: the ledger minus value that moved
	// without a journal on purpose.
	ledger -= transferNet + openingUnposted
	book, err := s.inventoryBookBalance(ctx, branchID)
	if err != nil {
		return nil, err
	}
	queue, err := s.UnpostedSources(ctx, branchID)
	if err != nil {
		return nil, err
	}
	pending := len(queue)
	if n, err := s.pendingStockMovements(ctx, branchID); err != nil {
		return nil, err
	} else {
		pending += n
	}
	rec := &InventoryRecon{Book: book, Ledger: ledger, Diff: book - ledger,
		Tolerance: 100 + int64(len(valued)), Pending: pending, MissingCost: len(missing),
		TransferNet: transferNet, OpeningUnposted: openingUnposted}
	abs := rec.Diff
	if abs < 0 {
		abs = -abs
	}
	rec.Consistent = abs <= rec.Tolerance
	return rec, nil
}

// inventoryBookBalance is the net (debit − credit) of the mapped Persediaan
// account for exactly this branch.
func (s *Service) inventoryBookBalance(ctx context.Context, branchID int64) (int64, error) {
	maps, err := s.repo.ListMappings(ctx)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	a := maps["inventory"]
	if a == nil {
		return 0, nil
	}
	footings, err := s.repo.AccountFootings(ctx, branchID, "", "")
	if err != nil {
		return 0, apperror.Internal(err)
	}
	f := footings[a.ID]
	return f[0] - f[1], nil
}

// pendingStockMovements counts adjustment / write-off movements without a live
// journal yet (they move the ledger before the books).
func (s *Service) pendingStockMovements(ctx context.Context, branchID int64) (int, error) {
	rows, err := s.repo.UnpostedStockMovements(ctx, branchID, 0, 1)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return len(rows), nil
}
