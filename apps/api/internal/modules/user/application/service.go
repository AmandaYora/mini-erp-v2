package application

import (
	"context"
	"strings"

	"golang.org/x/crypto/bcrypt"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/modules/user/domain"
	"mini-erp/internal/modules/user/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
)

// Service implements contracts.UserClient/RoleClient plus the admin use cases
// consumed by the presentation layer and the dev seed.
type Service struct {
	repo  *infrastructure.Repository
	audit auditcontracts.AuditClient
}

// NewService wires the user use cases.
func NewService(repo *infrastructure.Repository, audit auditcontracts.AuditClient) *Service {
	return &Service{repo: repo, audit: audit}
}

func normalizeEmail(email *string) *string {
	if email == nil {
		return nil
	}
	if trimmed := strings.TrimSpace(*email); trimmed != "" {
		return &trimmed
	}
	return nil
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// GetByID resolves a user or returns nil when missing.
func (s *Service) GetByID(ctx context.Context, id int64) (*contracts.User, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return u, nil
}

// GetByUsername resolves a user or returns nil when missing.
func (s *Service) GetByUsername(ctx context.Context, username string) (*contracts.User, error) {
	u, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return u, nil
}

// VerifyCredentials checks username + password + active status. Every failure
// maps to the same Indonesian message so callers cannot distinguish the cause.
func (s *Service) VerifyCredentials(ctx context.Context, username, password string) (*contracts.User, error) {
	u, err := s.repo.GetByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if u == nil || u.Status != "active" {
		return nil, apperror.Unauthorized()
	}
	hash, err := s.repo.GetPasswordHash(ctx, u.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, apperror.Unauthorized()
	}
	return u, nil
}

// GetRoleIDs returns the user's roles in stable (id-asc) order, so the
// initial active role after login is deterministic (KI-05).
func (s *Service) GetRoleIDs(ctx context.Context, userID int64) ([]int64, error) {
	ids, err := s.repo.GetRoleIDs(ctx, userID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if ids == nil {
		ids = []int64{}
	}
	return ids, nil
}

// GetBranchAccess returns the user's branch links.
func (s *Service) GetBranchAccess(ctx context.Context, userID int64) ([]contracts.BranchAccess, error) {
	access, err := s.repo.GetBranchAccess(ctx, userID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if access == nil {
		access = []contracts.BranchAccess{}
	}
	return access, nil
}

// CreateUserInput is the admin create payload (already validated structurally).
type CreateUserInput struct {
	Username string
	Email    *string
	FullName string
	Password string
	RoleIDs  []int64
	Branches []contracts.BranchAccess
}

// CreateUser creates the account with roles and branch links in one transaction.
func (s *Service) CreateUser(ctx context.Context, actorID int64, in CreateUserInput) (*contracts.User, error) {
	username := strings.TrimSpace(in.Username)
	if username == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "username", Message: "wajib diisi"}})
	}
	if dup, err := s.repo.ExistsByUsername(ctx, username, 0); err != nil {
		return nil, apperror.Internal(err)
	} else if dup {
		return nil, apperror.Conflict("Username '" + username + "' sudah digunakan")
	}
	email := normalizeEmail(in.Email)
	if email != nil {
		if dup, err := s.repo.ExistsByEmail(ctx, *email, 0); err != nil {
			return nil, apperror.Internal(err)
		} else if dup {
			return nil, apperror.Conflict("Email '" + *email + "' sudah digunakan")
		}
	}
	if len(in.Password) < domain.MinPasswordLength {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "password", Message: "minimal 8 karakter"}})
	}
	if err := s.requireRolesExist(ctx, in.RoleIDs); err != nil {
		return nil, err
	}
	branches, err := normalizeBranches(in.Branches)
	if err != nil {
		return nil, err
	}
	hash, err := hashPassword(in.Password)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	id, err := s.repo.CreateUser(ctx, &contracts.User{
		Username: username, Email: email, FullName: strings.TrimSpace(in.FullName),
	}, hash, in.RoleIDs, branches, actorID)
	if err != nil {
		return nil, dberr.Map(err)
	}
	u, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "users.create", Entity: "user", EntityID: id, BranchID: 0, ActorID: actorID, Note: username})
	}
	return u, nil
}

// UpdateUserInput is the admin edit payload (password changes go through
// ChangePassword, never silently alongside profile edits).
type UpdateUserInput struct {
	Username string
	Email    *string
	FullName string
	RoleIDs  []int64
	Branches []contracts.BranchAccess
}

// UpdateUser rewrites profile, roles, and branch links in one transaction.
func (s *Service) UpdateUser(ctx context.Context, actorID, userID int64, in UpdateUserInput) (*contracts.User, error) {
	existing, err := s.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, apperror.NotFound("Pengguna")
	}
	username := strings.TrimSpace(in.Username)
	if username == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "username", Message: "wajib diisi"}})
	}
	if dup, err := s.repo.ExistsByUsername(ctx, username, userID); err != nil {
		return nil, apperror.Internal(err)
	} else if dup {
		return nil, apperror.Conflict("Username '" + username + "' sudah digunakan")
	}
	email := normalizeEmail(in.Email)
	if email != nil {
		if dup, err := s.repo.ExistsByEmail(ctx, *email, userID); err != nil {
			return nil, apperror.Internal(err)
		} else if dup {
			return nil, apperror.Conflict("Email '" + *email + "' sudah digunakan")
		}
	}
	if err := s.requireRolesExist(ctx, in.RoleIDs); err != nil {
		return nil, err
	}
	branches, err := normalizeBranches(in.Branches)
	if err != nil {
		return nil, err
	}
	existing.Username = username
	existing.Email = email
	existing.FullName = strings.TrimSpace(in.FullName)
	if err := s.repo.UpdateUser(ctx, existing, in.RoleIDs, branches, actorID); err != nil {
		return nil, dberr.Map(err)
	}
	u, err := s.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "users.update", Entity: "user", EntityID: userID, BranchID: 0, ActorID: actorID, Note: username})
	}
	return u, nil
}

// requireRolesExist enforces at least one role and rejects unknown codes
// before anything is deleted (KI-13: replace-all must never strand a user).
func (s *Service) requireRolesExist(ctx context.Context, roleIDs []int64) error {
	if len(roleIDs) == 0 {
		return apperror.Validation("", []apperror.FieldError{{Field: "roleIds", Message: "pengguna wajib memiliki minimal satu role"}})
	}
	for _, id := range roleIDs {
		role, err := s.repo.RoleByID(ctx, id)
		if err != nil {
			return apperror.Internal(err)
		}
		if role == nil {
			return apperror.Validation("", []apperror.FieldError{{Field: "roleIds", Message: "role tidak dikenal"}})
		}
	}
	return nil
}

// normalizeBranches enforces at most one default; when links exist but none
// is marked, the first (lowest branch ID) becomes default deterministically
// so login never depends on invisible ordering (KI-20).
func normalizeBranches(branches []contracts.BranchAccess) ([]contracts.BranchAccess, error) {
	defaults := 0
	for _, b := range branches {
		if b.IsDefault {
			defaults++
		}
	}
	if defaults > 1 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "branches", Message: "tepat satu cabang default"}})
	}
	if len(branches) > 0 && defaults == 0 {
		branches[0].IsDefault = true
	}
	return branches, nil
}

// SetStatus flips active/inactive. Nobody may deactivate their own account
// (KI-19): that strands the last permission holder with no recovery path.
func (s *Service) SetStatus(ctx context.Context, actorID, userID int64, status string) error {
	if status != "active" && status != "inactive" {
		return apperror.Validation("", []apperror.FieldError{{Field: "status", Message: "harus active atau inactive"}})
	}
	if actorID == userID && status != "active" {
		return apperror.Forbidden()
	}
	existing, err := s.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if existing == nil {
		return apperror.NotFound("Pengguna")
	}
	if err := s.repo.SetStatus(ctx, userID, status, actorID); err != nil {
		return apperror.Internal(err)
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "users.status", Entity: "user", EntityID: userID, BranchID: 0, ActorID: actorID, Note: status})
	}
	return nil
}

// ChangePassword replaces the credential after a length check.
func (s *Service) ChangePassword(ctx context.Context, actorID, userID int64, password string) error {
	if len(password) < domain.MinPasswordLength {
		return apperror.Validation("", []apperror.FieldError{{Field: "password", Message: "minimal 8 karakter"}})
	}
	existing, err := s.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if existing == nil {
		return apperror.NotFound("Pengguna")
	}
	hash, err := hashPassword(password)
	if err != nil {
		return apperror.Internal(err)
	}
	if err := s.repo.SetPasswordHash(ctx, userID, hash, actorID); err != nil {
		return apperror.Internal(err)
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "users.password", Entity: "user", EntityID: userID, BranchID: 0, ActorID: actorID})
	}
	return nil
}

// ListUsersResult is a paginated user page.
type ListUsersResult struct {
	Users []*contracts.User
	Total int64
}

// ListUsers searches per word across username/full_name/email (KI-25:
// word-based like the other modules, not whole-phrase).
func (s *Service) ListUsers(ctx context.Context, search, status string, page, limit int) (*ListUsersResult, error) {
	total, err := s.repo.CountUsers(ctx, search, status)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	users, err := s.repo.ListUsers(ctx, search, status, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if users == nil {
		users = []*contracts.User{}
	}
	return &ListUsersResult{Users: users, Total: total}, nil
}

// GetRoleByCode resolves a role by code, or nil when missing.
func (s *Service) GetRoleByCode(ctx context.Context, code string) (*contracts.Role, error) {
	role, err := s.repo.RoleByCode(ctx, code)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return role, nil
}

// EnsurePermissionCatalog upserts the L1 permission catalog (idempotent seed
// helper — codes never change meaning once issued).
func (s *Service) EnsurePermissionCatalog(ctx context.Context) error {
	return s.EnsurePermissions(ctx, catalogSeeds())
}

func catalogSeeds() []contracts.PermissionSeed {
	out := make([]contracts.PermissionSeed, 0, len(domain.Catalog()))
	for _, p := range domain.Catalog() {
		out = append(out, contracts.PermissionSeed{Code: p.Code, Name: p.Name})
	}
	return out
}

// EnsurePermissions upserts arbitrary catalog codes (other modules' catalogs).
func (s *Service) EnsurePermissions(ctx context.Context, perms []contracts.PermissionSeed) error {
	for _, p := range perms {
		if err := s.repo.EnsurePermission(ctx, p.Code, p.Name); err != nil {
			return apperror.Internal(err)
		}
	}
	return nil
}

// AllPermissionCodes returns every catalog code for owner sync.
func (s *Service) AllPermissionCodes(ctx context.Context) ([]string, error) {
	return s.repo.PermissionCodes(ctx)
}

// ListPermissions returns the full catalog with display names.
func (s *Service) ListPermissions(ctx context.Context) ([]contracts.PermissionSeed, error) {
	perms, err := s.repo.ListPermissions(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return perms, nil
}

// GetRole resolves a role or returns nil when missing.
func (s *Service) GetRole(ctx context.Context, id int64) (*contracts.Role, error) {
	role, err := s.repo.RoleByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return role, nil
}

// List returns all roles.
func (s *Service) List(ctx context.Context) ([]*contracts.Role, error) {
	roles, err := s.repo.ListRoles(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if roles == nil {
		roles = []*contracts.Role{}
	}
	return roles, nil
}

// CreateRoleInput is the role create payload.
type CreateRoleInput struct {
	Code        string
	Name        string
	Description string
}

// CreateRole inserts a custom role (system roles are seeded, never created here).
func (s *Service) CreateRole(ctx context.Context, actorID int64, in CreateRoleInput) (*contracts.Role, error) {
	code := strings.TrimSpace(in.Code)
	if code == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "code", Message: "wajib diisi"}})
	}
	if existing, err := s.repo.RoleByCode(ctx, code); err != nil {
		return nil, apperror.Internal(err)
	} else if existing != nil {
		return nil, apperror.Conflict("Kode role '" + code + "' sudah digunakan")
	}
	id, err := s.repo.CreateRole(ctx, &contracts.Role{
		Code: code, Name: strings.TrimSpace(in.Name), Description: in.Description,
	})
	if err != nil {
		return nil, dberr.Map(err)
	}
	role, err := s.GetRole(ctx, id)
	if err != nil {
		return nil, err
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "roles.create", Entity: "role", EntityID: id, BranchID: 0, ActorID: actorID, Note: code})
	}
	return role, nil
}

// UpdateRole rewrites name/description. Code is immutable.
func (s *Service) UpdateRole(ctx context.Context, actorID, id int64, name, description string) (*contracts.Role, error) {
	role, err := s.GetRole(ctx, id)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, apperror.NotFound("Role")
	}
	role.Name = strings.TrimSpace(name)
	role.Description = description
	if err := s.repo.UpdateRole(ctx, role); err != nil {
		return nil, apperror.Internal(err)
	}
	updated, err := s.GetRole(ctx, id)
	if err != nil {
		return nil, err
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "roles.update", Entity: "role", EntityID: id, BranchID: 0, ActorID: actorID, Note: role.Code})
	}
	return updated, nil
}

// DeleteRole removes a custom role that no user holds. System roles and
// in-use roles are rejected with a business message, never a bare error.
func (s *Service) DeleteRole(ctx context.Context, actorID, id int64) error {
	role, err := s.GetRole(ctx, id)
	if err != nil {
		return err
	}
	if role == nil {
		return apperror.NotFound("Role")
	}
	if role.IsSystem {
		return apperror.Forbidden()
	}
	if inUse, err := s.repo.RoleInUse(ctx, id); err != nil {
		return apperror.Internal(err)
	} else if inUse {
		return apperror.Conflict("Role masih dipakai oleh pengguna dan tidak dapat dihapus")
	}
	if err := s.repo.DeleteRole(ctx, id); err != nil {
		return apperror.Internal(err)
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "roles.delete", Entity: "role", EntityID: id, BranchID: 0, ActorID: actorID, Note: role.Code})
	}
	return nil
}

// SetRolePermissions replaces the role's permission set. Unknown codes are
// rejected before anything is deleted (KI-27 class: no stranded empty role).
func (s *Service) SetRolePermissions(ctx context.Context, actorID, roleID int64, codes []string) error {
	role, err := s.GetRole(ctx, roleID)
	if err != nil {
		return err
	}
	if role == nil {
		return apperror.NotFound("Role")
	}
	ids := make([]int64, 0, len(codes))
	for _, code := range codes {
		id, found, err := s.repo.PermissionIDByCode(ctx, strings.TrimSpace(code))
		if err != nil {
			return apperror.Internal(err)
		}
		if !found {
			return apperror.Validation("", []apperror.FieldError{{Field: "permissions", Message: "permission tidak dikenal: " + code}})
		}
		ids = append(ids, id)
	}
	if err := s.repo.SetRolePermissions(ctx, roleID, ids); err != nil {
		return apperror.Internal(err)
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "roles.permissions", Entity: "role", EntityID: roleID, BranchID: 0, ActorID: actorID})
	}
	return nil
}

// GetPermissionCodes resolves distinct codes for the given roles.
func (s *Service) GetPermissionCodes(ctx context.Context, roleIDs []int64) ([]string, error) {
	codes, err := s.repo.GetPermissionCodes(ctx, roleIDs)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if codes == nil {
		codes = []string{}
	}
	return codes, nil
}
