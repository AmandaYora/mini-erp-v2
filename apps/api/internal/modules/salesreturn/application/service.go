package application

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/modules/salesreturn/contracts"
	"mini-erp/internal/modules/salesreturn/infrastructure"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/doclock"
	"mini-erp/internal/shared/timeutil"
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
	payments paymentcontracts.PaymentClient
	audit    auditcontracts.AuditClient
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
		OrderID: ret.SalesOrderID, Total: ret.Total, Status: ret.Status,
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
			OrderID: ret.SalesOrderID, Total: ret.Total, Status: ret.Status,
		})
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
}

// Create drafts a return against a confirmed/completed SO. Quantities cap at
// delivered-minus-returned per (product, variant) aggregate — never at the
// ordered figure, so undelivered goods cannot come back as stock.
func (s *Service) Create(ctx context.Context, actorID, branchID, soID int64, returnDate, notes, mode string, lines []LineInput, replacements []ReplacementInput) (*contracts.SalesReturn, error) {
	var out *contracts.SalesReturn
	err := doclock.Lock("so:"+strconv.FormatInt(soID, 10), func() error {
		res, err := s.createLocked(ctx, actorID, branchID, soID, returnDate, notes, mode, lines, replacements)
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
	uoms    map[string]float64 // uom -> factor
	price   map[string]int64   // uom -> snapshot unit price (last wins)
	pct     map[string]float64 // uom -> discount pct (same line as price)
	nominal map[string]int64   // uom -> summed discount nominal
	soQty   map[string]float64 // uom -> summed SO qty (prorate base)
	qtyBase float64
}

func aggregateSO(so *salescontracts.SalesOrder) map[string]*soAggregate {
	soAgg := map[string]*soAggregate{}
	for _, l := range so.Items {
		key := lineKey(l.ProductID, l.VariantID)
		agg, ok := soAgg[key]
		if !ok {
			agg = &soAggregate{uoms: map[string]float64{}, price: map[string]int64{},
				pct: map[string]float64{}, nominal: map[string]int64{}, soQty: map[string]float64{}}
			soAgg[key] = agg
		}
		agg.uoms[l.UOM] = l.UOMFactor
		agg.price[l.UOM] = l.UnitPrice
		agg.pct[l.UOM] = l.DiscountPct
		agg.nominal[l.UOM] += l.DiscountNominal
		agg.soQty[l.UOM] += l.Qty
		agg.qtyBase += l.QtyBase
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
	items := make([]*contracts.Item, 0, len(lines))
	var subtotal, discountTotal, taxTotal int64
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
		delivered, err := s.delivery.DeliveredQty(ctx, soID, line.ProductID, line.VariantID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		returned, err := s.repo.ReturnedQty(ctx, soID, line.ProductID, line.VariantID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		qtyBase := line.Qty * factor
		if delivered-returned-qtyBase < -1e-9 {
			return nil, apperror.Conflict("Melebihi yang bisa diretur (" + tag + "sisa " + strconv.FormatFloat(delivered-returned, 'f', -1, 64) + ")")
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
		// in, tax flips with it. Lines that cannot match a source line are
		// rejected above; never fall back to live catalog prices (that would
		// refund today's price instead of the price actually paid).
		nominalPart := prorate(agg.nominal[uom], line.Qty, agg.soQty[uom])
		net, base, tax := lineEconomics(line.Qty, unitPrice, agg.pct[uom], nominalPart, so.TaxType, so.TaxRate)
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

func (s *Service) createLocked(ctx context.Context, actorID, branchID, soID int64, returnDate, notes, mode string, lines []LineInput, replacements []ReplacementInput) (*contracts.SalesReturn, error) {
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
	replItems, err := s.priceReplacements(ctx, branchID, replacements)
	if err != nil {
		return nil, err
	}
	replStatus := contracts.ReplacementNotRequired
	if mode == contracts.ReturnModeExchange {
		replStatus = contracts.ReplacementPending
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
	}, replItems, actorID)
	if err != nil {
		return nil, dberr.Map(err)
	}
	return s.GetByID(ctx, id)
}

// Confirm restocks every line (MoveIn). Draft-only; the source SO keeps its
// status — history stands.
func (s *Service) Confirm(ctx context.Context, actorID, branchID, id int64) (*contracts.SalesReturn, error) {
	ret, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if ret.Status != contracts.StatusDraft {
		return nil, apperror.Conflict("Hanya draft yang bisa dikonfirmasi")
	}
	var out *contracts.SalesReturn
	err = doclock.Lock("so:"+strconv.FormatInt(ret.SalesOrderID, 10), func() error {
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
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "sales_returns.confirm", Entity: "sales_return", EntityID: out.ID, BranchID: branchID, ActorID: actorID, Note: out.Number})
	return out, nil
}

func (s *Service) confirmLocked(ctx context.Context, actorID, branchID int64, ret *contracts.SalesReturn) (*contracts.SalesReturn, error) {
	full, err := s.enrich(ctx, ret)
	if err != nil {
		return nil, err
	}
	var moved []*contracts.Item
	for _, it := range full.Items {
		if err := s.stock.MoveIn(ctx, branchID, it.ProductID, it.VariantID, it.LocationID,
			it.QtyBase, "sales_return", full.ID, actorID); err != nil {
			for _, done := range moved {
				_ = s.stock.MoveOut(ctx, branchID, done.ProductID, done.VariantID, done.LocationID,
					done.QtyBase, "sales_return_rollback", full.ID, actorID)
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
// out) — the KI-99/KI-101 cancel path that legacy promised but never built.
func (s *Service) Cancel(ctx context.Context, actorID, branchID, id int64) (*contracts.SalesReturn, error) {
	ret, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	// Exchange returns with a moving replacement cannot vanish: the
	// replacement shipment would strand without its parent document.
	if ret.ReturnMode == contracts.ReturnModeExchange &&
		(ret.ReplacementDeliveryStatus == contracts.ReplacementDispatched ||
			ret.ReplacementDeliveryStatus == contracts.ReplacementConfirmed) {
		return nil, apperror.Conflict("Retur tukar yang barang penggantinya sudah dikirim tidak dapat dibatalkan")
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
		if used, err := s.payments.HasActiveAllocations(ctx, paymentcontracts.OrderSalesReturn, ret.ID); err != nil {
			return nil, apperror.Internal(err)
		} else if used {
			return nil, apperror.Conflict("Retur sudah ada refund, batalkan pembayaran dulu")
		}
		var out *contracts.SalesReturn
		err := doclock.Lock("so:"+strconv.FormatInt(ret.SalesOrderID, 10), func() error {
			full, err := s.enrich(ctx, ret)
			if err != nil {
				return err
			}
			for _, it := range full.Items {
				if err := s.stock.MoveOut(ctx, branchID, it.ProductID, it.VariantID, it.LocationID,
					it.QtyBase, "sales_return_cancel", full.ID, actorID); err != nil {
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
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "sales_returns.cancel", Entity: "sales_return", EntityID: out.ID, BranchID: branchID, ActorID: actorID, Note: out.Number})
		return out, nil
	default:
		return nil, apperror.Conflict("Retur sudah dibatalkan")
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "sales_returns.cancel", Entity: "sales_return", EntityID: ret.ID, BranchID: branchID, ActorID: actorID, Note: ret.Number})
	return s.GetByID(ctx, id)
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
	items := make([]deliverycontracts.ReplacementLine, 0, len(full.ReplacementItems))
	for i, it := range full.ReplacementItems {
		tag := "pengganti baris " + strconv.Itoa(i+1) + ": "
		p, err := s.products.GetByID(ctx, it.ProductID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if p == nil || p.Status != "active" {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "replacementItems", Message: tag + "produk sudah diarsipkan"}})
		}
		if p.PurchasePrice <= 0 {
			return nil, apperror.Conflict("Produk " + p.Code + " tidak punya harga modal di katalog — isi harga beli dulu agar HPP tidak nol")
		}
		bal, err := s.stock.GetBalance(ctx, branchID, it.ProductID, it.VariantID, it.LocationID)
		if err != nil {
			return nil, err
		}
		if bal.Available() < it.QtyBase {
			return nil, apperror.Conflict("Stok tersedia tidak mencukupi (" + tag + "sisa " + strconv.FormatFloat(bal.Available(), 'f', -1, 64) + ")")
		}
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
			Items: items,
		})
		if err != nil {
			return err
		}
		if err := s.repo.SetReplacementStatus(ctx, ret.ID, contracts.ReplacementDispatched, actorID); err != nil {
			return apperror.Internal(err)
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
		confirmed, err := s.delivery.ConfirmReplacement(ctx, actorID, branchID, deliveryID, in)
		if err != nil {
			return err
		}
		if err := s.repo.SetReplacementStatus(ctx, ret.ID, contracts.ReplacementConfirmed, actorID); err != nil {
			return apperror.Internal(err)
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
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "sales_returns.settle", Entity: "sales_return", EntityID: ret.ID, BranchID: branchID, ActorID: actorID, Note: ret.Number})
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
		remaining := delivered - returned
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
	replItems, err := s.priceReplacements(ctx, branchID, replacements)
	if err != nil {
		return nil, err
	}
	return &ReturnPreview{
		Items: computed.items, ReplacementItems: replItems,
		Subtotal: computed.subtotal, DiscountTotal: computed.discountTotal,
		TaxTotal: computed.taxTotal, Total: computed.total, Mode: mode,
	}, nil
}

// errPaymentsNotWired is a programmer error: main.go must call SetPayments.
var errPaymentsNotWired = errors.New("payments client not wired")

func lineKey(productID, variantID int64) string {
	return strconv.FormatInt(productID, 10) + "/" + strconv.FormatInt(variantID, 10)
}

// lineEconomics mirrors finance lineEconomics (builders.go): gross − pct −
// nominal, then tax base/tax per type. Duplicated deliberately — each module
// owns its money math per the monorepo standard, so the two can never drift
// through a shared helper edited for one side's needs.
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

// priceReplacements validates + prices exchange replacement lines once at
// creation from the live catalog (a new sale in effect). Member pricing
// does not apply. Each line needs an active product + known variant +
// resolvable UOM + valid branch location.
func (s *Service) priceReplacements(ctx context.Context, branchID int64, in []ReplacementInput) ([]*contracts.ReplacementItem, error) {
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
		if err := s.stock.CheckLocation(ctx, branchID, line.LocationID); err != nil {
			return nil, err
		}
		unitPrice := p.SellingPrice
		out = append(out, &contracts.ReplacementItem{
			ProductID: line.ProductID, VariantID: line.VariantID, LocationID: line.LocationID,
			UOM: uom, UOMFactor: factor, Qty: line.Qty, QtyBase: line.Qty * factor,
			UnitPrice: unitPrice, LineTotal: int64(math.Round(line.Qty * float64(unitPrice))),
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

// OrderIDsByReturns maps return id -> originating sales order id.
func (s *Service) OrderIDsByReturns(ctx context.Context, ids []int64) (map[int64]int64, error) {
	out, err := s.repo.OrderIDsByReturns(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}
