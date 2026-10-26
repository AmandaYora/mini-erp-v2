package application

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	"mini-erp/internal/modules/goodsreceipt/contracts"
	"mini-erp/internal/modules/goodsreceipt/infrastructure"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/doclock"
	"mini-erp/internal/shared/timeutil"
	"mini-erp/internal/shared/validator"
)

// Service implements contracts.GoodsReceiptClient plus receipt administration.
//
// A receipt is recorded as `pending`, then its stock inflow is posted
// (idempotent per document) and the status flips to `posted` with a CAS. A
// failure between those steps leaves a visible `pending` receipt that
// Complete (finish) or Discard (undo) resolves; nothing is compensated behind
// the operator's back. The PO completes once every position is fully POSTED.
type Service struct {
	repo       *infrastructure.Repository
	purchasing purchasingcontracts.PurchaseOrderClient
	products   productcontracts.ProductClient
	stock      stockcontracts.StockClient
	branches   branchcontracts.BranchClient
	audit      auditcontracts.AuditClient
	payments   Payments
	// users resolves the receiving user's display name (GR-01).
	users usercontracts.UserClient
}

// Payments is the payment surface goodsreceipt consumes for order settlement.
type Payments interface {
	RecordForOrder(ctx context.Context, in paymentcontracts.RecordInput) (*paymentcontracts.Payment, bool, error)
	PaidForOrders(ctx context.Context, orderType string, ids []int64) (map[int64]int64, error)
}

// SetPayments wires the payment module (built after goodsreceipt).
func (s *Service) SetPayments(p Payments) { s.payments = p }

// Settlement modes for a cash-terms (COD) order that the receipt completes.
const (
	SettlePayNow      = "pay_now"
	SettleSwitchToNet = "switch_to_net"
)

// Settlement is how the supplier is paid when the receipt completes a COD order.
type Settlement struct {
	Mode        string
	Method      string
	PaymentDate string
	Reference   string
	DueDate     string
	// Notes is the operator's free text on the payment (GR-02, pay_now).
	Notes string
	// CashAccountID is the company rekening the transfer/check leaves from
	// (ADR-0026); 0 = the default.
	CashAccountID int64
}

// NewService wires goodsreceipt use cases. All cross-module links are contracts.
func NewService(
	repo *infrastructure.Repository,
	purchasing purchasingcontracts.PurchaseOrderClient,
	products productcontracts.ProductClient,
	stock stockcontracts.StockClient,
	branches branchcontracts.BranchClient,
	audit auditcontracts.AuditClient,
	users usercontracts.UserClient,
) *Service {
	return &Service{repo: repo, purchasing: purchasing, products: products,
		stock: stock, branches: branches, audit: audit, users: users}
}

// GetByID resolves a receipt with lines and order number, or nil when missing.
func (s *Service) GetByID(ctx context.Context, id int64) (*contracts.GoodsReceipt, error) {
	rec, err := s.repo.GetReceipt(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if rec == nil {
		return nil, nil
	}
	return s.enrich(ctx, rec)
}

// PurchaseOrderIDsByReceipts maps receipt id -> purchase order id in one read.
func (s *Service) PurchaseOrderIDsByReceipts(ctx context.Context, ids []int64) (map[int64]int64, error) {
	out, err := s.repo.PurchaseOrderIDsByReceipts(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}

// ListPostedAfter pages posted receipts by ascending id after a cursor.
func (s *Service) ListPostedAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*contracts.GoodsReceipt, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	recs, err := s.repo.PostedAfter(ctx, branchID, afterID, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return recs, nil
}

// ActiveReceiptCount counts the order's live (pending + posted) receipts.
func (s *Service) ActiveReceiptCount(ctx context.Context, poID int64) (int64, error) {
	n, err := s.repo.ActiveReceiptCount(ctx, poID)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return n, nil
}

// PendingReceiptCount counts the order's pending receipts.
func (s *Service) PendingReceiptCount(ctx context.Context, poID int64) (int64, error) {
	n, err := s.repo.PendingReceiptCount(ctx, poID)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return n, nil
}

// ReceivedQty sums POSTED received base qty for one PO line position.
func (s *Service) ReceivedQty(ctx context.Context, poID, productID, variantID int64) (float64, error) {
	qty, err := s.repo.ReceivedQty(ctx, poID, productID, variantID)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return qty, nil
}

func (s *Service) enrich(ctx context.Context, rec *contracts.GoodsReceipt) (*contracts.GoodsReceipt, error) {
	items, err := s.repo.ItemsByReceipt(ctx, rec.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if items == nil {
		items = []*contracts.Item{}
	}
	rec.Items = items
	po, err := s.purchasing.GetByID(ctx, rec.PurchaseOrderID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if po != nil {
		rec.OrderNumber = po.Number
	}
	rec.LineCount = len(items)
	if rec.CreatedBy > 0 && s.users != nil {
		if names, nerr := s.users.NamesByIDs(ctx, []int64{rec.CreatedBy}); nerr == nil {
			rec.ReceivedByName = names[rec.CreatedBy]
		}
	}
	return rec, nil
}

// LineInput is one receipt line.
type LineInput struct {
	ProductID  int64
	VariantID  int64
	LocationID int64
	UOM        string
	Qty        float64
}

// CreateOption sets optional receipt header data.
type CreateOption func(*createOptions)

type createOptions struct {
	supplierDeliveryNumber string
	clientRef              string
	settlement             *Settlement
}

// WithSettlement carries the COD settlement for the receipt that completes the order.
func WithSettlement(st *Settlement) CreateOption {
	return func(o *createOptions) { o.settlement = st }
}

// WithSupplierDeliveryNumber records the supplier's delivery-note number.
func WithSupplierDeliveryNumber(no string) CreateOption {
	return func(o *createOptions) { o.supplierDeliveryNumber = strings.TrimSpace(no) }
}

// WithClientRef makes Create idempotent: repeating a request with the same
// reference resumes that receipt instead of recording a second one.
func WithClientRef(ref string) CreateOption {
	return func(o *createOptions) { o.clientRef = strings.TrimSpace(ref) }
}

// Create validates against the PO (confirmed only, no over-receive across
// pending + posted receipts), records the receipt as pending, posts the stock
// inflow and flips it to posted, completing the PO when fully received.
//
// Serialized per PO (doclock, KI-92); the PO is re-read inside the lock.
func (s *Service) Create(ctx context.Context, actorID, branchID, poID int64, receivedAt, notes string, lines []LineInput, opts ...CreateOption) (*contracts.GoodsReceipt, error) {
	var o createOptions
	for _, opt := range opts {
		opt(&o)
	}
	var out *contracts.GoodsReceipt
	var replayed bool
	err := doclock.Lock("po:"+strconv.FormatInt(poID, 10), func() error {
		res, rep, err := s.createLocked(ctx, actorID, branchID, poID, receivedAt, notes, lines, o)
		if err != nil {
			return err
		}
		out, replayed = res, rep
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !replayed {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "goods_receipts.create", Entity: "goods_receipt", EntityID: out.ID, BranchID: branchID, ActorID: actorID, Note: out.OrderNumber})
	}
	return out, nil
}

func (s *Service) createLocked(ctx context.Context, actorID, branchID, poID int64, receivedAt, notes string, lines []LineInput, o createOptions) (*contracts.GoodsReceipt, bool, error) {
	if len(o.clientRef) > 64 {
		return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "clientRef", Message: "maksimal 64 karakter"}})
	}
	if len(o.supplierDeliveryNumber) > 60 {
		return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "supplierDeliveryNumber", Message: "maksimal 60 karakter"}})
	}
	po, err := s.purchasing.GetByID(ctx, poID)
	if err != nil {
		return nil, false, apperror.Internal(err)
	}
	if po == nil || po.BranchID != branchID {
		return nil, false, apperror.NotFound("Purchase order")
	}
	// Same client reference = same receipt: resume it, never record twice.
	if o.clientRef != "" {
		existing, err := s.repo.ReceiptByClientRef(ctx, branchID, o.clientRef)
		if err != nil {
			return nil, false, apperror.Internal(err)
		}
		if existing != nil {
			if existing.PurchaseOrderID != poID {
				return nil, false, apperror.Conflict("Referensi klien sudah dipakai untuk order lain")
			}
			out, err := s.finish(ctx, actorID, branchID, existing, o.settlement)
			return out, true, err
		}
	}
	if po.Status != purchasingcontracts.StatusConfirmed {
		return nil, false, apperror.Conflict("Order harus dikonfirmasi dulu")
	}
	today := timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	if strings.TrimSpace(receivedAt) != "" {
		d, err := timeutil.ParseDateInput(strings.TrimSpace(receivedAt))
		if err != nil {
			return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "receivedAt", Message: "format tanggal harus YYYY-MM-DD"}})
		}
		if d.In(timeutil.Jakarta).Format("2006-01-02") > today {
			return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "receivedAt", Message: "tidak boleh di masa depan"}})
		}
	} else {
		receivedAt = today
	}
	if len(lines) == 0 {
		return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: "wajib diisi"}})
	}
	// Aggregate PO lines by (product, variant): a PO may carry the same
	// product in two UOMs, and base quantities add up across them.
	type ordered struct {
		uoms    map[string]float64 // uom -> factor
		qtyBase float64
	}
	poAgg := map[string]*ordered{}
	for _, l := range po.Items {
		key := lineKey(l.ProductID, l.VariantID)
		agg, ok := poAgg[key]
		if !ok {
			agg = &ordered{uoms: map[string]float64{}}
			poAgg[key] = agg
		}
		agg.uoms[l.UOM] = l.UOMFactor
		agg.qtyBase += l.QtyBase
	}
	// Quota = ordered - (pending + posted receipts) - lines already accepted
	// in this request (P6): pending receipts hold their share.
	committed, err := s.repo.QtyByPO(ctx, poID, contracts.StatusPending, contracts.StatusPosted)
	if err != nil {
		return nil, false, apperror.Internal(err)
	}
	used := map[string]float64{}
	seen := map[string]bool{}
	// Service / non-stock lines carry no location and never move stock.
	untracked := map[string]bool{}
	for _, l := range po.Items {
		if !l.StockTracked {
			untracked[lineKey(l.ProductID, l.VariantID)] = true
		}
	}
	items := make([]*contracts.Item, 0, len(lines))
	var defaultLoc int64
	// One batch read for the whole document (F4-7), not a GetByID per line.
	productIDs := make([]int64, 0, len(lines))
	for _, l := range lines {
		productIDs = append(productIDs, l.ProductID)
	}
	loadedProducts, perr := s.products.GetByIDs(ctx, productIDs)
	if perr != nil {
		return nil, false, apperror.Internal(perr)
	}
	for i, line := range lines {
		tag := "baris " + strconv.Itoa(i+1) + ": "
		if line.Qty <= 0 {
			return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "qty harus lebih dari 0"}})
		}
		key := lineKey(line.ProductID, line.VariantID)
		agg, ok := poAgg[key]
		if !ok {
			return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "produk tidak ada di PO"}})
		}
		uom := strings.TrimSpace(line.UOM)
		factor, ok := agg.uoms[uom]
		if !ok {
			return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "satuan harus sama dengan PO"}})
		}
		dupKey := key + "/" + uom
		if seen[dupKey] {
			return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "produk dan satuan yang sama sudah ada di baris lain"}})
		}
		seen[dupKey] = true
		qtyBase := line.Qty * factor
		taken := committed[key] + used[key]
		if taken+qtyBase > agg.qtyBase+1e-9 {
			return nil, false, apperror.Conflict("Melebihi sisa PO (" + tag + "sisa " + strconv.FormatFloat(agg.qtyBase-taken, 'f', -1, 64) + ")")
		}
		used[key] += qtyBase
		if !untracked[key] {
			// GR-02: a line without a location lands in the branch's default one
			// (legacy resolveReceiveStockLocation); the form may still demand a pick.
			if line.LocationID == 0 {
				if defaultLoc == 0 {
					def, derr := s.defaultLocationID(ctx, branchID)
					if derr != nil {
						return nil, false, derr
					}
					defaultLoc = def
				}
				line.LocationID = defaultLoc
			}
			if err := s.stock.CheckLocation(ctx, branchID, line.LocationID); err != nil {
				return nil, false, err
			}
		}
		// Product archived after PO confirmation must not post stock.
		p := loadedProducts[line.ProductID]
		if p == nil || p.Status != "active" {
			return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "produk sudah diarsipkan"}})
		}
		known := false
		for _, v := range p.Variants {
			if v.ID == line.VariantID {
				known = true
				break
			}
		}
		if !known {
			return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "varian sudah diarsipkan"}})
		}
		items = append(items, &contracts.Item{
			ProductID: line.ProductID, VariantID: line.VariantID, LocationID: line.LocationID,
			UOM: uom, UOMFactor: factor, Qty: line.Qty, QtyBase: qtyBase,
		})
	}
	// Payment terms (prepaid must be paid; COD settles on the completing
	// receipt) are validated BEFORE anything is recorded.
	after := map[string]float64{}
	for k, v := range committed {
		after[k] = v
	}
	for k, v := range used {
		after[k] += v
	}
	if _, err := s.planSettlement(ctx, po, after, o.settlement); err != nil {
		return nil, false, err
	}
	// Allocated last, right before the insert, so a validation failure never
	// burns a document number.
	number, err := s.branches.NextDocumentNumber(ctx, branchID, branchcontracts.DocGoodsReceipt)
	if err != nil {
		return nil, false, apperror.Internal(err)
	}
	id, err := s.repo.CreateReceipt(ctx, &contracts.GoodsReceipt{
		Number: number, BranchID: branchID, PurchaseOrderID: poID,
		ReceivedAt: strings.TrimSpace(receivedAt), Notes: notes,
		SupplierDeliveryNumber: o.supplierDeliveryNumber, ClientRef: o.clientRef, Items: items,
	}, actorID)
	if err != nil {
		return nil, false, dberr.Map(err)
	}
	rec, err := s.repo.GetReceipt(ctx, id)
	if err != nil || rec == nil {
		return nil, false, apperror.Internal(errors.New("receipt vanished after insert"))
	}
	out, err := s.finish(ctx, actorID, branchID, rec, o.settlement)
	return out, false, err
}

// defaultLocationID is the branch's system default (non-damaged) location, the
// receiving target of a line that names none (GR-02).
func (s *Service) defaultLocationID(ctx context.Context, branchID int64) (int64, error) {
	locs, err := s.stock.ListLocations(ctx, branchID, "active")
	if err != nil {
		return 0, err
	}
	for _, l := range locs {
		if l.IsSystem && l.Kind != stockcontracts.KindDamaged {
			return l.ID, nil
		}
	}
	return 0, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: "lokasi stok default belum dikonfigurasi untuk cabang ini"}})
}

// finish drives a receipt to its final state: post stock (idempotent), flip
// pending -> posted (CAS), then complete the PO. Every step is safe to repeat,
// so it serves Create, Complete and a client-ref replay alike. A failure
// before the flip leaves the receipt pending and tells the operator so.
func (s *Service) finish(ctx context.Context, actorID, branchID int64, rec *contracts.GoodsReceipt, in *Settlement) (*contracts.GoodsReceipt, error) {
	if rec.Status == contracts.StatusVoid {
		return nil, apperror.Conflict("Penerimaan sudah dibatalkan (void)")
	}
	full, err := s.enrich(ctx, rec)
	if err != nil {
		return nil, err
	}
	var pay *contracts.SettlementPayment
	if rec.Status == contracts.StatusPending {
		untracked := map[string]bool{}
		if po, err := s.purchasing.GetByID(ctx, rec.PurchaseOrderID); err == nil && po != nil {
			for _, l := range po.Items {
				if !l.StockTracked {
					untracked[lineKey(l.ProductID, l.VariantID)] = true
				}
			}
		}
		postLines := make([]stockcontracts.PostLine, 0, len(full.Items))
		for _, it := range full.Items {
			if untracked[lineKey(it.ProductID, it.VariantID)] {
				continue
			}
			postLines = append(postLines, stockcontracts.PostLine{
				ProductID: it.ProductID, VariantID: it.VariantID, LocationID: it.LocationID, Qty: it.QtyBase,
			})
		}
		req := stockcontracts.PostRequest{
			BranchID: branchID, Direction: "in", RefType: stockcontracts.RefGoodsReceipt, RefID: rec.ID,
			ActorID: actorID, Lines: postLines, MovedAt: movedAt(rec.ReceivedAt),
		}
		if len(postLines) > 0 {
			if _, err := s.stock.Post(ctx, req); err != nil {
				return nil, pendingError(rec.Number, err)
			}
		}
		// Money effect: idempotent (client reference gr:<id> / CAS on the terms),
		// validated against the quantities this receipt brings the order to.
		if po, perr := s.purchasing.GetByID(ctx, rec.PurchaseOrderID); perr == nil && po != nil {
			all, qerr := s.repo.QtyByPO(ctx, rec.PurchaseOrderID, contracts.StatusPending, contracts.StatusPosted)
			if qerr != nil {
				return nil, apperror.Internal(qerr)
			}
			plan, err := s.planSettlement(ctx, po, all, in)
			if err != nil {
				return nil, pendingError(rec.Number, err)
			}
			var aerr error
			if pay, aerr = s.applySettlement(ctx, actorID, branchID, po, rec, plan); aerr != nil {
				return nil, pendingError(rec.Number, aerr)
			}
		}
		won, err := s.repo.Transition(ctx, rec.ID, []string{contracts.StatusPending}, contracts.StatusPosted)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if !won {
			cur, err := s.repo.GetReceipt(ctx, rec.ID)
			if err != nil {
				return nil, apperror.Internal(err)
			}
			if cur == nil || cur.Status != contracts.StatusPosted {
				return nil, apperror.Conflict("Status penerimaan berubah, muat ulang")
			}
		}
	}
	if err := s.completeOrderIfDone(ctx, actorID, branchID, rec.PurchaseOrderID); err != nil {
		return nil, err
	}
	out, err := s.GetByID(ctx, rec.ID)
	if out != nil {
		out.Payment = pay
	}
	return out, err
}

// pendingError tells the operator the receipt exists but its stock did not
// post, keeping the original error's category.
func pendingError(number string, err error) error {
	var ae *apperror.AppError
	if errors.As(err, &ae) && ae.Code == apperror.CodeConflict {
		return apperror.Conflict("Penerimaan " + number + " tersimpan sebagai pending, stok belum masuk: " + ae.Message +
			". Selesaikan dengan 'Posting ulang' atau batalkan dengan 'Batalkan'")
	}
	return err
}

// movedAt turns the receipt date into the effective movement time: today's
// receipts are stamped "now", earlier ones at the start of that Jakarta day.
func movedAt(receivedAt string) time.Time {
	if len(receivedAt) < 10 {
		return time.Time{}
	}
	d, err := time.ParseInLocation("2006-01-02", receivedAt[:10], timeutil.Jakarta)
	if err != nil {
		return time.Time{}
	}
	if d.Format("2006-01-02") == timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02") {
		return time.Time{}
	}
	return d
}

// completeOrderIfDone is the idempotent post-final step: when every PO
// position is fully POSTED, the PO completes. Never touches an order that is
// no longer "confirmed".
func (s *Service) completeOrderIfDone(ctx context.Context, actorID, branchID, poID int64) error {
	po, err := s.purchasing.GetByID(ctx, poID)
	if err != nil {
		return apperror.Internal(err)
	}
	if po == nil || po.Status != purchasingcontracts.StatusConfirmed {
		return nil
	}
	posted, err := s.repo.QtyByPO(ctx, poID, contracts.StatusPosted)
	if err != nil {
		return apperror.Internal(err)
	}
	ordered := map[string]float64{}
	for _, l := range po.Items {
		ordered[lineKey(l.ProductID, l.VariantID)] += l.QtyBase
	}
	for key, want := range ordered {
		if posted[key]+1e-9 < want {
			return nil
		}
	}
	if err := s.purchasing.SetStatus(ctx, branchID, poID, purchasingcontracts.StatusCompleted, actorID); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

// Complete re-runs the posting of a pending receipt (or confirms a posted one
// is fully closed out). Idempotent: stock is never written twice.
func (s *Service) Complete(ctx context.Context, actorID, branchID, id int64, in *Settlement) (*contracts.GoodsReceipt, error) {
	rec, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	var out *contracts.GoodsReceipt
	err = doclock.Lock("po:"+strconv.FormatInt(rec.PurchaseOrderID, 10), func() error {
		cur, err := s.mustOwnBranch(ctx, id, branchID)
		if err != nil {
			return err
		}
		out, err = s.finish(ctx, actorID, branchID, cur, in)
		return err
	})
	if err != nil {
		return nil, err
	}
	if rec.Status == contracts.StatusPending {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "goods_receipts.complete", Entity: "goods_receipt", EntityID: out.ID, BranchID: branchID, ActorID: actorID, Note: out.Number})
	}
	return out, nil
}

// Discard abandons a PENDING receipt: any stock it already moved is reversed
// and the receipt is voided, releasing its PO quota. Posted receipts are final
// (corrections flow through purchase returns). The reason is recorded on the
// receipt and in the audit entry; the web endpoint makes it mandatory,
// programmatic callers may pass "".
func (s *Service) Discard(ctx context.Context, actorID, branchID, id int64, reason string) (*contracts.GoodsReceipt, error) {
	reason, err := validator.Reason(reason, false)
	if err != nil {
		return nil, err
	}
	first, err := s.mustOwnBranch(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	var out *contracts.GoodsReceipt
	err = doclock.Lock("po:"+strconv.FormatInt(first.PurchaseOrderID, 10), func() error {
		rec, err := s.mustOwnBranch(ctx, id, branchID)
		if err != nil {
			return err
		}
		switch rec.Status {
		case contracts.StatusVoid:
			// Replay: already discarded (no second audit row).
			out, err = s.GetByID(ctx, rec.ID)
			return err
		case contracts.StatusPosted:
			return apperror.Conflict("Penerimaan yang sudah diposting tidak dapat dibatalkan (gunakan retur pembelian)")
		}
		if moved, err := s.stock.HasMovements(ctx, branchID, stockcontracts.RefGoodsReceipt, rec.ID); err != nil {
			return err
		} else if moved {
			if _, err := s.stock.Reverse(ctx, branchID, stockcontracts.RefGoodsReceipt, rec.ID, actorID); err != nil {
				return err
			}
		}
		won, err := s.repo.Discard(ctx, rec.ID, reason, actorID, func(ctx context.Context, tx *sql.Tx) error {
			note := rec.Number
			if reason != "" {
				note += ": " + reason
			}
			return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
				Action: "goods_receipts.discard", Entity: "goods_receipt", EntityID: rec.ID, BranchID: branchID, ActorID: actorID, Note: note,
				Before: map[string]any{"status": contracts.StatusPending},
				After:  map[string]any{"status": contracts.StatusVoid, "reason": reason},
			})
		})
		if err != nil {
			return apperror.Internal(err)
		}
		if !won {
			cur, err := s.repo.GetReceipt(ctx, rec.ID)
			if err != nil {
				return apperror.Internal(err)
			}
			if cur == nil || cur.Status != contracts.StatusVoid {
				return apperror.Conflict("Status penerimaan berubah, muat ulang")
			}
		}
		out, err = s.GetByID(ctx, rec.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// mustOwnBranch loads the receipt and denies cross-branch access as missing.
func (s *Service) mustOwnBranch(ctx context.Context, id, branchID int64) (*contracts.GoodsReceipt, error) {
	rec, err := s.repo.GetReceipt(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if rec == nil || rec.BranchID != branchID {
		return nil, apperror.NotFound("Penerimaan")
	}
	return rec, nil
}

func lineKey(productID, variantID int64) string {
	return strconv.FormatInt(productID, 10) + "/" + strconv.FormatInt(variantID, 10)
}

// ListResult is a paginated receipt page.
type ListResult struct {
	Receipts []*contracts.GoodsReceipt
	Total    int64
}

// List searches receipts of a branch (all statuses), newest first. The page is
// decorated in three grouped reads (order numbers, line counts, receiver
// names), never one query per row (GR-01).
func (s *Service) List(ctx context.Context, branchID, poID int64, page, limit int) (*ListResult, error) {
	f := infrastructure.ListFilter{BranchID: branchID, POID: poID}
	total, err := s.repo.Count(ctx, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	receipts, err := s.repo.List(ctx, f, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if receipts == nil {
		receipts = []*contracts.GoodsReceipt{}
	}
	if err := s.decorate(ctx, receipts); err != nil {
		return nil, err
	}
	return &ListResult{Receipts: receipts, Total: total}, nil
}

// decorate fills OrderNumber, LineCount and ReceivedByName of a page.
func (s *Service) decorate(ctx context.Context, receipts []*contracts.GoodsReceipt) error {
	if len(receipts) == 0 {
		return nil
	}
	recIDs := make([]int64, 0, len(receipts))
	poIDs := make([]int64, 0, len(receipts))
	userIDs := make([]int64, 0, len(receipts))
	for _, r := range receipts {
		recIDs = append(recIDs, r.ID)
		poIDs = append(poIDs, r.PurchaseOrderID)
		if r.CreatedBy > 0 {
			userIDs = append(userIDs, r.CreatedBy)
		}
	}
	counts, err := s.repo.LineCounts(ctx, recIDs)
	if err != nil {
		return apperror.Internal(err)
	}
	orders, err := s.purchasing.SummariesByIDs(ctx, poIDs)
	if err != nil {
		return apperror.Internal(err)
	}
	var names map[int64]string
	if s.users != nil && len(userIDs) > 0 {
		if names, err = s.users.NamesByIDs(ctx, userIDs); err != nil {
			return apperror.Internal(err)
		}
	}
	for _, r := range receipts {
		r.LineCount = counts[r.ID]
		if o := orders[r.PurchaseOrderID]; o != nil {
			r.OrderNumber = o.Number
		}
		r.ReceivedByName = names[r.CreatedBy]
	}
	return nil
}

// ReceivedQtyByOrders implements the contract (see there).
func (s *Service) ReceivedQtyByOrders(ctx context.Context, poIDs []int64) (map[int64]map[string]float64, error) {
	out, err := s.repo.ReceivedQtyByOrders(ctx, poIDs)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}

// PostedReceiptHints implements the contract (see there).
func (s *Service) PostedReceiptHints(ctx context.Context, poID int64) (*contracts.ReceiptHints, error) {
	h, err := s.repo.PostedHints(ctx, poID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return h, nil
}

// ListRecent lists POSTED receipts newest-first for the derived posting queue.
// One bounded read; the journal index decides what is still unposted.
func (s *Service) ListRecent(ctx context.Context, branchID int64, limit int) ([]*contracts.GoodsReceipt, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	receipts, err := s.repo.List(ctx, infrastructure.ListFilter{BranchID: branchID, PostedOnly: true}, limit, 0)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if receipts == nil {
		receipts = []*contracts.GoodsReceipt{}
	}
	return receipts, nil
}

// PostedQtyBefore implements the contract (see there).
func (s *Service) PostedQtyBefore(ctx context.Context, poID, receiptID int64) (map[string]float64, error) {
	out, err := s.repo.PostedQtyBefore(ctx, poID, receiptID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return out, nil
}
