package application

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/modules/salesreturn/contracts"
	"mini-erp/internal/modules/salesreturn/infrastructure"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/doclock"
	"mini-erp/internal/shared/money"
	"mini-erp/internal/shared/timeutil"
	"mini-erp/internal/shared/validator"
)

// Service implements contracts.SalesReturnClient plus return administration.
// Returns restock on confirm (MoveIn). The source SO is never reopened —
// history stands; money moves only through payments (KI-98 option b).
type Service struct {
	repo     *infrastructure.Repository
	sales    salescontracts.SalesOrderClient
	delivery deliverycontracts.DeliveryClient
	products productcontracts.ProductClient
	stock    stockcontracts.StockClient
	branches branchcontracts.BranchClient
	// payments is set by the composition root after the payment module is
	// built (setter breaks the payment↔returns construction cycle).
	reverser financecontracts.SourceReverser
	payments paymentcontracts.PaymentClient
	// costs is finance's cost reader for the replacement cost guard (RT-04);
	// late-bound like payments/reverser.
	costs CostSource
	audit auditcontracts.AuditClient
}

// NewService wires salesreturn use cases. All cross-module links are contracts.
func NewService(
	repo *infrastructure.Repository,
	sales salescontracts.SalesOrderClient,
	delivery deliverycontracts.DeliveryClient,
	products productcontracts.ProductClient,
	stock stockcontracts.StockClient,
	branches branchcontracts.BranchClient,
	audit auditcontracts.AuditClient,
) *Service {
	return &Service{repo: repo, sales: sales, delivery: delivery,
		products: products, stock: stock, branches: branches, audit: audit}
}

// SetReverser wires finance's source reverser (composition-root privilege).
func (s *Service) SetReverser(r financecontracts.SourceReverser) { s.reverser = r }

// SetPayments injects the payment client (composition-root privilege).
func (s *Service) SetPayments(p paymentcontracts.PaymentClient) {
	s.payments = p
}

// GetByID resolves a return with lines and order number, or nil when missing.
func (s *Service) GetByID(ctx context.Context, id int64) (*contracts.SalesReturn, error) {
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
	if err := s.repo.ReleaseAutoOffset(ctx, returnID); err != nil {
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
	out := projection(ret)
	if out.Kind == contracts.KindRefund {
		if out.CreditSettled, err = s.repo.CreditSettled(ctx, id); err != nil {
			return nil, apperror.Internal(err)
		}
	}
	return out, nil
}

// projection maps a return onto what payments can settle: the whole return
// value for a plain return; for an exchange the absolute replacement-vs-return
// difference (refund when the customer is owed, charge when they owe).
func projection(ret *contracts.SalesReturn) *contracts.Return {
	out := &contracts.Return{
		ID: ret.ID, Number: ret.Number, Branch: ret.BranchID, OrderID: ret.SalesOrderID, Status: ret.Status,
	}
	out.Total, out.Kind = contracts.MoneyEffect(ret.ReturnMode, ret.Total, ret.DifferenceAmount)
	return out
}

// OrderEffects implements the contract (see there).
func (s *Service) OrderEffects(ctx context.Context, branchID int64, orderIDs []int64) (map[int64]contracts.OrderEffect, error) {
	out, err := s.repo.OrderEffects(ctx, branchID, orderIDs)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
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

func (s *Service) enrich(ctx context.Context, ret *contracts.SalesReturn) (*contracts.SalesReturn, error) {
	items, err := s.repo.ItemsByReturn(ctx, ret.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if items == nil {
		items = []*contracts.Item{}
	}
	ret.Items = items
	repl, err := s.repo.ReplacementItems(ctx, ret.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if repl == nil {
		repl = []*contracts.ReplacementItem{}
	}
	ret.ReplacementItems = repl
	settlements, err := s.repo.Settlements(ctx, ret.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if settlements == nil {
		settlements = []*contracts.Settlement{}
	}
	ret.Settlements = settlements
	so, err := s.sales.GetByID(ctx, ret.SalesOrderID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if so != nil {
		ret.OrderNumber = so.Number
	}
	ret.AllowedSettlementTypes = []string{}
	if ret.Status == contracts.StatusConfirmed {
		pos := positionOf(ret.ReturnMode, ret.Total, ret.DifferenceAmount, ret.ReceivableReduction, ret.Settlements, so != nil && so.PartyID != 0)
		ret.ExternalRemaining, ret.ChargeRemaining, ret.AllowedSettlementTypes = pos.ExternalRemaining(), pos.ChargeRemaining(), pos.Allowed()
	}
	return ret, nil
}

// mustOwnBranch loads the return and denies cross-branch access as missing.
func (s *Service) mustOwnBranch(ctx context.Context, id, branchID int64) (*contracts.SalesReturn, error) {
	ret, err := s.repo.GetReturn(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if ret == nil || ret.BranchID != branchID {
		return nil, apperror.NotFound("Retur penjualan")
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
	// Condition: normal (default) | damaged | not_returned.
	Condition string
}

// CreateOption sets optional return header data.
type CreateOption func(*createOptions)

type createOptions struct{ reason string }

// WithReason records why the goods come back.
func WithReason(reason string) CreateOption {
	return func(o *createOptions) { o.reason = strings.TrimSpace(reason) }
}

// ReplacementInput is one exchange replacement line. Priced once at
// creation from the live catalog (a new sale in effect) — member pricing
// does not apply to replacements.
type ReplacementInput struct {
	ProductID  int64
	VariantID  int64
	LocationID int64
	UOM        string
	Qty        float64
	// UnitPrice overrides the catalog price (never below the minimum price).
	UnitPrice *int64
}

// Create drafts a return against a confirmed/completed SO. Quantities cap at
// delivered-minus-returned per (product, variant) aggregate — never at the
// ordered figure, so undelivered goods cannot come back as stock.
func (s *Service) Create(ctx context.Context, actorID, branchID, soID int64, returnDate, notes, mode string, lines []LineInput, replacements []ReplacementInput, opts ...CreateOption) (*contracts.SalesReturn, error) {
	var o createOptions
	for _, opt := range opts {
		opt(&o)
	}
	var out *contracts.SalesReturn
	err := doclock.Lock("so:"+strconv.FormatInt(soID, 10), func() error {
		res, err := s.createLocked(ctx, actorID, branchID, soID, returnDate, notes, mode, lines, replacements, o)
		if err != nil {
			return err
		}
		out = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "sales_returns.create", Entity: "sales_return", EntityID: out.ID, BranchID: branchID, ActorID: actorID, Note: out.Number})
	return out, nil
}

// normalizeMode defaults empty mode and cross-checks it against the
// replacement payload (shared by Create and Preview).
func normalizeMode(mode string, replacementCount int) (string, error) {
	if mode == "" {
		mode = contracts.ReturnModeReturnOnly
	}
	if mode != contracts.ReturnModeReturnOnly && mode != contracts.ReturnModeExchange {
		return "", apperror.Validation("", []apperror.FieldError{{Field: "returnMode", Message: "harus return_only atau exchange"}})
	}
	if mode == contracts.ReturnModeExchange && replacementCount == 0 {
		return "", apperror.Validation("", []apperror.FieldError{{Field: "replacementItems", Message: "retur tukar wajib membawa barang pengganti"}})
	}
	if mode == contracts.ReturnModeReturnOnly && replacementCount > 0 {
		return "", apperror.Validation("", []apperror.FieldError{{Field: "replacementItems", Message: "hanya untuk retur tukar"}})
	}
	return mode, nil
}

// soAggregate indexes SO lines per (product, variant) for matching.
type soAggregate struct {
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

func aggregateSO(so *salescontracts.SalesOrder) map[string]*soAggregate {
	soAgg := map[string]*soAggregate{}
	for _, l := range so.Items {
		key := lineKey(l.ProductID, l.VariantID)
		agg, ok := soAgg[key]
		if !ok {
			agg = &soAggregate{uoms: map[string]float64{}, econ: map[string]*money.LineEconomics{}}
			soAgg[key] = agg
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
	return soAgg
}

// computedReturn is the validated + priced result shared by Create and
// Preview: the same builder feeds both, so a preview can never drift from
// what Create would persist (D5, same pattern as finance Preview/Post).
type computedReturn struct {
	items         []*contracts.Item
	subtotal      int64
	discountTotal int64
	taxTotal      int64
	total         int64
}

// computeReturn validates return lines against the SO (caps, master data)
// and prices them with source-line economics. Read-only: no writes, no
// number allocation — safe for Preview.
func (s *Service) computeReturn(ctx context.Context, branchID, soID int64, so *salescontracts.SalesOrder, lines []LineInput) (*computedReturn, error) {
	soAgg := aggregateSO(so)
	// Quota = delivered - (draft + confirmed returns) - lines already accepted
	// in this request (P6). Drafts hold their share, and two lines of one
	// request on the same product add up instead of each seeing the full rest.
	committed, err := s.repo.CommittedQtyBySO(ctx, soID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	deliveredCache := map[string]float64{}
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
		agg, ok := soAgg[lineKey(line.ProductID, line.VariantID)]
		if !ok {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "produk tidak ada di SO"}})
		}
		uom := strings.TrimSpace(line.UOM)
		factor, ok := agg.uoms[uom]
		if !ok {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "satuan harus sama dengan SO"}})
		}
		key := lineKey(line.ProductID, line.VariantID)
		delivered, ok := deliveredCache[key]
		if !ok {
			var derr error
			if delivered, derr = s.delivery.DeliveredQty(ctx, soID, line.ProductID, line.VariantID); derr != nil {
				return nil, apperror.Internal(derr)
			}
			deliveredCache[key] = delivered
		}
		qtyBase := line.Qty * factor
		taken := committed[key] + used[key]
		if delivered-taken-qtyBase < -1e-9 {
			return nil, apperror.Conflict("Melebihi yang bisa diretur (" + tag + "sisa " + strconv.FormatFloat(delivered-taken, 'f', -1, 64) + ")")
		}
		used[key] += qtyBase
		cond := line.Condition
		if cond == "" {
			cond = contracts.ConditionNormal
		}
		locationID := line.LocationID
		switch cond {
		case contracts.ConditionNormal:
			if agg.tracked {
				if err := s.stock.CheckLocation(ctx, branchID, line.LocationID); err != nil {
					return nil, err
				}
			}
		case contracts.ConditionDamaged:
			// Damaged goods always land in the branch damaged location; the
			// client does not get to choose.
			dmg, derr := s.stock.DamagedLocation(ctx, branchID)
			if derr != nil {
				return nil, derr
			}
			locationID = dmg.ID
		case contracts.ConditionNotReturned:
			locationID = 0 // the customer keeps the goods: value only
		default:
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "kondisi harus normal, damaged, atau not_returned"}})
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
		// and tax split (taxable flag, rate) pro-rate in. Lines that cannot
		// match a source line are rejected above; never fall back to live
		// catalog prices (that would refund today's price instead of the price
		// actually paid).
		econ := agg.econ[uom]
		gross, net, base, tax := econ.Part(line.Qty)
		unitPrice, pct := econ.UnitPrice(), econ.DiscountPct()
		nominalPart := max(0, gross-net-money.Round(float64(gross)*pct/100))
		lineTotal := base + tax
		subtotal += base
		discountTotal += gross - net
		taxTotal += tax
		items = append(items, &contracts.Item{
			ProductID: line.ProductID, VariantID: line.VariantID, LocationID: locationID, Condition: cond,
			UOM: uom, UOMFactor: factor, Qty: line.Qty, QtyBase: qtyBase,
			UnitPrice: unitPrice, DiscountPct: pct, DiscountNominal: nominalPart,
			TaxBase: base, TaxAmount: tax, LineTotal: lineTotal,
		})
	}
	return &computedReturn{items: items, subtotal: subtotal,
		discountTotal: discountTotal, taxTotal: taxTotal, total: subtotal + taxTotal}, nil
}

func (s *Service) createLocked(ctx context.Context, actorID, branchID, soID int64, returnDate, notes, mode string, lines []LineInput, replacements []ReplacementInput, o createOptions) (*contracts.SalesReturn, error) {
	so, err := s.sales.GetByID(ctx, soID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if so == nil || so.BranchID != branchID {
		return nil, apperror.NotFound("Sales order")
	}
	if so.Status != salescontracts.StatusConfirmed && so.Status != salescontracts.StatusCompleted {
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
	if len(lines) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: "wajib diisi"}})
	}
	if mode, err = normalizeMode(mode, len(replacements)); err != nil {
		return nil, err
	}
	computed, err := s.computeReturn(ctx, branchID, soID, so, lines)
	if err != nil {
		return nil, err
	}
	items, subtotal, discountTotal, taxTotal := computed.items, computed.subtotal, computed.discountTotal, computed.taxTotal
	total := subtotal + taxTotal
	replItems, err := s.priceReplacements(ctx, branchID, so, replacements)
	if err != nil {
		return nil, err
	}
	replStatus := contracts.ReplacementNotRequired
	if mode == contracts.ReturnModeExchange {
		replStatus = contracts.ReplacementPending
	}
	if len(o.reason) > 255 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "reason", Message: "maksimal 255 karakter"}})
	}
	var replSubtotal, replTax, replTotal int64
	for _, ri := range replItems {
		replSubtotal += ri.TaxBase
		replTax += ri.TaxAmount
		replTotal += ri.LineTotal
	}
	difference := int64(0)
	if mode == contracts.ReturnModeExchange {
		difference = replTotal - total
	}
	number, err := s.branches.NextDocumentNumber(ctx, branchID, branchcontracts.DocSalesReturn)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	id, err := s.repo.CreateReturn(ctx, &contracts.SalesReturn{
		Number: number, BranchID: branchID, SalesOrderID: soID,
		ReturnDate: strings.TrimSpace(returnDate),
		Subtotal:   subtotal, DiscountTotal: discountTotal, TaxTotal: taxTotal,
		TaxType: so.TaxType, TaxRate: so.TaxRate,
		Total: total, Notes: notes, Items: items,
		ReturnMode: mode, ReplacementDeliveryStatus: replStatus,
		Reason: o.reason, ReplacementSubtotal: replSubtotal, ReplacementTaxTotal: replTax,
		ReplacementTotal: replTotal, DifferenceAmount: difference,
	}, replItems, actorID)
	if err != nil {
		return nil, dberr.Map(err)
	}
	return s.GetByID(ctx, id)
}

// Confirm restocks every line. Draft-only; the source SO keeps its status —
// history stands. Replay-safe (P4): the return is re-read inside the lock, the
// quota is re-checked, stock.Post is idempotent per document and the status
// flip is a CAS, so repeating a request that failed midway finishes the job
// and a repeat of a finished one is a no-op 200.
func (s *Service) Confirm(ctx context.Context, actorID, branchID, id int64, opts ...ConfirmOption) (*contracts.SalesReturn, error) {
	var o confirmOptions
	for _, opt := range opts {
		opt(&o)
	}
	ret, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	var out *contracts.SalesReturn
	err = doclock.Lock("so:"+strconv.FormatInt(ret.SalesOrderID, 10), func() error {
		res, _, err := s.confirmLocked(ctx, actorID, branchID, id, o)
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

func (s *Service) confirmLocked(ctx context.Context, actorID, branchID, id int64, o confirmOptions) (*contracts.SalesReturn, bool, error) {
	ret, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, false, err
	}
	switch ret.Status {
	case contracts.StatusConfirmed:
		// Already final: only the post-final steps may be missing (the offset,
		// and the counter hand-over of a POS exchange).
		if err := s.autoOffset(ctx, actorID, branchID, ret.ID); err != nil {
			return nil, false, err
		}
		if err := s.finishCounterExchange(ctx, actorID, branchID, ret, o); err != nil {
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
	if err := s.checkCommitted(ctx, ret.SalesOrderID, full); err != nil {
		return nil, false, err
	}
	so, err := s.sales.GetByID(ctx, ret.SalesOrderID)
	if err != nil || so == nil {
		return nil, false, apperror.Internal(errors.New("source order unavailable"))
	}
	// RT-02: a POS exchange hands the replacement over right now, so its cost
	// basis and stock are checked BEFORE the returned goods are booked in —
	// a refusal leaves the draft untouched.
	if isCounterExchange(so, full) && full.ReplacementDeliveryStatus == contracts.ReplacementPending {
		if err := s.checkReplacementReady(ctx, branchID, full.ReplacementItems); err != nil {
			return nil, false, err
		}
	}
	untracked := map[string]bool{}
	for _, l := range so.Items {
		if !l.StockTracked {
			untracked[lineKey(l.ProductID, l.VariantID)] = true
		}
	}
	lines := make([]stockcontracts.PostLine, 0, len(full.Items))
	for _, it := range full.Items {
		if untracked[lineKey(it.ProductID, it.VariantID)] || it.Condition == contracts.ConditionNotReturned {
			continue // service lines and kept goods return value, never stock
		}
		lines = append(lines, stockcontracts.PostLine{ProductID: it.ProductID, VariantID: it.VariantID, LocationID: it.LocationID, Qty: it.QtyBase})
	}
	if len(lines) > 0 {
		if _, err := s.stock.Post(ctx, stockcontracts.PostRequest{
			BranchID: branchID, Direction: "in", RefType: stockcontracts.RefSalesReturn, RefID: full.ID,
			ActorID: actorID, Lines: lines, AllowDamagedLocation: true,
		}); err != nil {
			return nil, false, err
		}
	}
	// The trail rides the confirm CAS (ADR-0011): audited only by the winner.
	won, err := s.repo.TransitionAudited(ctx, full.ID, []string{contracts.StatusDraft}, contracts.StatusConfirmed, actorID,
		func(ctx context.Context, tx *sql.Tx) error {
			return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
				Action: "sales_returns.confirm", Entity: "sales_return", EntityID: full.ID, BranchID: branchID, ActorID: actorID, Note: full.Number,
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
	if err := s.finishCounterExchange(ctx, actorID, branchID, full, o); err != nil {
		return nil, false, err
	}
	out, err := s.GetByID(ctx, full.ID)
	return out, replayed, err
}

// finishCounterExchange runs the counter hand-over when the confirmed return is
// a POS exchange; every other return is untouched (RT-02).
func (s *Service) finishCounterExchange(ctx context.Context, actorID, branchID int64, ret *contracts.SalesReturn, o confirmOptions) error {
	if ret.ReturnMode != contracts.ReturnModeExchange {
		return nil
	}
	so, err := s.sales.GetByID(ctx, ret.SalesOrderID)
	if err != nil || so == nil {
		return apperror.Internal(errors.New("source order unavailable"))
	}
	if !isCounterExchange(so, ret) {
		return nil
	}
	return s.handOverAtCounter(ctx, actorID, branchID, ret.ID, o)
}

// autoOffset is the idempotent post-final step of Confirm: a refund-kind
// return of a customer order that still has an open bill is offset against
// that bill (up to the smaller of the two), so the customer's receivable falls
// without anyone issuing cash. The offset is keyed offset:sr:<id>; the memo
// row and receivable_reduction are written once.
func (s *Service) autoOffset(ctx context.Context, actorID, branchID, id int64) error {
	if s.payments == nil {
		return nil
	}
	ret, err := s.repo.GetReturn(ctx, id)
	if err != nil || ret == nil {
		return apperror.Internal(errors.New("return unavailable for offset"))
	}
	proj := projection(ret)
	if ret.Status != contracts.StatusConfirmed || proj.Kind != contracts.KindRefund || proj.Total <= 0 {
		return nil
	}
	so, err := s.sales.GetByID(ctx, ret.SalesOrderID)
	if err != nil || so == nil {
		return apperror.Internal(errors.New("source order unavailable"))
	}
	if so.PartyID == 0 || so.Status == salescontracts.StatusCancelled {
		return nil // walk-in sales have no account to offset
	}
	ref := "offset:sr:" + strconv.FormatInt(ret.ID, 10)
	amount := ret.ReceivableReduction
	if amount == 0 {
		paid, err := s.payments.PaidForOrders(ctx, paymentcontracts.OrderSales, []int64{so.ID})
		if err != nil {
			return apperror.Internal(err)
		}
		if amount = offsetAmount(so.GrandTotal, paid[so.ID], proj.Total); amount <= 0 {
			return nil
		}
	}
	pay, _, err := s.payments.OffsetReturn(ctx, paymentcontracts.OffsetInput{
		ActorID: actorID, BranchID: branchID, PartyID: so.PartyID,
		ReturnType: paymentcontracts.OrderSalesReturn, ReturnID: ret.ID,
		OrderType: paymentcontracts.OrderSales, OrderID: so.ID, Amount: amount,
		ClientRef: ref, Notes: "Offset otomatis retur " + ret.Number,
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
	if ret.ReceivableReduction != pay.Amount {
		if err := s.repo.SetReduction(ctx, ret.ID, pay.Amount); err != nil {
			return apperror.Internal(err)
		}
		memos, err := s.repo.Settlements(ctx, ret.ID)
		if err != nil {
			return apperror.Internal(err)
		}
		for _, m := range memos {
			if m.Auto {
				return nil
			}
		}
		if _, err := s.repo.InsertSettlement(ctx, ret.ID, &contracts.Settlement{
			Type: contracts.SettleReduceReceivable, Date: timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02"),
			Amount: pay.Amount, Notes: "Offset otomatis ke tagihan order", Auto: true,
		}, actorID); err != nil {
			return apperror.Internal(err)
		}
	}
	return nil
}

// checkCommitted re-verifies, at confirm time, that Σ(draft + confirmed
// returns including this one) still fits within what was delivered.
func (s *Service) checkCommitted(ctx context.Context, soID int64, ret *contracts.SalesReturn) error {
	committed, err := s.repo.CommittedQtyBySO(ctx, soID)
	if err != nil {
		return apperror.Internal(err)
	}
	for _, it := range ret.Items {
		key := lineKey(it.ProductID, it.VariantID)
		delivered, err := s.delivery.DeliveredQty(ctx, soID, it.ProductID, it.VariantID)
		if err != nil {
			return apperror.Internal(err)
		}
		if committed[key] > delivered+1e-9 {
			return apperror.Conflict("Jumlah retur melebihi yang terkirim, periksa retur lain pada order ini")
		}
	}
	return nil
}

// Cancel voids a draft outright, or reverses a confirmed return (stock back
// out) — the KI-99/KI-101 cancel path that legacy promised but never built.
// The status is re-read inside the lock so two simultaneous cancels cannot
// both move stock; the second one replays.
//
// The reason is recorded on the return and in the audit entry (co-committed
// with the status flip); the web endpoint makes it mandatory, programmatic
// callers may pass "".
func (s *Service) Cancel(ctx context.Context, actorID, branchID, id int64, reason string) (*contracts.SalesReturn, error) {
	reason, err := validator.Reason(reason, false)
	if err != nil {
		return nil, err
	}
	first, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	var out *contracts.SalesReturn
	var replayed, wasConfirmed bool
	err = doclock.Lock("so:"+strconv.FormatInt(first.SalesOrderID, 10), func() error {
		ret, err := s.mustOwnBranch(ctx, id, branchID)
		if err != nil {
			return err
		}
		if ret.Status == contracts.StatusCancelled {
			replayed = true
			out, err = s.GetByID(ctx, ret.ID)
			return err
		}
		// Exchange returns with a moving replacement cannot vanish: the
		// replacement shipment would strand without its parent document.
		if ret.ReturnMode == contracts.ReturnModeExchange &&
			(ret.ReplacementDeliveryStatus == contracts.ReplacementDispatched ||
				ret.ReplacementDeliveryStatus == contracts.ReplacementConfirmed) {
			return apperror.Conflict("Retur tukar yang barang penggantinya sudah dikirim tidak dapat dibatalkan")
		}
		switch ret.Status {
		case contracts.StatusDraft:
			// A draft whose confirm died after the stock posting is reversed too.
			if moved, err := s.stock.HasMovements(ctx, branchID, stockcontracts.RefSalesReturn, ret.ID); err != nil {
				return err
			} else if moved {
				if _, err := s.stock.Reverse(ctx, branchID, stockcontracts.RefSalesReturn, ret.ID, actorID); err != nil {
					return err
				}
			}
		case contracts.StatusConfirmed:
			wasConfirmed = true
			if s.payments == nil {
				return apperror.Internal(errPaymentsNotWired)
			}
			if used, err := s.payments.HasActiveAllocations(ctx, paymentcontracts.OrderSalesReturn, ret.ID); err != nil {
				return apperror.Internal(err)
			} else if used {
				return apperror.Conflict("Retur sudah ada refund, batalkan pembayaran dulu")
			}
			if _, err := s.stock.Reverse(ctx, branchID, stockcontracts.RefSalesReturn, ret.ID, actorID); err != nil {
				return err
			}
		default:
			return apperror.Conflict("Retur sudah dibatalkan")
		}
		from := ret.Status
		won, err := s.repo.Cancel(ctx, ret.ID, reason, actorID, func(ctx context.Context, tx *sql.Tx) error {
			note := ret.Number
			if reason != "" {
				note += ": " + reason
			}
			return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
				Action: "sales_returns.cancel", Entity: "sales_return", EntityID: ret.ID, BranchID: branchID, ActorID: actorID, Note: note,
				Before: map[string]any{"status": from},
				After:  map[string]any{"status": contracts.StatusCancelled, "reason": reason},
			})
		})
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
		if _, rerr := s.reverser.ReverseForSource(ctx, actorID, branchID, "sales_return", id, "Retur dibatalkan"); rerr != nil {
			_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "finance.autoreverse_failed", Entity: "sales_return", EntityID: id, BranchID: branchID, ActorID: actorID, Note: rerr.Error()})
		}
		// The customer-balance reclassifications (K-04a) die with the return.
		// Each settlement is its own journal source; a failure is audited and
		// surfaces as an orphan, never undoes the cancellation.
		credits, cerr := s.repo.CreditSettlementIDs(ctx, id)
		if cerr != nil {
			_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "finance.autoreverse_failed", Entity: "sales_return_settlement", EntityID: id, BranchID: branchID, ActorID: actorID, Note: cerr.Error()})
		}
		for _, sid := range credits {
			if _, rerr := s.reverser.ReverseForSource(ctx, actorID, branchID, "sales_return_settlement", sid, "Retur dibatalkan"); rerr != nil {
				_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "finance.autoreverse_failed", Entity: "sales_return_settlement", EntityID: sid, BranchID: branchID, ActorID: actorID, Note: rerr.Error()})
			}
		}
	}
	return out, nil
}

// ReplacementDispatchInput carries the SJ-face fields for a replacement
// shipment. Lines always come from the stored replacement items.
type ReplacementDispatchInput struct {
	DeliveryDate       string
	Notes              string
	DriverName         string
	VehiclePlate       string
	WarehouseStaffName string
	DropLocationNote   string
	// AtCounter marks the POS counter hand-over (RT-02): no crew needed.
	AtCounter bool
}

// CreateReplacementDelivery drafts the replacement shipment of a confirmed
// exchange return from its stored replacement lines. Stock moves only at
// confirm (D4). The return must be confirmed and still pending; a second
// call sees dispatched status and stops (no double shipment).
func (s *Service) CreateReplacementDelivery(ctx context.Context, actorID, branchID, id int64, in ReplacementDispatchInput) (*deliverycontracts.DeliveryNote, error) {
	ret, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if ret.ReturnMode != contracts.ReturnModeExchange {
		return nil, apperror.Conflict("Hanya retur tukar yang punya barang pengganti")
	}
	if ret.Status != contracts.StatusConfirmed {
		return nil, apperror.Conflict("Konfirmasi retur dulu sebelum mengirim barang pengganti")
	}
	if ret.ReplacementDeliveryStatus != contracts.ReplacementPending {
		return nil, apperror.Conflict("Barang pengganti sudah dikirim untuk retur ini")
	}
	full, err := s.enrich(ctx, ret)
	if err != nil {
		return nil, err
	}
	if len(full.ReplacementItems) == 0 {
		return nil, apperror.Conflict("Retur ini tidak membawa barang pengganti")
	}
	// D6: every replacement line must have a cost basis (catalog purchase
	// price) and on-hand stock at its location — otherwise finance would
	// book zero COGS and the dispatch would strand mid-way.
	if err := s.checkReplacementReady(ctx, branchID, full.ReplacementItems); err != nil {
		return nil, err
	}
	items := make([]deliverycontracts.ReplacementLine, 0, len(full.ReplacementItems))
	for _, it := range full.ReplacementItems {
		items = append(items, deliverycontracts.ReplacementLine{
			ProductID: it.ProductID, VariantID: it.VariantID,
			LocationID: it.LocationID, UOM: it.UOM, Qty: it.Qty,
		})
	}
	var out *deliverycontracts.DeliveryNote
	err = doclock.Lock("sr:"+strconv.FormatInt(ret.ID, 10), func() error {
		fresh, err := s.repo.GetReturn(ctx, ret.ID)
		if err != nil {
			return apperror.Internal(err)
		}
		if fresh == nil || fresh.ReplacementDeliveryStatus != contracts.ReplacementPending {
			return apperror.Conflict("Barang pengganti sudah dikirim untuk retur ini")
		}
		dn, err := s.delivery.CreateReplacement(ctx, actorID, branchID, deliverycontracts.ReplacementDraft{
			SalesOrderID: ret.SalesOrderID, SalesReturnID: ret.ID,
			DeliveryDate: in.DeliveryDate, Notes: in.Notes,
			DriverName: in.DriverName, VehiclePlate: in.VehiclePlate,
			WarehouseStaff: in.WarehouseStaffName, DropNote: in.DropLocationNote,
			AtCounter: in.AtCounter, Items: items,
		})
		if err != nil {
			return err
		}
		if won, err := s.repo.SetReplacementStatus(ctx, ret.ID, contracts.ReplacementPending, contracts.ReplacementDispatched, actorID); err != nil {
			return apperror.Internal(err)
		} else if !won {
			return apperror.Conflict("Status pengiriman pengganti berubah, muat ulang")
		}
		out = dn
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "sales_returns.dispatch_replacement", Entity: "sales_return", EntityID: ret.ID, BranchID: branchID, ActorID: actorID, Note: out.Number})
	return out, nil
}

// ConfirmReplacementDelivery confirms the replacement shipment (stock out)
// and closes the exchange pipeline. Serialized per return like creation.
func (s *Service) ConfirmReplacementDelivery(ctx context.Context, actorID, branchID, returnID, deliveryID int64, in deliverycontracts.ReplacementConfirm) (*deliverycontracts.DeliveryNote, error) {
	ret, err := s.mustOwnBranch(ctx, returnID, branchID)
	if err != nil {
		return nil, err
	}
	if ret.ReturnMode != contracts.ReturnModeExchange {
		return nil, apperror.Conflict("Hanya retur tukar yang punya barang pengganti")
	}
	if ret.ReplacementDeliveryStatus != contracts.ReplacementDispatched {
		return nil, apperror.Conflict("Tidak ada pengiriman pengganti yang menunggu konfirmasi")
	}
	dn, err := s.delivery.GetByID(ctx, deliveryID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if dn == nil || dn.BranchID != branchID ||
		dn.DocumentKind != deliverycontracts.DocumentKindReplacement ||
		dn.SalesReturnID != returnID {
		return nil, apperror.NotFound("Surat jalan pengganti")
	}
	var out *deliverycontracts.DeliveryNote
	err = doclock.Lock("sr:"+strconv.FormatInt(ret.ID, 10), func() error {
		fresh, err := s.repo.GetReturn(ctx, ret.ID)
		if err != nil {
			return apperror.Internal(err)
		}
		if fresh == nil || fresh.ReplacementDeliveryStatus != contracts.ReplacementDispatched {
			return apperror.Conflict("Tidak ada pengiriman pengganti yang menunggu konfirmasi")
		}
		// K-04e: an exchange that costs more can collect its surcharge with the
		// hand-over (pay_now). Validated here, where the return and its charge
		// are known, BEFORE anything moves; delivery then records the payment
		// inside its own confirm saga (stock -> payment -> status flip).
		if in.Settlement != nil {
			resolved, err := s.resolveReplacementSettlement(ctx, fresh, in.Settlement)
			if err != nil {
				return err
			}
			in.Settlement = resolved
		}
		confirmed, err := s.delivery.ConfirmReplacement(ctx, actorID, branchID, deliveryID, in)
		if err != nil {
			return err
		}
		if won, err := s.repo.SetReplacementStatus(ctx, ret.ID, contracts.ReplacementDispatched, contracts.ReplacementConfirmed, actorID); err != nil {
			return apperror.Internal(err)
		} else if !won {
			return apperror.Conflict("Status pengiriman pengganti berubah, muat ulang")
		}
		if in.Settlement != nil {
			// Memo row so the settlement position shows the collection; the
			// money itself is the payment recorded above.
			if _, err := s.repo.InsertSettlement(ctx, ret.ID, &contracts.Settlement{
				Type: contracts.SettleCollectPayment, Date: timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02"),
				Amount: in.Settlement.Amount, PaymentMethod: in.Settlement.Method,
				ReferenceNumber: in.Settlement.Reference, Notes: "Tambah bayar saat serah " + confirmed.Number, Auto: true,
			}, actorID); err != nil {
				_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "sales_returns.settle_memo_failed", Entity: "sales_return", EntityID: ret.ID, BranchID: branchID, ActorID: actorID, Note: err.Error()})
			}
		}
		out = confirmed
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "sales_returns.confirm_replacement", Entity: "sales_return", EntityID: ret.ID, BranchID: branchID, ActorID: actorID, Note: out.Number})
	return out, nil
}

// CancelReplacementDelivery takes back a replacement shipment that is still a
// DRAFT (wrong goods, wrong crew, wrong date…) and reopens the exchange: the
// draft SJ is voided through delivery's narrow CancelReplacement (which also
// reverses any stock a failed confirm moved) and the return's replacement
// status goes dispatched -> pending, so the user can dispatch a corrected
// replacement or cancel the return. A confirmed replacement shipment stays:
// the goods reached the customer, so the correction is a new return.
//
// Replay-safe (ADR-0016), serialized per return like dispatch/confirm:
// validate → void every live draft (idempotent) → CAS dispatched -> pending
// (audited in the same transaction). A retry after a failure between the
// two writes finds no live draft and only flips the status; a repeat after
// success finds "pending" with a cancelled replacement note and replays.
func (s *Service) CancelReplacementDelivery(ctx context.Context, actorID, branchID, id int64, reason string) (*contracts.SalesReturn, error) {
	reason, err := validator.Reason(reason, false)
	if err != nil {
		return nil, err
	}
	ret, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if ret.ReturnMode != contracts.ReturnModeExchange {
		return nil, apperror.Conflict("Hanya retur tukar yang punya barang pengganti")
	}
	err = doclock.Lock("sr:"+strconv.FormatInt(ret.ID, 10), func() error {
		fresh, err := s.repo.GetReturn(ctx, ret.ID)
		if err != nil {
			return apperror.Internal(err)
		}
		if fresh == nil {
			return apperror.NotFound("Retur penjualan")
		}
		notes, err := s.replacementNotes(ctx, branchID, fresh)
		if err != nil {
			return err
		}
		switch fresh.ReplacementDeliveryStatus {
		case contracts.ReplacementDispatched:
		case contracts.ReplacementPending:
			for _, n := range notes {
				if n.Status == deliverycontracts.StatusCancelled {
					return nil // replay: already taken back
				}
			}
			return apperror.Conflict("Barang pengganti belum dikirim, tidak ada yang dibatalkan")
		case contracts.ReplacementConfirmed:
			return apperror.Conflict("Surat jalan pengganti sudah dikonfirmasi; barang pengganti tidak dapat ditarik dengan pembatalan")
		default:
			return apperror.Conflict("Retur ini tidak punya pengiriman pengganti")
		}
		var drafts []*deliverycontracts.DeliveryNote
		for _, n := range notes {
			switch n.Status {
			case deliverycontracts.StatusConfirmed:
				return apperror.Conflict("Surat jalan pengganti " + n.Number + " sudah dikonfirmasi; barang pengganti tidak dapat ditarik dengan pembatalan")
			case deliverycontracts.StatusDraft:
				drafts = append(drafts, n)
			}
		}
		numbers := make([]string, 0, len(drafts))
		for _, n := range drafts {
			if _, err := s.delivery.CancelReplacement(ctx, actorID, branchID, n.ID, reason); err != nil {
				return err
			}
			numbers = append(numbers, n.Number)
		}
		won, err := s.repo.SetReplacementStatusAudited(ctx, fresh.ID, contracts.ReplacementDispatched, contracts.ReplacementPending, actorID,
			func(ctx context.Context, tx *sql.Tx) error {
				note := fresh.Number
				if len(numbers) > 0 {
					note += " (" + strings.Join(numbers, ", ") + ")"
				}
				if reason != "" {
					note += ": " + reason
				}
				return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
					Action: "sales_returns.cancel_replacement", Entity: "sales_return", EntityID: fresh.ID, BranchID: branchID, ActorID: actorID, Note: note,
					Before: map[string]any{"replacementDeliveryStatus": contracts.ReplacementDispatched},
					After:  map[string]any{"replacementDeliveryStatus": contracts.ReplacementPending, "reason": reason, "deliveryNotes": numbers},
				})
			})
		if err != nil {
			return apperror.Internal(err)
		}
		if !won {
			cur, err := s.repo.GetReturn(ctx, fresh.ID)
			if err != nil {
				return apperror.Internal(err)
			}
			if cur == nil || cur.ReplacementDeliveryStatus != contracts.ReplacementPending {
				return apperror.Conflict("Status pengiriman pengganti berubah, muat ulang")
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, ret.ID)
}

// replacementNotes lists the return's replacement shipments (every status)
// from the order's shipment list.
func (s *Service) replacementNotes(ctx context.Context, branchID int64, ret *contracts.SalesReturn) ([]*deliverycontracts.DeliveryNote, error) {
	all, err := s.delivery.ListBySO(ctx, branchID, ret.SalesOrderID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var out []*deliverycontracts.DeliveryNote
	for _, n := range all {
		if n.DocumentKind == deliverycontracts.DocumentKindReplacement && n.SalesReturnID == ret.ID {
			out = append(out, n)
		}
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
func (s *Service) AddSettlement(ctx context.Context, actorID, branchID, id int64, in SettlementInput) (*contracts.SalesReturn, error) {
	// Serialized per return: two concurrent settlements would both pass the
	// remaining-room check and together settle more than the return is owed.
	var out *contracts.SalesReturn
	err := doclock.Lock("sr:"+strconv.FormatInt(id, 10), func() error {
		res, err := s.addSettlementLocked(ctx, actorID, branchID, id, in)
		out = res
		return err
	})
	return out, err
}

func (s *Service) addSettlementLocked(ctx context.Context, actorID, branchID, id int64, in SettlementInput) (*contracts.SalesReturn, error) {
	ret, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if ret.Status != contracts.StatusConfirmed {
		return nil, apperror.Conflict("Penyelesaian hanya untuk retur terkonfirmasi")
	}
	switch in.Type {
	case contracts.SettleCollectPayment, contracts.SettleReduceReceivable,
		contracts.SettleRefund, contracts.SettleCustomerCredit:
	default:
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "type", Message: "jenis penyelesaian tidak dikenal"}})
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
	// K-04c: legacy-style rules from the direction of the difference  - the
	// position is derived from the same rows the detail view shows. The order
	// party is needed only for customer_credit (the balance needs an owner).
	so, err := s.sales.GetByID(ctx, ret.SalesOrderID)
	if err != nil || so == nil {
		return nil, apperror.Internal(errors.New("source order unavailable"))
	}
	pos := positionOf(full.ReturnMode, full.Total, full.DifferenceAmount, full.ReceivableReduction, full.Settlements, so.PartyID != 0)
	if err := pos.validate(in.Type, in.Amount); err != nil {
		return nil, err
	}
	// N-40: money already refunded / offset through payments is gone too; the
	// balance can only keep what is still unrefunded and not yet credited.
	if in.Type == contracts.SettleCustomerCredit && s.payments != nil {
		paid, perr := s.payments.PaidForOrders(ctx, paymentcontracts.OrderSalesReturn, []int64{ret.ID})
		if perr != nil {
			return nil, apperror.Internal(perr)
		}
		var credited int64
		for _, st := range full.Settlements {
			if st.Type == contracts.SettleCustomerCredit {
				credited += st.Amount
			}
		}
		if room := pos.Owed - paid[ret.ID] - credited; in.Amount > room {
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
			Action: "sales_returns.settle", Entity: "sales_return", EntityID: ret.ID, BranchID: branchID, ActorID: actorID, Note: ret.Number,
			After: map[string]any{"settlementId": settlementID, "type": in.Type, "amount": in.Amount, "date": date},
		})
	}); err != nil {
		return nil, apperror.Internal(err)
	}
	return s.GetByID(ctx, ret.ID)
}

// ContextLine is one SO line with its returnable remainder.
type ContextLine struct {
	ProductID   int64
	VariantID   int64
	ProductCode string
	ProductName string
	UOM         string
	OrderedQty  float64
	Delivered   float64
	Returned    float64
	Remaining   float64
}

// ReturnContext describes what can still be returned for an SO: per-line
// remainders plus the branch's locations for the location picker.
func (s *Service) ReturnContext(ctx context.Context, branchID, soID int64) (map[string]any, error) {
	so, err := s.sales.GetByID(ctx, soID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if so == nil || so.BranchID != branchID {
		return nil, apperror.NotFound("Sales order")
	}
	committedAll, err := s.repo.CommittedQtyBySO(ctx, soID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	lines := make([]any, 0, len(so.Items))
	for _, l := range so.Items {
		delivered, err := s.delivery.DeliveredQty(ctx, soID, l.ProductID, l.VariantID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		returned, err := s.repo.ReturnedQty(ctx, soID, l.ProductID, l.VariantID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		// Drafts hold their share too, so the form never offers goods that a
		// pending return already claims.
		remaining := delivered - committedAll[lineKey(l.ProductID, l.VariantID)]
		if remaining < 0 {
			remaining = 0
		}
		lines = append(lines, map[string]any{
			"productId": l.ProductID, "variantId": l.VariantID,
			"productCode": l.ProductCode, "productName": l.ProductName,
			"uom": l.UOM, "uomFactor": l.UOMFactor, "orderedQty": l.Qty,
			"delivered": delivered, "returned": returned, "remaining": remaining,
		})
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
		"salesOrderId": so.ID, "orderNumber": so.Number, "status": so.Status,
		"lines": lines, "locations": locItems,
	}, nil
}

// ReturnPreview is the dry-run result of Create: identical numbers, nothing
// persisted, no document number consumed.
type ReturnPreview struct {
	Items            []*contracts.Item
	ReplacementItems []*contracts.ReplacementItem
	Subtotal         int64
	DiscountTotal    int64
	TaxTotal         int64
	Total            int64
	Mode             string
	// Settlement outlook (K-04d): the numbers confirmation will arrive at,
	// from the same rules and the same offset helper AddSettlement/Confirm use.
	// Difference is replacement - returned for an exchange (0 otherwise).
	Difference int64
	// OwedToCustomer / ChargeToCustomer: value owed to the customer vs value
	// the customer owes (exchange costing more).
	OwedToCustomer   int64
	ChargeToCustomer int64
	// OrderOutstanding is what the customer still owes on the source order
	// before this return; ReceivableReduction is the part the automatic offset
	// will take off it at confirmation.
	OrderOutstanding    int64
	ReceivableReduction int64
	// ExternalRemaining is what can then be refunded or kept as customer
	// balance; AllowedSettlementTypes are the types legal for the rest.
	ExternalRemaining      int64
	AllowedSettlementTypes []string
	HasParty               bool
}

// PreviewReturn runs the exact Create builder without writing anything.
func (s *Service) PreviewReturn(ctx context.Context, branchID, soID int64, mode string, lines []LineInput, replacements []ReplacementInput) (*ReturnPreview, error) {
	so, err := s.sales.GetByID(ctx, soID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if so == nil || so.BranchID != branchID {
		return nil, apperror.NotFound("Sales order")
	}
	if so.Status != salescontracts.StatusConfirmed && so.Status != salescontracts.StatusCompleted {
		return nil, apperror.Conflict("Retur hanya untuk order terkonfirmasi")
	}
	if len(lines) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: "wajib diisi"}})
	}
	var verr error
	if mode, verr = normalizeMode(mode, len(replacements)); verr != nil {
		return nil, verr
	}
	computed, err := s.computeReturn(ctx, branchID, soID, so, lines)
	if err != nil {
		return nil, err
	}
	replItems, err := s.priceReplacements(ctx, branchID, so, replacements)
	if err != nil {
		return nil, err
	}
	var replTotal int64
	for _, ri := range replItems {
		replTotal += ri.LineTotal
	}
	difference := int64(0)
	if mode == contracts.ReturnModeExchange {
		difference = replTotal - computed.total
	}
	pos := positionOf(mode, computed.total, difference, 0, nil, so.PartyID != 0)
	var outstanding, reduction int64
	if s.payments != nil && so.PartyID != 0 && pos.Owed > 0 {
		paid, perr := s.payments.PaidForOrders(ctx, paymentcontracts.OrderSales, []int64{so.ID})
		if perr != nil {
			return nil, apperror.Internal(perr)
		}
		if outstanding = so.GrandTotal - paid[so.ID]; outstanding < 0 {
			outstanding = 0
		}
		reduction = offsetAmount(so.GrandTotal, paid[so.ID], pos.Owed)
	}
	pos.Reduction = reduction
	return &ReturnPreview{
		Items: computed.items, ReplacementItems: replItems,
		Subtotal: computed.subtotal, DiscountTotal: computed.discountTotal,
		TaxTotal: computed.taxTotal, Total: computed.total, Mode: mode,
		Difference: difference, OwedToCustomer: pos.Owed, ChargeToCustomer: pos.Charge,
		OrderOutstanding: outstanding, ReceivableReduction: reduction,
		ExternalRemaining: pos.ExternalRemaining(), AllowedSettlementTypes: pos.Allowed(),
		HasParty: pos.HasParty,
	}, nil
}

// errPaymentsNotWired is a programmer error: main.go must call SetPayments.
var errPaymentsNotWired = errors.New("payments client not wired")

func lineKey(productID, variantID int64) string {
	return strconv.FormatInt(productID, 10) + "/" + strconv.FormatInt(variantID, 10)
}

// priceReplacements validates + prices exchange replacement lines once at
// creation from the live catalog (a new sale in effect). Member pricing
// does not apply. Each line needs an active product + known variant +
// resolvable UOM + valid branch location.
func (s *Service) priceReplacements(ctx context.Context, branchID int64, so *salescontracts.SalesOrder, in []ReplacementInput) ([]*contracts.ReplacementItem, error) {
	out := make([]*contracts.ReplacementItem, 0, len(in))
	for i, line := range in {
		tag := "pengganti baris " + strconv.Itoa(i+1) + ": "
		if line.Qty <= 0 {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "replacementItems", Message: tag + "qty harus lebih dari 0"}})
		}
		p, err := s.products.GetByID(ctx, line.ProductID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if p == nil || p.Status != "active" {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "replacementItems", Message: tag + "produk sudah diarsipkan"}})
		}
		known := false
		for _, v := range p.Variants {
			if v.ID == line.VariantID {
				known = true
				break
			}
		}
		if !known {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "replacementItems", Message: tag + "varian sudah diarsipkan"}})
		}
		uom := strings.TrimSpace(line.UOM)
		factor, _, err := s.products.ResolveUOM(ctx, line.ProductID, uom)
		if err != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "replacementItems", Message: tag + "satuan tidak dikenal"}})
		}
		if p.Tracked && p.Type != "jasa" {
			if err := s.stock.CheckLocation(ctx, branchID, line.LocationID); err != nil {
				return nil, err
			}
		}
		// The replacement is a new sale: catalog price (or an explicit price
		// no lower than the minimum), taxed like the source order. Catalog and
		// minimum prices are per sales unit; another unit is priced by its
		// base-qty ratio (the same rule as sales order lines).
		unitPrice := money.PriceForUnit(p.SellingPrice, factor, p.SalesFactor)
		if line.UnitPrice != nil {
			if *line.UnitPrice < 0 {
				return nil, apperror.Validation("", []apperror.FieldError{{Field: "replacementItems", Message: tag + "harga tidak boleh negatif"}})
			}
			unitPrice = *line.UnitPrice
		}
		net, base, tax, total := money.LineAmounts(line.Qty, unitPrice, 0, 0, so.TaxType, so.TaxRate).Split()
		if p.MinSellingPrice > 0 && net < money.MinLineNet(p.MinSellingPrice, line.Qty*factor, p.SalesFactor) {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "replacementItems", Message: tag + "harga di bawah harga minimum"}})
		}
		out = append(out, &contracts.ReplacementItem{
			ProductID: line.ProductID, VariantID: line.VariantID, LocationID: line.LocationID,
			UOM: uom, UOMFactor: factor, Qty: line.Qty, QtyBase: line.Qty * factor,
			UnitPrice: unitPrice, TaxBase: base, TaxAmount: tax, LineTotal: total,
		})
	}
	return out, nil
}

// ListResult is a paginated return page.
type ListResult struct {
	Returns []*contracts.SalesReturn
	Total   int64
}

// List searches returns of a branch, newest first.
func (s *Service) List(ctx context.Context, branchID, soID int64, status string, page, limit int) (*ListResult, error) {
	total, err := s.repo.Count(ctx, branchID, soID, status)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	returns, err := s.repo.List(ctx, branchID, soID, status, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if returns == nil {
		returns = []*contracts.SalesReturn{}
	}
	return &ListResult{Returns: returns, Total: total}, nil
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
func (s *Service) ActiveReturnCount(ctx context.Context, soID int64) (int64, error) {
	n, err := s.repo.ActiveReturnCount(ctx, soID)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return n, nil
}

// OrderIDsByReturns maps return id -> originating sales order id.
func (s *Service) OrderIDsByReturns(ctx context.Context, ids []int64) (map[int64]int64, error) {
	out, err := s.repo.OrderIDsByReturns(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}

// ReturnedByProduct implements the contract (see there).
func (s *Service) ReturnedByProduct(ctx context.Context, branchID int64, from, to string) (map[int64]float64, error) {
	out, err := s.repo.ReturnedByProduct(ctx, branchID, from, to)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}

// NetAdjustment implements the contract (see there).
func (s *Service) NetAdjustment(ctx context.Context, branchID int64, from, to string) (int64, error) {
	adj, err := s.repo.NetAdjustment(ctx, branchID, from, to)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return adj, nil
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

// resolveReplacementSettlement checks a pay_now surcharge against the return
// and fills the default amount (K-04e): only an exchange whose replacement
// costs MORE (charge kind) can collect, never more than the surcharge still
// open (what payments already allocated to the return counts as collected).
func (s *Service) resolveReplacementSettlement(ctx context.Context, ret *contracts.SalesReturn, in *deliverycontracts.ReplacementSettlement) (*deliverycontracts.ReplacementSettlement, error) {
	if s.payments == nil {
		return nil, apperror.Internal(errPaymentsNotWired)
	}
	proj := projection(ret)
	if proj.Kind != contracts.KindCharge || proj.Total <= 0 {
		return nil, apperror.Conflict("Tambah bayar hanya untuk retur tukar yang barang penggantinya lebih mahal dari barang retur")
	}
	paid, err := s.payments.PaidForOrders(ctx, paymentcontracts.OrderSalesReturn, []int64{ret.ID})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	open := proj.Total - paid[ret.ID]
	if open <= 0 {
		return nil, apperror.Conflict("Selisih tukar sudah lunas")
	}
	out := *in
	if out.Amount == 0 {
		out.Amount = open
	}
	if out.Amount < 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "settlement.amount", Message: "harus lebih dari 0"}})
	}
	if out.Amount > open {
		return nil, apperror.Conflict("Melebihi selisih yang harus dibayar customer (sisa " + strconv.FormatInt(open, 10) + ")")
	}
	return &out, nil
}
