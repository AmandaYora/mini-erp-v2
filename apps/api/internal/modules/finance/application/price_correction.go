package application

import (
	"context"
	"database/sql"
	"fmt"
	"math"

	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/modules/finance/infrastructure"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/money"
	"mini-erp/internal/shared/timeutil"
)

// SourcePurchasePriceCorrection journals a purchase price correction after
// receipt (ADR-0022): source_doc_id is the purchasing correction id. The
// received part's value change splits between Persediaan (the units still on
// hand, with a value-only cost-ledger row of exactly the same amount) and HPP
// (the units already issued); PPN Masukan takes the tax change and Hutang the
// total, on the supplier.
const SourcePurchasePriceCorrection = "purchase_price_correction"

// RequireOpenPeriod implements contracts.PurchaseCorrectionGate.
func (s *Service) RequireOpenPeriod(ctx context.Context, branchID int64, date string) error {
	return s.requireOpenPeriod(ctx, branchID, date)
}

// SplitReceivedValue implements contracts.PurchaseCorrectionGate (preview).
// The ledger is synced first so the positions are current.
func (s *Service) SplitReceivedValue(ctx context.Context, branchID int64, items []contracts.ReceivedValueDelta) ([]contracts.ReceivedValueDelta, error) {
	if err := s.drainCosting(ctx, branchID); err != nil {
		return nil, err
	}
	return s.splitReceived(ctx, branchID, items)
}

// splitReceived splits each position's value change by the units still on
// hand: the on-hand share is min(position qty, received qty) of the received
// qty (moving average: the received units merged into the position, and every
// issue since relieved its share at the old cost). The stock share never takes
// the position's value below zero; whatever it cannot carry goes to HPP.
func (s *Service) splitReceived(ctx context.Context, branchID int64, items []contracts.ReceivedValueDelta) ([]contracts.ReceivedValueDelta, error) {
	out := make([]contracts.ReceivedValueDelta, 0, len(items))
	for _, d := range items {
		p, err := s.repo.CostPositionOf(ctx, branchID, d.ProductID, d.VariantID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		d.OnHandQty = math.Max(0, math.Min(p.Qty, d.ReceivedQtyBase))
		d.Stock = 0
		if d.ReceivedQtyBase > positionEpsilon && d.OnHandQty > positionEpsilon {
			d.Stock = money.Prorate(d.Delta, d.OnHandQty, d.ReceivedQtyBase)
		}
		if value := money.Round(p.Total); d.Stock < 0 && value+d.Stock < 0 {
			d.Stock = -max(value, 0)
		}
		d.Issued = d.Delta - d.Stock
		out = append(out, d)
	}
	return out, nil
}

// signedLeg books a signed amount on the debit (+) or credit (−) side.
func signedLeg(key string, amount int64, productID, variantID, partyID int64, note string) rawLeg {
	l := rawLeg{key: key, productID: productID, variantID: variantID, partyID: partyID, note: note}
	if amount >= 0 {
		l.debit = amount
	} else {
		l.credit = -amount
	}
	return l
}

// buildPurchasePriceCorrection: per position Dr/Cr Persediaan (the on-hand
// share) and Dr/Cr HPP (the issued share), service lines Dr/Cr Beban, Dr/Cr PPN
// Masukan, and Cr/Dr Hutang on the supplier for the sum — balanced by
// construction (the split of each position adds up to its delta).
func (s *Service) buildPurchasePriceCorrection(ctx context.Context, branchID, id int64) ([]*contracts.JournalLine, string, string, error) {
	c, err := s.purchasing.GetCorrection(ctx, id)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	if c == nil || c.BranchID != branchID || c.Kind != purchasingcontracts.CorrectionPrice {
		return nil, "", "", apperror.NotFound("Koreksi harga beli")
	}
	type pos struct {
		productID, variantID int64
		name                 string
	}
	var order []pos
	deltas := map[pos]*contracts.ReceivedValueDelta{}
	legs := make([]rawLeg, 0, len(c.Lines)*2+2)
	var payable int64
	for _, l := range c.Lines {
		if l.ValueDelta == 0 {
			continue
		}
		payable += l.ValueDelta
		if !l.StockTracked {
			// A purchased service was expensed at receipt: so is its correction.
			legs = append(legs, signedLeg("expense", l.ValueDelta, 0, 0, 0, "Koreksi harga jasa "+l.ProductName))
			continue
		}
		k := pos{l.ProductID, l.VariantID, l.ProductName}
		d := deltas[k]
		if d == nil {
			d = &contracts.ReceivedValueDelta{ProductID: l.ProductID, VariantID: l.VariantID}
			deltas[k] = d
			order = append(order, k)
		}
		d.ReceivedQtyBase += l.ReceivedQtyBase
		d.Delta += l.ValueDelta
	}
	items := make([]contracts.ReceivedValueDelta, 0, len(order))
	for _, k := range order {
		items = append(items, *deltas[k])
	}
	split, err := s.splitReceived(ctx, branchID, items)
	if err != nil {
		return nil, "", "", err
	}
	for i, d := range split {
		name := order[i].name
		if d.Stock != 0 {
			legs = append(legs, signedLeg("inventory", d.Stock, d.ProductID, d.VariantID, 0, "Koreksi harga beli "+name))
		}
		if d.Issued != 0 {
			legs = append(legs, signedLeg("cogs", d.Issued, d.ProductID, d.VariantID, 0, "Koreksi harga beli (sudah keluar) "+name))
		}
	}
	var tax int64
	for _, l := range c.Lines {
		tax += l.TaxDelta
	}
	if tax != 0 {
		legs = append(legs, signedLeg("ppn_in", tax, 0, 0, 0, "PPN Masukan koreksi "+c.OrderNumber))
	}
	payable += tax
	if payable == 0 {
		return nil, "", "", apperror.Conflict("Nilai koreksi harga nol")
	}
	legs = append(legs, signedLeg("payable", -payable, 0, 0, c.PartyID, c.PartyName))
	lines, err := s.finish(ctx, legs)
	if err != nil {
		return nil, "", "", err
	}
	return lines, fmt.Sprintf("Koreksi harga beli %s: %s", c.OrderNumber, c.Reason), c.Date, nil
}

// valueRow is one value-only cost-ledger row mirroring a Persediaan leg.
type valueRow struct {
	productID, variantID int64
	value                int64
}

// inventoryRows lists the Persediaan legs of an entry as signed ledger values
// (sign flips with negate, for a reversal).
func (s *Service) inventoryRows(ctx context.Context, lines []*contracts.JournalLine, negate bool) ([]valueRow, error) {
	inv, err := s.mustMap(ctx, "inventory")
	if err != nil {
		return nil, err
	}
	var rows []valueRow
	for _, l := range lines {
		if l.AccountID != inv.ID {
			continue
		}
		v := l.Debit - l.Credit
		if negate {
			v = -v
		}
		rows = append(rows, valueRow{productID: l.ProductID, variantID: l.VariantID, value: v})
	}
	return rows, nil
}

// writeValueRows stores the rows inside the journal's transaction, so the
// books and the ledger move together or not at all. The caller holds the
// branch cost lock (synthetic ids are reserved under it).
func (s *Service) writeValueRows(ctx context.Context, tx *sql.Tx, branchID, correctionID int64, rows []valueRow) error {
	if len(rows) == 0 {
		return nil
	}
	ids, err := s.repo.NextSyntheticCostIDs(ctx, len(rows))
	if err != nil {
		return err
	}
	stamp := timeutil.NowUTC().Format("2006-01-02 15:04:05.000000")
	for i, r := range rows {
		if err := s.repo.InsertValueRowTx(ctx, tx, branchID, r.productID, r.variantID, ids[i],
			infrastructure.RefPurchasePriceCorrection, correctionID, r.value, stamp); err != nil {
			return err
		}
	}
	return nil
}

// postPriceCorrection journals a price correction and writes its value-only
// cost rows in the same transaction, under the branch cost lock (the split
// reads the positions the rows then change). Idempotent like Post: the caller
// already returned an existing journal.
func (s *Service) postPriceCorrection(ctx context.Context, actorID, branchID, id int64, catchUp bool) (*contracts.JournalEntry, error) {
	if err := s.drainCosting(ctx, branchID); err != nil {
		return nil, err
	}
	var out *contracts.JournalEntry
	err := s.costLock(ctx, branchID, costLockWaitSync, func() error {
		lines, memo, date, err := s.buildLines(ctx, branchID, SourcePurchasePriceCorrection, id)
		if err != nil {
			return err
		}
		if catchUp {
			date = timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
			memo = "Susulan: " + memo
		}
		rows, err := s.inventoryRows(ctx, lines, false)
		if err != nil {
			return err
		}
		entry, err := s.insertEntryWith(ctx, actorID, branchID, date, memo, SourcePurchasePriceCorrection, id, lines, "journals.post",
			map[string]any{"valueRows": len(rows)},
			func(ctx context.Context, tx *sql.Tx, _ int64) error {
				return s.writeValueRows(ctx, tx, branchID, id, rows)
			})
		if err != nil {
			if ae, ok := err.(*apperror.AppError); ok && ae.Code == apperror.CodeConflict {
				if existing, lerr := s.repo.EntryBySource(ctx, branchID, SourcePurchasePriceCorrection, id); lerr == nil && existing != nil {
					out = existing
					return nil
				}
			}
			return err
		}
		out = entry
		return nil
	})
	return out, err
}
