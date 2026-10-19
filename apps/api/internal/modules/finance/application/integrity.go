package application

import (
	"context"

	"mini-erp/internal/modules/finance/infrastructure"
	"mini-erp/internal/shared/apperror"
)

// IntegrityReport is the finance consistency check: the daily rollup must equal
// the journal lines, every journal must balance, and the Persediaan account must
// equal the cost ledger.
type IntegrityReport struct {
	RollupDiffs []infrastructure.RollupDiff `json:"rollupDiffs"`
	Unbalanced  int64                       `json:"unbalanced"`
	// Inventory reconciles the Persediaan account with the cost ledger (K-05).
	Inventory  *InventoryRecon `json:"inventory"`
	Consistent bool            `json:"consistent"`
}

// VerifyRollups compares the rollup with the lines (read-only).
func (s *Service) VerifyRollups(ctx context.Context, branchID int64) (*IntegrityReport, error) {
	diffs, err := s.repo.VerifyRollups(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	unbalanced, err := s.repo.CountUnbalanced(ctx, branchID, "1970-01-01", "2999-12-31")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if diffs == nil {
		diffs = []infrastructure.RollupDiff{}
	}
	rec, err := s.ReconcileInventory(ctx, branchID)
	if err != nil {
		return nil, err
	}
	// While documents are still unjournalled the gap may be theirs: it is
	// reported but does not fail the check (readiness blocks the close on them).
	inventoryOK := rec.Consistent || rec.Pending > 0
	return &IntegrityReport{RollupDiffs: diffs, Unbalanced: unbalanced, Inventory: rec,
		Consistent: len(diffs) == 0 && unbalanced == 0 && inventoryOK}, nil
}

// RebuildRollups recomputes the branch's daily rollup from the lines.
func (s *Service) RebuildRollups(ctx context.Context, branchID int64) error {
	if err := s.repo.RebuildRollups(ctx, branchID); err != nil {
		return apperror.Internal(err)
	}
	return nil
}
