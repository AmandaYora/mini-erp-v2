package application

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"mini-erp/internal/modules/goodsreceipt/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
)

var errPaymentsNotWired = errors.New("goodsreceipt payments not wired")

// outstanding is what the order still owes the supplier.
func (s *Service) outstanding(ctx context.Context, po *purchasingcontracts.PurchaseOrder) (int64, error) {
	if s.payments == nil {
		return 0, apperror.Internal(errPaymentsNotWired)
	}
	paid, err := s.payments.PaidForOrders(ctx, paymentcontracts.OrderPurchase, []int64{po.ID})
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return po.GrandTotal - paid[po.ID], nil
}

// completesOrder: after counts base qty received (pending + posted, including
// the receipt in hand) per position; the order completes when every position
// is covered.
func completesOrder(po *purchasingcontracts.PurchaseOrder, after map[string]float64) bool {
	ordered := map[string]float64{}
	for _, l := range po.Items {
		ordered[lineKey(l.ProductID, l.VariantID)] += l.QtyBase
	}
	for key, want := range ordered {
		if after[key]+1e-9 < want {
			return false
		}
	}
	return true
}

// planSettlement applies the payment-terms rules to a receipt:
//
//   - prepaid: the order must already be fully paid; a settlement makes no sense.
//   - net: no settlement.
//   - cod: only the receipt that COMPLETES the order settles it; if money is
//     still owed then, pay_now | switch_to_net is mandatory.
func (s *Service) planSettlement(ctx context.Context, po *purchasingcontracts.PurchaseOrder, after map[string]float64, in *Settlement) (*Settlement, error) {
	switch po.PaymentTerms {
	case "prepaid":
		if in != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "settlement", Message: "tidak berlaku untuk order prabayar"}})
		}
		out, err := s.outstanding(ctx, po)
		if err != nil {
			return nil, err
		}
		if out > 0 {
			return nil, apperror.Conflict("Order prabayar belum lunas; penerimaan barang belum dapat dicatat")
		}
		return nil, nil
	case "cod":
		if !completesOrder(po, after) {
			return nil, nil // not the final receipt: nothing to settle yet
		}
		out, err := s.outstanding(ctx, po)
		if err != nil {
			return nil, err
		}
		if out <= 0 {
			return nil, nil
		}
		if in == nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "settlement",
				Message: "order tunai belum lunas: pilih pay_now (bayar sekarang) atau switch_to_net (jadikan tempo)"}})
		}
		switch in.Mode {
		case SettlePayNow:
			c := *in
			if len(strings.TrimSpace(c.Notes)) > 255 {
				return nil, apperror.Validation("", []apperror.FieldError{{Field: "settlement.notes", Message: "maksimal 255 karakter"}})
			}
			if c.Method == "" {
				c.Method = paymentcontracts.MethodCash
			}
			return &c, nil
		case SettleSwitchToNet:
			if strings.TrimSpace(in.DueDate) == "" {
				return nil, apperror.Validation("", []apperror.FieldError{{Field: "settlement.dueDate", Message: "wajib diisi untuk switch_to_net"}})
			}
			if _, err := timeutil.ParseDateInput(strings.TrimSpace(in.DueDate)); err != nil {
				return nil, apperror.Validation("", []apperror.FieldError{{Field: "settlement.dueDate", Message: "format tanggal harus YYYY-MM-DD"}})
			}
			c := *in
			return &c, nil
		default:
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "settlement.mode", Message: "harus pay_now atau switch_to_net"}})
		}
	default: // net
		if in != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "settlement", Message: "hanya untuk order tunai (COD)"}})
		}
		return nil, nil
	}
}

// applySettlement executes a planned settlement. Both effects are idempotent.
// A pay_now returns the payment it recorded (or replayed) so the caller can
// attach a transfer slip to it afterwards (GR-02).
func (s *Service) applySettlement(ctx context.Context, actorID, branchID int64, po *purchasingcontracts.PurchaseOrder, rec *contracts.GoodsReceipt, st *Settlement) (*contracts.SettlementPayment, error) {
	if st == nil {
		return nil, nil
	}
	switch st.Mode {
	case SettlePayNow:
		if s.payments == nil {
			return nil, apperror.Internal(errPaymentsNotWired)
		}
		notes := "Pelunasan saat penerimaan " + rec.Number
		if extra := strings.TrimSpace(st.Notes); extra != "" {
			notes += ": " + extra
		}
		p, _, err := s.payments.RecordForOrder(ctx, paymentcontracts.RecordInput{
			ActorID: actorID, BranchID: branchID, OrderType: paymentcontracts.OrderPurchase, OrderID: po.ID,
			Method: st.Method, PaidAt: st.PaymentDate, Reference: st.Reference,
			CashAccountID: st.CashAccountID,
			ClientRef:     "gr:" + strconv.FormatInt(rec.ID, 10),
			Notes:         notes,
		})
		if err != nil || p == nil {
			return nil, err
		}
		return &contracts.SettlementPayment{ID: p.ID, Number: p.Number}, nil
	case SettleSwitchToNet:
		return nil, s.purchasing.SwitchToNet(ctx, branchID, po.ID, st.DueDate, actorID)
	}
	return nil, nil
}
