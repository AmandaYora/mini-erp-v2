package application

import (
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	"mini-erp/internal/modules/purchasing/contracts"
	"mini-erp/internal/modules/purchasing/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/doclock"
	"mini-erp/internal/shared/money"
	"mini-erp/internal/shared/timeutil"
)

// Corrections of human input errors on confirmed orders, without touching
// what the books already hold (ADR-0023):
//
//   - header-only edit while receipts/payments/returns hang off the order;
//   - cancel with a reason; an order with a POSTED receipt cannot be cancelled
//     (a posted receipt is never undone — the goods go back by purchase return);
//   - short close ("tutup sisa order"): lines cut to what was received;
//   - price correction after receipt ("koreksi harga beli"): lines re-priced,
//     the value difference of the received part journalled by finance.
//
// Every correction keeps a history row with the line economics before and
// after. Receipts already posted stay valued at the OLD economics wherever
// they are rebuilt (contracts.LineAt), so a correction never double-counts.

// maxReasonLen caps every free-text reason (varchar(500)).
const maxReasonLen = 500

// SetCorrectionGate wires finance's period check and stock/HPP split
// (composition-root privilege; late-bound like the dependents).
func (s *Service) SetCorrectionGate(g financecontracts.PurchaseCorrectionGate) { s.gate = g }

// orderDeps is what hangs off an order.
type orderDeps struct {
	pendingReceipts int64
	postedReceipts  int64
	payments        bool
	returns         int64
}

func (d orderDeps) any() bool {
	return d.pendingReceipts > 0 || d.postedReceipts > 0 || d.payments || d.returns > 0
}

// label names what blocks a free edit, for the refusal message.
func (d orderDeps) label() string {
	var parts []string
	if d.pendingReceipts+d.postedReceipts > 0 {
		parts = append(parts, "penerimaan barang")
	}
	if d.payments {
		parts = append(parts, "pembayaran")
	}
	if d.returns > 0 {
		parts = append(parts, "retur")
	}
	return "Order sudah memiliki " + strings.Join(parts, ", ")
}

// cancelBlock names the first dependent that blocks a cancel and what to do.
func (d orderDeps) cancelBlock() error {
	switch {
	case d.postedReceipts > 0:
		return apperror.Conflict("Order sudah memiliki penerimaan barang yang terposting, dan penerimaan terposting tidak bisa dibatalkan. " +
			"Kembalikan barangnya lewat retur pembelian, atau pakai \"Tutup sisa order\" bila sisa pesanan tidak akan datang.")
	case d.pendingReceipts > 0:
		return apperror.Conflict("Order masih memiliki penerimaan pending; selesaikan atau buang penerimaan itu dulu")
	case d.payments:
		return apperror.Conflict("Order sudah memiliki pembayaran aktif; batalkan pembayarannya dulu")
	case d.returns > 0:
		return apperror.Conflict("Order sudah memiliki retur (draf atau terkonfirmasi); batalkan returnya dulu")
	}
	return nil
}

// dependentsOf reads what hangs off an order (receipts split pending/posted).
func (s *Service) dependentsOf(ctx context.Context, poID int64) (orderDeps, error) {
	var d orderDeps
	if s.dep.receipts == nil || s.dep.payments == nil || s.dep.returns == nil {
		return d, apperror.Internal(errDependentsNotWired)
	}
	live, err := s.dep.receipts.ActiveReceiptCount(ctx, poID)
	if err != nil {
		return d, apperror.Internal(err)
	}
	if live > 0 {
		if d.pendingReceipts, err = s.dep.receipts.PendingReceiptCount(ctx, poID); err != nil {
			return d, apperror.Internal(err)
		}
		d.postedReceipts = live - d.pendingReceipts
	}
	if d.payments, err = s.dep.payments.HasActiveAllocations(ctx, paymentcontracts.OrderPurchase, poID); err != nil {
		return d, apperror.Internal(err)
	}
	if d.returns, err = s.dep.returns.ActiveReturnCount(ctx, poID); err != nil {
		return d, apperror.Internal(err)
	}
	return d, nil
}

// HasDependents reports whether anything hangs off the order (the detail page
// picks the full form or the header-only dialog with it).
func (s *Service) HasDependents(ctx context.Context, poID int64) (bool, error) {
	d, err := s.dependentsOf(ctx, poID)
	return d.any(), err
}

// --- header-only edit ------------------------------------------------------

// HeaderInput is what stays editable on any open or completed order.
type HeaderInput struct {
	DueDate               string
	PaymentTermDays       int
	Notes                 string
	SupplierInvoiceNumber string
	SupplierInvoiceDate   string
	TaxInvoiceNumber      string
	TaxInvoiceDate        string
	AssignedUserID        int64
	Reason                string
}

func headerOf(in OrderInput) HeaderInput {
	return HeaderInput{
		DueDate: in.DueDate, PaymentTermDays: in.PaymentTermDays, Notes: in.Notes,
		SupplierInvoiceNumber: in.SupplierInvoiceNumber, SupplierInvoiceDate: in.SupplierInvoiceDate,
		TaxInvoiceNumber: in.TaxInvoiceNumber, TaxInvoiceDate: in.TaxInvoiceDate,
		AssignedUserID: in.AssignedUserID, Reason: in.Reason,
	}
}

// UpdateHeader rewrites only the header fields of a draft, confirmed or
// completed order (due date, credit days, notes, supplier invoice, tax
// invoice, person in charge). Nothing economic moves, so it is allowed under
// receipts, payments and returns. Audited with before/after and the reason.
func (s *Service) UpdateHeader(ctx context.Context, actorID, branchID, id int64, in HeaderInput) (*contracts.PurchaseOrder, error) {
	if len(strings.TrimSpace(in.Reason)) > maxReasonLen {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "reason", Message: "maksimal 500 karakter"}})
	}
	var out *contracts.PurchaseOrder
	err := doclock.Lock("po:"+itoa64(id), func() error {
		return s.withPaymentLock(ctx, branchID, id, func() error {
			existing, err := s.mustOwnBranch(ctx, id, branchID)
			if err != nil {
				return err
			}
			if existing.Status == contracts.StatusCancelled {
				return apperror.Conflict("Order yang dibatalkan tidak bisa diubah")
			}
			res, err := s.applyHeader(ctx, actorID, branchID, existing, in)
			out = res
			return err
		})
	})
	return out, err
}

// applyHeader validates and stores the header fields of an order the caller
// already locked and read.
func (s *Service) applyHeader(ctx context.Context, actorID, branchID int64, existing *contracts.PurchaseOrder, in HeaderInput) (*contracts.PurchaseOrder, error) {
	if in.PaymentTermDays < 0 || in.PaymentTermDays > 3650 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "paymentTermDays", Message: "harus 0–3650"}})
	}
	if d := strings.TrimSpace(in.DueDate); d != "" {
		if _, err := timeutil.ParseDateInput(d); err != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "dueDate", Message: "format tanggal harus YYYY-MM-DD"}})
		}
	}
	invNo, invDate, err := resolveSupplierInvoice(in.SupplierInvoiceNumber, in.SupplierInvoiceDate)
	if err != nil {
		return nil, err
	}
	taxNo, taxDate, err := resolveTaxInvoice(in.TaxInvoiceNumber, in.TaxInvoiceDate)
	if err != nil {
		return nil, err
	}
	if err := s.checkAssignee(ctx, branchID, in.AssignedUserID); err != nil {
		return nil, err
	}
	due := resolveDueDate(in.DueDate, invDate, in.PaymentTermDays)
	if existing.PaymentTerms == "net" && due == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "dueDate",
			Message: "wajib diisi untuk termin net (atau isi hari termin dan tanggal faktur supplier)"}})
	}
	before := headerSnapshot(existing)
	existing.DueDate = due
	existing.Notes = in.Notes
	existing.SupplierInvoiceNumber, existing.SupplierInvoiceDate = invNo, invDate
	existing.TaxInvoiceNumber, existing.TaxInvoiceDate, existing.AssignedUserID = taxNo, taxDate, in.AssignedUserID
	existing.PaymentTermDays = in.PaymentTermDays
	if err := s.repo.UpdateHeaderLight(ctx, existing, actorID); err != nil {
		return nil, dberr.Map(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{
		Action: "purchase_orders.update", Entity: "purchase_order", EntityID: existing.ID, BranchID: branchID, ActorID: actorID,
		Note: existing.Number + " (header)", Before: before, After: headerSnapshot(existing),
		Metadata: map[string]any{"scope": "header", "reason": strings.TrimSpace(in.Reason)},
	})
	return s.GetByID(ctx, existing.ID)
}

func headerSnapshot(o *contracts.PurchaseOrder) map[string]any {
	return map[string]any{
		"dueDate": dayOf(o.DueDate), "paymentTermDays": o.PaymentTermDays, "notes": o.Notes,
		"supplierInvoiceNumber": o.SupplierInvoiceNumber, "supplierInvoiceDate": dayOf(o.SupplierInvoiceDate),
		"taxInvoiceNumber": o.TaxInvoiceNumber, "taxInvoiceDate": dayOf(o.TaxInvoiceDate),
		"assignedUserId": o.AssignedUserID,
	}
}

// orderSnapshot is the header plus the economic fields (full edit audit).
func orderSnapshot(o *contracts.PurchaseOrder) map[string]any {
	m := headerSnapshot(o)
	m["partyId"], m["orderDate"], m["paymentTerms"] = o.PartyID, dayOf(o.OrderDate), o.PaymentTerms
	m["taxType"], m["taxRate"], m["grandTotal"] = o.TaxType, o.TaxRate, o.GrandTotal
	return m
}

func itemsSnapshot(items []*contracts.Item) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{
			"productId": it.ProductID, "variantId": it.VariantID, "uom": it.UOM, "qty": it.Qty,
			"unitPrice": it.UnitPrice, "discountPct": it.DiscountPct, "discountNominal": it.DiscountNominal,
			"lineTotal": it.LineTotal,
		})
	}
	return out
}

func dayOf(v string) string {
	if len(v) >= 10 {
		return v[:10]
	}
	return v
}

// economicChanges names every non-header difference between the stored order
// and an update payload: those are refused (never dropped) once something
// hangs off the order.
func economicChanges(o *contracts.PurchaseOrder, in OrderInput) []string {
	var out []string
	if in.PartyID != o.PartyID {
		out = append(out, "pemasok")
	}
	if in.PaymentTerms != o.PaymentTerms {
		out = append(out, "termin")
	}
	if in.TaxType != o.TaxType || in.TaxRate != o.TaxRate {
		out = append(out, "pajak order")
	}
	if d := strings.TrimSpace(in.OrderDate); d != "" && d != dayOf(o.OrderDate) {
		out = append(out, "tanggal order")
	}
	type lineSig struct{ key, price, tax, text string }
	sigOf := func(product, variant int64, uom string, qty float64) string {
		return strconv.FormatInt(product, 10) + "/" + strconv.FormatInt(variant, 10) + "/" + strings.TrimSpace(uom) +
			"/" + strconv.FormatFloat(qty, 'f', 6, 64)
	}
	have := make([]lineSig, 0, len(o.Items))
	for _, it := range o.Items {
		have = append(have, lineSig{
			key:   sigOf(it.ProductID, it.VariantID, it.UOM, it.Qty),
			price: strconv.FormatInt(it.UnitPrice, 10) + "/" + strconv.FormatFloat(it.DiscountPct, 'f', 4, 64) + "/" + strconv.FormatInt(it.DiscountNominal, 10),
			tax:   strconv.FormatBool(it.Taxable) + "/" + strconv.FormatFloat(it.TaxRate, 'f', 4, 64),
			text:  strings.TrimSpace(it.DiscountReason) + "\x00" + strings.TrimSpace(it.Note),
		})
	}
	want := make([]lineSig, 0, len(in.Items))
	for _, it := range in.Items {
		taxable := it.Taxable == nil || *it.Taxable
		rate := in.TaxRate
		if it.TaxRate != nil && in.TaxType != "none" {
			rate = *it.TaxRate
		}
		if !taxable || in.TaxType == "none" {
			rate = 0
		}
		want = append(want, lineSig{
			key:   sigOf(it.ProductID, it.VariantID, it.UOM, it.Qty),
			price: strconv.FormatInt(it.UnitPrice, 10) + "/" + strconv.FormatFloat(it.DiscountPct, 'f', 4, 64) + "/" + strconv.FormatInt(it.DiscountNominal, 10),
			tax:   strconv.FormatBool(taxable) + "/" + strconv.FormatFloat(rate, 'f', 4, 64),
			text:  strings.TrimSpace(it.DiscountReason) + "\x00" + strings.TrimSpace(it.Note),
		})
	}
	byKey := func(l []lineSig) { sort.Slice(l, func(i, j int) bool { return l[i].key < l[j].key }) }
	byKey(have)
	byKey(want)
	if len(have) != len(want) {
		return append(out, "baris barang/qty")
	}
	var price, tax, text bool
	for i := range have {
		if have[i].key != want[i].key {
			return append(out, "baris barang/qty")
		}
		price = price || have[i].price != want[i].price
		tax = tax || have[i].tax != want[i].tax
		text = text || have[i].text != want[i].text
	}
	if price {
		out = append(out, "harga/diskon")
	}
	if tax {
		out = append(out, "pajak baris")
	}
	if text {
		out = append(out, "keterangan baris")
	}
	return out
}

// --- shared correction plumbing -------------------------------------------

func lineKey(productID, variantID int64, uom string) string {
	return strconv.FormatInt(productID, 10) + "/" + strconv.FormatInt(variantID, 10) + "/" + strings.TrimSpace(uom)
}

// receivedByLine maps item id to the line-UOM qty its POSTED receipts brought
// in. Receipts are matched to the FIRST line of a product/variant/uom, the same
// rule the receipt journal uses.
func (s *Service) receivedByLine(ctx context.Context, o *contracts.PurchaseOrder) (map[int64]float64, error) {
	posted, err := s.dep.receipts.PostedQtyBefore(ctx, o.ID, math.MaxInt64)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make(map[int64]float64, len(o.Items))
	seen := map[string]bool{}
	for _, it := range o.Items {
		k := lineKey(it.ProductID, it.VariantID, it.UOM)
		if seen[k] {
			continue
		}
		seen[k] = true
		out[it.ID] = posted[k]
	}
	return out, nil
}

// headerDelta folds line changes into header money without recomputing the
// untouched lines (a migrated order keeps its own totals): subtotal = Σ net,
// discount = Σ(gross − net), tax and grand from the stored split.
type headerDelta struct{ subtotal, discount, tax, grand int64 }

func lineNet(taxType string, total, tax int64) int64 {
	if taxType == "exclude" {
		return total - tax
	}
	return total
}

func (h *headerDelta) add(taxType string, oldIt, newIt *contracts.Item) {
	oldNet, newNet := lineNet(taxType, oldIt.LineTotal, oldIt.TaxAmount), lineNet(taxType, newIt.LineTotal, newIt.TaxAmount)
	oldGross := int64(float64(oldIt.UnitPrice)*oldIt.Qty + 0.5)
	newGross := int64(float64(newIt.UnitPrice)*newIt.Qty + 0.5)
	h.subtotal += newNet - oldNet
	h.discount += (newGross - newNet) - (oldGross - oldNet)
	h.tax += newIt.TaxAmount - oldIt.TaxAmount
	h.grand += newIt.LineTotal - oldIt.LineTotal
}

// paidOf is what was allocated to the order (payments and offsets).
func (s *Service) paidOf(ctx context.Context, id int64) (int64, error) {
	paid, err := s.dep.payments.PaidForOrders(ctx, paymentcontracts.OrderPurchase, []int64{id})
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return paid[id], nil
}

func checkReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", apperror.Validation("", []apperror.FieldError{{Field: "reason", Message: "wajib diisi"}})
	}
	if len(reason) > maxReasonLen {
		return "", apperror.Validation("", []apperror.FieldError{{Field: "reason", Message: "maksimal 500 karakter"}})
	}
	return reason, nil
}

// CorrectionLineView is one line of a correction preview.
type CorrectionLineView struct {
	ItemID          int64   `json:"itemId"`
	ProductID       int64   `json:"productId"`
	VariantID       int64   `json:"variantId"`
	ProductName     string  `json:"productName"`
	UOM             string  `json:"uom"`
	StockTracked    bool    `json:"stockTracked"`
	Qty             float64 `json:"qty"`
	NewQty          float64 `json:"newQty"`
	ReceivedQty     float64 `json:"receivedQty"`
	OldUnitPrice    int64   `json:"oldUnitPrice"`
	NewUnitPrice    int64   `json:"newUnitPrice"`
	OldDiscountPct  float64 `json:"oldDiscountPct"`
	NewDiscountPct  float64 `json:"newDiscountPct"`
	OldDiscountNom  int64   `json:"oldDiscountNominal"`
	NewDiscountNom  int64   `json:"newDiscountNominal"`
	OldLineTotal    int64   `json:"oldLineTotal"`
	NewLineTotal    int64   `json:"newLineTotal"`
	ValueDelta      int64   `json:"valueDelta"`
	TaxDelta        int64   `json:"taxDelta"`
	ReceivedQtyBase float64 `json:"-"`
}

// CorrectionPreview is what a correction would do (and, after commit, did).
type CorrectionPreview struct {
	Kind          string               `json:"kind"`
	Lines         []CorrectionLineView `json:"lines"`
	OldGrandTotal int64                `json:"oldGrandTotal"`
	NewGrandTotal int64                `json:"newGrandTotal"`
	Paid          int64                `json:"paid"`
	// Outstanding is the new total minus what was paid; negative = the
	// supplier was overpaid (the payment module shows it as their balance).
	Outstanding int64 `json:"outstanding"`
	// Price correction only: the received part's change, split by account.
	ValueDelta   int64                                 `json:"valueDelta"`
	TaxDelta     int64                                 `json:"taxDelta"`
	PayableDelta int64                                 `json:"payableDelta"`
	StockDelta   int64                                 `json:"stockDelta"`
	CogsDelta    int64                                 `json:"cogsDelta"`
	ExpenseDelta int64                                 `json:"expenseDelta"`
	Positions    []financecontracts.ReceivedValueDelta `json:"positions"`
	CorrectionID int64                                 `json:"correctionId,omitempty"`
}

// correctionPlan is a validated correction ready to write.
type correctionPlan struct {
	order   *contracts.PurchaseOrder
	items   []*contracts.Item
	delta   headerDelta
	lines   []*contracts.CorrectionLine
	preview *CorrectionPreview
}

func (p *correctionPlan) write(kind, reason string, actorID int64) infrastructure.CorrectionWrite {
	o := p.order
	return infrastructure.CorrectionWrite{
		Order: o, Items: p.items, ActorID: actorID,
		Subtotal: o.Subtotal + p.delta.subtotal, Discount: o.DiscountTotal + p.delta.discount,
		Tax: o.TaxTotal + p.delta.tax, Grand: o.GrandTotal + p.delta.grand,
		Correction: &contracts.Correction{
			Kind: kind, Date: timeutil.TodayWIB(), Reason: reason,
			OldGrandTotal: o.GrandTotal, NewGrandTotal: o.GrandTotal + p.delta.grand,
			ValueDelta: p.preview.ValueDelta, TaxDelta: p.preview.TaxDelta, Lines: p.lines,
		},
	}
}

// lockedCorrection runs fn under the order lock and the payment lock (the
// total changes under the payments), like an edit.
func (s *Service) lockedCorrection(ctx context.Context, branchID, id int64, fn func() error) error {
	if _, err := s.mustOwnBranch(ctx, id, branchID); err != nil {
		return err
	}
	return doclock.Lock("po:"+itoa64(id), func() error {
		return s.withPaymentLock(ctx, branchID, id, fn)
	})
}

func correctionWriteErr(err error) error {
	if errors.Is(err, infrastructure.ErrOrderMoved) {
		return apperror.Conflict("Order berubah saat dikoreksi, muat ulang")
	}
	var ae *apperror.AppError
	if errors.As(err, &ae) {
		return err
	}
	return dberr.Map(err)
}

// --- short close ("tutup sisa order") ---------------------------------------

func (s *Service) planShortClose(ctx context.Context, branchID, id int64) (*correctionPlan, error) {
	o, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if o.Status != contracts.StatusConfirmed {
		return nil, apperror.Conflict("Hanya order terkonfirmasi yang sisanya bisa ditutup")
	}
	if o, err = s.enrich(ctx, o); err != nil {
		return nil, err
	}
	deps, err := s.dependentsOf(ctx, id)
	if err != nil {
		return nil, err
	}
	if deps.pendingReceipts > 0 {
		return nil, apperror.Conflict("Order masih memiliki penerimaan pending; selesaikan atau buang penerimaan itu dulu")
	}
	if deps.postedReceipts == 0 {
		return nil, apperror.Conflict("Belum ada barang yang diterima; bila order tidak jadi, batalkan order")
	}
	received, err := s.receivedByLine(ctx, o)
	if err != nil {
		return nil, err
	}
	paid, err := s.paidOf(ctx, id)
	if err != nil {
		return nil, err
	}
	p := &correctionPlan{order: o, preview: &CorrectionPreview{Kind: contracts.CorrectionShortClose, Lines: []CorrectionLineView{},
		Positions: []financecontracts.ReceivedValueDelta{}, OldGrandTotal: o.GrandTotal, Paid: paid}}
	for _, it := range o.Items {
		r := received[it.ID]
		if r >= it.Qty-1e-9 {
			continue // fully received: nothing to close on this line
		}
		cut := *it
		cut.Qty = r
		cut.QtyBase = r * it.UOMFactor
		cut.DiscountNominal = money.Prorate(it.DiscountNominal, r, it.Qty)
		// The stored split, pro-rated exactly as the receipts booked it: the
		// order total then equals the payable the receipt journals hold.
		cut.TaxBase = money.Prorate(it.TaxBase, r, it.Qty)
		cut.TaxAmount = money.Prorate(it.TaxAmount, r, it.Qty)
		cut.LineTotal = cut.TaxBase + cut.TaxAmount
		p.delta.add(o.TaxType, it, &cut)
		p.items = append(p.items, &cut)
		p.lines = append(p.lines, &contracts.CorrectionLine{
			OrderItemID: it.ID, ProductID: it.ProductID, VariantID: it.VariantID, ProductName: it.ProductName,
			UOM: it.UOM, StockTracked: it.StockTracked, ReceivedQty: r, ReceivedQtyBase: r * it.UOMFactor,
			OldQty: it.Qty, NewQty: r, OldUnitPrice: it.UnitPrice, NewUnitPrice: it.UnitPrice,
			OldDiscountPct: it.DiscountPct, NewDiscountPct: it.DiscountPct,
			OldDiscountNominal: it.DiscountNominal, NewDiscountNominal: cut.DiscountNominal,
			OldTaxBase: it.TaxBase, NewTaxBase: cut.TaxBase, OldTaxAmount: it.TaxAmount, NewTaxAmount: cut.TaxAmount,
			OldLineTotal: it.LineTotal, NewLineTotal: cut.LineTotal,
		})
		p.preview.Lines = append(p.preview.Lines, CorrectionLineView{
			ItemID: it.ID, ProductID: it.ProductID, VariantID: it.VariantID, ProductName: it.ProductName, UOM: it.UOM,
			StockTracked: it.StockTracked, Qty: it.Qty, NewQty: r, ReceivedQty: r,
			OldUnitPrice: it.UnitPrice, NewUnitPrice: it.UnitPrice, OldDiscountPct: it.DiscountPct, NewDiscountPct: it.DiscountPct,
			OldDiscountNom: it.DiscountNominal, NewDiscountNom: cut.DiscountNominal,
			OldLineTotal: it.LineTotal, NewLineTotal: cut.LineTotal,
		})
	}
	if len(p.items) == 0 {
		return nil, apperror.Conflict("Semua barang sudah diterima penuh; tidak ada sisa order yang perlu ditutup")
	}
	p.preview.NewGrandTotal = o.GrandTotal + p.delta.grand
	p.preview.Outstanding = p.preview.NewGrandTotal - paid
	return p, nil
}

// PreviewShortClose shows what closing the rest of the order would do.
func (s *Service) PreviewShortClose(ctx context.Context, branchID, id int64) (*CorrectionPreview, error) {
	p, err := s.planShortClose(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	return p.preview, nil
}

// ShortClose closes the rest of a partly received order ("tutup sisa order"):
// every line is cut to its received qty (posted receipts) with its stored
// split pro-rated, the order completes, and the reason is kept. The payable
// was journalled per receipt, so no journal moves: the supplier balance becomes
// received value − paid.
func (s *Service) ShortClose(ctx context.Context, actorID, branchID, id int64, reason string) (*contracts.PurchaseOrder, *CorrectionPreview, error) {
	reason, err := checkReason(reason)
	if err != nil {
		return nil, nil, err
	}
	var preview *CorrectionPreview
	err = s.lockedCorrection(ctx, branchID, id, func() error {
		p, err := s.planShortClose(ctx, branchID, id)
		if err != nil {
			return err
		}
		cid, err := s.repo.ApplyCorrection(ctx, p.write(contracts.CorrectionShortClose, reason, actorID),
			func(ctx context.Context, tx infrastructure.Execer, cid int64) error {
				return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
					Action: "purchase_orders.short_close", Entity: "purchase_order", EntityID: id, BranchID: branchID, ActorID: actorID,
					Note:   p.order.Number,
					Before: map[string]any{"status": p.order.Status, "grandTotal": p.preview.OldGrandTotal},
					After:  map[string]any{"status": contracts.StatusCompleted, "grandTotal": p.preview.NewGrandTotal},
					Metadata: map[string]any{"reason": reason, "correctionId": cid, "lines": len(p.lines),
						"paid": p.preview.Paid, "outstanding": p.preview.Outstanding},
				})
			})
		if err != nil {
			return correctionWriteErr(err)
		}
		p.preview.CorrectionID = cid
		preview = p.preview
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	o, err := s.GetByID(ctx, id)
	return o, preview, err
}

// --- price correction after receipt ("koreksi harga beli") ------------------

// PriceLineInput is one line's corrected price. Nil discounts keep the line's.
type PriceLineInput struct {
	LineID          int64
	UnitPrice       int64
	DiscountPct     *float64
	DiscountNominal *int64
}

func (s *Service) planPriceCorrection(ctx context.Context, branchID, id int64, in []PriceLineInput, split bool) (*correctionPlan, error) {
	o, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	switch o.Status {
	case contracts.StatusConfirmed, contracts.StatusCompleted:
	case contracts.StatusCancelled:
		return nil, apperror.Conflict("Order yang dibatalkan tidak bisa dikoreksi")
	default:
		return nil, apperror.Conflict("Order draf diubah lewat tombol Ubah, bukan koreksi harga")
	}
	if len(in) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "lines", Message: "wajib diisi"}})
	}
	if o, err = s.enrich(ctx, o); err != nil {
		return nil, err
	}
	deps, err := s.dependentsOf(ctx, id)
	if err != nil {
		return nil, err
	}
	if deps.pendingReceipts > 0 {
		return nil, apperror.Conflict("Order masih memiliki penerimaan pending; selesaikan atau buang penerimaan itu dulu")
	}
	if deps.returns > 0 {
		return nil, apperror.Conflict("Order sudah memiliki retur pembelian (draf atau terkonfirmasi) yang memakai harga lama. " +
			"Batalkan returnya dulu, koreksi harga, lalu buat ulang returnya.")
	}
	byID := make(map[int64]*contracts.Item, len(o.Items))
	for _, it := range o.Items {
		byID[it.ID] = it
	}
	received, err := s.receivedByLine(ctx, o)
	if err != nil {
		return nil, err
	}
	paid, err := s.paidOf(ctx, id)
	if err != nil {
		return nil, err
	}
	p := &correctionPlan{order: o, preview: &CorrectionPreview{Kind: contracts.CorrectionPrice, Lines: []CorrectionLineView{},
		Positions: []financecontracts.ReceivedValueDelta{}, OldGrandTotal: o.GrandTotal, Paid: paid}}
	var fields []apperror.FieldError
	seen := map[int64]bool{}
	for i, l := range in {
		tag := "baris " + strconv.Itoa(i+1) + ": "
		it := byID[l.LineID]
		if it == nil {
			fields = append(fields, apperror.FieldError{Field: "lines", Message: tag + "baris order tidak ditemukan"})
			continue
		}
		if seen[l.LineID] {
			fields = append(fields, apperror.FieldError{Field: "lines", Message: tag + "baris ganda"})
			continue
		}
		seen[l.LineID] = true
		pct, nominal := it.DiscountPct, it.DiscountNominal
		if l.DiscountPct != nil {
			pct = *l.DiscountPct
		}
		if l.DiscountNominal != nil {
			nominal = *l.DiscountNominal
		}
		switch {
		case l.UnitPrice < 0:
			fields = append(fields, apperror.FieldError{Field: "lines", Message: tag + "harga tidak boleh negatif"})
			continue
		case pct < 0 || pct > 100:
			fields = append(fields, apperror.FieldError{Field: "lines", Message: tag + "diskon harus 0–100"})
			continue
		case nominal < 0:
			fields = append(fields, apperror.FieldError{Field: "lines", Message: tag + "diskon nominal tidak boleh negatif"})
			continue
		}
		if l.UnitPrice == it.UnitPrice && pct == it.DiscountPct && nominal == it.DiscountNominal {
			continue // unchanged line
		}
		// The line's own tax treatment (taxable flag, rate), as PO pricing does.
		taxType, rate := o.TaxType, it.TaxRate
		if !it.Taxable || o.TaxType == "none" {
			taxType, rate = "none", 0
		}
		_, base, tax, total := money.LineAmounts(it.Qty, l.UnitPrice, pct, nominal, taxType, rate).Split()
		if total < 0 {
			fields = append(fields, apperror.FieldError{Field: "lines", Message: tag + "total baris negatif"})
			continue
		}
		next := *it
		next.UnitPrice, next.DiscountPct, next.DiscountNominal = l.UnitPrice, pct, nominal
		next.TaxBase, next.TaxAmount, next.LineTotal = base, tax, total
		r := received[it.ID]
		// The received part changes by exactly what its receipts booked versus
		// what they would book now (both pro-rated the way receipts are).
		valueDelta := money.Prorate(base, r, it.Qty) - money.Prorate(it.TaxBase, r, it.Qty)
		taxDelta := money.Prorate(tax, r, it.Qty) - money.Prorate(it.TaxAmount, r, it.Qty)
		p.delta.add(o.TaxType, it, &next)
		p.items = append(p.items, &next)
		p.lines = append(p.lines, &contracts.CorrectionLine{
			OrderItemID: it.ID, ProductID: it.ProductID, VariantID: it.VariantID, ProductName: it.ProductName,
			UOM: it.UOM, StockTracked: it.StockTracked, ReceivedQty: r, ReceivedQtyBase: r * it.UOMFactor,
			OldQty: it.Qty, NewQty: it.Qty, OldUnitPrice: it.UnitPrice, NewUnitPrice: l.UnitPrice,
			OldDiscountPct: it.DiscountPct, NewDiscountPct: pct, OldDiscountNominal: it.DiscountNominal, NewDiscountNominal: nominal,
			OldTaxBase: it.TaxBase, NewTaxBase: base, OldTaxAmount: it.TaxAmount, NewTaxAmount: tax,
			OldLineTotal: it.LineTotal, NewLineTotal: total, ValueDelta: valueDelta, TaxDelta: taxDelta,
		})
		p.preview.Lines = append(p.preview.Lines, CorrectionLineView{
			ItemID: it.ID, ProductID: it.ProductID, VariantID: it.VariantID, ProductName: it.ProductName, UOM: it.UOM,
			StockTracked: it.StockTracked, Qty: it.Qty, NewQty: it.Qty, ReceivedQty: r, ReceivedQtyBase: r * it.UOMFactor,
			OldUnitPrice: it.UnitPrice, NewUnitPrice: l.UnitPrice, OldDiscountPct: it.DiscountPct, NewDiscountPct: pct,
			OldDiscountNom: it.DiscountNominal, NewDiscountNom: nominal, OldLineTotal: it.LineTotal, NewLineTotal: total,
			ValueDelta: valueDelta, TaxDelta: taxDelta,
		})
		p.preview.ValueDelta += valueDelta
		p.preview.TaxDelta += taxDelta
		if !it.StockTracked {
			p.preview.ExpenseDelta += valueDelta
		}
	}
	if len(fields) > 0 {
		return nil, apperror.Validation("", fields)
	}
	if len(p.items) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "lines", Message: "tidak ada harga yang berubah"}})
	}
	p.preview.NewGrandTotal = o.GrandTotal + p.delta.grand
	p.preview.Outstanding = p.preview.NewGrandTotal - paid
	p.preview.PayableDelta = p.preview.ValueDelta + p.preview.TaxDelta
	if p.preview.ValueDelta != 0 || p.preview.TaxDelta != 0 {
		if s.gate == nil {
			return nil, apperror.Internal(errors.New("purchasing correction gate not wired"))
		}
		// The journal is dated today: its month must still be open.
		if err := s.gate.RequireOpenPeriod(ctx, branchID, timeutil.TodayWIB()); err != nil {
			return nil, err
		}
		if split {
			if err := s.splitPreview(ctx, branchID, p.preview); err != nil {
				return nil, err
			}
		}
	}
	return p, nil
}

// splitPreview asks finance how the received stock value change splits
// between what is still on hand and what was already issued (per position).
func (s *Service) splitPreview(ctx context.Context, branchID int64, pv *CorrectionPreview) error {
	byPos := map[string]*financecontracts.ReceivedValueDelta{}
	var order []string
	for _, l := range pv.Lines {
		if !l.StockTracked || l.ValueDelta == 0 {
			continue
		}
		k := strconv.FormatInt(l.ProductID, 10) + "/" + strconv.FormatInt(l.VariantID, 10)
		d := byPos[k]
		if d == nil {
			d = &financecontracts.ReceivedValueDelta{ProductID: l.ProductID, VariantID: l.VariantID}
			byPos[k] = d
			order = append(order, k)
		}
		d.ReceivedQtyBase += l.ReceivedQtyBase
		d.Delta += l.ValueDelta
	}
	if len(order) == 0 {
		return nil
	}
	items := make([]financecontracts.ReceivedValueDelta, 0, len(order))
	for _, k := range order {
		items = append(items, *byPos[k])
	}
	out, err := s.gate.SplitReceivedValue(ctx, branchID, items)
	if err != nil {
		return err
	}
	pv.Positions = out
	for _, d := range out {
		pv.StockDelta += d.Stock
		pv.CogsDelta += d.Issued
	}
	return nil
}

// PreviewPriceCorrection shows what a price correction would change: line
// totals, order total, the received part's value/tax change, and its split
// between stock still on hand (Persediaan) and stock already issued (HPP).
func (s *Service) PreviewPriceCorrection(ctx context.Context, branchID, id int64, in []PriceLineInput) (*CorrectionPreview, error) {
	p, err := s.planPriceCorrection(ctx, branchID, id, in, true)
	if err != nil {
		return nil, err
	}
	return p.preview, nil
}

// CorrectPrices re-prices lines of a confirmed or completed order after
// receipt. The order (the supplier sub-ledger) takes the new prices and totals;
// a history row keeps old/new per line and the received part's value change,
// which finance journals as source purchase_price_correction (Persediaan for
// the part on hand with a matching cost-ledger row, HPP for the part issued,
// PPN Masukan, Hutang on the supplier). Audited with the reason.
func (s *Service) CorrectPrices(ctx context.Context, actorID, branchID, id int64, in []PriceLineInput, reason string) (*contracts.PurchaseOrder, *CorrectionPreview, error) {
	reason, err := checkReason(reason)
	if err != nil {
		return nil, nil, err
	}
	var preview *CorrectionPreview
	err = s.lockedCorrection(ctx, branchID, id, func() error {
		p, err := s.planPriceCorrection(ctx, branchID, id, in, false)
		if err != nil {
			return err
		}
		cid, err := s.repo.ApplyCorrection(ctx, p.write(contracts.CorrectionPrice, reason, actorID),
			func(ctx context.Context, tx infrastructure.Execer, cid int64) error {
				return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
					Action: "purchase_orders.price_correct", Entity: "purchase_order", EntityID: id, BranchID: branchID, ActorID: actorID,
					Note:   p.order.Number,
					Before: map[string]any{"grandTotal": p.preview.OldGrandTotal, "lines": correctionSide(p.lines, false)},
					After:  map[string]any{"grandTotal": p.preview.NewGrandTotal, "lines": correctionSide(p.lines, true)},
					Metadata: map[string]any{"reason": reason, "correctionId": cid, "valueDelta": p.preview.ValueDelta,
						"taxDelta": p.preview.TaxDelta},
				})
			})
		if err != nil {
			return correctionWriteErr(err)
		}
		p.preview.CorrectionID = cid
		preview = p.preview
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	o, err := s.GetByID(ctx, id)
	return o, preview, err
}

func correctionSide(lines []*contracts.CorrectionLine, after bool) []map[string]any {
	out := make([]map[string]any, 0, len(lines))
	for _, l := range lines {
		price, pct, nom, total := l.OldUnitPrice, l.OldDiscountPct, l.OldDiscountNominal, l.OldLineTotal
		if after {
			price, pct, nom, total = l.NewUnitPrice, l.NewDiscountPct, l.NewDiscountNominal, l.NewLineTotal
		}
		out = append(out, map[string]any{"itemId": l.OrderItemID, "product": l.ProductName,
			"unitPrice": price, "discountPct": pct, "discountNominal": nom, "lineTotal": total})
	}
	return out
}

// --- contract reads ----------------------------------------------------------

// GetCorrection implements the contract (see there).
func (s *Service) GetCorrection(ctx context.Context, id int64) (*contracts.Correction, error) {
	c, err := s.repo.GetCorrection(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if c == nil {
		return nil, nil
	}
	if err := s.decorateCorrections(ctx, []*contracts.Correction{c}); err != nil {
		return nil, err
	}
	return c, nil
}

// ListPriceCorrectionsAfter implements the contract (see there).
func (s *Service) ListPriceCorrectionsAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*contracts.Correction, error) {
	cs, err := s.repo.ListPriceCorrectionsAfter(ctx, branchID, afterID, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if err := s.decorateCorrections(ctx, cs); err != nil {
		return nil, err
	}
	return cs, nil
}

// CorrectionsByOrder implements the contract (see there).
func (s *Service) CorrectionsByOrder(ctx context.Context, orderID int64) ([]*contracts.Correction, error) {
	cs, err := s.repo.CorrectionsByOrder(ctx, orderID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return cs, nil
}

// OrderCorrections lists an order's corrections with names (detail page).
func (s *Service) OrderCorrections(ctx context.Context, orderID int64) ([]*contracts.Correction, error) {
	cs, err := s.CorrectionsByOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if err := s.decorateCorrections(ctx, cs); err != nil {
		return nil, err
	}
	return cs, nil
}

// decorateCorrections fills order number, supplier and creator name in one
// grouped read each.
func (s *Service) decorateCorrections(ctx context.Context, cs []*contracts.Correction) error {
	if len(cs) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(cs))
	users := make([]int64, 0, len(cs))
	for _, c := range cs {
		ids = append(ids, c.OrderID)
		if c.CreatedBy > 0 {
			users = append(users, c.CreatedBy)
		}
	}
	sums, err := s.repo.SummariesByIDs(ctx, ids)
	if err != nil {
		return apperror.Internal(err)
	}
	partyIDs := make([]int64, 0, len(sums))
	for _, o := range sums {
		partyIDs = append(partyIDs, o.PartyID)
	}
	names, err := s.parties.NamesByIDs(ctx, partyIDs)
	if err != nil {
		return err
	}
	var userNames map[int64]string
	if s.users != nil && len(users) > 0 {
		userNames, _ = s.users.NamesByIDs(ctx, users) // display only
	}
	for _, c := range cs {
		if o := sums[c.OrderID]; o != nil {
			c.OrderNumber, c.PartyID, c.PartyName = o.Number, o.PartyID, names[o.PartyID]
		}
		c.CreatedByName = userNames[c.CreatedBy]
	}
	return nil
}
