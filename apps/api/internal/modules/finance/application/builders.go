package application

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/modules/finance/infrastructure"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/shared/apperror"
)

// rawLeg is one leg under construction (account by code, dimensions raw).
type rawLeg struct {
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
		a, err := s.repo.AccountByCode(ctx, l.code)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if a == nil || a.Status != "active" {
			return nil, apperror.Conflict("Akun '" + l.code + "' tidak tersedia")
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

func lineKeyOf(productID, variantID int64, uom string) string {
	return strconv.FormatInt(productID, 10) + "/" + strconv.FormatInt(variantID, 10) + "/" + strings.TrimSpace(uom)
}

// recordedValue returns a source document's own recorded movement value for
// one position: exact historical cost, immune to positions the document
// itself emptied (the live average then reads zero while the relieved units
// were really worth the pre-out average). Outs record negative, ins
// positive. Movements synced before ref tracking have no rows: ok=false and
// the caller falls back to the live average (old behavior, unchanged).
func recordedValue(moves map[string]infrastructure.MoveValue, productID, variantID int64) (float64, bool) {
	mv, ok := moves[infrastructure.CostKeyOf(productID, variantID)]
	if !ok {
		return 0, false
	}
	return mv.Value, true
}

// lineEconomics recomputes one order line's net / tax base / tax from its
// stored economics. Finance recomputes instead of reading LineTotal because
// LineTotal is tax-INCLUSIVE: booking revenue or inventory from it overstates
// both by the tax whenever the document is tax-inclusive.
//
// Formula mirrors the order pricing domains (gross − pct − nominal, half away
// from zero at each step); each module owns its math per the monorepo
// standard, so this is deliberate duplication, not a missing shared helper.
//
//	none:    base = net,               tax = 0
//	exclude: base = net,               tax = round(net × rate/100)
//	include: base = round(net/(1+r)),  tax = net − base
//
// `base` is always the amount that belongs in Persediaan / Penjualan, and
// base+tax is always what the counterparty owes. That invariant is what makes
// every entry below balanced by construction.
func lineEconomics(qty float64, unit int64, pct float64, nominal int64, taxType string, rate float64) (net, base, tax int64) {
	gross := round(qty * float64(unit))
	net = gross - round(float64(gross)*pct/100) - nominal
	switch taxType {
	case "exclude":
		base = net
		tax = round(float64(net) * rate / 100)
	case "include":
		if rate > 0 {
			base = round(float64(net) / (1 + rate/100))
		} else {
			base = net
		}
		tax = net - base
	default:
		base = net
	}
	return net, base, tax
}

// prorate scales a line amount by the delivered/received share of that line.
// Returns 0 for a zero-qty source line rather than dividing by zero.
func prorate(amount int64, part, whole float64) int64 {
	if whole <= 0 {
		return 0
	}
	return round(float64(amount) * part / whole)
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
	for _, it := range gr.Items {
		pl, ok := byKey[lineKeyOf(it.ProductID, it.VariantID, it.UOM)]
		if !ok {
			return nil, "", "", apperror.Conflict("Baris penerimaan tak cocok PO")
		}
		_, base, tax := lineEconomics(pl.Qty, pl.UnitPrice, pl.DiscountPct, pl.DiscountNominal, po.TaxType, po.TaxRate)
		value := prorate(base, it.Qty, pl.Qty)
		inventory += value
		taxIn += prorate(tax, it.Qty, pl.Qty)
		legs = append(legs, rawLeg{
			code: "1300", debit: value,
			productID: it.ProductID, variantID: it.VariantID, note: pl.ProductName,
		})
	}
	payable := inventory + taxIn
	if payable <= 0 {
		return nil, "", "", apperror.Conflict("Nilai penerimaan nol")
	}
	if taxIn != 0 {
		legs = append(legs, rawLeg{code: "1400", debit: taxIn, note: "PPN Masukan " + po.Number})
	}
	legs = append(legs, rawLeg{code: "2100", credit: payable, partyID: po.PartyID, note: po.PartyName})
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
	// Replacement shipments carry no revenue: the goods were already sold
	// on the original order. Posting them here would double-book revenue.
	// Warranty cost, if material, goes through a manual journal (D6: the
	// dispatch-time catalog-price guard keeps zero-cost goods out of stock
	// moves, so the books never silently absorb them either).
	if note.DocumentKind == deliverycontracts.DocumentKindReplacement {
		return nil, "", "", apperror.Conflict("Surat jalan pengganti tidak dijurnal otomatis")
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
	var revenue, tax int64
	for _, it := range note.Items {
		sl, ok := byKey[lineKeyOf(it.ProductID, it.VariantID, it.UOM)]
		if !ok {
			return nil, "", "", apperror.Conflict("Baris surat jalan tak cocok SO")
		}
		_, base, lineTax := lineEconomics(sl.Qty, sl.UnitPrice, sl.DiscountPct, sl.DiscountNominal, so.TaxType, so.TaxRate)
		rev := prorate(base, it.Qty, sl.Qty)
		revenue += rev
		tax += prorate(lineTax, it.Qty, sl.Qty)
		revenueLegs = append(revenueLegs, rawLeg{
			code: "4000", credit: rev,
			productID: it.ProductID, variantID: it.VariantID, note: sl.ProductName,
		})
		var cogs int64
		if v, ok := recordedValue(moves, it.ProductID, it.VariantID); ok {
			// The SJ's own out-movement, recorded negative.
			cogs = round(-v)
		} else {
			avg, err := s.averageCost(ctx, branchID, it.ProductID, it.VariantID)
			if err != nil {
				return nil, "", "", err
			}
			cogs = round(it.QtyBase * avg)
		}
		if cogs != 0 {
			costLegs = append(costLegs,
				rawLeg{code: "5000", debit: cogs, productID: it.ProductID, variantID: it.VariantID, note: sl.ProductName},
				rawLeg{code: "1300", credit: cogs, productID: it.ProductID, variantID: it.VariantID, note: sl.ProductName},
			)
		}
	}
	receivable := revenue + tax
	if receivable <= 0 {
		return nil, "", "", apperror.Conflict("Nilai surat jalan nol")
	}
	legs := make([]rawLeg, 0, len(revenueLegs)+len(costLegs)+2)
	legs = append(legs, rawLeg{code: "1200", debit: receivable, partyID: so.PartyID, note: so.PartyName})
	legs = append(legs, revenueLegs...)
	if tax != 0 {
		legs = append(legs, rawLeg{code: "2200", credit: tax, note: "PPN Keluaran " + so.Number})
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
	var cashKey string
	switch p.Method {
	case "cash":
		cashKey = "cash"
	case "transfer":
		cashKey = "bank"
	case "offset":
		// Contra legs still need a cash-side account for double entry;
		// the method tag (not the account) is what cash reports filter on.
		// Direction decides the side below, account is the default cash box.
		cashKey = "cash"
	default:
		return nil, "", "", apperror.Conflict("Metode pembayaran tak dikenal")
	}
	cash, err := s.mustMap(ctx, cashKey)
	if err != nil {
		return nil, "", "", err
	}
	var legs []rawLeg
	var memo string
	switch p.Direction {
	case "in":
		recv, err := s.mustMap(ctx, "receivable")
		if err != nil {
			return nil, "", "", err
		}
		legs = []rawLeg{
			{code: cash.Code, debit: p.Amount, note: p.Number},
			{code: recv.Code, credit: p.Amount, partyID: p.PartyID, note: p.PartyName},
		}
		memo = fmt.Sprintf("Terima %s", p.Number)
	case "out":
		pay, err := s.mustMap(ctx, "payable")
		if err != nil {
			return nil, "", "", err
		}
		legs = []rawLeg{
			{code: pay.Code, debit: p.Amount, partyID: p.PartyID, note: p.PartyName},
			{code: cash.Code, credit: p.Amount, note: p.Number},
		}
		memo = fmt.Sprintf("Bayar %s", p.Number)
	default:
		return nil, "", "", apperror.Conflict("Arah pembayaran tak dikenal")
	}
	lines, err := s.finish(ctx, legs)
	if err != nil {
		return nil, "", "", err
	}
	return lines, memo, p.PaidAt, nil
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
	// The party lives on the originating SO, not on the return document.
	// Without it the credit to Piutang would land in an "unknown party"
	// bucket and the receivables report would never clear.
	var partyID int64
	var partyName string
	if so, err := s.sales.GetByID(ctx, full.SalesOrderID); err != nil {
		return nil, "", "", apperror.Internal(err)
	} else if so != nil {
		partyID, partyName = so.PartyID, so.PartyName
	}
	legs := make([]rawLeg, 0, len(full.Items)*3+2)
	moves, err := s.repo.MoveValues(ctx, branchID, "sales_return", returnID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	for _, it := range full.Items {
		// Restock at the return's own recorded in-value (= pre-restock
		// average by construction of the cost ledger).
		var restock int64
		if v, ok := recordedValue(moves, it.ProductID, it.VariantID); ok {
			restock = round(v)
		} else {
			avg, err := s.averageCost(ctx, branchID, it.ProductID, it.VariantID)
			if err != nil {
				return nil, "", "", err
			}
			restock = round(it.QtyBase * avg)
		}
		if restock != 0 {
			legs = append(legs,
				rawLeg{code: "1300", debit: restock, productID: it.ProductID, variantID: it.VariantID},
				rawLeg{code: "5000", credit: restock, productID: it.ProductID, variantID: it.VariantID},
			)
		}
		// Revenue reversal books the TAX BASE per product (D1): the tax
		// itself has its own leg below, so booking LineTotal here would
		// double-count it for tax-inclusive orders.
		if it.TaxBase != 0 {
			legs = append(legs, rawLeg{
				code: "4100", debit: it.TaxBase,
				productID: it.ProductID, variantID: it.VariantID,
			})
		}
	}
	if full.Total <= 0 {
		return nil, "", "", apperror.Conflict("Nilai retur nol")
	}
	if full.TaxTotal != 0 {
		legs = append(legs, rawLeg{code: "2200", debit: full.TaxTotal})
	}
	legs = append(legs, rawLeg{code: "1200", credit: full.Total, partyID: partyID, note: partyName})
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
	var partyID int64
	var partyName string
	if po, err := s.purchasing.GetByID(ctx, full.PurchaseOrderID); err != nil {
		return nil, "", "", apperror.Internal(err)
	} else if po != nil {
		partyID, partyName = po.PartyID, po.PartyName
	}
	legs := make([]rawLeg, 0, len(full.Items)+4)
	legs = append(legs, rawLeg{code: "2100", debit: full.Total, partyID: partyID, note: partyName})
	if full.TaxTotal != 0 {
		legs = append(legs, rawLeg{code: "1400", credit: full.TaxTotal})
	}
	moves, err := s.repo.MoveValues(ctx, branchID, "purchase_return", returnID)
	if err != nil {
		return nil, "", "", apperror.Internal(err)
	}
	var relieved int64
	for _, it := range full.Items {
		// Relieve at the return's own recorded out-value (stored negative).
		var value int64
		if v, ok := recordedValue(moves, it.ProductID, it.VariantID); ok {
			value = round(-v)
		} else {
			avg, err := s.averageCost(ctx, branchID, it.ProductID, it.VariantID)
			if err != nil {
				return nil, "", "", err
			}
			value = round(it.QtyBase * avg)
		}
		relieved += value
		legs = append(legs, rawLeg{
			code: "1300", credit: value,
			productID: it.ProductID, variantID: it.VariantID,
		})
	}
	// The variance is priced off the pre-tax subtotal: with tax on its own
	// leg, the old total-based formula would drag VAT into the inventory
	// variance account.
	switch diff := full.Subtotal - relieved; {
	case diff > 0:
		legs = append(legs, rawLeg{code: "6200", credit: diff, note: "Selisih harga retur " + full.Number})
	case diff < 0:
		legs = append(legs, rawLeg{code: "6200", debit: -diff, note: "Selisih harga retur " + full.Number})
	}
	lines, err := s.finish(ctx, legs)
	if err != nil {
		return nil, "", "", err
	}
	return lines, fmt.Sprintf("Retur %s", full.Number), full.ReturnDate, nil
}
