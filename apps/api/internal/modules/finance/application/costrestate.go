package application

import (
	"context"
	"errors"
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

// OP-04: cost restatement (legacy `cost:rebuild`). The cost ledger prices every
// outflow at the moving average of the moment it was ingested. When the chain
// underneath changed (a late-committing movement, a repaired inflow cost) those
// outflow prices go stale and so do the journals booked from them. This tool
// replays each position's chain from the ledger itself, reports what every
// outflow SHOULD have cost, and — on commit — rewrites those rows and books ONE
// balanced correction journal (Persediaan against HPP) so the K-05
// reconciliation stays at zero.
//
// Inputs that are never restated: inflows (their cost is evidence — a receipt's
// PO economics, an opening cost, a cost-only supplement row) and cross-branch
// transfers (the other branch's ledger carries the same cost and no journal
// exists for them by design). Only finance's own ledger table is read; no stock
// data is needed, so the tool stays inside the module boundary.

// SourceCostRestatement marks the correction journal.
const SourceCostRestatement = "cost_restatement"

// restateTolerance is the smallest total drift (whole rupiah) worth correcting;
// below it the difference is storage rounding noise (decimal(18,2) totals).
const restateTolerance = 0.5

// RestateGate is one pre/post condition of a restatement.
type RestateGate struct {
	Key    string `json:"key"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// RestateProduct is the restated difference of one position.
type RestateProduct struct {
	ProductID   int64   `json:"productId"`
	VariantID   int64   `json:"variantId"`
	ProductCode string  `json:"productCode"`
	ProductName string  `json:"productName"`
	Rows        int     `json:"rows"`
	ValueBefore float64 `json:"valueBefore"`
	ValueAfter  float64 `json:"valueAfter"`
	// Delta is the whole-rupiah change of the position's value (after − before):
	// positive books Dr Persediaan / Cr HPP, negative the reverse.
	Delta int64 `json:"delta"`
}

// RestateSkipped is a position left untouched on purpose.
type RestateSkipped struct {
	ProductID int64  `json:"productId"`
	VariantID int64  `json:"variantId"`
	Reason    string `json:"reason"`
}

// RestateReport is the outcome of a dry run or a commit.
type RestateReport struct {
	Commit      bool             `json:"commit"`
	BranchID    int64            `json:"branchId"`
	Positions   int              `json:"positions"`
	RowsChanged int              `json:"rowsChanged"`
	Products    []RestateProduct `json:"products"`
	Skipped     []RestateSkipped `json:"skipped"`
	Delta       int64            `json:"delta"`
	Gates       []RestateGate    `json:"gates"`
	EntryID     int64            `json:"entryId,omitempty"`
	EntryNumber string           `json:"entryNumber,omitempty"`
	ReconBefore *InventoryRecon  `json:"reconBefore,omitempty"`
	ReconAfter  *InventoryRecon  `json:"reconAfter,omitempty"`
	Applied     bool             `json:"applied"`
	Date        string           `json:"date"`
	rowUpdates  []restatedRow
}

type restatedRow struct {
	id    int64
	unit  float64
	total float64
}

// RestateCostOptions drives RestateCost.
type RestateCostOptions struct {
	BranchID int64
	ActorID  int64 // 0 = operator console (system)
	Commit   bool  // false = dry run: nothing is written
	Date     string
}

// RestateCost recomputes the branch's outflow costs from its ledger. Dry run
// (the default) only reports; Commit applies it behind the gates.
func (s *Service) RestateCost(ctx context.Context, o RestateCostOptions) (*RestateReport, error) {
	date, err := normalizeEntryDate(o.Date, "date")
	if err != nil {
		return nil, err
	}
	// Reconcile before taking the costing lock (it syncs the ledger itself).
	before, err := s.ReconcileInventory(ctx, o.BranchID)
	if err != nil {
		return nil, err
	}
	var rep *RestateReport
	err = doclock.Lock("fincost:"+strconv.FormatInt(o.BranchID, 10), func() error {
		r, err := s.restateLocked(ctx, o, date, before)
		rep = r
		return err
	})
	if err != nil {
		return rep, err
	}
	if rep.Applied {
		after, err := s.ReconcileInventory(ctx, o.BranchID)
		if err != nil {
			return rep, err
		}
		rep.ReconAfter = after
	}
	return rep, nil
}

func (s *Service) restateLocked(ctx context.Context, o RestateCostOptions, date string, before *InventoryRecon) (*RestateReport, error) {
	if _, err := s.syncLocked(ctx, o.BranchID); err != nil {
		return nil, err
	}
	rows, err := s.repo.CostRowsOrdered(ctx, o.BranchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	rep := &RestateReport{Commit: o.Commit, BranchID: o.BranchID, Date: date, ReconBefore: before,
		Products: []RestateProduct{}, Skipped: []RestateSkipped{}, Gates: []RestateGate{}}
	s.replayChains(rows, rep)
	s.decorateRestate(ctx, rep)

	// Gates. Each is reported on a dry run too, so the operator sees what a
	// commit would run into.
	debit, credit, unbalanced, err := s.repo.TrialBalance(ctx, o.BranchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	balanced := debit == credit && unbalanced == 0
	rep.Gates = append(rep.Gates, RestateGate{Key: "trial_balance", OK: balanced,
		Detail: fmt.Sprintf("debit %d, kredit %d, jurnal tidak seimbang %d", debit, credit, unbalanced)})
	rep.Gates = append(rep.Gates, RestateGate{Key: "inventory_recon", OK: before.Consistent,
		Detail: fmt.Sprintf("buku %d, ledger %d, selisih %d (toleransi %d)", before.Book, before.Ledger, before.Diff, before.Tolerance)})
	rep.Gates = append(rep.Gates, RestateGate{Key: "pending_documents", OK: before.Pending == 0,
		Detail: fmt.Sprintf("%d dokumen/mutasi belum dijurnal (jurnalkan atau abaikan dulu: nilainya ikut dibaca dari ledger yang akan dikoreksi)", before.Pending)})
	openPeriodErr := s.requireOpenPeriod(ctx, o.BranchID, date)
	rep.Gates = append(rep.Gates, RestateGate{Key: "period_open", OK: openPeriodErr == nil, Detail: "periode " + date[:7]})

	if !o.Commit || rep.RowsChanged == 0 {
		return rep, nil
	}
	for _, g := range rep.Gates {
		if !g.OK {
			return rep, apperror.Conflict("Restatement ditolak, gerbang '" + g.Key + "' gagal: " + g.Detail)
		}
	}
	if err := s.commitRestatement(ctx, o, date, rep); err != nil {
		return rep, err
	}
	return rep, nil
}

// replayChains walks every position in chain order and fills rep with the
// restated outflows. Positions that would end negative are skipped whole: a
// negative result is a symptom of something else (an oversell, a missing
// inflow) and restating it would only move the problem.
func (s *Service) replayChains(rows []infrastructure.RestateRow, rep *RestateReport) {
	i := 0
	for i < len(rows) {
		j := i
		for j < len(rows) && rows[j].ProductID == rows[i].ProductID && rows[j].VariantID == rows[i].VariantID {
			j++
		}
		chain := rows[i:j]
		i = j
		rep.Positions++

		var qty, total, valueBefore float64
		var updates []restatedRow
		var drift float64
		for _, r := range chain {
			valueBefore += r.Total
			if r.Direction == "in" {
				qty += r.Qty
				total += r.Total
				continue
			}
			newTotal := r.Total
			if !transferRef(r.RefType) && qty > 1e-9 {
				avg := total / qty
				u := math.Round(avg*10000) / 10000
				t := -math.Round(r.Qty*avg*100) / 100
				if math.Abs(t-r.Total) >= restateTolerance {
					newTotal = t
					updates = append(updates, restatedRow{id: r.ID, unit: u, total: t})
					drift += t - r.Total
				}
			}
			qty -= r.Qty
			total += newTotal
		}
		if len(updates) == 0 {
			continue
		}
		p := RestateProduct{ProductID: chain[0].ProductID, VariantID: chain[0].VariantID,
			Rows: len(updates), ValueBefore: valueBefore, ValueAfter: valueBefore + drift}
		if qty < -1e-9 || total < -restateTolerance {
			rep.Skipped = append(rep.Skipped, RestateSkipped{ProductID: p.ProductID, VariantID: p.VariantID,
				Reason: fmt.Sprintf("hasil akhir negatif (qty %.4f, nilai %.2f); di luar cakupan restatement, periksa terpisah", qty, total)})
			continue
		}
		p.Delta = money.Round(drift)
		rep.RowsChanged += len(updates)
		rep.rowUpdates = append(rep.rowUpdates, updates...)
		rep.Products = append(rep.Products, p)
		rep.Delta += p.Delta
	}
}

func transferRef(refType string) bool {
	return len(refType) >= 8 && refType[:8] == "transfer"
}

// decorateRestate names the products of the report.
func (s *Service) decorateRestate(ctx context.Context, rep *RestateReport) {
	ids := make([]int64, 0, len(rep.Products))
	for _, p := range rep.Products {
		ids = append(ids, p.ProductID)
	}
	labels := s.productLabels(ctx, ids)
	for i := range rep.Products {
		if l, ok := labels[rep.Products[i].ProductID]; ok {
			rep.Products[i].ProductCode, rep.Products[i].ProductName = l.Code, l.Name
		}
	}
}

func (s *Service) commitRestatement(ctx context.Context, o RestateCostOptions, date string, rep *RestateReport) error {
	maps, err := s.repo.ListMappings(ctx)
	if err != nil {
		return apperror.Internal(err)
	}
	inventory, cogs := maps["inventory"], maps["cogs"]
	if inventory == nil || cogs == nil || inventory.Status != "active" || cogs.Status != "active" {
		return apperror.Validation("", []apperror.FieldError{{Field: "mapping",
			Message: "pemetaan akun 'inventory' dan 'cogs' wajib aktif"}})
	}
	sort.Slice(rep.Products, func(i, j int) bool {
		if rep.Products[i].ProductID == rep.Products[j].ProductID {
			return rep.Products[i].VariantID < rep.Products[j].VariantID
		}
		return rep.Products[i].ProductID < rep.Products[j].ProductID
	})
	var legs []*contracts.JournalLine
	for _, p := range rep.Products {
		if p.Delta == 0 {
			continue
		}
		amt := p.Delta
		invDebit := amt > 0
		if amt < 0 {
			amt = -amt
		}
		inv := &contracts.JournalLine{AccountID: inventory.ID, ProductID: p.ProductID, VariantID: p.VariantID,
			Description: "Restatement biaya persediaan"}
		cg := &contracts.JournalLine{AccountID: cogs.ID, ProductID: p.ProductID, VariantID: p.VariantID,
			Description: "Restatement biaya persediaan"}
		if invDebit {
			inv.Debit, cg.Credit = amt, amt
		} else {
			inv.Credit, cg.Debit = amt, amt
		}
		legs = append(legs, inv, cg)
	}
	updates := make([]infrastructure.RestateUpdate, 0, len(rep.rowUpdates))
	for _, u := range rep.rowUpdates {
		updates = append(updates, infrastructure.RestateUpdate{ID: u.id, Unit: u.unit, Total: u.total})
	}
	summary := map[string]any{"rows": rep.RowsChanged, "positions": len(rep.Products), "delta": rep.Delta,
		"skipped": len(rep.Skipped), "date": date}
	top := rep.Products
	if len(top) > 20 {
		top = top[:20]
	}
	detail := make([]map[string]any, 0, len(top))
	for _, p := range top {
		detail = append(detail, map[string]any{"productId": p.ProductID, "variantId": p.VariantID,
			"before": p.ValueBefore, "after": p.ValueAfter, "delta": p.Delta})
	}
	summary["products"] = detail
	audit := func(ctx context.Context, tx auditcontracts.AuditTx, entryID int64, number string) error {
		return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
			Action: "finance.cost_restatement", Entity: "cost_ledger", EntityID: entryID, BranchID: o.BranchID,
			ActorID: o.ActorID, ActorType: actorType(o.ActorID), Note: number, Metadata: summary,
		})
	}

	var entry *contracts.JournalEntry
	if len(legs) > 0 {
		b, err := s.branches.GetByID(ctx, o.BranchID)
		if err != nil {
			return apperror.Internal(err)
		}
		if b == nil {
			return apperror.NotFound("Cabang")
		}
		sourceID, err := s.repo.NextSourceID(ctx, o.BranchID, SourceCostRestatement)
		if err != nil {
			return apperror.Internal(err)
		}
		entry = &contracts.JournalEntry{
			BranchID: o.BranchID, Date: date,
			Memo:       fmt.Sprintf("Restatement biaya persediaan (%d baris, %d produk)", rep.RowsChanged, len(rep.Products)),
			Numbering:  &contracts.Numbering{Prefix: "JV", BranchCode: b.Code, Year: date[:4]},
			SourceType: SourceCostRestatement, SourceID: sourceID, Lines: legs,
		}
	}
	id, err := s.repo.CommitRestatement(ctx, o.BranchID, o.ActorID, entry, updates, audit)
	if err != nil {
		var tb *infrastructure.TrialBalanceError
		if errors.As(err, &tb) {
			return apperror.Conflict("Restatement dibatalkan: " + tb.Error())
		}
		return dberr.Map(err)
	}
	rep.Applied = true
	if entry != nil {
		rep.EntryID, rep.EntryNumber = id, entry.Number
	}
	return nil
}

func actorType(actorID int64) string {
	if actorID == 0 {
		return "system"
	}
	return "user"
}
