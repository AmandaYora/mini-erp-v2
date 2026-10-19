package application

import (
	"context"
	"strconv"

	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/modules/salesreturn/contracts"
	"mini-erp/internal/shared/apperror"
)

// CostSource reads the moving-average unit cost finance holds for a position
// (late-bound: finance is built after this module).
type CostSource interface {
	AverageUnitCost(ctx context.Context, branchID, productID, variantID int64) (int64, error)
}

// SetCosts wires finance's cost reader (composition-root privilege).
func (s *Service) SetCosts(c CostSource) { s.costs = c }

// ConfirmOption tweaks Confirm.
type ConfirmOption func(*confirmOptions)

type confirmOptions struct {
	// counterSettlement collects the exchange surcharge with the counter
	// hand-over of a POS exchange (RT-02); ignored for every other return.
	counterSettlement *deliverycontracts.ReplacementSettlement
}

// WithCounterSettlement carries the pay_now surcharge of a POS exchange.
func WithCounterSettlement(st *deliverycontracts.ReplacementSettlement) ConfirmOption {
	return func(o *confirmOptions) { o.counterSettlement = st }
}

// assertReplacementCostBasis refuses replacement goods whose cost cannot be
// known (RT-04, legacy resolveReturnUnitCost): the moving average finance
// holds for the position first, then the catalog purchase price. Without
// either the cost of goods sold would book zero and the document would hang
// in "Menunggu data" at closing, so it is stopped upstream.
func (s *Service) assertReplacementCostBasis(ctx context.Context, branchID, productID, variantID int64, code string) error {
	if s.costs != nil {
		avg, err := s.costs.AverageUnitCost(ctx, branchID, productID, variantID)
		if err != nil {
			return apperror.Internal(err)
		}
		if avg > 0 {
			return nil
		}
	}
	prices, err := s.products.PurchasePricesByIDs(ctx, []int64{productID})
	if err != nil {
		return apperror.Internal(err)
	}
	if prices[productID] <= 0 {
		return apperror.Conflict("Barang pengganti " + code + " belum punya harga modal. Isi Harga Beli produk, atau posting pembelian/saldo awal produk ini dulu, sebelum menyerahkan/mengirim barang pengganti")
	}
	return nil
}

// checkReplacementReady runs the guards that must pass BEFORE any replacement
// stock leaves: every line has a cost basis and enough available stock at its
// location. Shared by the delivery-note path and the POS counter path.
func (s *Service) checkReplacementReady(ctx context.Context, branchID int64, items []*contracts.ReplacementItem) error {
	for i, it := range items {
		tag := "pengganti baris " + strconv.Itoa(i+1) + ": "
		p, err := s.products.GetByID(ctx, it.ProductID)
		if err != nil {
			return apperror.Internal(err)
		}
		if p == nil || p.Status != "active" {
			return apperror.Validation("", []apperror.FieldError{{Field: "replacementItems", Message: tag + "produk sudah diarsipkan"}})
		}
		if err := s.assertReplacementCostBasis(ctx, branchID, it.ProductID, it.VariantID, p.Code); err != nil {
			return err
		}
		bal, err := s.stock.GetBalance(ctx, branchID, it.ProductID, it.VariantID, it.LocationID)
		if err != nil {
			return err
		}
		if bal.Available() < it.QtyBase {
			return apperror.Conflict("Stok tersedia tidak mencukupi (" + tag + "sisa " + strconv.FormatFloat(bal.Available(), 'f', -1, 64) + ")")
		}
	}
	return nil
}

// isCounterExchange: an exchange on a POS-channel order hands the replacement
// over at the counter in the same step as the return (RT-02); a regular order
// ships it later through a replacement delivery note.
func isCounterExchange(so *salescontracts.SalesOrder, ret *contracts.SalesReturn) bool {
	return so != nil && so.Channel == salescontracts.ChannelPOS &&
		ret.ReturnMode == contracts.ReturnModeExchange
}

// handOverAtCounter completes a confirmed POS exchange: the replacement goods
// leave stock right now and the replacement document is closed, atomically
// from the operator's view (RT-02, legacy "tukar POS langsung diserahkan di
// konter"). The vehicle is an auto-created and auto-confirmed replacement note
// flagged as counter hand-over (no crew, no recipient paperwork): it is the
// document stock and finance already know for replacement goods, so the
// journal (new sale: revenue, tax, cost) and the stock-out come out exactly as
// on a regular exchange. Idempotent: a retry resumes whatever step is missing
// (pending → draft note → confirmed note) and a finished return is a no-op.
func (s *Service) handOverAtCounter(ctx context.Context, actorID, branchID, id int64, o confirmOptions) error {
	ret, err := s.repo.GetReturn(ctx, id)
	if err != nil || ret == nil {
		return apperror.Internal(err)
	}
	if ret.ReplacementDeliveryStatus == contracts.ReplacementConfirmed {
		return nil
	}
	if ret.ReplacementDeliveryStatus == contracts.ReplacementPending {
		if _, err := s.CreateReplacementDelivery(ctx, actorID, branchID, id, ReplacementDispatchInput{
			Notes: "Diserahkan langsung di konter POS", AtCounter: true,
		}); err != nil {
			return err
		}
	}
	// The note is the return's only draft replacement shipment of this order.
	notes, err := s.delivery.ListBySO(ctx, branchID, ret.SalesOrderID)
	if err != nil {
		return apperror.Internal(err)
	}
	var draft *deliverycontracts.DeliveryNote
	for _, n := range notes {
		if n.DocumentKind == deliverycontracts.DocumentKindReplacement && n.SalesReturnID == id && n.Status == deliverycontracts.StatusDraft {
			draft = n
			break
		}
	}
	if draft == nil {
		return apperror.Conflict("Surat jalan pengganti retur ini tidak ditemukan; muat ulang")
	}
	_, err = s.ConfirmReplacementDelivery(ctx, actorID, branchID, id, draft.ID, deliverycontracts.ReplacementConfirm{
		Settlement: o.counterSettlement,
	})
	return err
}
