package application

import (
	"context"
	"database/sql"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	"mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/modules/stock/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/timeutil"
)

// MoveLocation relocates stock between two locations of the same branch in
// ONE transaction (J4): the transfer document is born "received" and both
// movements (out at the source, in at the destination) commit together with
// it. There is no draft or in-transit state to strand when something fails —
// a short line, an archived destination or a deadlock rolls back the document
// and every movement, and the whole unit is retried on lock failures.
//
// The document keeps its number and the paired out/in movements, so the trail
// is the same one the three-step flow produced. The number is allocated just
// before the transaction (a failed move may burn one, F2a-11).
func (s *Service) MoveLocation(ctx context.Context, actorID, branchID, fromLocationID, toLocationID int64, notes string, items []TransferItemInput) (*infrastructure.Transfer, error) {
	t, err := s.prepareTransfer(ctx, branchID, branchID, fromLocationID, toLocationID, notes, items)
	if err != nil {
		return nil, err
	}
	out, err := transferOutLeg(t)
	if err != nil {
		return nil, err
	}
	in, err := transferInLeg(t)
	if err != nil {
		return nil, err
	}
	legs := []leg{
		{branchID: branchID, direction: "out", lines: out},
		{branchID: branchID, direction: "in", lines: in},
	}
	if err := s.validateLegs(ctx, legs, false); err != nil {
		return nil, err
	}
	number, err := s.branches.NextDocumentNumber(ctx, branchID, branchcontracts.DocStockTransfer)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	t.Number = number
	t.TransferDate = timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	var id int64
	err = dberr.RetryTx(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		docID, err := s.repo.CreateTransferTx(ctx, tx, t, "received", actorID)
		if err != nil {
			return err
		}
		id = docID
		ref := legRef{refType: contracts.RefTransfer, refID: docID, actorID: actorID, note: "Pindah " + number}
		if _, err = s.applyTx(ctx, tx, ref, legs); err != nil {
			return err
		}
		// The trail co-commits with the movement (ADR-0011).
		return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
			Action: "stock_transfers.move", Entity: "stock_transfer", EntityID: docID, BranchID: branchID,
			ActorID: actorID, Note: number,
			Metadata: map[string]any{"fromLocationId": fromLocationID, "toLocationId": toLocationID, "items": len(items)},
		})
	})
	if err != nil {
		return nil, asAppError(err)
	}
	done, err := s.GetTransfer(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	return done, nil
}
