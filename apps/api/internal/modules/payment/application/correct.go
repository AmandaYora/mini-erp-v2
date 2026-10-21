package application

import (
	"context"
	"errors"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	partycontracts "mini-erp/internal/modules/party/contracts"
	"mini-erp/internal/modules/payment/contracts"
	"mini-erp/internal/modules/payment/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/doclock"
	"mini-erp/internal/shared/timeutil"
)

// Correcting a payment ("Koreksi pembayaran").
//
// A payment entered with the wrong method, amount, date or reference cannot be
// fixed by cancel + re-enter when it settles a completed COD/prepaid order:
// the cancel would reopen the order (guardCancel) and the new payment first
// would overpay it. A correction does both as one step: the replacement keeps
// the party, the direction and the orders of the original, and is computed as
// if the original were already cancelled.
//
// Steps (ADR-0016), under the same lock as every money movement of the party:
//  1. one module transaction writes the replacement (client ref
//     "correct:<oldID>", corrects_payment_id = old) and CAS-cancels the
//     original with the reason, plus the audit entry — never half done here;
//  2. after commit, the original's journal is reversed through finance
//     (idempotent). The replacement is active, so the derived posting queue
//     picks it up like any payment.
// Repeating the same request replays step 1 (the client ref finds the
// replacement) and re-runs step 2, which heals a crash between the two.

// MinCorrectReasonLen is the shortest accepted correction reason.
const MinCorrectReasonLen = 5

// CorrectInput is one correction of a payment's user-entered data.
type CorrectInput struct {
	ActorID        int64
	BranchID       int64
	PaymentID      int64
	Method         string
	Amount         int64
	PaidAt         string // YYYY-MM-DD; empty or the original's day keeps the original instant
	Reference      string
	AmountTendered int64
	Notes          string // empty keeps the original's notes
	Reason         string
}

func correctionRef(oldID int64) string { return "correct:" + strconv.FormatInt(oldID, 10) }

// Correct replaces a payment with a corrected one and cancels the original.
// replayed=true when the same correction was already done.
func (s *Service) Correct(ctx context.Context, in CorrectInput) (*contracts.Payment, bool, error) {
	in.Method = strings.TrimSpace(in.Method)
	in.PaidAt = strings.TrimSpace(in.PaidAt)
	in.Reference = strings.TrimSpace(in.Reference)
	in.Notes = strings.TrimSpace(in.Notes)
	in.Reason = strings.TrimSpace(in.Reason)
	if n := len([]rune(in.Reason)); n < MinCorrectReasonLen || n > MaxReasonLen {
		return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "reason", Message: "alasan koreksi wajib diisi (5–255 karakter)"}})
	}
	old, err := s.mustOwnBranch(ctx, in.PaymentID, in.BranchID)
	if err != nil {
		return nil, false, err
	}
	allocs, err := s.repo.AllocationsByPayment(ctx, old.ID)
	if err != nil {
		return nil, false, apperror.Internal(err)
	}
	key := make([]AllocationInput, 0, len(allocs))
	for _, a := range allocs {
		key = append(key, AllocationInput{OrderType: a.OrderType, OrderID: a.OrderID})
	}
	var out *contracts.Payment
	var replayed bool
	err = doclock.Lock(lockKey(old.PartyID, key), func() error {
		res, rep, err := s.correctLocked(ctx, in)
		if err != nil {
			return err
		}
		out, replayed = res, rep
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	s.reverseJournals(ctx, in.ActorID, in.BranchID, []int64{old.ID}, "Pembayaran "+old.Number+" dikoreksi oleh "+out.Number)
	return out, replayed, nil
}

func (s *Service) correctLocked(ctx context.Context, in CorrectInput) (*contracts.Payment, bool, error) {
	old, err := s.mustOwnBranch(ctx, in.PaymentID, in.BranchID)
	if err != nil {
		return nil, false, err
	}
	ref := correctionRef(old.ID)
	if out, ok, err := s.correctionReplay(ctx, in, old, ref); ok || err != nil {
		return out, ok, err
	}
	if old.Status != "active" {
		return nil, false, apperror.Conflict("Pembayaran " + old.Number + " sudah dibatalkan; tidak ada yang bisa dikoreksi")
	}
	if old.Method == contracts.MethodOffset || old.Method == contracts.MethodAdvance || old.PairID != 0 || old.CreditSourceType != "" {
		return nil, false, apperror.Conflict("Pembayaran " + old.Number + " adalah kaki offset/pemakaian saldo dan tidak bisa dikoreksi. Batalkan pasangannya lalu catat ulang")
	}
	if in.Method != contracts.MethodCash && in.Method != contracts.MethodTransfer && in.Method != contracts.MethodCheck {
		return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "method", Message: "harus cash, transfer, atau check"}})
	}
	if in.Amount <= 0 {
		return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "amount", Message: "harus lebih dari 0"}})
	}
	if len(in.Reference) > 100 {
		return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "reference", Message: "maksimal 100 karakter"}})
	}
	if in.AmountTendered != 0 {
		if in.Method != contracts.MethodCash {
			return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "amountTendered", Message: "hanya untuk pembayaran tunai"}})
		}
		if in.AmountTendered < in.Amount {
			return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "amountTendered", Message: "uang diterima kurang dari nominal pembayaran"}})
		}
	}
	paidAt := old.PaidAt
	if in.PaidAt != "" && !strings.HasPrefix(old.PaidAt, in.PaidAt) {
		if _, err := timeutil.ParseDateInput(in.PaidAt); err != nil {
			return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "paidAt", Message: "format tanggal harus YYYY-MM-DD"}})
		}
		if timeutil.AfterTodayWIB(in.PaidAt) { // ADR-0021
			return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "paidAt", Message: "tidak boleh di masa depan"}})
		}
		paidAt = in.PaidAt
	}
	notes := in.Notes
	if notes == "" {
		notes = old.Notes
	}
	oldAllocs, err := s.repo.AllocationsByPayment(ctx, old.ID)
	if err != nil {
		return nil, false, apperror.Internal(err)
	}
	if len(oldAllocs) == 0 {
		return nil, false, apperror.Conflict("Pembayaran " + old.Number + " tidak teralokasi ke dokumen apa pun dan tidak bisa dikoreksi")
	}
	stored, err := s.reallocate(ctx, old, oldAllocs, in.Amount)
	if err != nil {
		return nil, false, err
	}
	party := &partycontracts.Party{ID: old.PartyID, Type: old.PartyType}
	pay, err := s.numberedPayment(ctx, in.BranchID, party, old.Direction, in.Amount, in.Method, paidAt, notes, stored,
		createOptions{clientRef: ref, amountTendered: in.AmountTendered, reference: in.Reference})
	if err != nil {
		return nil, false, err
	}
	pay.CorrectsPaymentID = old.ID
	id, err := s.repo.CorrectPayment(ctx, pay, old.ID, in.ActorID, in.Reason, s.auditNewID(func(id int64) auditcontracts.Entry {
		return auditcontracts.Entry{
			Action: "payments.correct", Entity: "payment", EntityID: old.ID, BranchID: in.BranchID, ActorID: in.ActorID,
			Note: old.Number + " → " + pay.Number,
			Before: map[string]any{
				"paymentId": old.ID, "number": old.Number, "status": "active",
				"method": old.Method, "amount": old.Amount, "paidAt": old.PaidAt, "reference": old.Reference,
				"allocations": oldAllocs,
			},
			After: map[string]any{
				"paymentId": id, "number": pay.Number, "originalStatus": "cancelled",
				"method": pay.Method, "amount": pay.Amount, "paidAt": pay.PaidAt, "reference": pay.Reference,
				"allocations": stored,
			},
			Metadata: map[string]any{"reason": in.Reason},
		}
	}))
	if errors.Is(err, infrastructure.ErrNotActive) {
		return nil, false, apperror.Conflict("Pembayaran " + old.Number + " baru saja dibatalkan proses lain; muat ulang halaman")
	}
	if err != nil {
		// A twin on another instance wrote the same correction first.
		if out, ok, rerr := s.correctionReplay(ctx, in, old, ref); ok || rerr != nil {
			return out, ok, rerr
		}
		return nil, false, dberr.Map(err)
	}
	out, err := s.GetByID(ctx, id)
	return out, false, err
}

// correctionReplay finds a correction already written for old. The same
// request replays it (ok=true); different data is refused: the replacement is
// the payment to correct now.
func (s *Service) correctionReplay(ctx context.Context, in CorrectInput, old *contracts.Payment, ref string) (*contracts.Payment, bool, error) {
	existing, err := s.repo.PaymentByClientRef(ctx, in.BranchID, ref)
	if err != nil {
		return nil, false, apperror.Internal(err)
	}
	if existing == nil {
		return nil, false, nil
	}
	if existing.Amount != in.Amount || existing.Method != in.Method {
		return nil, false, apperror.Conflict("Pembayaran " + old.Number + " sudah dikoreksi oleh " + existing.Number + "; bila masih salah, koreksi pembayaran " + existing.Number)
	}
	out, err := s.enrich(ctx, existing)
	return out, err == nil, err
}

// reallocate spreads the corrected amount over the original's orders in their
// original order. Each order can take what it owes once the original payment
// is set aside; more than all of them owe is refused, and so is leaving a
// completed COD/prepaid order unpaid (that needs the normal flow: a return or
// a separate payment).
func (s *Service) reallocate(ctx context.Context, old *contracts.Payment, oldAllocs []*contracts.Allocation, amount int64) ([]*contracts.Allocation, error) {
	remaining := amount
	stored := make([]*contracts.Allocation, 0, len(oldAllocs))
	for _, a := range oldAllocs {
		ref, err := s.resolveOrder(ctx, old.BranchID, old.PartyID, a.OrderType, a.OrderID)
		if err != nil {
			return nil, err
		}
		capacity := ref.outstanding + a.Amount
		take := capacity
		if take > remaining {
			take = remaining
		}
		if take < 0 {
			take = 0
		}
		if o, err := s.cashSettledOrder(ctx, a.OrderType, a.OrderID); err != nil {
			return nil, err
		} else if o != nil && take < capacity {
			return nil, apperror.Conflict("Order " + o.label + " sudah selesai; nominal koreksi " + strconv.FormatInt(amount, 10) +
				" membuatnya belum lunas (kurang " + strconv.FormatInt(capacity-take, 10) + "). Gunakan alur biasa: retur atau pembayaran terpisah")
		}
		if take > 0 {
			stored = append(stored, &contracts.Allocation{OrderType: a.OrderType, OrderID: a.OrderID, Amount: take})
			remaining -= take
		}
	}
	if remaining > 0 {
		return nil, apperror.Conflict("Melebihi sisa tagihan dokumen pembayaran ini (lebih " + strconv.FormatInt(remaining, 10) + "). Catat kelebihannya sebagai pembayaran terpisah")
	}
	return stored, nil
}
