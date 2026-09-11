package application

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
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

// GetReturn resolves the payment-facing projection, or nil when missing.
func (s *Service) GetReturn(ctx context.Context, id int64) (*contracts.Return, error) {
	ret, err := s.repo.GetReturn(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if ret == nil {
		return nil, nil
	}
	return &contracts.Return{
		ID: ret.ID, Number: ret.Number, Branch: ret.BranchID,
		OrderID: ret.PurchaseOrderID, Total: ret.Total, Status: ret.Status,
	}, nil
}

// ListByBranch lists the branch's returns for balance computation.
func (s *Service) ListByBranch(ctx context.Context, branchID int64) ([]*contracts.Return, error) {
	returns, err := s.repo.List(ctx, branchID, 0, "", 10000, 0)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]*contracts.Return, 0, len(returns))
	for _, ret := range returns {
		out = append(out, &contracts.Return{
			ID: ret.ID, Number: ret.Number, Branch: ret.BranchID,
			OrderID: ret.PurchaseOrderID, Total: ret.Total, Status: ret.Status,
		})
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
func (s *Service) Create(ctx context.Context, actorID, branchID, poID int64, returnDate, notes string, lines []LineInput) (*contracts.PurchaseReturn, error) {
	var out *contracts.PurchaseReturn
	err := doclock.Lock("po:"+strconv.FormatInt(poID, 10), func() error {
		res, err := s.createLocked(ctx, actorID, branchID, poID, returnDate, notes, lines)
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
	uoms    map[string]float64 // uom -> factor
	price   map[string]int64   // uom -> snapshot unit price (last wins)
	pct     map[string]float64 // uom -> discount pct (same line as price)
	nominal map[string]int64   // uom -> summed discount nominal
	poQty   map[string]float64 // uom -> summed PO qty (prorate base)
	qtyBase float64
}

func aggregatePO(po *purchasingcontracts.PurchaseOrder) map[string]*poAggregate {
	poAgg := map[string]*poAggregate{}
	for _, l := range po.Items {
		key := lineKey(l.ProductID, l.VariantID)
		agg, ok := poAgg[key]
		if !ok {
			agg = &poAggregate{uoms: map[string]float64{}, price: map[string]int64{},
				pct: map[string]float64{}, nominal: map[string]int64{}, poQty: map[string]float64{}}
			poAgg[key] = agg
		}
		agg.uoms[l.UOM] = l.UOMFactor
		agg.price[l.UOM] = l.UnitPrice
		agg.pct[l.UOM] = l.DiscountPct
		agg.nominal[l.UOM] += l.DiscountNominal
		agg.poQty[l.UOM] += l.Qty
		agg.qtyBase += l.QtyBase
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
	items := make([]*contracts.Item, 0, len(lines))
	var subtotal, discountTotal, taxTotal int64
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
		received, err := s.goodsreceipt.ReceivedQty(ctx, poID, line.ProductID, line.VariantID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		returned, err := s.repo.ReturnedQty(ctx, poID, line.ProductID, line.VariantID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		qtyBase := line.Qty * factor
		if received-returned-qtyBase < -1e-9 {
			return nil, apperror.Conflict("Melebihi yang bisa diretur (" + tag + "sisa " + strconv.FormatFloat(received-returned, 'f', -1, 64) + ")")
		}
		if err := s.stock.CheckLocation(ctx, branchID, line.LocationID); err != nil {
			return nil, err
		}
		p, err := s.products.GetByID(ctx, line.ProductID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
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
		unitPrice := agg.price[uom]
		// D1: value follows the source line economics — discount pro-rates
		// in, tax flips with it. Unmatched lines are rejected above; never
		// fall back to live catalog prices.
		nominalPart := prorate(agg.nominal[uom], line.Qty, agg.poQty[uom])
		net, base, tax := lineEconomics(line.Qty, unitPrice, agg.pct[uom], nominalPart, po.TaxType, po.TaxRate)
		gross := int64(math.Round(line.Qty * float64(unitPrice)))
		lineTotal := base + tax
		subtotal += base
		discountTotal += gross - net
		taxTotal += tax
		items = append(items, &contracts.Item{
			ProductID: line.ProductID, VariantID: line.VariantID, LocationID: line.LocationID,
			UOM: uom, UOMFactor: factor, Qty: line.Qty, QtyBase: qtyBase,
			UnitPrice: unitPrice, DiscountPct: agg.pct[uom], DiscountNominal: nominalPart,
			TaxBase: base, TaxAmount: tax, LineTotal: lineTotal,
		})
	}
	return &computedReturn{items: items, subtotal: subtotal,
		discountTotal: discountTotal, taxTotal: taxTotal, total: subtotal + taxTotal}, nil
}

func (s *Service) createLocked(ctx context.Context, actorID, branchID, poID int64, returnDate, notes string, lines []LineInput) (*contracts.PurchaseReturn, error) {
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
	} else {
		returnDate = timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
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
		ReturnDate: strings.TrimSpace(returnDate),
		Subtotal:   subtotal, DiscountTotal: discountTotal, TaxTotal: taxTotal,
		TaxType: po.TaxType, TaxRate: po.TaxRate,
		Total: total, Notes: notes, Items: items,
	}, actorID)
	if err != nil {
		return nil, dberr.Map(err)
	}
	return s.GetByID(ctx, id)
}

// Confirm ships the goods back out (MoveOut). Draft-only.
func (s *Service) Confirm(ctx context.Context, actorID, branchID, id int64) (*contracts.PurchaseReturn, error) {
	ret, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if ret.Status != contracts.StatusDraft {
		return nil, apperror.Conflict("Hanya draft yang bisa dikonfirmasi")
	}
	var out *contracts.PurchaseReturn
	err = doclock.Lock("po:"+strconv.FormatInt(ret.PurchaseOrderID, 10), func() error {
		res, err := s.confirmLocked(ctx, actorID, branchID, ret)
		if err != nil {
			return err
		}
		out = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "purchase_returns.confirm", Entity: "purchase_return", EntityID: out.ID, BranchID: branchID, ActorID: actorID, Note: out.Number})
	return out, nil
}

func (s *Service) confirmLocked(ctx context.Context, actorID, branchID int64, ret *contracts.PurchaseReturn) (*contracts.PurchaseReturn, error) {
	full, err := s.enrich(ctx, ret)
	if err != nil {
		return nil, err
	}
	for _, it := range full.Items {
		bal, err := s.stock.GetBalance(ctx, branchID, it.ProductID, it.VariantID, it.LocationID)
		if err != nil {
			return nil, err
		}
		if bal.Available() < it.QtyBase {
			return nil, apperror.Conflict("Stok tersedia tidak mencukupi")
		}
	}
	var moved []*contracts.Item
	for _, it := range full.Items {
		if err := s.stock.MoveOut(ctx, branchID, it.ProductID, it.VariantID, it.LocationID,
			it.QtyBase, "purchase_return", full.ID, actorID); err != nil {
			for _, done := range moved {
				_ = s.stock.MoveIn(ctx, branchID, done.ProductID, done.VariantID, done.LocationID,
					done.QtyBase, "purchase_return_rollback", full.ID, actorID)
			}
			return nil, err
		}
		moved = append(moved, it)
	}
	if err := s.repo.SetStatus(ctx, full.ID, contracts.StatusConfirmed, actorID); err != nil {
		return nil, apperror.Internal(err)
	}
	return s.GetByID(ctx, full.ID)
}

// Cancel voids a draft outright, or reverses a confirmed return (stock back
// in).
func (s *Service) Cancel(ctx context.Context, actorID, branchID, id int64) (*contracts.PurchaseReturn, error) {
	ret, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	switch ret.Status {
	case contracts.StatusDraft:
		if err := s.repo.SetStatus(ctx, ret.ID, contracts.StatusCancelled, actorID); err != nil {
			return nil, apperror.Internal(err)
		}
	case contracts.StatusConfirmed:
		if s.payments == nil {
			return nil, apperror.Internal(errPaymentsNotWired)
		}
		if used, err := s.payments.HasActiveAllocations(ctx, paymentcontracts.OrderPurchaseReturn, ret.ID); err != nil {
			return nil, apperror.Internal(err)
		} else if used {
			return nil, apperror.Conflict("Retur sudah ada refund, batalkan pembayaran dulu")
		}
		var out *contracts.PurchaseReturn
		err := doclock.Lock("po:"+strconv.FormatInt(ret.PurchaseOrderID, 10), func() error {
			full, err := s.enrich(ctx, ret)
			if err != nil {
				return err
			}
			for _, it := range full.Items {
				if err := s.stock.MoveIn(ctx, branchID, it.ProductID, it.VariantID, it.LocationID,
					it.QtyBase, "purchase_return_cancel", full.ID, actorID); err != nil {
					return err
				}
			}
			if err := s.repo.SetStatus(ctx, ret.ID, contracts.StatusCancelled, actorID); err != nil {
				return apperror.Internal(err)
			}
			res, err := s.GetByID(ctx, ret.ID)
			if err != nil {
				return err
			}
			out = res
			return nil
		})
		if err != nil {
			return nil, err
		}
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "purchase_returns.cancel", Entity: "purchase_return", EntityID: out.ID, BranchID: branchID, ActorID: actorID, Note: out.Number})
		return out, nil
	default:
		return nil, apperror.Conflict("Retur sudah dibatalkan")
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "purchase_returns.cancel", Entity: "purchase_return", EntityID: ret.ID, BranchID: branchID, ActorID: actorID, Note: ret.Number})
	return s.GetByID(ctx, id)
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
	var settled int64
	for _, st := range full.Settlements {
		settled += st.Amount
	}
	if settled+in.Amount > ret.Total {
		return nil, apperror.Conflict("Melebihi sisa yang belum diselesaikan (sisa " + strconv.FormatInt(ret.Total-settled, 10) + ")")
	}
	if _, err := s.repo.InsertSettlement(ctx, ret.ID, &contracts.Settlement{
		Type: in.Type, Date: date, Amount: in.Amount,
		PaymentMethod:   strings.TrimSpace(in.PaymentMethod),
		ReferenceNumber: strings.TrimSpace(in.ReferenceNumber),
		Notes:           strings.TrimSpace(in.Notes),
	}, actorID); err != nil {
		return nil, apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "purchase_returns.settle", Entity: "purchase_return", EntityID: ret.ID, BranchID: branchID, ActorID: actorID, Note: ret.Number})
	return s.GetByID(ctx, ret.ID)
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
		remaining := received - returned
		if remaining < 0 {
			remaining = 0
		}
		lines = append(lines, map[string]any{
			"productId": l.ProductID, "variantId": l.VariantID,
			"productCode": l.ProductCode, "productName": l.ProductName,
			"uom": l.UOM, "uomFactor": l.UOMFactor, "orderedQty": l.Qty,
			"received": received, "returned": returned, "remaining": remaining,
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
		"purchaseOrderId": po.ID, "orderNumber": po.Number, "status": po.Status,
		"lines": lines, "locations": locItems,
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
	return &ReturnPreview{
		Items:    computed.items,
		Subtotal: computed.subtotal, DiscountTotal: computed.discountTotal,
		TaxTotal: computed.taxTotal, Total: computed.total,
	}, nil
}

func lineKey(productID, variantID int64) string {
	return strconv.FormatInt(productID, 10) + "/" + strconv.FormatInt(variantID, 10)
}

// lineEconomics mirrors finance lineEconomics (builders.go): gross − pct −
// nominal, then tax base/tax per type. Duplicated deliberately — each module
// owns its money math per the monorepo standard.
func lineEconomics(qty float64, unit int64, pct float64, nominal int64, taxType string, rate float64) (net, base, tax int64) {
	gross := int64(math.Round(qty * float64(unit)))
	net = gross - int64(math.Round(float64(gross)*pct/100)) - nominal
	switch taxType {
	case "exclude":
		base = net
		tax = int64(math.Round(float64(net) * rate / 100))
	case "include":
		if rate > 0 {
			base = int64(math.Round(float64(net) / (1 + rate/100)))
		} else {
			base = net
		}
		tax = net - base
	default:
		base = net
	}
	return net, base, tax
}

// prorate scales a nominal amount by the returned share of its source line.
// Returns 0 for a zero-qty source rather than dividing by zero.
func prorate(amount int64, part, whole float64) int64 {
	if whole <= 0 {
		return 0
	}
	return int64(math.Round(float64(amount) * part / whole))
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
