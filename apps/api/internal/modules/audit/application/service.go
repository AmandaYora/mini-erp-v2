package application

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/audit/infrastructure"
	"mini-erp/internal/shared/apperror"
)

// Service owns audit-trail reads plus the best-effort write path.
type Service struct {
	repo *infrastructure.Repository
	log  *slog.Logger
	// unexpected/now back RecordUnexpected (OP-06): dedupe window + budget.
	unexpected unexpectedState
	now        func() time.Time
}

// NewService wires the audit use cases.
func NewService(repo *infrastructure.Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// Log appends one trail record. It is best-effort by design: the caller's
// mutation already committed, so a storage failure is logged server-side and
// never fails the operation.
func (s *Service) Log(ctx context.Context, e contracts.Entry) error {
	if strings.TrimSpace(e.Action) == "" || strings.TrimSpace(e.Entity) == "" {
		return apperror.Validation("", []apperror.FieldError{{Field: "action", Message: "aksi dan entitas wajib diisi"}})
	}
	if err := s.repo.Log(ctx, e); err != nil {
		s.log.Error("audit log failed", "action", e.Action, "entity", e.Entity, "error", err)
		return apperror.Internal(err)
	}
	return nil
}

// LogTx appends one trail record inside the caller's transaction (ADR-0011).
// Unlike Log it reports storage failures to the caller: money/stock paths
// roll back the whole transaction on error instead of losing the trail.
func (s *Service) LogTx(ctx context.Context, tx contracts.AuditTx, e contracts.Entry) error {
	if strings.TrimSpace(e.Action) == "" || strings.TrimSpace(e.Entity) == "" {
		return apperror.Validation("", []apperror.FieldError{{Field: "action", Message: "aksi dan entitas wajib diisi"}})
	}
	return s.repo.LogTx(ctx, tx, e)
}

// List returns newest-first records for one branch, plus global (branch-less)
// events which every branch may see.
func (s *Service) List(ctx context.Context, branchID int64, action, entity string, page, limit int) ([]contracts.Record, int64, error) {
	total, err := s.repo.Count(ctx, branchID, action, entity)
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	records, err := s.repo.List(ctx, branchID, action, entity, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	return records, total, nil
}

// Query returns newest-first records with before/after detail for the
// console view (F3b-8): cross-branch and filterable.
func (s *Service) Query(ctx context.Context, f contracts.ListFilter) ([]contracts.Record, int64, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = 20
	}
	recs, total, err := s.repo.Query(ctx, f)
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	return recs, total, nil
}

// Purge deletes up to limit audit rows older than cutoff (F4-9 retention).
// Bounded and idempotent: housekeeping loops until the count drops below limit.
func (s *Service) Purge(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	return s.repo.Purge(ctx, cutoff, limit)
}
