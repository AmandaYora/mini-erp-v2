package application

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/modules/finance/infrastructure"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	purchasereturncontracts "mini-erp/internal/modules/purchasereturn/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	salesreturncontracts "mini-erp/internal/modules/salesreturn/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/money"
	"mini-erp/internal/shared/timeutil"
)

// rawLeg is one leg under construction (account by code, dimensions raw).
type rawLeg struct {
	// key is the mapping key the leg posts to; code is an explicit account
	// code for the rare caller that already resolved one.
	key       string
	code      string
	debit     int64
	credit    int64
	productID int64
	variantID int64
	partyID   int64
	note      string
}

// buildLines routes one source document to its compound entry:
// lines, memo, entry date. Pure computation + reads; writing is Post's job.
func (s *Service) buildLines(ctx context.Context, branchID int64, docType string, docID int64) ([]*contracts.JournalLine, string, string, error) {
	switch docType {
	case SourceGoodsReceipt:
		return s.buildGoodsReceipt(ctx, branchID, docID)
	case SourceDelivery:
		return s.buildDelivery(ctx, branchID, docID)
	case SourcePayment:
		return s.buildPayment(ctx, branchID, docID)
	case SourceSalesReturn:
		return s.buildSalesReturn(ctx, branchID, docID)
	case SourcePurchaseReturn:
		return s.buildPurchaseReturn(ctx, branchID, docID)
	case SourceSalesReturnSettlement:
		return s.buildSalesReturnSettlement(ctx, branchID, docID)
	case SourcePurchaseReturnSettlement:
		return s.buildPurchaseReturnSettlement(ctx, branchID, docID)
	case SourceStockMovement:
		return s.buildStockMovement(ctx, branchID, docID)
	default:
		return nil, "", "", apperror.Validation("", []apperror.FieldError{{Field: "docType", Message: "tipe dokumen tidak dikenal"}})
	}
}

// finish resolves account codes to ids and carries dimensions through.
func (s *Service) finish(ctx context.Context, raw []rawLeg) ([]*contracts.JournalLine, error) {
	out := make([]*contracts.JournalLine, 0, len(raw))
	for _, l := range raw {
		if (l.debit == 0 && l.credit == 0) || (l.debit > 0 && l.credit > 0) {
			continue // zero legs carry no information; drop them silently
		}
		var a *contracts.Account
		var err error
		if l.key != "" {
			a, err = s.mustMap(ctx, l.key)
		} else {
			a, err = s.repo.AccountByCode(ctx, l.code)
			if err == nil && (a == nil || a.Status != "active") {
				err = apperror.Conflict("Akun '" + l.code + "' tidak tersedia")
			}
		}
		if err != nil {
			if _, ok := err.(*apperror.AppError); ok {
				return nil, err
			}
			return nil, apperror.Internal(err)
		}
		out = append(out, &contracts.JournalLine{
			AccountID: a.ID, AccountCode: a.Code, AccountName: a.Name,
			Debit: l.debit, Credit: l.credit,
			ProductID: l.productID, VariantID: l.variantID, PartyID: l.partyID,
			Description: l.note,
		})
	}
	return out, nil
}

// prorateWindow values the slice [prior, prior+part] of a line of `whole` units
// as the difference of two cumulative roundings. Summed over consecutive
// documents it telescopes to the exact line amount, whatever the split.
func prorateWindow(amount int64, prior, part, whole float64) int64 {
	return money.Prorate(amount, prior+part, whole) - money.Prorate(amount, prior, whole)
}

func lineKeyOf(productID, variantID int64, uom string) string {
	return strconv.FormatInt(productID, 10) + "/" + strconv.FormatInt(variantID, 10) + "/" + strings.TrimSpace(uom)
}

// docValues hands a source document's own recorded movement value out to its
// lines: exact historical cost, immune to positions the document itself
// emptied (the live average then reads zero while the relieved units were
// really worth the pre-out average). Outs record negative, ins positive.
//
// The ledger groups a document's value per (product, variant), while a
// document can carry several lines of one position (the same product from two
// locations, or in two UOMs). Each line takes its base-quantity share,
// cumulatively — (already taken + this qty) of the moved qty, minus what the
// earlier lines took — so the lines add up to the recorded value exactly and
// no line books the whole position again.
type docValues struct {
	moves map[string]infrastructure.MoveValue
	taken map[string][2]float64 // qty and value already handed out per position
}

func newDocValues(moves map[string]infrastructure.MoveValue) *docValues {
	return &docValues{moves: moves, taken: map[string][2]float64{}}
}

// take returns one line's share of the recorded value. ok is false when the
// document has no ledger rows for the position (costing not synced yet).
func (d *docValues) take(productID, variantID int64, qtyBase float64) (float64, bool) {
	key := infrastructure.CostKeyOf(productID, variantID)
	mv, ok := d.moves[key]
	if !ok {
		return 0, false
	}
	t := d.taken[key]
	moved := math.Abs(mv.Qty)
	cum := t[0] + qtyBase
	upTo := mv.Value
	if qtyBase > 0 && moved > positionEpsilon && cum < moved-positionEpsilon {
		upTo = float64(money.Round(mv.Value * cum / moved))
	}
	d.taken[key] = [2]float64{cum, upTo}
	return upTo - t[1], true
}

// buildGoodsReceipt: Dr Persediaan (per produk, pro-rata PO tax base) +
// Dr PPN Masukan / Cr Hutang Usaha (per pemasok).
//
// Inventory is debited at the line's TAX BASE, never at its tax-inclusive
// net: for a tax-inclusive PO the old aggregate form booked the tax twice —
// once inside inventory and again as PPN Masukan — and overstated the payable
// by the same amount. Payable is the sum of what was actually debited, so
// rounding can never break the entry.
func (s *Service) buildGoodsReceipt(ctx context.Context, branchID, receiptID int64) ([]*contracts.JournalLine, string, string, error) {
	gr, err := s.goodsreceipt.GetByID(ctx, receiptID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	if gr == nil || gr.BranchID != branchID {
		return nil, "", "", apperror.NotFound("Penerimaan")
	}
	po, err := s.purchasing.GetByID(ctx, gr.PurchaseOrderID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	if po == nil || po.BranchID != branchID {
		return nil, "", "", apperror.NotFound("Penerimaan")
	}
	byKey := map[string]*purchasingcontracts.Item{}
	for _, l := range po.Items {
		key := lineKeyOf(l.ProductID, l.VariantID, l.UOM)
		if _, seen := byKey[key]; !seen {
			byKey[key] = l
		}
	}
	legs := make([]rawLeg, 0, len(gr.Items)+2)
	var inventory, taxIn int64
	trackedLine := map[string]bool{}
	for _, l := range po.Items {
		if l.StockTracked {
			trackedLine[lineKeyOf(l.ProductID, l.VariantID, l.UOM)] = true
		}
	}
	before, err := s.goodsreceipt.PostedQtyBefore(ctx, po.ID, gr.ID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	for _, it := range gr.Items {
		pl, ok := byKey[lineKeyOf(it.ProductID, it.VariantID, it.UOM)]
		if !ok {
			return nil, "", "", apperror.Conflict("Baris penerimaan tak cocok PO")
		}
		// The line's own stored tax split (its taxable flag and rate, PO-03),
		// never a recomputation at the header rate: an exempt line carries no
		// PPN Masukan and a line rate wins over the order rate.
		base, tax := pl.TaxBase, pl.TaxAmount
		// The receipt that completes a line takes the rounding remainder, so the
		// receipts of one line add up to its value exactly.
		prior := before[lineKeyOf(it.ProductID, it.VariantID, it.UOM)]
		value := prorateWindow(base, prior, it.Qty, pl.Qty)
		inventory += value
		taxIn += prorateWindow(tax, prior, it.Qty, pl.Qty)
		// A purchased SERVICE is an expense, not inventory (no stock moved).
		debitKey := "inventory"
		if !trackedLine[lineKeyOf(it.ProductID, it.VariantID, it.UOM)] {
			debitKey = "expense"
		}
		legs = append(legs, rawLeg{
			key: debitKey, debit: value,
			productID: it.ProductID, variantID: it.VariantID, note: pl.ProductName,
		})
	}
	payable := inventory + taxIn
	if payable <= 0 {
		return nil, "", "", apperror.Conflict("Nilai penerimaan nol")
	}
	if taxIn != 0 {
		legs = append(legs, rawLeg{key: "ppn_in", debit: taxIn, note: "PPN Masukan " + po.Number})
	}
	legs = append(legs, rawLeg{key: "payable", credit: payable, partyID: po.PartyID, note: po.PartyName})
	lines, err := s.finish(ctx, legs)
	if err != nil {
		return nil, "", "", err
	}
	return lines, fmt.Sprintf("Terima #%d atas %s", gr.ID, po.Number), gr.ReceivedAt, nil
}

// buildDelivery: Dr Piutang (per pelanggan) / Cr Penjualan + Cr PPN Keluaran,
// plus Dr HPP / Cr Persediaan at moving average — every economic leg carrying
// its product so margin is answerable from the books.
//
// Values are pro-rated to the lines ACTUALLY ON THIS SURAT JALAN. The old
// aggregate form booked the whole order's revenue, tax and receivable on each
// note, so an order shipped in two parts was billed to the books twice.
// Partial delivery is a supported flow (delivery/application/service.go
// caps each note against the SO remainder), so this was reachable, not
// theoretical.
func (s *Service) buildDelivery(ctx context.Context, branchID, noteID int64) ([]*contracts.JournalLine, string, string, error) {
	note, err := s.delivery.GetByID(ctx, noteID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	if note == nil || note.BranchID != branchID {
		return nil, "", "", apperror.NotFound("Surat jalan")
	}
	if note.Status != deliverycontracts.StatusConfirmed {
		return nil, "", "", apperror.Conflict("Surat jalan belum dikonfirmasi atau sudah dibatalkan")
	}
	// A replacement shipment is a NEW sale (the exchange's replacement goods):
	// it books its own revenue, tax and cost (F2b-2).
	if note.DocumentKind == deliverycontracts.DocumentKindReplacement {
		return s.buildReplacementDelivery(ctx, branchID, note)
	}
	so, err := s.sales.GetByID(ctx, note.SalesOrderID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	if so == nil || so.BranchID != branchID {
		return nil, "", "", apperror.NotFound("Surat jalan")
	}
	byKey := map[string]*salescontracts.Item{}
	for _, l := range so.Items {
		key := lineKeyOf(l.ProductID, l.VariantID, l.UOM)
		if _, seen := byKey[key]; !seen {
			byKey[key] = l
		}
	}
	revenueLegs := make([]rawLeg, 0, len(note.Items))
	costLegs := make([]rawLeg, 0, len(note.Items)*2)
	moves, err := s.repo.MoveValues(ctx, branchID, "delivery", noteID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	vals := newDocValues(moves)
	var revenue, tax int64
	before, err := s.delivery.ConfirmedQtyBefore(ctx, so.ID, note.ID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	for _, it := range note.Items {
		sl, ok := byKey[lineKeyOf(it.ProductID, it.VariantID, it.UOM)]
		if !ok {
			return nil, "", "", apperror.Conflict("Baris surat jalan tak cocok SO")
		}
		// The line's own stored tax split: a line marked tax-exempt books no PPN
		// Keluaran and its receivable equals what the order bills.
		base, lineTax := sl.TaxBase, sl.TaxAmount
		// The shipment that completes a line takes the rounding remainder, so the
		// shipments of one line add up to its value exactly (3 x 1/3 of 100.000
		// books 33.333 + 33.333 + 33.334, not 99.999).
		prior := before[lineKeyOf(it.ProductID, it.VariantID, it.UOM)]
		rev := prorateWindow(base, prior, it.Qty, sl.Qty)
		revenue += rev
		tax += prorateWindow(lineTax, prior, it.Qty, sl.Qty)
		revenueLegs = append(revenueLegs, rawLeg{
			key: "revenue", credit: rev,
			productID: it.ProductID, variantID: it.VariantID, note: sl.ProductName,
		})
		var cogs int64
		if sl.StockTracked {
			// The SJ's own out-movement, recorded negative. A tracked product
			// with no recorded cost is NEVER booked at zero: the document waits
			// until its cost is known (F2b-11).
			v, ok := vals.take(it.ProductID, it.VariantID, it.QtyBase)
			if !ok {
				return nil, "", "", apperror.Conflict("Biaya persediaan surat jalan belum tercatat — sinkronkan costing")
			}
			if cogs = money.Round(-v); cogs <= 0 {
				return nil, "", "", apperror.Conflict("Harga modal produk " + sl.ProductName + " belum tersedia")
			}
		}
		if cogs != 0 {
			costLegs = append(costLegs,
				rawLeg{key: "cogs", debit: cogs, productID: it.ProductID, variantID: it.VariantID, note: sl.ProductName},
				rawLeg{key: "inventory", credit: cogs, productID: it.ProductID, variantID: it.VariantID, note: sl.ProductName},
			)
		}
	}
	receivable := revenue + tax
	if receivable <= 0 {
		return nil, "", "", apperror.Conflict("Nilai surat jalan nol")
	}
	legs := make([]rawLeg, 0, len(revenueLegs)+len(costLegs)+2)
	legs = append(legs, rawLeg{key: "receivable", debit: receivable, partyID: so.PartyID, note: so.PartyName})
	legs = append(legs, revenueLegs...)
	if tax != 0 {
		legs = append(legs, rawLeg{key: "ppn_out", credit: tax, note: "PPN Keluaran " + so.Number})
	}
	legs = append(legs, costLegs...)
	lines, err := s.finish(ctx, legs)
	if err != nil {
		return nil, "", "", err
	}
	return lines, fmt.Sprintf("Kirim %s atas %s", note.Number, so.Number), note.DeliveryDate, nil
}

// buildPayment: cash in (Dr Kas/Bank / Cr Piutang) or out (Dr Hutang /
// Cr Kas/Bank). Offset legs post exactly like cash — method "offset" only
// separates them in cash reports. The receivable/payable leg carries the
// party so a payment reduces that party's balance in the books, not just in
// the payment module's own ledger.
func (s *Service) buildPayment(ctx context.Context, branchID, paymentID int64) ([]*contracts.JournalLine, string, string, error) {
	p, err := s.payment.GetByID(ctx, paymentID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	if p == nil || p.BranchID != branchID {
		return nil, "", "", apperror.NotFound("Pembayaran")
	}
	if p.Status != "active" {
		return nil, "", "", apperror.Conflict("Pembayaran sudah dibatalkan")
	}
	kind, err := classifyPayment(p)
	if err != nil {
		return nil, "", "", err
	}
	if kind.skip {
		return nil, "", "", apperror.Conflict("Pembayaran offset tidak dijurnal: retur sudah mengurangi tagihan di pembukuan")
	}
	var legs []rawLeg
	switch p.Direction {
	case paymentcontracts.DirectionIn: // money comes in: Dr cash/bank, Cr counter-account
		legs = []rawLeg{
			{key: kind.cashKey, debit: p.Amount, partyID: advanceParty(kind, p), note: p.Number},
			{key: kind.counterKey, credit: p.Amount, partyID: p.PartyID, note: p.PartyName},
		}
	default: // money goes out: Dr counter-account, Cr cash/bank
		legs = []rawLeg{
			{key: kind.counterKey, debit: p.Amount, partyID: p.PartyID, note: p.PartyName},
			{key: kind.cashKey, credit: p.Amount, partyID: advanceParty(kind, p), note: p.Number},
		}
	}
	lines, err := s.finish(ctx, legs)
	if err != nil {
		return nil, "", "", err
	}
	return lines, kind.memo + " " + p.Number, p.PaidAt, nil
}

// advanceParty is the party dimension of the saldo leg of a saldo use (N-41):
// the advance account is a per-party balance, so its leg names the party. Cash
// and bank legs stay party-less.
func advanceParty(kind paymentKind, p *paymentcontracts.Payment) int64 {
	if kind.advance {
		return p.PartyID
	}
	return 0
}

// buildSalesReturn: restock at average (Dr Persediaan / Cr HPP) plus revenue
// reversal at selling prices (Dr Retur Penjualan / Cr Piutang), both per
// product. The return's Total is the sum of its line totals (salesreturn
// owns that invariant), so per-line legs foot exactly to it.
func (s *Service) buildSalesReturn(ctx context.Context, branchID, returnID int64) ([]*contracts.JournalLine, string, string, error) {
	full, err := s.salesReturns.GetByID(ctx, returnID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	if full == nil || full.BranchID != branchID {
		return nil, "", "", apperror.NotFound("Retur")
	}
	if full.Status != salesreturncontracts.StatusConfirmed {
		return nil, "", "", apperror.Conflict("Retur belum dikonfirmasi atau sudah dibatalkan")
	}
	// The party lives on the originating SO, not on the return document.
	// Without it the credit to Piutang would land in an "unknown party"
	// bucket and the receivables report would never clear.
	var partyID int64
	var partyName string
	soTracked := map[string]bool{}
	if so, err := s.sales.GetByID(ctx, full.SalesOrderID); err != nil {
		return nil, "", "", apperror.Internal(err)
	} else if so != nil {
		partyID, partyName = so.PartyID, so.PartyName
		for _, l := range so.Items {
			if l.StockTracked {
				soTracked[strconv.FormatInt(l.ProductID, 10)+"/"+strconv.FormatInt(l.VariantID, 10)] = true
			}
		}
	}
	legs := make([]rawLeg, 0, len(full.Items)*3+2)
	moves, err := s.repo.MoveValues(ctx, branchID, "sales_return", returnID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	vals := newDocValues(moves)
	for _, it := range full.Items {
		// Restock at the return's own recorded in-value (= pre-restock
		// average by construction of the cost ledger). Goods the customer
		// keeps (not_returned) never came back: value is returned, no restock.
		var restock int64
		tracked := soTracked[strconv.FormatInt(it.ProductID, 10)+"/"+strconv.FormatInt(it.VariantID, 10)]
		if it.Condition != salesreturncontracts.ConditionNotReturned && tracked {
			v, ok := vals.take(it.ProductID, it.VariantID, it.QtyBase)
			if !ok {
				return nil, "", "", apperror.Conflict("Biaya persediaan retur belum tercatat — sinkronkan costing")
			}
			if restock = money.Round(v); restock <= 0 {
				return nil, "", "", apperror.Conflict("Harga modal produk pada retur belum tersedia")
			}
		}
		if restock != 0 {
			legs = append(legs,
				rawLeg{key: "inventory", debit: restock, productID: it.ProductID, variantID: it.VariantID},
				rawLeg{key: "cogs", credit: restock, productID: it.ProductID, variantID: it.VariantID},
			)
		}
		// Revenue reversal books the TAX BASE per product (D1): the tax
		// itself has its own leg below, so booking LineTotal here would
		// double-count it for tax-inclusive orders.
		if it.TaxBase != 0 {
			legs = append(legs, rawLeg{
				key: "sales_return", debit: it.TaxBase,
				productID: it.ProductID, variantID: it.VariantID,
			})
		}
	}
	if full.Total <= 0 {
		return nil, "", "", apperror.Conflict("Nilai retur nol")
	}
	if full.TaxTotal != 0 {
		legs = append(legs, rawLeg{key: "ppn_out", debit: full.TaxTotal})
	}
	legs = append(legs, rawLeg{key: "receivable", credit: full.Total, partyID: partyID, note: partyName})
	lines, err := s.finish(ctx, legs)
	if err != nil {
		return nil, "", "", err
	}
	return lines, fmt.Sprintf("Retur %s", full.Number), full.ReturnDate, nil
}

// buildPurchaseReturn: relieve AP at PO prices (per pemasok), inventory at
// average (per produk), and plug the difference to the adjustment account
// (purchase price variance). The plug is a single unattributed leg on
// purpose — a variance belongs to no one product.
func (s *Service) buildPurchaseReturn(ctx context.Context, branchID, returnID int64) ([]*contracts.JournalLine, string, string, error) {
	full, err := s.purchaseReturns.GetByID(ctx, returnID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	if full == nil || full.BranchID != branchID {
		return nil, "", "", apperror.NotFound("Retur")
	}
	if full.Status != purchasereturncontracts.StatusConfirmed {
		return nil, "", "", apperror.Conflict("Retur belum dikonfirmasi atau sudah dibatalkan")
	}
	var partyID int64
	var partyName string
	trackedPO := map[string]bool{}
	if po, err := s.purchasing.GetByID(ctx, full.PurchaseOrderID); err != nil {
		return nil, "", "", apperror.Internal(err)
	} else if po != nil {
		partyID, partyName = po.PartyID, po.PartyName
		for _, l := range po.Items {
			if l.StockTracked {
				trackedPO[strconv.FormatInt(l.ProductID, 10)+"/"+strconv.FormatInt(l.VariantID, 10)] = true
			}
		}
	}
	legs := make([]rawLeg, 0, len(full.Items)+4)
	legs = append(legs, rawLeg{key: "payable", debit: full.Total, partyID: partyID, note: partyName})
	if full.TaxTotal != 0 {
		legs = append(legs, rawLeg{key: "ppn_in", credit: full.TaxTotal})
	}
	moves, err := s.repo.MoveValues(ctx, branchID, "purchase_return", returnID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	vals := newDocValues(moves)
	var relieved int64
	for _, it := range full.Items {
		if !trackedPO[strconv.FormatInt(it.ProductID, 10)+"/"+strconv.FormatInt(it.VariantID, 10)] {
			// A service line was never inventory: the credit goes back to the
			// expense account it was booked to, at its own value (no variance).
			relieved += it.TaxBase
			legs = append(legs, rawLeg{key: "expense", credit: it.TaxBase, note: "Retur jasa"})
			continue
		}
		// Relieve at the return's own recorded out-value (stored negative).
		var value int64
		v, ok := vals.take(it.ProductID, it.VariantID, it.QtyBase)
		if !ok {
			return nil, "", "", apperror.Conflict("Biaya persediaan retur belum tercatat — sinkronkan costing")
		}
		if value = money.Round(-v); value <= 0 {
			return nil, "", "", apperror.Conflict("Harga modal produk pada retur belum tersedia")
		}
		relieved += value
		legs = append(legs, rawLeg{
			key: "inventory", credit: value,
			productID: it.ProductID, variantID: it.VariantID,
		})
	}
	// The variance is priced off the pre-tax subtotal: with tax on its own
	// leg, the old total-based formula would drag VAT into the inventory
	// variance account.
	switch diff := full.Subtotal - relieved; {
	case diff > 0:
		legs = append(legs, rawLeg{key: "purchase_return_difference", credit: diff, note: "Selisih harga retur " + full.Number})
	case diff < 0:
		legs = append(legs, rawLeg{key: "purchase_return_difference", debit: -diff, note: "Selisih harga retur " + full.Number})
	}
	lines, err := s.finish(ctx, legs)
	if err != nil {
		return nil, "", "", err
	}
	return lines, fmt.Sprintf("Retur %s", full.Number), full.ReturnDate, nil
}

// dayOf trims a stored datetime to its date part.
func dayOf(v string) string {
	if len(v) >= 10 {
		return v[:10]
	}
	return v
}

// buildSalesReturnSettlement reclassifies the part of a return kept as the
// customer's balance (K-04a, legacy buildSalesReturnSettlementPlan "customer
// credit" branch): the return journal credited Piutang for its full value, so
// any excess over the open bill sits as a NEGATIVE receivable. Dr Piutang /
// Cr Uang Muka Pelanggan (both on the customer) moves that credit out of
// Piutang into its own liability line, so the balance sheet shows what we owe
// the customer instead of netting it into receivables.
func (s *Service) buildSalesReturnSettlement(ctx context.Context, branchID, settlementID int64) ([]*contracts.JournalLine, string, string, error) {
	st, err := s.salesReturns.GetCreditSettlement(ctx, settlementID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	if st == nil || st.BranchID != branchID {
		return nil, "", "", apperror.NotFound("Penyelesaian retur")
	}
	if st.ReturnStatus != salesreturncontracts.StatusConfirmed {
		return nil, "", "", apperror.Conflict("Retur belum dikonfirmasi atau sudah dibatalkan")
	}
	if st.Amount <= 0 {
		return nil, "", "", apperror.Conflict("Nilai penyelesaian nol")
	}
	so, err := s.sales.GetByID(ctx, st.OrderID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	if so == nil || so.PartyID == 0 {
		return nil, "", "", apperror.Conflict("Saldo customer butuh order dengan data customer")
	}
	lines, err := s.finish(ctx, []rawLeg{
		{key: "receivable", debit: st.Amount, partyID: so.PartyID, note: so.PartyName},
		{key: "customer_advance", credit: st.Amount, partyID: so.PartyID, note: so.PartyName},
	})
	if err != nil {
		return nil, "", "", err
	}
	return lines, fmt.Sprintf("Saldo customer retur %s", st.ReturnNumber), dayOf(st.Date), nil
}

// buildPurchaseReturnSettlement is the supplier mirror (legacy
// buildPurchaseReturnSettlementPlan "supplier credit" branch): the return
// journal debited Hutang for its full value, so the excess over the open bill
// sits as a DEBIT balance in Hutang. Dr Uang Muka Supplier / Cr Hutang (both on
// the supplier) turns it into an asset we can see and recover.
func (s *Service) buildPurchaseReturnSettlement(ctx context.Context, branchID, settlementID int64) ([]*contracts.JournalLine, string, string, error) {
	st, err := s.purchaseReturns.GetCreditSettlement(ctx, settlementID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	if st == nil || st.BranchID != branchID {
		return nil, "", "", apperror.NotFound("Penyelesaian retur")
	}
	if st.ReturnStatus != purchasereturncontracts.StatusConfirmed {
		return nil, "", "", apperror.Conflict("Retur belum dikonfirmasi atau sudah dibatalkan")
	}
	if st.Amount <= 0 {
		return nil, "", "", apperror.Conflict("Nilai penyelesaian nol")
	}
	po, err := s.purchasing.GetByID(ctx, st.OrderID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	if po == nil || po.PartyID == 0 {
		return nil, "", "", apperror.Conflict("Saldo supplier butuh order dengan data supplier")
	}
	lines, err := s.finish(ctx, []rawLeg{
		{key: "supplier_advance", debit: st.Amount, partyID: po.PartyID, note: po.PartyName},
		{key: "payable", credit: st.Amount, partyID: po.PartyID, note: po.PartyName},
	})
	if err != nil {
		return nil, "", "", err
	}
	return lines, fmt.Sprintf("Saldo supplier retur %s", st.ReturnNumber), dayOf(st.Date), nil
}

// buildReplacementDelivery books the replacement goods of an exchange as a new
// sale (F2b-2): Dr Piutang (customer of the source order) / Cr Penjualan per
// product at the stored tax base / Cr PPN Keluaran, plus cost of goods from the
// shipment's own recorded stock-out. The signed difference to the returned
// goods is NOT journalled separately: the net receivable already is that
// difference.
func (s *Service) buildReplacementDelivery(ctx context.Context, branchID int64, note *deliverycontracts.DeliveryNote) ([]*contracts.JournalLine, string, string, error) {
	ret, err := s.salesReturns.GetByID(ctx, note.SalesReturnID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	if ret == nil || ret.BranchID != branchID {
		return nil, "", "", apperror.NotFound("Retur")
	}
	so, err := s.sales.GetByID(ctx, ret.SalesOrderID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	if so == nil {
		return nil, "", "", apperror.NotFound("Order")
	}
	byKey := map[string]*salesreturncontracts.ReplacementItem{}
	for _, ri := range ret.ReplacementItems {
		byKey[lineKeyOf(ri.ProductID, ri.VariantID, ri.UOM)] = ri
	}
	moves, err := s.repo.MoveValues(ctx, branchID, "delivery", note.ID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	vals := newDocValues(moves)
	var revenue, tax int64
	legs := make([]rawLeg, 0, len(note.Items)*3+2)
	for _, it := range note.Items {
		ri, ok := byKey[lineKeyOf(it.ProductID, it.VariantID, it.UOM)]
		if !ok {
			return nil, "", "", apperror.Conflict("Baris surat jalan pengganti tak cocok retur tukar")
		}
		base := money.Prorate(ri.TaxBase, it.Qty, ri.Qty)
		lineTax := money.Prorate(ri.TaxAmount, it.Qty, ri.Qty)
		revenue += base
		tax += lineTax
		legs = append(legs, rawLeg{key: "revenue", credit: base, productID: it.ProductID, variantID: it.VariantID, note: "Barang pengganti"})
		v, ok := vals.take(it.ProductID, it.VariantID, it.QtyBase)
		if !ok {
			return nil, "", "", apperror.Conflict("Biaya persediaan surat jalan pengganti belum tercatat — sinkronkan costing")
		}
		if cogs := money.Round(-v); cogs != 0 {
			legs = append(legs,
				rawLeg{key: "cogs", debit: cogs, productID: it.ProductID, variantID: it.VariantID, note: "Barang pengganti"},
				rawLeg{key: "inventory", credit: cogs, productID: it.ProductID, variantID: it.VariantID, note: "Barang pengganti"},
			)
		}
	}
	receivable := revenue + tax
	if receivable <= 0 {
		return nil, "", "", apperror.Conflict("Nilai surat jalan pengganti nol")
	}
	legs = append(legs, rawLeg{key: "receivable", debit: receivable, partyID: so.PartyID, note: so.PartyName})
	if tax != 0 {
		legs = append(legs, rawLeg{key: "ppn_out", credit: tax, note: "PPN Keluaran " + so.Number})
	}
	lines, err := s.finish(ctx, legs)
	if err != nil {
		return nil, "", "", err
	}
	return lines, fmt.Sprintf("Kirim pengganti %s atas %s", note.Number, so.Number), note.DeliveryDate, nil
}

// buildStockMovement journals one stock adjustment, write-off or damaged
// arrival at the value its cost-ledger row recorded (exact, never the live
// average):
//
//	adjustment in  (surplus)  Dr inventory   / Cr adjustment
//	damaged arrival (in)      Dr inventory   / Cr adjustment
//	adjustment out (shortage) Dr adjustment  / Cr inventory
//	write-off      (out)      Dr writeoff    / Cr inventory
//
// A damaged arrival is stock that appeared with no source document, exactly
// like a surplus; booking it keeps the ledger and Persediaan equal, and its
// later write-off then relieves value the books actually hold.
//
// A zero or estimated-with-no-price value blocks the document (F2b-11).
func (s *Service) buildStockMovement(ctx context.Context, branchID, movementID int64) ([]*contracts.JournalLine, string, string, error) {
	row, err := s.repo.CostRowByMovement(ctx, branchID, movementID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	if row == nil || !slices.Contains(infrastructure.JournaledMovementRefs, row.RefType) {
		return nil, "", "", apperror.NotFound("Mutasi stok")
	}
	value := money.Round(row.TotalCost)
	if value < 0 {
		value = -value
	}
	if value == 0 {
		return nil, "", "", apperror.Conflict("Harga modal produk pada mutasi stok belum tersedia")
	}
	var legs []rawLeg
	var memo string
	switch {
	case row.RefType == "write_off":
		legs = []rawLeg{
			{key: "writeoff", debit: value, productID: row.ProductID, variantID: row.VariantID, note: "Hapus stok rusak"},
			{key: "inventory", credit: value, productID: row.ProductID, variantID: row.VariantID, note: "Hapus stok rusak"},
		}
		memo = "Hapus stok rusak"
	case row.RefType == "damaged_arrival":
		legs = []rawLeg{
			{key: "inventory", debit: value, productID: row.ProductID, variantID: row.VariantID, note: "Barang rusak masuk tanpa dokumen"},
			{key: "adjustment", credit: value, productID: row.ProductID, variantID: row.VariantID, note: "Barang rusak masuk tanpa dokumen"},
		}
		memo = "Barang rusak masuk"
	case row.Direction == "in":
		legs = []rawLeg{
			{key: "inventory", debit: value, productID: row.ProductID, variantID: row.VariantID, note: "Selisih stok lebih"},
			{key: "adjustment", credit: value, productID: row.ProductID, variantID: row.VariantID, note: "Selisih stok lebih"},
		}
		memo = "Koreksi stok (lebih)"
	default:
		legs = []rawLeg{
			{key: "adjustment", debit: value, productID: row.ProductID, variantID: row.VariantID, note: "Selisih stok kurang"},
			{key: "inventory", credit: value, productID: row.ProductID, variantID: row.VariantID, note: "Selisih stok kurang"},
		}
		memo = "Koreksi stok (kurang)"
	}
	lines, err := s.finish(ctx, legs)
	if err != nil {
		return nil, "", "", err
	}
	// moved_at is a UTC instant: the journal takes its WIB calendar day.
	return lines, memo, timeutil.WIBDay(row.MovedAt), nil
}
