package application

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/modules/sales/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/doclock"
	"mini-erp/internal/shared/money"
)

// Corrections of an order entered wrong, by the users themselves: edit a
// confirmed order (full while nothing hangs off it, header-only after), cancel
// with a reason, and "tutup sisa order" (short-close) for an order that will
// never ship in full. Every correction leaves a before/after trail.

// maxReasonLen is the width of the reason columns (cancel_reason,
// short_close_reason).
const maxReasonLen = 255

// Edit modes of an order as the detail page offers them.
const (
	EditFull   = "full"   // the whole form (draft, or confirmed with no documents)
	EditHeader = "header" // header-light fields only: documents hang off the order
	EditNone   = "none"   // completed / cancelled: read-only
)

// reasonErrors validates an optional correction reason.
func reasonErrors(reason string, required bool) []apperror.FieldError {
	reason = strings.TrimSpace(reason)
	if required && reason == "" {
		return []apperror.FieldError{{Field: "reason", Message: "wajib diisi"}}
	}
	if utf8.RuneCountInString(reason) > maxReasonLen {
		return []apperror.FieldError{{Field: "reason", Message: "maksimal 255 karakter"}}
	}
	return nil
}

// day cuts a stored date/datetime to its YYYY-MM-DD part for the trail (a due
// date reads back with a time, the input carries none).
func day(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 10 {
		return s[:10]
	}
	return s
}

// lineLabel names a line in the trail: code/variant/uom (unique per order,
// priceLines refuses duplicates; legacy duplicates get a suffix).
func lineLabel(it *contracts.Item) string {
	label := it.ProductCode
	if it.VariantCode != "" && it.VariantCode != it.ProductCode {
		label += "/" + it.VariantCode
	}
	return label + " " + it.UOM
}

// orderSnapshot is the audit picture of an order: the header fields a user can
// correct, the economic totals and a per-line summary.
func orderSnapshot(o *contracts.SalesOrder) map[string]any {
	lines := map[string]any{}
	for _, it := range o.Items {
		key := lineLabel(it)
		for n := 2; lines[key] != nil; n++ {
			key = lineLabel(it) + " #" + strconv.Itoa(n)
		}
		lines[key] = map[string]any{
			"qty": it.Qty, "unitPrice": it.UnitPrice, "discountPct": it.DiscountPct,
			"discountNominal": it.DiscountNominal, "taxable": it.Taxable, "lineTotal": it.LineTotal,
		}
	}
	shipTo := strings.Join(nonEmpty(o.ShipToLabel, o.ShipToRecipient, o.ShipToPhone, o.ShipToAddress), " · ")
	return map[string]any{
		"status": o.Status, "partyId": o.PartyID, "channel": o.Channel,
		"orderDate": day(o.OrderDate), "dueDate": day(o.DueDate), "paymentTerms": o.PaymentTerms,
		"taxType": o.TaxType, "taxRate": o.TaxRate, "notes": o.Notes, "shipTo": shipTo,
		"taxInvoiceNumber": o.TaxInvoiceNumber, "taxInvoiceDate": day(o.TaxInvoiceDate),
		"assignedUserId": o.AssignedUserID,
		"subtotal": o.Subtotal, "discountTotal": o.DiscountTotal, "taxTotal": o.TaxTotal,
		"grandTotal": o.GrandTotal, "lines": lines,
	}
}

func nonEmpty(parts ...string) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, strings.TrimSpace(p))
		}
	}
	return out
}

// changedOnly keeps the keys whose value differs between two snapshots (one
// level deep into "lines"), so a trail row shows what the correction changed
// and nothing else. Both results are nil when nothing changed.
func changedOnly(before, after map[string]any) (map[string]any, map[string]any) {
	b, a := map[string]any{}, map[string]any{}
	keys := map[string]bool{}
	for k := range before {
		keys[k] = true
	}
	for k := range after {
		keys[k] = true
	}
	for k := range keys {
		bv, bok := before[k]
		av, aok := after[k]
		bm, bIsMap := bv.(map[string]any)
		am, aIsMap := av.(map[string]any)
		if bIsMap && aIsMap {
			if cb, ca := changedOnly(bm, am); cb != nil || ca != nil {
				b[k], a[k] = cb, ca
			}
			continue
		}
		if fmt.Sprint(bv) == fmt.Sprint(av) && bok == aok {
			continue
		}
		if bok {
			b[k] = bv
		}
		if aok {
			a[k] = av
		}
	}
	if len(b) == 0 && len(a) == 0 {
		return nil, nil
	}
	return b, a
}

// withDiff completes an order audit entry with only the changed fields plus
// the correction's metadata (mode, reason).
func withDiff(e auditcontracts.Entry, before, after map[string]any, meta map[string]any) auditcontracts.Entry {
	b, a := changedOnly(before, after)
	if b != nil {
		e.Before = b
	}
	if a != nil {
		e.After = a
	}
	if len(meta) > 0 {
		e.Metadata = meta
	}
	return e
}

// correctionMeta is the metadata of an update trail row.
func correctionMeta(mode, reason string) map[string]any {
	meta := map[string]any{"mode": mode}
	if r := strings.TrimSpace(reason); r != "" {
		meta["reason"] = r
	}
	return meta
}

// CorrectionState is what the detail page may offer for an order.
type CorrectionState struct {
	// EditMode: full | header | none (see the Edit* constants).
	EditMode string `json:"editMode"`
	// EditLockReason names the documents that restrict a confirmed order to
	// header edits (the server's own guard message).
	EditLockReason string `json:"editLockReason"`
	// CanCloseShort: a confirmed order with at least one confirmed shipment
	// and no draft shipment may be short-closed.
	CanCloseShort bool `json:"canCloseShort"`
}

// Correction reports the corrections open to an order (detail read: three
// dependent counts + one shipment list, confirmed orders only).
func (s *Service) Correction(ctx context.Context, o *contracts.SalesOrder) (CorrectionState, error) {
	switch o.Status {
	case contracts.StatusDraft:
		return CorrectionState{EditMode: EditFull}, nil
	case contracts.StatusConfirmed:
	default:
		return CorrectionState{EditMode: EditNone}, nil
	}
	st := CorrectionState{EditMode: EditFull}
	if err := s.assertNoDependents(ctx, o.ID); err != nil {
		var ae *apperror.AppError
		if !errors.As(err, &ae) || ae.Code != apperror.CodeConflict {
			return st, err
		}
		st.EditMode, st.EditLockReason = EditHeader, ae.Message
	}
	confirmed, drafts, err := s.orderShipments(ctx, o.BranchID, o.ID)
	if err != nil {
		return st, err
	}
	st.CanCloseShort = confirmed > 0 && drafts == 0
	return st, nil
}

// orderShipments counts the order's confirmed and draft order-kind shipments
// (replacement shipments of exchanges belong to the return, not the order).
func (s *Service) orderShipments(ctx context.Context, branchID, soID int64) (confirmed, drafts int, err error) {
	if s.dep.deliveries == nil {
		return 0, 0, apperror.Internal(errDependentsNotWired)
	}
	notes, err := s.dep.deliveries.ListBySO(ctx, branchID, soID)
	if err != nil {
		return 0, 0, apperror.Internal(err)
	}
	for _, n := range notes {
		if n.DocumentKind != "" && n.DocumentKind != deliverycontracts.DocumentKindOrder {
			continue
		}
		switch n.Status {
		case deliverycontracts.StatusConfirmed:
			confirmed++
		case deliverycontracts.StatusDraft:
			drafts++
		}
	}
	return confirmed, drafts, nil
}

// CancelOrderWithReason voids a draft or confirmed SO — only while nothing
// hangs off it — and stores why (cancel_reason, cancelled_at/by) in the same
// CAS statement. Serialized per order; the status is re-read inside the lock,
// the dependents are consulted and the move is a CAS. A repeat on a cancelled
// order is a successful no-op (the first reason stays).
func (s *Service) CancelOrderWithReason(ctx context.Context, actorID, branchID, id int64, reason string) (*contracts.SalesOrder, error) {
	reason = strings.TrimSpace(reason)
	if fields := reasonErrors(reason, false); len(fields) > 0 {
		return nil, apperror.Validation("", fields)
	}
	if _, err := s.mustOwnBranch(ctx, id, branchID); err != nil {
		return nil, err
	}
	var before *contracts.SalesOrder
	var replayed bool
	err := doclock.Lock("so:"+strconv.FormatInt(id, 10), func() error {
		return s.withPaymentLock(ctx, branchID, id, func() error {
			existing, err := s.mustOwnBranch(ctx, id, branchID)
			if err != nil {
				return err
			}
			before = existing
			switch existing.Status {
			case contracts.StatusCancelled:
				replayed = true
				return nil
			case contracts.StatusCompleted:
				return apperror.Conflict("Order sudah final")
			}
			if err := s.assertNoDependents(ctx, id); err != nil {
				return err
			}
			won, err := s.repo.CancelWithReason(ctx, id, reason, actorID)
			if err != nil {
				return apperror.Internal(err)
			}
			if !won {
				cur, err := s.repo.GetOrder(ctx, id)
				if err != nil {
					return apperror.Internal(err)
				}
				if cur == nil || cur.Status != contracts.StatusCancelled {
					return apperror.Conflict("Status order berubah, muat ulang")
				}
				replayed = true
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	if !replayed {
		after := map[string]any{"status": contracts.StatusCancelled}
		if reason != "" {
			after["cancelReason"] = reason
		}
		_ = s.audit.Log(ctx, auditcontracts.Entry{
			Action: "sales_orders.cancel", Entity: "sales_order", EntityID: id, BranchID: branchID, ActorID: actorID, Note: before.Number,
			Before: map[string]any{"status": before.Status}, After: after,
		})
	}
	return s.GetByID(ctx, id)
}

// ShortCloseLine is one order line in a short-close plan.
type ShortCloseLine struct {
	ItemID       int64   `json:"itemId"`
	ProductCode  string  `json:"productCode"`
	ProductName  string  `json:"productName"`
	VariantName  string  `json:"variantName"`
	UOM          string  `json:"uom"`
	OrderedQty   float64 `json:"orderedQty"`
	DeliveredQty float64 `json:"deliveredQty"`
	OldLineTotal int64   `json:"oldLineTotal"`
	NewLineTotal int64   `json:"newLineTotal"`
	// Removed: nothing of this line was delivered, it leaves the order.
	Removed bool `json:"removed"`
}

// ShortClosePlan is what "tutup sisa order" would do to an order: the lines cut
// to the delivered quantity, the new totals and the money picture after it.
type ShortClosePlan struct {
	OrderID          int64            `json:"orderId"`
	Number           string           `json:"number"`
	Lines            []ShortCloseLine `json:"lines"`
	OldGrandTotal    int64            `json:"oldGrandTotal"`
	NewSubtotal      int64            `json:"newSubtotal"`
	NewDiscountTotal int64            `json:"newDiscountTotal"`
	NewTaxTotal      int64            `json:"newTaxTotal"`
	NewGrandTotal    int64            `json:"newGrandTotal"`
	// PaidAmount is the cash kept on the order (net of return offsets/refunds);
	// OutstandingAfter / RefundDueAfter are the payment summary of the
	// reduced order (refundDue > 0 = the customer paid more than it now owes).
	PaidAmount       int64 `json:"paidAmount"`
	OutstandingAfter int64 `json:"outstandingAfter"`
	RefundDueAfter   int64 `json:"refundDueAfter"`

	source  *contracts.SalesOrder // the order as it stands, with lines
	order   *contracts.SalesOrder // the reduced order: kept lines with ids, new header money
	removed []int64
}

// ShortClosePreview computes the plan without writing (the same builder feeds
// ShortClose, so the preview cannot drift from what the close writes).
func (s *Service) ShortClosePreview(ctx context.Context, branchID, id int64) (*ShortClosePlan, error) {
	o, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	return s.shortClosePlan(ctx, o)
}

// shortClosePlan validates the order's state and cuts every line to what the
// order's confirmed shipments delivered. Each cut line is re-priced with its
// OWN stored unit price, discount % and tax flag through the money kernel,
// exactly as order pricing does; the nominal discount is pro-rated to the
// delivered share (money.Prorate), the same way returns and the per-shipment
// journal split it, so the order now bills what the books recognised.
func (s *Service) shortClosePlan(ctx context.Context, o *contracts.SalesOrder) (*ShortClosePlan, error) {
	if o.Status != contracts.StatusConfirmed {
		return nil, apperror.Conflict("Hanya order terkonfirmasi yang bisa ditutup sisanya")
	}
	confirmed, drafts, err := s.orderShipments(ctx, o.BranchID, o.ID)
	if err != nil {
		return nil, err
	}
	if drafts > 0 {
		return nil, apperror.Conflict("Masih ada surat jalan draf; konfirmasi atau batalkan surat jalannya dulu")
	}
	if confirmed == 0 {
		return nil, apperror.Conflict("Belum ada surat jalan terkonfirmasi; batalkan order bila tidak jadi dikirim")
	}
	o, err = s.enrich(ctx, o)
	if err != nil {
		return nil, err
	}
	// Line-UOM qty per product/variant/uom over every confirmed order-kind note.
	delivered, err := s.dep.deliveries.ConfirmedQtyBefore(ctx, o.ID, math.MaxInt64)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	plan := &ShortClosePlan{OrderID: o.ID, Number: o.Number, OldGrandTotal: o.GrandTotal, source: o}
	reduced := *o
	reduced.Items = nil
	for _, it := range o.Items {
		key := strconv.FormatInt(it.ProductID, 10) + "/" + strconv.FormatInt(it.VariantID, 10) + "/" + strings.TrimSpace(it.UOM)
		// Several lines of one position (legacy data) take the delivered
		// quantity in line order.
		got := math.Min(delivered[key], it.Qty)
		if got < 1e-9 {
			got = 0
		}
		delivered[key] -= got
		line := ShortCloseLine{
			ItemID: it.ID, ProductCode: it.ProductCode, ProductName: it.ProductName, VariantName: it.VariantName,
			UOM: it.UOM, OrderedQty: it.Qty, DeliveredQty: got, OldLineTotal: it.LineTotal,
		}
		if got == 0 {
			line.Removed = true
			plan.removed = append(plan.removed, it.ID)
			plan.Lines = append(plan.Lines, line)
			continue
		}
		cut := *it
		if got+1e-9 < it.Qty {
			taxType, taxRate := o.TaxType, o.TaxRate
			if !it.Taxable {
				taxType, taxRate = "none", 0
			}
			cut.DiscountNominal = money.Prorate(it.DiscountNominal, got, it.Qty)
			_, base, tax, total := money.LineAmounts(got, it.UnitPrice, it.DiscountPct, cut.DiscountNominal, taxType, taxRate).Split()
			cut.Qty, cut.QtyBase = got, got*it.UOMFactor
			cut.LineTotal, cut.TaxBase, cut.TaxAmount = total, base, tax
		}
		line.NewLineTotal = cut.LineTotal
		plan.Lines = append(plan.Lines, line)
		reduced.Items = append(reduced.Items, &cut)
	}
	if len(reduced.Items) == 0 {
		return nil, apperror.Conflict("Tidak ada barang terkirim yang bisa ditagihkan; batalkan surat jalannya atau order ini")
	}
	reduced.Subtotal, reduced.DiscountTotal, reduced.TaxTotal, reduced.GrandTotal = storedTotals(o.TaxType, reduced.Items)
	plan.NewSubtotal, plan.NewDiscountTotal = reduced.Subtotal, reduced.DiscountTotal
	plan.NewTaxTotal, plan.NewGrandTotal = reduced.TaxTotal, reduced.GrandTotal
	plan.order = &reduced

	view := reduced
	view.Status = contracts.StatusCompleted
	sums, err := s.PaymentSummaries(ctx, []*contracts.SalesOrder{&view})
	if err != nil {
		return nil, err
	}
	sum := sums[o.ID]
	plan.PaidAmount, plan.OutstandingAfter, plan.RefundDueAfter = sum.PaidAmount, sum.OutstandingAmount, sum.RefundDue
	return plan, nil
}

// storedTotals folds stored line amounts into header money the way totals()
// folds freshly priced lines: subtotal = Σ net (incl. tax for "include"),
// discount = Σ (gross − net).
func storedTotals(orderTaxType string, items []*contracts.Item) (subtotal, discount, tax, grand int64) {
	for _, it := range items {
		net := it.LineTotal
		if it.Taxable && orderTaxType == "exclude" {
			net -= it.TaxAmount
		}
		gross := money.Round(float64(it.UnitPrice) * it.Qty)
		subtotal += net
		discount += gross - net
		tax += it.TaxAmount
		grand += it.LineTotal
	}
	return subtotal, discount, tax, grand
}

// ShortClose ("tutup sisa order") ends a confirmed, partially delivered order:
// every line is cut to what confirmed shipments delivered (lines with nothing
// delivered leave the order), totals are re-priced from the lines' own stored
// prices, and the order completes with who/when/why stamped. No journal is
// written: revenue and receivable were booked per shipment already, so the
// order now simply stops billing the undelivered rest. A payment above the new
// total is not refused; the payment summary reports it as refundDue.
//
// A repeat on an order this already closed replays successfully.
func (s *Service) ShortClose(ctx context.Context, actorID, branchID, id int64, reason string) (*contracts.SalesOrder, error) {
	reason = strings.TrimSpace(reason)
	if fields := reasonErrors(reason, true); len(fields) > 0 {
		return nil, apperror.Validation("", fields)
	}
	if _, err := s.mustOwnBranch(ctx, id, branchID); err != nil {
		return nil, err
	}
	err := doclock.Lock("so:"+strconv.FormatInt(id, 10), func() error {
		return s.withPaymentLock(ctx, branchID, id, func() error {
			existing, err := s.mustOwnBranch(ctx, id, branchID)
			if err != nil {
				return err
			}
			if existing.Status == contracts.StatusCompleted {
				return s.shortCloseReplay(ctx, existing)
			}
			plan, err := s.shortClosePlan(ctx, existing)
			if err != nil {
				return err
			}
			full := plan.source
			before := orderSnapshot(full)
			afterOrder := *plan.order
			afterOrder.Status = contracts.StatusCompleted
			after := orderSnapshot(&afterOrder)
			meta := map[string]any{"reason": reason, "refundDue": plan.RefundDueAfter}
			hook := func(ctx context.Context, tx infrastructure.ExecTx) error {
				return s.audit.LogTx(ctx, tx, withDiff(auditcontracts.Entry{
					Action: "sales_orders.close_short", Entity: "sales_order", EntityID: full.ID, BranchID: branchID, ActorID: actorID, Note: full.Number,
				}, before, after, meta))
			}
			won, err := s.repo.ShortClose(ctx, plan.order, plan.removed, reason, actorID, hook)
			if err != nil {
				return apperror.Internal(err)
			}
			if !won {
				cur, err := s.repo.GetOrder(ctx, id)
				if err != nil {
					return apperror.Internal(err)
				}
				if cur == nil {
					return apperror.NotFound("Sales order")
				}
				if cur.Status == contracts.StatusCompleted {
					return s.shortCloseReplay(ctx, cur)
				}
				return apperror.Conflict("Status order berubah, muat ulang")
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// shortCloseReplay accepts a completed order only when it was short-closed
// (a repeated request); an order completed by its shipments is a conflict.
func (s *Service) shortCloseReplay(ctx context.Context, o *contracts.SalesOrder) error {
	if err := s.repo.FillCorrections(ctx, o); err != nil {
		return apperror.Internal(err)
	}
	if o.ShortClosedAt != "" {
		return nil
	}
	return apperror.Conflict("Order sudah selesai (terkirim penuh)")
}
