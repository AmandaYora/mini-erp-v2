package application

import (
	"context"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/company/contracts"
	"mini-erp/internal/modules/company/infrastructure"
	"mini-erp/internal/shared/apperror"
)

// Service implements contracts.CompanyClient plus company administration.
type Service struct {
	repo  *infrastructure.Repository
	audit auditcontracts.AuditClient
}

// NewService wires company use cases.
func NewService(repo *infrastructure.Repository, audit auditcontracts.AuditClient) *Service {
	return &Service{repo: repo, audit: audit}
}

// GetProfile returns the singleton profile, or nil when never saved.
func (s *Service) GetProfile(ctx context.Context) (*contracts.Profile, error) {
	p, err := s.repo.GetProfile(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return p, nil
}

// SaveProfile validates and upserts the singleton profile.
func (s *Service) SaveProfile(ctx context.Context, actorID int64, in contracts.Profile) (*contracts.Profile, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "name", Message: "wajib diisi"}})
	}
	in.Name = strings.TrimSpace(in.Name)
	p, err := s.repo.SaveProfile(ctx, &in, actorID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "company.profile", Entity: "company_profile", EntityID: p.ID, BranchID: 0, ActorID: actorID, Note: p.Name})
	}
	return p, nil
}

// GetSettings returns all operational settings.
func (s *Service) GetSettings(ctx context.Context) (map[string]string, error) {
	settings, err := s.repo.GetSettings(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return settings, nil
}

// SaveSettings merges keys (never replace-all).
func (s *Service) SaveSettings(ctx context.Context, actorID int64, settings map[string]string) error {
	if len(settings) == 0 {
		return apperror.Validation("", []apperror.FieldError{{Field: "settings", Message: "tidak ada pengaturan yang dikirim"}})
	}
	if err := s.repo.SaveSettings(ctx, settings, actorID); err != nil {
		return apperror.Internal(err)
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "company.settings", Entity: "company_settings", BranchID: 0, ActorID: actorID})
	}
	return nil
}
