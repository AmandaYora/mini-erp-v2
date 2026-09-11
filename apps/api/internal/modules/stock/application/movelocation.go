package application

import (
	"context"

	"mini-erp/internal/modules/stock/infrastructure"
)

// MoveLocation relocates stock between two locations of the same branch in
// one call (J4): draft → dispatch → receive through the transfer document,
// so a 10-second job costs one screen instead of three while keeping the
// document number, the paired out/in movements, and the audit trail.
//
// Guards are inherited, not reimplemented: different locations and
// availability are enforced by Create/Dispatch, and a short line aborts the
// whole move (DispatchTransfer never dispatches partially). When dispatch
// fails the draft document remains visible and cancellable — the move is
// atomic in effect (nothing moved) but leaves its draft behind for review.
func (s *Service) MoveLocation(ctx context.Context, actorID, branchID, fromLocationID, toLocationID int64, notes string, items []TransferItemInput) (*infrastructure.Transfer, error) {
	t, err := s.CreateTransfer(ctx, actorID, branchID, branchID, fromLocationID, toLocationID, notes, items)
	if err != nil {
		return nil, err
	}
	t, err = s.DispatchTransfer(ctx, actorID, branchID, t.ID)
	if err != nil {
		return nil, err
	}
	return s.ReceiveTransfer(ctx, actorID, branchID, t.ID)
}
