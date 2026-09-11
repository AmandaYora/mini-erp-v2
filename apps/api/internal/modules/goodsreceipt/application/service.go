package application

import (
	"context"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/goodsreceipt/contracts"
	"mini-erp/internal/modules/goodsreceipt/infrastructure"
	productcontracts "mini-erp/internal/modules/product/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/doclock"
	"mini-erp/internal/shared/timeutil"
)

// Service implements contracts.GoodsReceiptClient plus receipt administration.
// Receipts post stock inflows immediately (MoveIn per line) and complete the
// PO once every line is fully received.
type Service struct {
	repo       *infrastructure.Repository
	purchasing purchasingcontracts.PurchaseOrderClient
	products   productcontracts.ProductClient
	stock      stockcontracts.StockClient
	audit      auditcontracts.AuditClient
}

// NewService wires goodsreceipt use cases. All cross-module links are contracts.
func NewService(
	repo *infrastructure.Repository,
	purchasing purchasingcontracts.PurchaseOrderClient,
	products productcontracts.ProductClient,
	stock stockcontracts.StockClient,
	audit auditcontracts.AuditClient,
) *Service {
	return &Service{repo: repo, purchasing: purchasing, products: products, stock: stock, audit: audit}
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

// ReceivedQty sums received base qty for one PO line position.
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

// Create validates against the PO (confirmed only, no over-receive), records
// the receipt, posts one MoveIn per line, and completes the PO when fully
// received.
//
// Concurrency: serialized per PO (doclock) so two simultaneous receipts
// cannot both read the same remaining figure and jointly overrun it (KI-92).
// The PO is re-read inside the lock — the pre-lock read only decides the
// fast 404/conflict path.
//
// Atomicity: remaining-qty math and the will-complete decision happen before
// any write. If a MoveIn fails afterwards, already-posted lines are reversed
// (saga compensation) so balances stay correct; the immutable receipt row
// remains as the intent record for the operator.
func (s *Service) Create(ctx context.Context, actorID, branchID, poID int64, receivedAt, notes string, lines []LineInput) (*contracts.GoodsReceipt, error) {
	var out *contracts.GoodsReceipt
	err := doclock.Lock("po:"+strconv.FormatInt(poID, 10), func() error {
		res, err := s.createLocked(ctx, actorID, branchID, poID, receivedAt, notes, lines)
		if err != nil {
			return err
		}
		out = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "goods_receipts.create", Entity: "goods_receipt", EntityID: out.ID, BranchID: branchID, ActorID: actorID, Note: out.OrderNumber})
	return out, nil
}

func (s *Service) createLocked(ctx context.Context, actorID, branchID, poID int64, receivedAt, notes string, lines []LineInput) (*contracts.GoodsReceipt, error) {
	po, err := s.purchasing.GetByID(ctx, poID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if po == nil || po.BranchID != branchID {
		return nil, apperror.NotFound("Purchase order")
	}
	if po.Status != purchasingcontracts.StatusConfirmed {
		return nil, apperror.Conflict("Order harus dikonfirmasi dulu")
	}
	if strings.TrimSpace(receivedAt) != "" {
		if _, err := timeutil.ParseDateInput(strings.TrimSpace(receivedAt)); err != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "receivedAt", Message: "format tanggal harus YYYY-MM-DD"}})
		}
	} else {
		receivedAt = timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	}
	if len(lines) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: "wajib diisi"}})
	}
	// Aggregate PO lines by (product, variant): a PO may carry the same
	// product in two UOMs, and base quantities add up across them.
	type ordered struct {
		pid, vid int64
		uoms     map[string]float64 // uom -> factor
		qtyBase  float64
	}
	poAgg := map[string]*ordered{}
	for _, l := range po.Items {
		key := lineKey(l.ProductID, l.VariantID)
		agg, ok := poAgg[key]
		if !ok {
			agg = &ordered{pid: l.ProductID, vid: l.VariantID, uoms: map[string]float64{}}
			poAgg[key] = agg
		}
		agg.uoms[l.UOM] = l.UOMFactor
		agg.qtyBase += l.QtyBase
	}
	type ready struct {
		productID, variantID, locationID int64
		qty, qtyBase                     float64
		uom                              string
		factor                           float64
	}
	var readyLines []ready
	items := make([]*contracts.Item, 0, len(lines))
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
		received, err := s.repo.ReceivedQty(ctx, poID, line.ProductID, line.VariantID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		qtyBase := line.Qty * factor
		if received+qtyBase > agg.qtyBase+1e-9 {
			return nil, apperror.Conflict("Melebihi sisa PO (" + tag + "sisa " + strconv.FormatFloat(agg.qtyBase-received, 'f', -1, 64) + ")")
		}
		if err := s.stock.CheckLocation(ctx, branchID, line.LocationID); err != nil {
			return nil, err
		}
		// Product archived after PO confirmation must not post stock.
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
		readyLines = append(readyLines, ready{
			productID: line.ProductID, variantID: line.VariantID,
			locationID: line.LocationID, qty: line.Qty, qtyBase: qtyBase,
			uom: uom, factor: factor,
		})
		items = append(items, &contracts.Item{
			ProductID: line.ProductID, VariantID: line.VariantID, LocationID: line.LocationID,
			UOM: uom, UOMFactor: factor,
			Qty: line.Qty, QtyBase: qtyBase,
		})
	}
	// Decide completion before writing: after this receipt, is every
	// (product, variant) aggregate fully received?
	incoming := map[string]float64{}
	for _, l := range readyLines {
		incoming[lineKey(l.productID, l.variantID)] += l.qtyBase
	}
	willComplete := true
	for key, agg := range poAgg {
		received, err := s.repo.ReceivedQty(ctx, poID, agg.pid, agg.vid)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if received+incoming[key]+1e-9 < agg.qtyBase {
			willComplete = false
			break
		}
	}
	id, err := s.repo.CreateReceipt(ctx, &contracts.GoodsReceipt{
		BranchID: branchID, PurchaseOrderID: poID,
		ReceivedAt: strings.TrimSpace(receivedAt), Notes: notes, Items: items,
	}, actorID)
	if err != nil {
		return nil, dberr.Map(err)
	}
	type moved struct {
		productID, variantID, locationID int64
		qtyBase                          float64
	}
	var done []moved
	for _, l := range readyLines {
		if err := s.stock.MoveIn(ctx, branchID, l.productID, l.variantID, l.locationID,
			l.qtyBase, "goods_receipt", id, actorID); err != nil {
			for _, d := range done {
				_ = s.stock.MoveOut(ctx, branchID, d.productID, d.variantID, d.locationID,
					d.qtyBase, "goods_receipt_rollback", id, actorID)
			}
			return nil, err
		}
		done = append(done, moved{productID: l.productID, variantID: l.variantID, locationID: l.locationID, qtyBase: l.qtyBase})
	}
	if willComplete {
		if err := s.purchasing.SetStatus(ctx, branchID, poID, purchasingcontracts.StatusCompleted, actorID); err != nil {
			return nil, apperror.Internal(err)
		}
	}
	return s.GetByID(ctx, id)
}

func lineKey(productID, variantID int64) string {
	return strconv.FormatInt(productID, 10) + "/" + strconv.FormatInt(variantID, 10)
}

// ListResult is a paginated receipt page.
type ListResult struct {
	Receipts []*contracts.GoodsReceipt
	Total    int64
}

// List searches receipts of a branch, newest first.
func (s *Service) List(ctx context.Context, branchID, poID int64, page, limit int) (*ListResult, error) {
	total, err := s.repo.Count(ctx, branchID, poID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	receipts, err := s.repo.List(ctx, branchID, poID, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if receipts == nil {
		receipts = []*contracts.GoodsReceipt{}
	}
	return &ListResult{Receipts: receipts, Total: total}, nil
}

// ListRecent lists receipts newest-first for the derived posting queue.
// One bounded read; the journal index decides what is still unposted.
func (s *Service) ListRecent(ctx context.Context, branchID int64, limit int) ([]*contracts.GoodsReceipt, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	res, err := s.List(ctx, branchID, 0, 1, limit)
	if err != nil {
		return nil, err
	}
	return res.Receipts, nil
}
