package application

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/modules/finance/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
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
	err = s.costLock(ctx, branchID, costLockWaitRestate, func() error {
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
// ledger's value (K-05 / F-05). Every gap between them must be explained by a
// named cause; whatever is left (Unexplained) means a journal was reversed by
// hand, stock moved without being journalled, or an imported opening
// disagrees with the imported stock — and the check fails.
type InventoryRecon struct {
	Book   int64 `json:"book"`
	Ledger int64 `json:"ledger"`
	Diff   int64 `json:"diff"` // book − ledger
	// Tolerance is the rounding allowance: journals post whole rupiah per leg;
	// ledger rows are whole rupiah too, but imported legacy rows and receipts
	// split over several locations can carry a fraction.
	Tolerance int64 `json:"tolerance"`
	// Pending counts documents still waiting for a journal (ignored ones
	// excluded); PendingValue is the ledger value they already moved (the books
	// catch up when they are posted).
	Pending      int   `json:"pending"`
	PendingValue int64 `json:"pendingValue"`
	// IgnoredValue is the ledger value of documents an accountant took out of
	// the posting queue on purpose: a permanent, deliberate gap.
	IgnoredValue int64 `json:"ignoredValue"`
	MissingCost  int   `json:"missingCost"`
	// EmptyValued counts positions with no quantity left that still hold
	// value (EmptyValue in total): a cancellation is priced from its original
	// document, so it can leave a remainder in an emptied position. The books
	// hold that value too, so it is part of Ledger — reported, not a gap.
	EmptyValued int   `json:"emptyValued"`
	EmptyValue  int64 `json:"emptyValue"`
	// TransferNet is the ledger value cross-branch transfers moved into (+) or
	// out of (-) this branch. Transfers carry no journal by design, so it is
	// excluded from Ledger before comparing.
	TransferNet int64 `json:"transferNet"`
	// OpeningUnposted is the opening-stock value awaiting the finance opening
	// journal; excluded from Ledger until that journal exists.
	OpeningUnposted int64 `json:"openingUnposted"`
	// Unexplained is Diff after the pending and ignored documents: the part
	// no known cause accounts for. Consistent = |Unexplained| <= Tolerance.
	Unexplained int64 `json:"unexplained"`
	Consistent  bool  `json:"consistent"`
}

// ReconcileInventory computes the book-vs-ledger comparison for a branch.
func (s *Service) ReconcileInventory(ctx context.Context, branchID int64) (*InventoryRecon, error) {
	valued, missing, err := s.openingPositions(ctx, branchID)
	if err != nil {
		return nil, err
	}
	// The ledger side is EVERY position's value, not only the valued stock
	// the opening reads: the books carry the remainder an emptied position can
	// keep (or the negative value a reverse-priced cancellation leaves), so
	// leaving those out would report a gap that is not there.
	positions, err := s.repo.CostPositions(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var ledger, emptyValue int64
	emptyValued := 0
	for _, p := range positions {
		v := money.Round(p.Total)
		ledger += v
		if math.Abs(p.Qty) <= 1e-9 && v != 0 {
			emptyValued++
			emptyValue += v
		}
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
	// Pending is what still waits for a journal: the queue (which already holds
	// the stock movements after the cutover boundary) minus the documents an
	// accountant ignored on purpose — those are reported by their value.
	pending := 0
	for _, it := range queue {
		if !it.Ignored {
			pending++
		}
	}
	pendingValue, ignoredValue, err := s.queueLedgerValue(ctx, branchID, queue)
	if err != nil {
		return nil, err
	}
	rec := &InventoryRecon{Book: book, Ledger: ledger, Diff: book - ledger,
		Tolerance: 100 + int64(len(valued)), Pending: pending, PendingValue: pendingValue, IgnoredValue: ignoredValue,
		MissingCost: len(missing), EmptyValued: emptyValued, EmptyValue: emptyValue,
		TransferNet: transferNet, OpeningUnposted: openingUnposted}
	rec.Unexplained = rec.Diff + pendingValue + ignoredValue
	rec.Consistent = absInt64(rec.Unexplained) <= rec.Tolerance
	return rec, nil
}

// queueLedgerValue sums what the unjournalled documents of the queue already
// moved in the cost ledger (signed: + added stock value, − relieved it),
// split into still-pending and deliberately ignored documents. Only documents
// that move stock carry ledger value; the rest add nothing.
func (s *Service) queueLedgerValue(ctx context.Context, branchID int64, queue []QueueItem) (pending, ignored int64, err error) {
	for _, it := range queue {
		var v float64
		switch it.DocType {
		case SourceGoodsReceipt, SourceDelivery, SourceSalesReturn, SourcePurchaseReturn:
			moves, err := s.repo.MoveValues(ctx, branchID, it.DocType, it.DocID)
			if err != nil {
				return 0, 0, apperror.Internal(err)
			}
			for _, m := range moves {
				v += m.Value
			}
		case SourceStockMovement:
			row, err := s.repo.CostRowByMovement(ctx, branchID, it.DocID)
			if err != nil {
				return 0, 0, apperror.Internal(err)
			}
			if row != nil {
				v = row.TotalCost
			}
		default:
			continue
		}
		if it.Ignored {
			ignored += int64(math.Round(v))
		} else {
			pending += int64(math.Round(v))
		}
	}
	return pending, ignored, nil
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
