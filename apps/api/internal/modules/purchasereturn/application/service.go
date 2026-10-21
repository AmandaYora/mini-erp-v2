package application

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	goodsreceiptcontracts "mini-erp/internal/modules/goodsreceipt/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/modules/purchasereturn/contracts"
	"mini-erp/internal/modules/purchasereturn/infrastructure"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/doclock"
	"mini-erp/internal/shared/money"
	"mini-erp/internal/shared/timeutil"
)

// Service implements contracts.PurchaseReturnClient plus return administration.
// Returns ship stock back out on confirm (MoveOut). The source PO keeps its
// status — history stands; money moves only through payments.
type Service struct {
	repo         *infrastructure.Repository
	purchasing   purchasingcontracts.PurchaseOrderClient
	goodsreceipt goodsreceiptcontracts.GoodsReceiptClient
	products     productcontracts.ProductClient
	stock        stockcontracts.StockClient
	branches     branchcontracts.BranchClient
	// payments is set by the composition root after the payment module is
	// built (setter breaks the payment↔returns construction cycle).
	reverser financecontracts.SourceReverser
	payments paymentcontracts.PaymentClient
	audit    auditcontracts.AuditClient
}

// NewService wires purchasereturn use cases. All cross-module links are contracts.
func NewService(
	repo *infrastructure.Repository,
	purchasing purchasingcontracts.PurchaseOrderClient,
	goodsreceipt goodsreceiptcontracts.GoodsReceiptClient,
	products productcontracts.ProductClient,
	stock stockcontracts.StockClient,
	branches branchcontracts.BranchClient,
	audit auditcontracts.AuditClient,
) *Service {
	return &Service{repo: repo, purchasing: purchasing, goodsreceipt: goodsreceipt,
		products: products, stock: stock, branches: branches, audit: audit}
}

// SetReverser wires finance's source reverser (composition-root privilege).
func (s *Service) SetReverser(r financecontracts.SourceReverser) { s.reverser = r }

// SetPayments injects the payment client (composition-root privilege).
func (s *Service) SetPayments(p paymentcontracts.PaymentClient) {
	s.payments = p
}

// errPaymentsNotWired is a programmer error: main.go must call SetPayments.
var errPaymentsNotWired = errors.New("payments client not wired")

// GetByID resolves a return with lines and order number, or nil when missing.
func (s *Service) GetByID(ctx context.Context, id int64) (*contracts.PurchaseReturn, error) {
	ret, err := s.repo.GetReturn(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if ret == nil {
		return nil, nil
	}
	return s.enrich(ctx, ret)
}

// ReleaseAutoOffset implements contracts.ReturnsClient for the payment module.
func (s *Service) ReleaseAutoOffset(ctx context.Context, returnID int64) error {
	if err := s.repo.SetReduction(ctx, returnID, 0); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

// GetReturn resolves the payment-facing projection, or nil when missing.
func (s *Service) GetReturn(ctx context.Context, id int64) (*contracts.Return, error) {
	ret, err := s.repo.GetReturn(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if ret == nil {
		return nil, nil
	}
	credited, err := s.repo.CreditSettled(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &contracts.Return{
		ID: ret.ID, Number: ret.Number, Branch: ret.BranchID,
		OrderID: ret.PurchaseOrderID, Total: ret.Total, Status: ret.Status,
		CreditSettled: credited,
	}, nil
}

// ConfirmedByOrders lists the confirmed returns of the given orders.
func (s *Service) ConfirmedByOrders(ctx context.Context, branchID int64, orderIDs []int64) ([]*contracts.Return, error) {
	out, err := s.repo.ConfirmedByOrders(ctx, branchID, orderIDs)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}

// CreditSettledByReturns implements the payment-facing contract (see there).
func (s *Service) CreditSettledByReturns(ctx context.Context, ids []int64) (map[int64]int64, error) {
	out, err := s.repo.CreditSettledByReturns(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}

func (s *Service) enrich(ctx context.Context, ret *contracts.PurchaseReturn) (*contracts.PurchaseReturn, error) {
	items, err := s.repo.ItemsByReturn(ctx, ret.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if items == nil {
		items = []*contracts.Item{}
	}
	ret.Items = items
	settlements, err := s.repo.Settlements(ctx, ret.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if settlements == nil {
		settlements = []*contracts.Settlement{}
	}
	ret.Settlements = settlements
	po, err := s.purchasing.GetByID(ctx, ret.PurchaseOrderID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if po != nil {
		ret.OrderNumber = po.Number
	}
	ret.AllowedSettlementTypes = []string{}
	if ret.Status == contracts.StatusConfirmed {
		pos := positionOf(ret.Total, ret.PayableReduction, ret.Settlements)
		ret.ExternalRemaining, ret.AllowedSettlementTypes = pos.ExternalRemaining(), pos.Allowed()
	}
	return ret, nil
}

// mustOwnBranch loads the return and denies cross-branch access as missing.
func (s *Service) mustOwnBranch(ctx context.Context, id, branchID int64) (*contracts.PurchaseReturn, error) {
	ret, err := s.repo.GetReturn(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if ret == nil || ret.BranchID != branchID {
		return nil, apperror.NotFound("Retur pembelian")
	}
	return ret, nil
}

// LineInput is one return line.
type LineInput struct {
	ProductID  int64
	VariantID  int64
	LocationID int64
	UOM        string
	Qty        float64
}

// Create drafts a return against a confirmed/completed PO. Quantities cap at
// received-minus-returned per (product, variant) aggregate — goods never
// received cannot go back to the supplier.
// CreateOption sets optional return header data.
type CreateOption func(*createOptions)

type createOptions struct{ reason string }

// WithReason records why the goods go back to the supplier.
func WithReason(reason string) CreateOption {
	return func(o *createOptions) { o.reason = strings.TrimSpace(reason) }
}

func (s *Service) Create(ctx context.Context, actorID, branchID, poID int64, returnDate, notes string, lines []LineInput, opts ...CreateOption) (*contracts.PurchaseReturn, error) {
	var o createOptions
	for _, opt := range opts {
		opt(&o)
	}
	var out *contracts.PurchaseReturn
	err := doclock.Lock("po:"+strconv.FormatInt(poID, 10), func() error {
		res, err := s.createLocked(ctx, actorID, branchID, poID, returnDate, notes, lines, o)
		if err != nil {
			return err
		}
		out = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "purchase_returns.create", Entity: "purchase_return", EntityID: out.ID, BranchID: branchID, ActorID: actorID, Note: out.Number})
	return out, nil
}

// poAggregate indexes PO lines per (product, variant) for matching.
type poAggregate struct {
	uoms map[string]float64 // uom -> factor
	// econ holds, per UOM, the source lines' STORED economics (gross, net,
	// tax base and tax per line — each line's own discount, taxable flag and
	// rate). A return line takes its quantity share of them, so it gives back
	// exactly what the order billed: an exempt line returns no tax, duplicate
	// lines average by quantity, never "last line wins".
	econ    map[string]*money.LineEconomics
	qtyBase float64
	tracked bool // false: service line, no stock
}

func aggregatePO(po *purchasingcontracts.PurchaseOrder) map[string]*poAggregate {
	poAgg := map[string]*poAggregate{}
	for _, l := range po.Items {
		key := lineKey(l.ProductID, l.VariantID)
		agg, ok := poAgg[key]
		if !ok {
			agg = &poAggregate{uoms: map[string]float64{}, econ: map[string]*money.LineEconomics{}}
			poAgg[key] = agg
		}
		agg.uoms[l.UOM] = l.UOMFactor
		e := agg.econ[l.UOM]
		if e == nil {
			e = &money.LineEconomics{}
			agg.econ[l.UOM] = e
		}
		e.Add(l.Qty, l.UnitPrice, l.DiscountPct, l.DiscountNominal, l.TaxBase, l.TaxAmount)
		agg.qtyBase += l.QtyBase
		agg.tracked = agg.tracked || l.StockTracked
	}
	return poAgg
}

// computedReturn is the validated + priced result shared by Create and
// Preview so a preview can never drift from what Create would persist.
type computedReturn struct {
	items         []*contracts.Item
	subtotal      int64
	discountTotal int64
	taxTotal      int64
	total         int64
}

// computeReturn validates return lines against the PO (caps, master data)
// and prices them with source-line economics. Read-only: no writes, no
// number allocation — safe for Preview.
func (s *Service) computeReturn(ctx context.Context, branchID, poID int64, po *purchasingcontracts.PurchaseOrder, lines []LineInput) (*computedReturn, error) {
	poAgg := aggregatePO(po)
	// Quota = received - (draft + confirmed returns) - lines already accepted
	// in this request (P6).
	committed, err := s.repo.CommittedQtyByPO(ctx, poID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	receivedCache := map[string]float64{}
	used := map[string]float64{}
	items := make([]*contracts.Item, 0, len(lines))
	var subtotal, discountTotal, taxTotal int64
	// One batch read for the whole document (F4-7), not a GetByID per line.
	productIDs := make([]int64, 0, len(lines))
	for _, l := range lines {
		productIDs = append(productIDs, l.ProductID)
	}
	loadedProducts, perr := s.products.GetByIDs(ctx, productIDs)
	if perr != nil {
		return nil, apperror.Internal(perr)
	}
	for i, line := range lines {
		tag := "baris " + strconv.Itoa(i+1) + ": "
		if line.Qty <= 0 {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "qty harus lebih dari 0"}})
		}
		agg, ok := poAgg[lineKey(line.ProductID, line.VariantID)]
		if !ok {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "produk tidak ada di PO"}})
		}
		uom := strings.TrimSpace(line.UOM)
		factor, ok := agg.uoms[uom]
		if !ok {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "satuan harus sama dengan PO"}})
		}
		key := lineKey(line.ProductID, line.VariantID)
		received, ok := receivedCache[key]
		if !ok {
			var rerr error
			if received, rerr = s.goodsreceipt.ReceivedQty(ctx, poID, line.ProductID, line.VariantID); rerr != nil {
				return nil, apperror.Internal(rerr)
			}
			receivedCache[key] = received
		}
		qtyBase := line.Qty * factor
		taken := committed[key] + used[key]
		if received-taken-qtyBase < -1e-9 {
			return nil, apperror.Conflict("Melebihi yang bisa diretur (" + tag + "sisa " + strconv.FormatFloat(received-taken, 'f', -1, 64) + ")")
		}
		used[key] += qtyBase
		if agg.tracked {
			if err := s.stock.CheckLocation(ctx, branchID, line.LocationID); err != nil {
				return nil, err
			}
		}
		p := loadedProducts[line.ProductID]
		if p == nil || p.Status != "active" {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "produk sudah diarsipkan"}})
		}
		known := false
		for _, v := range p.Variants {
			if v.ID == line.VariantID {
				known = true
				break
			}
		}
		if !known {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "varian sudah diarsipkan"}})
		}
		// D1: value follows the source line economics — its stored discount
		// and tax split (taxable flag, own rate) pro-rate in; duplicate PO
		// lines average by quantity. Unmatched lines are rejected above; never
		// fall back to live catalog prices.
		econ := agg.econ[uom]
		gross, net, base, tax := econ.Part(line.Qty)
		unitPrice, pct := econ.UnitPrice(), econ.DiscountPct()
		nominalPart := max(0, gross-net-money.Round(float64(gross)*pct/100))
		lineTotal := base + tax
		subtotal += base
		discountTotal += gross - net
		taxTotal += tax
		items = append(items, &contracts.Item{
			ProductID: line.ProductID, VariantID: line.VariantID, LocationID: line.LocationID,
			UOM: uom, UOMFactor: factor, Qty: line.Qty, QtyBase: qtyBase,
			UnitPrice: unitPrice, DiscountPct: pct, DiscountNominal: nominalPart,
			TaxBase: base, TaxAmount: tax, LineTotal: lineTotal,
		})
	}
	return &computedReturn{items: items, subtotal: subtotal,
		discountTotal: discountTotal, taxTotal: taxTotal, total: subtotal + taxTotal}, nil
}

func (s *Service) createLocked(ctx context.Context, actorID, branchID, poID int64, returnDate, notes string, lines []LineInput, o createOptions) (*contracts.PurchaseReturn, error) {
	if len(o.reason) > 255 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "reason", Message: "maksimal 255 karakter"}})
	}
	po, err := s.purchasing.GetByID(ctx, poID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if po == nil || po.BranchID != branchID {
		return nil, apperror.NotFound("Purchase order")
	}
	if po.Status != purchasingcontracts.StatusConfirmed && po.Status != purchasingcontracts.StatusCompleted {
		return nil, apperror.Conflict("Retur hanya untuk order terkonfirmasi")
	}
	if strings.TrimSpace(returnDate) != "" {
		if _, err := timeutil.ParseDateInput(strings.TrimSpace(returnDate)); err != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "returnDate", Message: "format tanggal harus YYYY-MM-DD"}})
		}
		if timeutil.AfterTodayWIB(returnDate) {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "returnDate", Message: "tidak boleh di masa depan"}})
		}
	} else {
		returnDate = timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	}
	// A return cannot be dated in the future (legacy rule).
	if strings.TrimSpace(returnDate) > timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02") {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "returnDate", Message: "tidak boleh di masa depan"}})
	}
	// RT-04: nor before the goods came in (legacy: not earlier than the receive date).
	hints, herr := s.goodsreceipt.PostedReceiptHints(ctx, poID)
	if herr != nil {
		return nil, apperror.Internal(herr)
	}
	if err := checkReturnDate(strings.TrimSpace(returnDate), hints.FirstReceivedAt); err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: "wajib diisi"}})
	}
	computed, err := s.computeReturn(ctx, branchID, poID, po, lines)
	if err != nil {
		return nil, err
	}
	items, subtotal, discountTotal, taxTotal := computed.items, computed.subtotal, computed.discountTotal, computed.taxTotal
	total := subtotal + taxTotal
	number, err := s.branches.NextDocumentNumber(ctx, branchID, branchcontracts.DocPurchaseReturn)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	id, err := s.repo.CreateReturn(ctx, &contracts.PurchaseReturn{
		Number: number, BranchID: branchID, PurchaseOrderID: poID,
		ReturnDate: strings.TrimSpace(returnDate), Reason: o.reason,
		Subtotal: subtotal, DiscountTotal: discountTotal, TaxTotal: taxTotal,
		TaxType: po.TaxType, TaxRate: po.TaxRate,
		Total: total, Notes: notes, Items: items,
	}, actorID)
	if err != nil {
		return nil, dberr.Map(err)
	}
	return s.GetByID(ctx, id)
}

// Confirm ships the goods back out. Draft-only, replay-safe (P4): re-read
// inside the lock, quota re-checked, stock.Post idempotent per document, status
// flip is a CAS. Repeating a half-finished request completes it.
func (s *Service) Confirm(ctx context.Context, actorID, branchID, id int64) (*contracts.PurchaseReturn, error) {
	ret, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	var out *contracts.PurchaseReturn
	err = doclock.Lock("po:"+strconv.FormatInt(ret.PurchaseOrderID, 10), func() error {
		res, _, err := s.confirmLocked(ctx, actorID, branchID, id)
		if err != nil {
			return err
		}
		out = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) confirmLocked(ctx context.Context, actorID, branchID, id int64) (*contracts.PurchaseReturn, bool, error) {
	ret, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, false, err
	}
	switch ret.Status {
	case contracts.StatusConfirmed:
		// Already final: only the post-final offset may be missing.
		if err := s.autoOffset(ctx, actorID, branchID, ret.ID); err != nil {
			return nil, false, err
		}
		out, err := s.GetByID(ctx, ret.ID)
		return out, true, err
	case contracts.StatusDraft:
	default:
		return nil, false, apperror.Conflict("Hanya draft yang bisa dikonfirmasi")
	}
	full, err := s.enrich(ctx, ret)
	if err != nil {
		return nil, false, err
	}
	if err := s.checkCommitted(ctx, ret.PurchaseOrderID, full); err != nil {
		return nil, false, err
	}
	po, err := s.purchasing.GetByID(ctx, ret.PurchaseOrderID)
	if err != nil || po == nil {
		return nil, false, apperror.Internal(errors.New("source order unavailable"))
	}
	untracked := map[string]bool{}
	for _, l := range po.Items {
		if !l.StockTracked {
			untracked[lineKey(l.ProductID, l.VariantID)] = true
		}
	}
	lines := make([]stockcontracts.PostLine, 0, len(full.Items))
	for _, it := range full.Items {
		if untracked[lineKey(it.ProductID, it.VariantID)] {
			continue // service lines return value, never stock
		}
		lines = append(lines, stockcontracts.PostLine{ProductID: it.ProductID, VariantID: it.VariantID, LocationID: it.LocationID, Qty: it.QtyBase})
	}
	if len(lines) > 0 {
		if _, err := s.stock.Post(ctx, stockcontracts.PostRequest{
			BranchID: branchID, Direction: "out", RefType: stockcontracts.RefPurchaseReturn, RefID: full.ID,
			ActorID: actorID, Lines: lines,
		}); err != nil {
			return nil, false, err
		}
	}
	// The trail rides the confirm CAS (ADR-0011): audited only by the winner.
	won, err := s.repo.TransitionAudited(ctx, full.ID, []string{contracts.StatusDraft}, contracts.StatusConfirmed, actorID,
		func(ctx context.Context, tx *sql.Tx) error {
			return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
				Action: "purchase_returns.confirm", Entity: "purchase_return", EntityID: full.ID, BranchID: branchID, ActorID: actorID, Note: full.Number,
				Before: map[string]any{"status": contracts.StatusDraft}, After: map[string]any{"status": contracts.StatusConfirmed, "total": full.Total},
			})
		})
	if err != nil {
		return nil, false, apperror.Internal(err)
	}
	replayed := false
	if !won {
		cur, err := s.repo.GetReturn(ctx, full.ID)
		if err != nil {
			return nil, false, apperror.Internal(err)
		}
		if cur == nil || cur.Status != contracts.StatusConfirmed {
			return nil, false, apperror.Conflict("Status retur berubah, muat ulang")
		}
		replayed = true
	}
	if err := s.autoOffset(ctx, actorID, branchID, full.ID); err != nil {
		return nil, false, err
	}
	out, err := s.GetByID(ctx, full.ID)
	return out, replayed, err
}

// autoOffset is the idempotent post-final step of Confirm: the return is
// offset against the supplier's open bill (up to the smaller of the two), so
// what we owe falls without cash moving. Keyed offset:pr:<id>.
func (s *Service) autoOffset(ctx context.Context, actorID, branchID, id int64) error {
	if s.payments == nil {
		return nil
	}
	ret, err := s.repo.GetReturn(ctx, id)
	if err != nil || ret == nil {
		return apperror.Internal(errors.New("return unavailable for offset"))
	}
	if ret.Status != contracts.StatusConfirmed || ret.Total <= 0 {
		return nil
	}
	po, err := s.purchasing.GetByID(ctx, ret.PurchaseOrderID)
	if err != nil || po == nil {
		return apperror.Internal(errors.New("source order unavailable"))
	}
	if po.Status == purchasingcontracts.StatusCancelled {
		return nil
	}
	amount := ret.PayableReduction
	if amount == 0 {
		paid, err := s.payments.PaidForOrders(ctx, paymentcontracts.OrderPurchase, []int64{po.ID})
		if err != nil {
			return apperror.Internal(err)
		}
		if amount = offsetAmount(po.GrandTotal, paid[po.ID], ret.Total); amount <= 0 {
			return nil
		}
	}
	pay, _, err := s.payments.OffsetReturn(ctx, paymentcontracts.OffsetInput{
		ActorID: actorID, BranchID: branchID, PartyID: po.PartyID,
		ReturnType: paymentcontracts.OrderPurchaseReturn, ReturnID: ret.ID,
		OrderType: paymentcontracts.OrderPurchase, OrderID: po.ID, Amount: amount,
		ClientRef: "offset:pr:" + strconv.FormatInt(ret.ID, 10), Notes: "Offset otomatis retur " + ret.Number,
	})
	if err != nil {
		return err
	}
	if pay.Status != "active" {
		// A repeated confirm replays the pair by its client ref; when that
		// pair was cancelled since, the offset was undone on purpose
		// (ReleaseAutoOffset) and must not be mirrored again.
		return nil
	}
	if ret.PayableReduction != pay.Amount {
		if err := s.repo.SetReduction(ctx, ret.ID, pay.Amount); err != nil {
			return apperror.Internal(err)
		}
	}
	return nil
}

// checkCommitted re-verifies, at confirm time, that Σ(draft + confirmed
// returns including this one) still fits within what was received.
func (s *Service) checkCommitted(ctx context.Context, poID int64, ret *contracts.PurchaseReturn) error {
	committed, err := s.repo.CommittedQtyByPO(ctx, poID)
	if err != nil {
		return apperror.Internal(err)
	}
	for _, it := range ret.Items {
		key := lineKey(it.ProductID, it.VariantID)
		received, err := s.goodsreceipt.ReceivedQty(ctx, poID, it.ProductID, it.VariantID)
		if err != nil {
			return apperror.Internal(err)
		}
		if committed[key] > received+1e-9 {
			return apperror.Conflict("Jumlah retur melebihi yang diterima, periksa retur lain pada order ini")
		}
	}
	return nil
}

// Cancel voids a draft outright, or reverses a confirmed return (stock back
// in). The status is re-read inside the lock so two simultaneous cancels
// cannot both move stock; the second one replays.
func (s *Service) Cancel(ctx context.Context, actorID, branchID, id int64) (*contracts.PurchaseReturn, error) {
	first, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	var out *contracts.PurchaseReturn
	var replayed, wasConfirmed bool
	err = doclock.Lock("po:"+strconv.FormatInt(first.PurchaseOrderID, 10), func() error {
		ret, err := s.mustOwnBranch(ctx, id, branchID)
		if err != nil {
			return err
		}
		if ret.Status == contracts.StatusCancelled {
			replayed = true
			out, err = s.GetByID(ctx, ret.ID)
			return err
		}
		switch ret.Status {
		case contracts.StatusDraft:
			// A draft whose confirm died after the stock posting is reversed too.
			if moved, err := s.stock.HasMovements(ctx, branchID, stockcontracts.RefPurchaseReturn, ret.ID); err != nil {
				return err
			} else if moved {
				if _, err := s.stock.Reverse(ctx, branchID, stockcontracts.RefPurchaseReturn, ret.ID, actorID); err != nil {
					return err
				}
			}
		case contracts.StatusConfirmed:
			wasConfirmed = true
			if s.payments == nil {
				return apperror.Internal(errPaymentsNotWired)
			}
			if used, err := s.payments.HasActiveAllocations(ctx, paymentcontracts.OrderPurchaseReturn, ret.ID); err != nil {
				return apperror.Internal(err)
			} else if used {
				return apperror.Conflict("Retur sudah ada refund, batalkan pembayaran dulu")
			}
			if _, err := s.stock.Reverse(ctx, branchID, stockcontracts.RefPurchaseReturn, ret.ID, actorID); err != nil {
				return err
			}
		default:
			return apperror.Conflict("Retur sudah dibatalkan")
		}
		won, err := s.repo.Transition(ctx, ret.ID, []string{contracts.StatusDraft, contracts.StatusConfirmed}, contracts.StatusCancelled, actorID)
		if err != nil {
			return apperror.Internal(err)
		}
		if !won {
			// Lost a race: fine when the winner cancelled it, otherwise stale.
			cur, err := s.repo.GetReturn(ctx, ret.ID)
			if err != nil {
				return apperror.Internal(err)
			}
			if cur == nil || cur.Status != contracts.StatusCancelled {
				return apperror.Conflict("Status retur berubah, muat ulang")
			}
			replayed = true
		}
		out, err = s.GetByID(ctx, ret.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	if !replayed && wasConfirmed && s.reverser != nil {
		// The journal of a confirmed return must not outlive the return. A
		// failure here never undoes the cancellation: it is audited and shows
		// up as an orphan in the readiness gate.
		if _, rerr := s.reverser.ReverseForSource(ctx, actorID, branchID, "purchase_return", id, "Retur dibatalkan"); rerr != nil {
			_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "finance.autoreverse_failed", Entity: "purchase_return", EntityID: id, BranchID: branchID, ActorID: actorID, Note: rerr.Error()})
		}
		// The supplier-balance reclassifications (K-04a) die with the return.
		// Each settlement is its own journal source; a failure is audited and
		// surfaces as an orphan, never undoes the cancellation.
		credits, cerr := s.repo.CreditSettlementIDs(ctx, id)
		if cerr != nil {
			_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "finance.autoreverse_failed", Entity: "purchase_return_settlement", EntityID: id, BranchID: branchID, ActorID: actorID, Note: cerr.Error()})
		}
		for _, sid := range credits {
			if _, rerr := s.reverser.ReverseForSource(ctx, actorID, branchID, "purchase_return_settlement", sid, "Retur dibatalkan"); rerr != nil {
				_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "finance.autoreverse_failed", Entity: "purchase_return_settlement", EntityID: sid, BranchID: branchID, ActorID: actorID, Note: rerr.Error()})
			}
		}
	}
	if !replayed {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "purchase_returns.cancel", Entity: "purchase_return", EntityID: out.ID, BranchID: branchID, ActorID: actorID, Note: out.Number})
	}
	return out, nil
}

// SettlementInput is one memo row recording how return value was settled.
type SettlementInput struct {
	Type            string
	Date            string
	Amount          int64
	PaymentMethod   string
	ReferenceNumber string
	Notes           string
}

// AddSettlement records how part of the return value was settled (memo-level:
// money itself still moves through payments). Total settlements can never
// exceed the return total.
func (s *Service) AddSettlement(ctx context.Context, actorID, branchID, id int64, in SettlementInput) (*contracts.PurchaseReturn, error) {
	// Serialized per return: two concurrent settlements would both pass the
	// remaining-room check and together settle more than the return is owed.
	var out *contracts.PurchaseReturn
	err := doclock.Lock("pr:"+strconv.FormatInt(id, 10), func() error {
		res, err := s.addSettlementLocked(ctx, actorID, branchID, id, in)
		out = res
		return err
	})
	return out, err
}

func (s *Service) addSettlementLocked(ctx context.Context, actorID, branchID, id int64, in SettlementInput) (*contracts.PurchaseReturn, error) {
	ret, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if ret.Status != contracts.StatusConfirmed {
		return nil, apperror.Conflict("Penyelesaian hanya untuk retur terkonfirmasi")
	}
	date := strings.TrimSpace(in.Date)
	if date != "" {
		if _, err := timeutil.ParseDateInput(date); err != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "date", Message: "format tanggal harus YYYY-MM-DD"}})
		}
		if timeutil.AfterTodayWIB(date) {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "date", Message: "tidak boleh di masa depan"}})
		}
	} else {
		date = timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	}
	if in.Amount <= 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "amount", Message: "harus lebih dari 0"}})
	}
	full, err := s.enrich(ctx, ret)
	if err != nil {
		return nil, err
	}
	// K-04c: legacy-style rules; the return value first reduces the open bill
	// (automatic), only the excess can be refunded or kept as supplier balance.
	pos := positionOf(full.Total, full.PayableReduction, full.Settlements)
	if err := pos.validate(in.Type, in.Amount); err != nil {
		return nil, err
	}
	// N-40: money already refunded / offset through payments is gone too; the
	// balance can only keep what is still unrefunded and not yet credited.
	if in.Type == contracts.SettleSupplierCredit && s.payments != nil {
		paid, perr := s.payments.PaidForOrders(ctx, paymentcontracts.OrderPurchaseReturn, []int64{ret.ID})
		if perr != nil {
			return nil, apperror.Internal(perr)
		}
		var credited int64
		for _, st := range full.Settlements {
			if st.Type == contracts.SettleSupplierCredit {
				credited += st.Amount
			}
		}
		if room := full.Total - paid[ret.ID] - credited; in.Amount > room {
			if room < 0 {
				room = 0
			}
			return nil, apperror.Conflict("Melebihi sisa retur yang belum direfund/diofset (sisa " + strconv.FormatInt(room, 10) + ")")
		}
	}
	if _, err := s.repo.InsertSettlementAudited(ctx, ret.ID, &contracts.Settlement{
		Type: in.Type, Date: date, Amount: in.Amount,
		PaymentMethod:   strings.TrimSpace(in.PaymentMethod),
		ReferenceNumber: strings.TrimSpace(in.ReferenceNumber),
		Notes:           strings.TrimSpace(in.Notes),
	}, actorID, func(ctx context.Context, tx *sql.Tx, settlementID int64) error {
		return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
			Action: "purchase_returns.settle", Entity: "purchase_return", EntityID: ret.ID, BranchID: branchID, ActorID: actorID, Note: ret.Number,
			After: map[string]any{"settlementId": settlementID, "type": in.Type, "amount": in.Amount, "date": date},
		})
	}); err != nil {
		return nil, apperror.Internal(err)
	}
	return s.GetByID(ctx, ret.ID)
}

// CancelledAmong returns which of ids are cancelled.
func (s *Service) CancelledAmong(ctx context.Context, branchID int64, ids []int64) ([]int64, error) {
	out, err := s.repo.CancelledAmong(ctx, branchID, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}

// ListConfirmedAfter pages confirmed returns by ascending id after a cursor.
func (s *Service) ListConfirmedAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*contracts.Return, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	rows, err := s.repo.ConfirmedAfter(ctx, branchID, afterID, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]*contracts.Return, 0, len(rows))
	for _, ret := range rows {
		out = append(out, &contracts.Return{ID: ret.ID, Number: ret.Number, Branch: ret.BranchID, Total: ret.Total, Date: ret.ReturnDate, Status: ret.Status})
	}
	return out, nil
}

// ActiveReturnCount counts the order's live (draft + confirmed) returns.
func (s *Service) ActiveReturnCount(ctx context.Context, poID int64) (int64, error) {
	n, err := s.repo.ActiveReturnCount(ctx, poID)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return n, nil
}

// ReturnedQtyByOrders implements the contract (see there).
func (s *Service) ReturnedQtyByOrders(ctx context.Context, poIDs []int64) (map[int64]map[string]float64, error) {
	out, err := s.repo.ConfirmedQtyByOrders(ctx, poIDs)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}

// ReturnContext describes what can still be returned for a PO: per-line
// remainders plus the branch's locations for the location picker.
func (s *Service) ReturnContext(ctx context.Context, branchID, poID int64) (map[string]any, error) {
	po, err := s.purchasing.GetByID(ctx, poID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if po == nil || po.BranchID != branchID {
		return nil, apperror.NotFound("Purchase order")
	}
	committedAll, err := s.repo.CommittedQtyByPO(ctx, poID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	hints, err := s.goodsreceipt.PostedReceiptHints(ctx, poID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	lines := make([]any, 0, len(po.Items))
	for _, l := range po.Items {
		received, err := s.goodsreceipt.ReceivedQty(ctx, poID, l.ProductID, l.VariantID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		returned, err := s.repo.ReturnedQty(ctx, poID, l.ProductID, l.VariantID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		// Drafts hold their share too, so the form never offers goods that a
		// pending return already claims.
		remaining := received - committedAll[lineKey(l.ProductID, l.VariantID)]
		if remaining < 0 {
			remaining = 0
		}
		line := map[string]any{
			"productId": l.ProductID, "variantId": l.VariantID,
			"productCode": l.ProductCode, "productName": l.ProductName,
			"uom": l.UOM, "uomFactor": l.UOMFactor, "orderedQty": l.Qty,
			"received": received, "returned": returned, "remaining": remaining,
		}
		// RT-04: the source location defaults to where the goods were last received.
		if loc := hints.Locations[lineKey(l.ProductID, l.VariantID)]; loc > 0 {
			line["defaultLocationId"] = loc
		}
		lines = append(lines, line)
	}
	locs, err := s.stock.ListLocations(ctx, branchID, "active")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	locItems := make([]any, 0, len(locs))
	for _, l := range locs {
		locItems = append(locItems, map[string]any{
			"id": l.ID, "code": l.Code, "name": l.Name,
		})
	}
	return map[string]any{
		"purchaseOrderId": po.ID, "orderNumber": po.Number, "status": po.Status,
		"firstReceivedAt": hints.FirstReceivedAt,
		"lines":           lines, "locations": locItems,
	}, nil
}

// ReturnPreview is the dry-run result of Create: identical numbers, nothing
// persisted, no document number consumed.
type ReturnPreview struct {
	Items         []*contracts.Item
	Subtotal      int64
	DiscountTotal int64
	TaxTotal      int64
	Total         int64
	// Settlement outlook (K-04d): what confirmation will arrive at, from the
	// same rules and offset helper AddSettlement/Confirm use. OrderOutstanding
	// is what we still owe the supplier on the order before this return;
	// PayableReduction the part the automatic offset takes off it;
	// ExternalRemaining the excess that can be refunded or kept as supplier
	// balance; AllowedSettlementTypes the types legal for that excess.
	OrderOutstanding       int64
	PayableReduction       int64
	ExternalRemaining      int64
	AllowedSettlementTypes []string
}

// PreviewReturn runs the exact Create builder without writing anything.
func (s *Service) PreviewReturn(ctx context.Context, branchID, poID int64, lines []LineInput) (*ReturnPreview, error) {
	po, err := s.purchasing.GetByID(ctx, poID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if po == nil || po.BranchID != branchID {
		return nil, apperror.NotFound("Purchase order")
	}
	if po.Status != purchasingcontracts.StatusConfirmed && po.Status != purchasingcontracts.StatusCompleted {
		return nil, apperror.Conflict("Retur hanya untuk order terkonfirmasi")
	}
	if len(lines) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: "wajib diisi"}})
	}
	computed, err := s.computeReturn(ctx, branchID, poID, po, lines)
	if err != nil {
		return nil, err
	}
	var outstanding, reduction int64
	if s.payments != nil && po.Status != purchasingcontracts.StatusCancelled {
		paid, perr := s.payments.PaidForOrders(ctx, paymentcontracts.OrderPurchase, []int64{po.ID})
		if perr != nil {
			return nil, apperror.Internal(perr)
		}
		if outstanding = po.GrandTotal - paid[po.ID]; outstanding < 0 {
			outstanding = 0
		}
		reduction = offsetAmount(po.GrandTotal, paid[po.ID], computed.total)
	}
	pos := positionOf(computed.total, reduction, nil)
	return &ReturnPreview{
		Items:    computed.items,
		Subtotal: computed.subtotal, DiscountTotal: computed.discountTotal,
		TaxTotal: computed.taxTotal, Total: computed.total,
		OrderOutstanding: outstanding, PayableReduction: reduction,
		ExternalRemaining: pos.ExternalRemaining(), AllowedSettlementTypes: pos.Allowed(),
	}, nil
}

func lineKey(productID, variantID int64) string {
	return strconv.FormatInt(productID, 10) + "/" + strconv.FormatInt(variantID, 10)
}

// ListResult is a paginated return page.
type ListResult struct {
	Returns []*contracts.PurchaseReturn
	Total   int64
}

// List searches returns of a branch, newest first.
func (s *Service) List(ctx context.Context, branchID, poID int64, status string, page, limit int) (*ListResult, error) {
	total, err := s.repo.Count(ctx, branchID, poID, status)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	returns, err := s.repo.List(ctx, branchID, poID, status, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if returns == nil {
		returns = []*contracts.PurchaseReturn{}
	}
	return &ListResult{Returns: returns, Total: total}, nil
}

// GetCreditSettlement implements the contract (see there).
func (s *Service) GetCreditSettlement(ctx context.Context, id int64) (*contracts.CreditSettlement, error) {
	out, err := s.repo.GetCreditSettlement(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}

// ListCreditSettlementsAfter implements the contract (see there).
func (s *Service) ListCreditSettlementsAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*contracts.CreditSettlement, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	out, err := s.repo.CreditSettlementsAfter(ctx, branchID, afterID, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}

// CancelledSettlementsAmong implements the contract (see there).
func (s *Service) CancelledSettlementsAmong(ctx context.Context, branchID int64, ids []int64) ([]int64, error) {
	out, err := s.repo.CancelledCreditSettlementsAmong(ctx, branchID, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}
