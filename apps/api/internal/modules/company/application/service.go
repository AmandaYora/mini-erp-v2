package application

import (
	"context"
	"database/sql"
	"regexp"
	"sort"
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

// logTx writes the trail row inside the write's transaction (ADR-0011).
func (s *Service) logTx(ctx context.Context, tx *sql.Tx, e auditcontracts.Entry) error {
	if s.audit == nil {
		return nil
	}
	return s.audit.LogTx(ctx, tx, e)
}

func profileSnapshot(p *contracts.Profile) map[string]string {
	return map[string]string{
		"name": p.Name, "legalName": p.LegalName, "address": p.Address, "city": p.City,
		"phone": p.Phone, "email": p.Email, "taxId": p.TaxID,
	}
}

var secretKeyRe = regexp.MustCompile(`(?i)(secret|token|password|passwd|api[_-]?key|private)`)

// redactSetting keeps credentials out of the trail (F3b-8): free-form keys can
// carry anything, so secret-looking names are masked and long values clipped.
func redactSetting(key, value string) string {
	if value == "" {
		return ""
	}
	if secretKeyRe.MatchString(key) {
		return "[disembunyikan]"
	}
	if len(value) > 2000 {
		return value[:2000] + "…"
	}
	return value
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
	prev, err := s.repo.GetProfile(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var before any
	if prev != nil {
		before = profileSnapshot(prev)
	}
	p, err := s.repo.SaveProfile(ctx, &in, actorID, func(ctx context.Context, tx *sql.Tx, id int64) error {
		return s.logTx(ctx, tx, auditcontracts.Entry{
			Action: "company.profile", Entity: "company_profile", EntityID: id, ActorID: actorID, Note: in.Name,
			Before: before, After: profileSnapshot(&in),
		})
	})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return p, nil
}

// PrintProfile returns only what printing needs (F3a-6): company identity and
// the allow-listed print settings. It never exposes other company_settings
// keys, so it is safe for every authenticated user with a branch.
func (s *Service) PrintProfile(ctx context.Context) (*contracts.PrintProfile, error) {
	out := &contracts.PrintProfile{Settings: map[string]string{}}
	p, err := s.repo.GetProfile(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if p != nil {
		out.Name, out.LegalName, out.Address, out.City, out.Phone = p.Name, p.LegalName, p.Address, p.City, p.Phone
	}
	settings, err := s.repo.GetSettings(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for _, k := range contracts.PrintSettingKeys {
		if v, ok := settings[k]; ok {
			out.Settings[k] = v
		}
	}
	return out, nil
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
	clean, err := normalizeSettings(settings)
	if err != nil {
		return err
	}
	prev, err := s.repo.GetSettings(ctx)
	if err != nil {
		return apperror.Internal(err)
	}
	keys := make([]string, 0, len(clean))
	before, after := map[string]string{}, map[string]string{}
	for k, v := range clean {
		keys = append(keys, k)
		before[k] = redactSetting(k, prev[k])
		after[k] = redactSetting(k, v)
	}
	sort.Strings(keys)
	if err := s.repo.SaveSettings(ctx, clean, actorID, func(ctx context.Context, tx *sql.Tx) error {
		return s.logTx(ctx, tx, auditcontracts.Entry{
			Action: "company.settings", Entity: "company_settings", ActorID: actorID, Note: strings.Join(keys, ","),
			Before: before, After: after,
		})
	}); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

// EffectiveSettings returns stored settings with the registered defaults
// filled in, i.e. what the readers will actually see.
func (s *Service) EffectiveSettings(ctx context.Context) (map[string]string, error) {
	stored, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	return withDefaults(stored), nil
}
