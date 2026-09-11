package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/branch/contracts"
	"mini-erp/internal/modules/branch/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/timeutil"
	usercontracts "mini-erp/internal/modules/user/contracts"
)

// Service implements contracts.BranchClient plus branch administration.
type Service struct {
	repo  *infrastructure.Repository
	users usercontracts.UserClient
	audit auditcontracts.AuditClient
}

// NewService wires branch use cases.
func NewService(repo *infrastructure.Repository, users usercontracts.UserClient, audit auditcontracts.AuditClient) *Service {
	return &Service{repo: repo, users: users, audit: audit}
}

// GetByID resolves a branch or returns nil when missing.
func (s *Service) GetByID(ctx context.Context, id int64) (*contracts.Branch, error) {
	b, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return b, nil
}

// GetByIDs resolves known branches, skipping unknown IDs.
func (s *Service) GetByIDs(ctx context.Context, ids []int64) ([]*contracts.Branch, error) {
	branches, err := s.repo.GetByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if branches == nil {
		branches = []*contracts.Branch{}
	}
	return branches, nil
}

// CreateBranchInput is the branch create payload.
type CreateBranchInput struct {
	Code    string
	Name    string
	Address string
	City    string
	Phone   string
	IsHead  bool
}

// CreateBranch inserts the branch with all document sequences in one
// transaction. The server enforces the 10-char code limit (KI-41: the
// browser limit alone let overlong codes through to a 500).
func (s *Service) CreateBranch(ctx context.Context, actorID int64, in CreateBranchInput) (*contracts.Branch, error) {
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	if code == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "code", Message: "wajib diisi"}})
	}
	if len(code) > 10 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "code", Message: "maksimal 10 karakter"}})
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "name", Message: "wajib diisi"}})
	}
	if dup, err := s.repo.ExistsByCode(ctx, code, 0); err != nil {
		return nil, apperror.Internal(err)
	} else if dup {
		return nil, apperror.Conflict("Kode cabang '" + code + "' sudah digunakan")
	}
	id, err := s.repo.Create(ctx, &contracts.Branch{
		Code: code, Name: strings.TrimSpace(in.Name), Address: in.Address,
		City: in.City, Phone: in.Phone, IsHead: in.IsHead,
	}, actorID, infrastructure.AllDocKinds(), timeutil.NowUTC())
	if err != nil {
		return nil, dberr.Map(err)
	}
	b, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "branches.create", Entity: "branch", EntityID: id, BranchID: id, ActorID: actorID, Note: code})
	}
	return b, nil
}

// UpdateBranchInput is the branch edit payload. Code changes go through the
// issued-documents lock below, never silently.
type UpdateBranchInput struct {
	Code    string
	Name    string
	Address string
	City    string
	Phone   string
	Status  string
	IsHead  bool
}

// UpdateBranch rewrites a branch. Closing (status=inactive) is a supported
// operation (KI-36); the code is immutable once documents exist (OQ-A18).
func (s *Service) UpdateBranch(ctx context.Context, actorID, id int64, in UpdateBranchInput) (*contracts.Branch, error) {
	existing, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, apperror.NotFound("Cabang")
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "name", Message: "wajib diisi"}})
	}
	if in.Status != "active" && in.Status != "inactive" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "status", Message: "harus active atau inactive"}})
	}
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	if code == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "code", Message: "wajib diisi"}})
	}
	if len(code) > 10 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "code", Message: "maksimal 10 karakter"}})
	}
	if code != existing.Code {
		if dup, err := s.repo.ExistsByCode(ctx, code, id); err != nil {
			return nil, apperror.Internal(err)
		} else if dup {
			return nil, apperror.Conflict("Kode cabang '" + code + "' sudah digunakan")
		}
		if used, err := s.repo.TryChangeCode(ctx, id, code, actorID); err != nil {
			return nil, apperror.Internal(err)
		} else if used {
			return nil, apperror.Conflict("Kode cabang tidak dapat diubah karena sudah ada dokumen terbit")
		}
		existing.Code = code
	}
	existing.Name = strings.TrimSpace(in.Name)
	existing.Address = in.Address
	existing.City = in.City
	existing.Phone = in.Phone
	existing.Status = in.Status
	existing.IsHead = in.IsHead
	if err := s.repo.Update(ctx, existing, actorID); err != nil {
		return nil, apperror.Internal(err)
	}
	b, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "branches.update", Entity: "branch", EntityID: id, BranchID: id, ActorID: actorID, Note: existing.Code})
	}
	return b, nil
}

// ListResult is a paginated branch page.
type ListResult struct {
	Branches []*contracts.Branch
	Total    int64
}

// List searches branches per word with an optional status filter.
func (s *Service) List(ctx context.Context, search, status string, page, limit int) (*ListResult, error) {
	total, err := s.repo.Count(ctx, search, status)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	branches, err := s.repo.List(ctx, search, status, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if branches == nil {
		branches = []*contracts.Branch{}
	}
	return &ListResult{Branches: branches, Total: total}, nil
}

// ListAll lists every branch, name-ordered (assistant bot company-wide
// aggregation). Small table, uncapped.
func (s *Service) ListAll(ctx context.Context) ([]*contracts.Branch, error) {
	branches, err := s.repo.List(ctx, "", "", 1000, 0)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if branches == nil {
		branches = []*contracts.Branch{}
	}
	return branches, nil
}

// AccessibleBranch is one active branch the user may open (J6, legacy F-05:
// only the user's branches, only active, default marked, login suffices).
type AccessibleBranch struct {
	Branch    *contracts.Branch
	IsDefault bool
}

// MyAccess returns the caller's accessible ACTIVE branches with the default
// mark. Cost: 2 grouped reads (access rows + one GetByIDs) — never one
// query per branch (P3). Inactive branches are dropped (F-05.2); unknown IDs
// are skipped, never fatal.
func (s *Service) MyAccess(ctx context.Context, userID int64) ([]AccessibleBranch, error) {
	access, err := s.users.GetBranchAccess(ctx, userID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if len(access) == 0 {
		return []AccessibleBranch{}, nil
	}
	defs := make(map[int64]bool, len(access))
	ids := make([]int64, 0, len(access))
	for _, a := range access {
		if _, seen := defs[a.BranchID]; seen {
			if a.IsDefault {
				defs[a.BranchID] = true
			}
			continue
		}
		defs[a.BranchID] = a.IsDefault
		ids = append(ids, a.BranchID)
	}
	branches, err := s.repo.GetByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]AccessibleBranch, 0, len(branches))
	for _, b := range branches {
		if b == nil || b.Status != "active" {
			continue
		}
		out = append(out, AccessibleBranch{Branch: b, IsDefault: defs[b.ID]})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDefault != out[j].IsDefault {
			return out[i].IsDefault
		}
		if out[i].Branch.Code != out[j].Branch.Code {
			return out[i].Branch.Code < out[j].Branch.Code
		}
		return out[i].Branch.ID < out[j].Branch.ID
	})
	return out, nil
}

// NextDocumentNumber allocates the next number for kind at this branch.
// Formats replicate legacy shapes; sequences run continuous across years
// (OQ-A17) except order kinds, which reset monthly as they actually did.
func (s *Service) NextDocumentNumber(ctx context.Context, branchID int64, kind contracts.DocKind) (string, error) {
	branch, err := s.GetByID(ctx, branchID)
	if err != nil {
		return "", err
	}
	if branch == nil {
		return "", apperror.NotFound("Cabang")
	}
	now := timeutil.NowUTC()
	key := infrastructure.SequenceKey(kind, now)
	n, err := s.repo.NextNumber(ctx, branchID, key)
	if err != nil {
		return "", apperror.Internal(err)
	}
	year := now.In(timeutil.Jakarta).Format("2006")
	month := now.In(timeutil.Jakarta).Format("01")
	switch kind {
	case contracts.DocPurchaseOrder:
		return fmt.Sprintf("ORD-%s/PB/%s/%s/%05d", branch.Code, year, month, n), nil
	case contracts.DocSalesOrder:
		return fmt.Sprintf("ORD-%s/PJ/%s/%s/%05d", branch.Code, year, month, n), nil
	case contracts.DocPayment:
		return fmt.Sprintf("PAY-%s/%s/%05d", branch.Code, year, n), nil
	case contracts.DocDeliveryNote:
		return fmt.Sprintf("SJ-%s/%s/%05d", branch.Code, year, n), nil
	case contracts.DocSalesReturn:
		return fmt.Sprintf("RTR-%s/%s/%05d", branch.Code, year, n), nil
	case contracts.DocPurchaseReturn:
		return fmt.Sprintf("RTB-%s/%s/%05d", branch.Code, year, n), nil
	case contracts.DocStockTransfer:
		return fmt.Sprintf("TRF-%s/%s/%05d", branch.Code, year, n), nil
	default:
		return "", apperror.Validation("", []apperror.FieldError{{Field: "kind", Message: "jenis dokumen tidak dikenal"}})
	}
}
