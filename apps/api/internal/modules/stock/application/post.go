package application

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"

	"mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/modules/stock/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
)

// position identifies one stock position within a document posting.
type position struct{ product, variant, location int64 }

type postLine struct {
	position
	qty            float64
	unitCost       float64
	reservationKey string
}

// legRef is the document-wide data every leg of a posting shares.
type legRef struct {
	refType string
	refID   int64
	actorID int64
	movedAt time.Time
	note    string
}

// leg is one direction of a document at one branch (a transfer has two).
type leg struct {
	branchID  int64
	direction string
	lines     []*postLine
}

// mergeLines validates a request's lines and sums those on the same position.
func mergeLines(direction string, in []contracts.PostLine) ([]*postLine, error) {
	if len(in) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "lines", Message: "minimal satu baris"}})
	}
	merged := map[position]*postLine{}
	order := []position{}
	for i, l := range in {
		if l.Qty <= 0 {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "qty", Message: "harus lebih dari 0 (baris " + strconv.Itoa(i+1) + ")"}})
		}
		if l.ReservationKey != "" && direction != "out" {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "reservationKey", Message: "hanya untuk gerakan keluar"}})
		}
		pos := position{l.ProductID, l.VariantID, l.LocationID}
		if cur, ok := merged[pos]; ok {
			if cur.reservationKey != l.ReservationKey {
				return nil, apperror.Validation("", []apperror.FieldError{{Field: "reservationKey", Message: "kunci reservasi berbeda pada posisi yang sama"}})
			}
			// Two costed lines on one position blend into one weighted unit cost.
			if cur.unitCost > 0 || l.UnitCost > 0 {
				cur.unitCost = (cur.unitCost*cur.qty + l.UnitCost*l.Qty) / (cur.qty + l.Qty)
			}
			cur.qty += l.Qty
			continue
		}
		merged[pos] = &postLine{position: pos, qty: l.Qty, unitCost: l.UnitCost, reservationKey: l.ReservationKey}
		order = append(order, pos)
	}
	out := make([]*postLine, 0, len(order))
	for _, pos := range order {
		out = append(out, merged[pos])
	}
	return out, nil
}

// validateLegs checks product/location liveness OUTSIDE the transaction, so
// no pool connection is held (with locks) while master data is read.
func (s *Service) validateLegs(ctx context.Context, legs []leg, allowDamaged bool) error {
	checkedProduct := map[[2]int64]bool{}
	checkedLocation := map[[2]int64]bool{}
	var productIDs []int64
	for _, lg := range legs {
		for _, l := range lg.lines {
			productIDs = append(productIDs, l.product)
		}
	}
	products, err := s.loadProducts(ctx, productIDs)
	if err != nil {
		return err
	}
	for _, lg := range legs {
		for _, l := range lg.lines {
			key := [2]int64{l.product, l.variant}
			if !checkedProduct[key] {
				p, _, err := pickLiveVariant(products[l.product], l.variant)
				if err != nil {
					return err
				}
				if p.Type == "jasa" {
					return apperror.Validation("", []apperror.FieldError{{Field: "productId", Message: "produk jasa tidak memiliki stok: " + p.Name}})
				}
				checkedProduct[key] = true
			}
			lk := [2]int64{lg.branchID, l.location}
			if !checkedLocation[lk] {
				if _, err := s.resolveLocationKind(ctx, lg.branchID, l.location, allowDamaged); err != nil {
					return err
				}
				checkedLocation[lk] = true
			}
		}
	}
	return nil
}

// entry is one position of one leg, ready to be locked and written.
type entry struct {
	branchID  int64
	direction string
	line      *postLine
}

// applyTx writes every leg's positions inside tx. All positions of all legs
// are sorted together (branch, product, variant, location, direction), so any
// two documents with overlapping positions lock them in the same order and
// cannot deadlock. A position that already carries this document's movement
// is skipped (replay). Returns posted/replayed counts.
func (s *Service) applyTx(ctx context.Context, tx *sql.Tx, ref legRef, legs []leg) (contracts.PostResult, error) {
	var entries []entry
	for _, lg := range legs {
		for _, l := range lg.lines {
			entries = append(entries, entry{branchID: lg.branchID, direction: lg.direction, line: l})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.branchID != b.branchID {
			return a.branchID < b.branchID
		}
		if a.line.product != b.line.product {
			return a.line.product < b.line.product
		}
		if a.line.variant != b.line.variant {
			return a.line.variant < b.line.variant
		}
		if a.line.location != b.line.location {
			return a.line.location < b.line.location
		}
		return a.direction < b.direction
	})
	var result contracts.PostResult
	for _, e := range entries {
		replayed, err := s.postPosition(ctx, tx, ref, e)
		if err != nil {
			return result, err
		}
		if replayed {
			result.Replayed++
		} else {
			result.Posted++
		}
	}
	return result, nil
}

// Post writes a document's movements atomically and idempotently (F2a-0).
//
//  1. Validate everything without side effects (liveness, locations).
//  2. Sum lines per position; lock positions in sorted order (no deadlock).
//  3. One transaction, retried on 1213/1205, covers all positions. A position
//     that already carries this document's movement is skipped (Replayed).
//
// Stock shortage rolls the whole transaction back: no partial movement.
func (s *Service) Post(ctx context.Context, req contracts.PostRequest) (*contracts.PostResult, error) {
	return s.postWithHook(ctx, req, nil)
}

// postWithHook is Post with an optional hook that runs in the same transaction
// after the movements are written (the audit row co-commits, ADR-0011).
func (s *Service) postWithHook(ctx context.Context, req contracts.PostRequest, hook func(ctx context.Context, tx *sql.Tx, res contracts.PostResult) error) (*contracts.PostResult, error) {
	if req.Direction != "in" && req.Direction != "out" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "direction", Message: "harus in atau out"}})
	}
	if req.RefType == "" || req.RefID <= 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "refId", Message: "dokumen sumber wajib diisi"}})
	}
	lines, err := mergeLines(req.Direction, req.Lines)
	if err != nil {
		return nil, err
	}
	legs := []leg{{branchID: req.BranchID, direction: req.Direction, lines: lines}}
	if err := s.validateLegs(ctx, legs, req.AllowDamagedLocation); err != nil {
		return nil, err
	}
	ref := legRef{refType: req.RefType, refID: req.RefID, actorID: req.ActorID, movedAt: req.MovedAt, note: req.Note}
	var result contracts.PostResult
	err = dberr.RetryTx(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		var err error
		result, err = s.applyTx(ctx, tx, ref, legs)
		if err != nil || hook == nil {
			return err
		}
		return hook(ctx, tx, result)
	})
	if err != nil {
		return nil, asAppError(err)
	}
	return &result, nil
}

// postPosition writes one position inside tx. Returns replayed=true when the
// document already wrote this movement.
func (s *Service) postPosition(ctx context.Context, tx *sql.Tx, ref legRef, e entry) (bool, error) {
	l := e.line
	if err := s.repo.EnsureBalanceTx(ctx, tx, e.branchID, l.product, l.variant, l.location); err != nil {
		return false, err
	}
	// Sweep expired holds BEFORE loading the hold we may consume, so a hold
	// that just expired is seen as expired (its reserved qty already returned).
	if err := s.repo.ExpireReservationsTx(ctx, tx, e.branchID, l.product, l.variant, l.location); err != nil {
		return false, err
	}
	var res *infrastructure.Reservation
	if l.reservationKey != "" {
		r, err := s.repo.ReservationByKeyTx(ctx, tx, l.reservationKey)
		if err != nil {
			return false, err
		}
		res = r
	}
	bal, err := s.repo.GetBalanceTx(ctx, tx, e.branchID, l.product, l.variant, l.location)
	if err != nil {
		return false, err
	}
	// Every writer of this position is serialized by the balance row lock, so
	// this check is race-free; the unique key is only the backstop.
	exists, err := s.repo.MovementExistsTx(ctx, tx, ref.refType, ref.refID, l.product, l.variant, l.location, e.direction)
	if err != nil {
		return false, err
	}
	if exists {
		return true, nil
	}

	delta := l.qty
	var reservedDelta float64
	useRes := false
	if e.direction == "out" {
		delta = -l.qty
		// A hold only counts while it is live, matches the position and has
		// enough left; an expired or missing hold falls back to plain
		// availability so a sale never fails merely because the TTL passed.
		useRes = res != nil && res.Status == "active" && res.HasLocation &&
			res.BranchID == e.branchID && res.ProductID == l.product && res.VariantID == l.variant &&
			res.LocationID == l.location && res.Remaining >= l.qty
		if useRes {
			if bal.OnHand < l.qty {
				return false, apperror.Conflict(shortage(l, "Stok fisik tidak mencukupi"))
			}
			reservedDelta = -l.qty
		} else if bal.Available() < l.qty {
			return false, apperror.Conflict(shortage(l, "Stok tersedia tidak mencukupi"))
		}
	}

	// Movement first: a duplicate key here (a racing writer on a stale
	// snapshot) means the document already posted — replay, nothing changed yet.
	err = s.repo.InsertMovementTx(ctx, tx, infrastructure.MovementRow{
		BranchID: e.branchID, ProductID: l.product, VariantID: l.variant, LocationID: l.location,
		Direction: e.direction, MovementType: e.direction, Qty: l.qty,
		Before: bal.OnHand, After: bal.OnHand + delta, UnitCost: costHint(e.direction, l.unitCost),
		RefType: ref.refType, RefID: ref.refID, Notes: ref.note, ActorID: ref.actorID, MovedAt: ref.movedAt,
	})
	if isDuplicateKey(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if res != nil && useRes {
		remaining := res.Remaining - l.qty
		status := "active"
		if remaining <= 0 {
			remaining, status = 0, "consumed"
		}
		if err := s.repo.UpdateReservationTx(ctx, tx, res.ID, remaining, status); err != nil {
			return false, err
		}
	}
	if err := s.repo.AddBalanceTx(ctx, tx, e.branchID, l.product, l.variant, l.location, delta, reservedDelta); err != nil {
		return false, err
	}
	return false, nil
}

// costHint keeps an explicit unit cost only for inflows (an outflow is costed
// by finance from the running average, never from the caller).
func costHint(direction string, unit float64) float64 {
	if direction == "in" {
		return unit
	}
	return 0
}

func shortage(l *postLine, msg string) string {
	return msg + " (produk " + strconv.FormatInt(l.product, 10) + ", lokasi " + strconv.FormatInt(l.location, 10) + ")"
}

func isDuplicateKey(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

// asAppError keeps typed application errors and wraps anything else as 500.
func asAppError(err error) error {
	var ae *apperror.AppError
	if errors.As(err, &ae) {
		return ae
	}
	return apperror.Internal(err)
}

// reverseTx writes the opposite movement for every movement of a document at
// the branch that has none yet, inside tx. Reversing an "in" requires the
// stock to still be available.
func (s *Service) reverseTx(ctx context.Context, tx *sql.Tx, branchID int64, refType string, refID, actorID int64) (contracts.PostResult, error) {
	var result contracts.PostResult
	orig, err := s.repo.MovementsByRefTx(ctx, tx, branchID, refType, refID)
	if err != nil {
		return result, err
	}
	for _, m := range orig {
		opposite := "in"
		if m.Direction == "in" {
			opposite = "out"
		}
		if err := s.repo.EnsureBalanceTx(ctx, tx, branchID, m.ProductID, m.VariantID, m.LocationID); err != nil {
			return result, err
		}
		if err := s.repo.ExpireReservationsTx(ctx, tx, branchID, m.ProductID, m.VariantID, m.LocationID); err != nil {
			return result, err
		}
		bal, err := s.repo.GetBalanceTx(ctx, tx, branchID, m.ProductID, m.VariantID, m.LocationID)
		if err != nil {
			return result, err
		}
		done, err := s.repo.MovementExistsTx(ctx, tx, refType+contracts.CancelSuffix, refID, m.ProductID, m.VariantID, m.LocationID, opposite)
		if err != nil {
			return result, err
		}
		if done {
			result.Replayed++
			continue
		}
		delta := m.Qty
		if opposite == "out" {
			if bal.Available() < m.Qty {
				return result, apperror.Conflict("Stok sudah terpakai, dokumen tidak dapat dibatalkan (produk " +
					strconv.FormatInt(m.ProductID, 10) + ")")
			}
			delta = -m.Qty
		}
		if err := s.repo.InsertMovementTx(ctx, tx, infrastructure.MovementRow{
			BranchID: branchID, ProductID: m.ProductID, VariantID: m.VariantID, LocationID: m.LocationID,
			Direction: opposite, MovementType: opposite, Qty: m.Qty,
			Before: bal.OnHand, After: bal.OnHand + delta,
			RefType: refType + contracts.CancelSuffix, RefID: refID, ActorID: actorID,
		}); err != nil {
			if isDuplicateKey(err) {
				result.Replayed++
				continue
			}
			return result, err
		}
		if err := s.repo.AddBalanceTx(ctx, tx, branchID, m.ProductID, m.VariantID, m.LocationID, delta, 0); err != nil {
			return result, err
		}
		result.Posted++
	}
	return result, nil
}

// Reverse undoes every movement of a document once (see reverseTx), as one
// retried transaction.
func (s *Service) Reverse(ctx context.Context, branchID int64, refType string, refID, actorID int64) (*contracts.PostResult, error) {
	if refType == "" || refID <= 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "refId", Message: "dokumen sumber wajib diisi"}})
	}
	var result contracts.PostResult
	err := dberr.RetryTx(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		var err error
		result, err = s.reverseTx(ctx, tx, branchID, refType, refID, actorID)
		return err
	})
	if err != nil {
		return nil, asAppError(err)
	}
	return &result, nil
}

// HasMovements reports whether the document wrote any movement.
func (s *Service) HasMovements(ctx context.Context, branchID int64, refType string, refID int64) (bool, error) {
	ok, err := s.repo.HasRefMovements(ctx, branchID, refType, refID)
	if err != nil {
		return false, apperror.Internal(err)
	}
	return ok, nil
}
