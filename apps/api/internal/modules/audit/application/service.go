package application

import (
	"context"
	"log/slog"
	"strings"

	"mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/audit/infrastructure"
	"mini-erp/internal/shared/apperror"
)

// Service owns audit-trail reads plus the best-effort write path.
type Service struct {
	repo *infrastructure.Repository
	log  *slog.Logger
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
