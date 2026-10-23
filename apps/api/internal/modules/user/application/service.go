package application

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	authcontracts "mini-erp/internal/modules/auth/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	"mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/modules/user/domain"
	"mini-erp/internal/modules/user/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/doclock"
)

// Service implements contracts.UserClient/RoleClient plus the admin use cases
// consumed by the presentation layer and the dev seed.
type Service struct {
	repo  *infrastructure.Repository
	audit auditcontracts.AuditClient
	// authz is announced to, never asked. Auth resolves a session once and
	// reuses it; this module owns the rows that decide what that session may
	// do, so it must say when they change. Nil is a valid no-op (the dev
	// seed wires no auth), leaving auth's TTL as the only bound.
	authz authcontracts.AuthzInvalidator
	// revoker revokes sessions on status/password change (F1-4). Nil = no-op.
	revoker authcontracts.SessionRevoker
	// branches names branch ids for the admin list (U-01). Nil = names omitted.
	branches BranchDirectory
	// seedPasswords caches the data-health A1 bcrypt verdicts per hash.
	seedPasswords seedPasswordCache
}

// BranchDirectory is the slice of branch.BranchClient the user list needs.
type BranchDirectory interface {
	GetByIDs(ctx context.Context, ids []int64) ([]*branchcontracts.Branch, error)
}

// SetBranchDirectory injects the branch contract (branch is built after user,
// since it depends on UserClient, so this cannot be a constructor argument).
func (s *Service) SetBranchDirectory(b BranchDirectory) { s.branches = b }

// NewService wires the user use cases.
func NewService(repo *infrastructure.Repository, audit auditcontracts.AuditClient) *Service {
	return &Service{repo: repo, audit: audit}
}

// SetAuthzInvalidator injects auth's invalidator from the composition root.
// Auth is built after user (it depends on it), so this cannot be a
// constructor argument without inverting the dependency.
func (s *Service) SetAuthzInvalidator(authz authcontracts.AuthzInvalidator) {
	s.authz = authz
}

// SetSessionRevoker injects auth's revoker (F1-4).
func (s *Service) SetSessionRevoker(r authcontracts.SessionRevoker) { s.revoker = r }

// logTx writes the trail row inside the write's transaction (ADR-0011). A
// service built without an audit client (dev seed) skips it.
func (s *Service) logTx(ctx context.Context, tx *sql.Tx, e auditcontracts.Entry) error {
	if s.audit == nil {
		return nil
	}
	return s.audit.LogTx(ctx, tx, e)
}

// userSnapshot is the audit view of an account: profile, roles and branch
// links only. It never carries the password hash or any secret (F3b-8).
func userSnapshot(u *contracts.User, roleIDs []int64, branches []contracts.BranchAccess) map[string]any {
	links := make([]map[string]any, 0, len(branches))
	for _, b := range branches {
		links = append(links, map[string]any{"branchId": b.BranchID, "isDefault": b.IsDefault})
	}
	ids := append([]int64{}, roleIDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	email := ""
	if u.Email != nil {
		email = *u.Email
	}
	return map[string]any{
		"username": u.Username, "email": email, "fullName": u.FullName, "phone": u.Phone,
		"displayTitle": u.DisplayTitle, "status": u.Status, "roleIds": ids, "branches": links,
	}
}

// userSnapshotOf reads the stored roles and branch links for the before view.
func (s *Service) userSnapshotOf(ctx context.Context, u *contracts.User) (map[string]any, error) {
	roleIDs, err := s.repo.GetRoleIDs(ctx, u.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	branches, err := s.repo.GetBranchAccess(ctx, u.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return userSnapshot(u, roleIDs, branches), nil
}

func roleSnapshot(r *contracts.Role) map[string]any {
	return map[string]any{"code": r.Code, "name": r.Name, "description": r.Description}
}

// permissionDiff lists what a replacement adds and removes (a full code list
// per role would drown the trail).
func permissionDiff(before, after []string) map[string]any {
	was := map[string]bool{}
	for _, c := range before {
		was[c] = true
	}
	now := map[string]bool{}
	added := []string{}
	for _, c := range after {
		c = strings.TrimSpace(c)
		now[c] = true
		if !was[c] {
			added = append(added, c)
		}
	}
	removed := []string{}
	for _, c := range before {
		if !now[c] {
			removed = append(removed, c)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return map[string]any{"added": added, "removed": removed, "total": len(now)}
}

// invalidateAll announces a write that can change any session's permissions.
func (s *Service) invalidateAll() {
	if s.authz != nil {
		s.authz.InvalidateAll()
	}
}

// invalidateUser announces a write scoped to one user's authorization.
func (s *Service) invalidateUser(userID int64) {
	if s.authz != nil {
		s.authz.InvalidateUser(userID)
	}
}

// managerLockKey serializes the (rare) writes that can change the pengelola
// count, so two concurrent operations cannot both observe "one left" and
// commit zero managers between each other's read and write (F1-7). The
// in-tx before/after check in the repository is the second half; the lock is
// valid because the deployment is a single instance (see doclock).
const managerLockKey = "user:managers"

// noManagerLeft maps the repository guard sentinel to a business message.
func noManagerLeft(err error) error {
	if errors.Is(err, infrastructure.ErrNoManagerLeft) {
		return apperror.Conflict("Operasi ditolak: harus tersisa minimal satu pengelola aktif")
	}
	return apperror.Internal(err)
}

// isSuperadminActor reports whether the actor may see and touch reserved
// identities. Actor 0 is the system (seed/bootstrap/tests): always allowed.
func (s *Service) isSuperadminActor(ctx context.Context, actorID int64) (bool, error) {
	if actorID == 0 {
		return true, nil
	}
	ok, err := s.repo.UserHasReservedRole(ctx, actorID)
	if err != nil {
		return false, apperror.Internal(err)
	}
	return ok, nil
}

// holderDenied returns NotFound when a non-superadmin actor addresses a
// holder of the superadmin role (F1-7: invisible, never forbidden —
// forbidden would confirm the account exists).
func (s *Service) holderDenied(ctx context.Context, actorID, userID int64) error {
	holder, err := s.repo.UserHasReservedRole(ctx, userID)
	if err != nil {
		return apperror.Internal(err)
	}
	if !holder {
		return nil
	}
	super, err := s.isSuperadminActor(ctx, actorID)
	if err != nil {
		return err
	}
	if !super {
		return apperror.NotFound("Pengguna")
	}
	return nil
}

// rejectReservedRole refuses the superadmin role through the API (F1-7:
// reserved for the default developer account). roleIDs are validated to
// exist before this runs, so unknown IDs never reach the lookup.
func (s *Service) rejectReservedRole(ctx context.Context, roleIDs []int64) error {
	for _, id := range roleIDs {
		role, err := s.repo.RoleByID(ctx, id)
		if err != nil {
			return apperror.Internal(err)
		}
		if role != nil && role.IsReserved {
			return apperror.Validation("", []apperror.FieldError{{Field: "roleIds", Message: "Role ini khusus akun pengembang default"}})
		}
	}
	return nil
}

// roleHiddenFrom returns NotFound when the role is reserved and the actor
// holds no reserved role (invisible, never forbidden).
func (s *Service) roleHiddenFrom(ctx context.Context, actorID int64, role *contracts.Role) error {
	if role == nil || !role.IsReserved {
		return nil
	}
	super, err := s.isSuperadminActor(ctx, actorID)
	if err != nil {
		return err
	}
	if !super {
		return apperror.NotFound("Role")
	}
	return nil
}

// validateProfileFields bounds the optional/length-limited profile columns.
func validateProfileFields(username, fullName, phone, title string) error {
	var fields []apperror.FieldError
	if len([]rune(username)) > 100 {
		fields = append(fields, apperror.FieldError{Field: "username", Message: "maksimal 100 karakter"})
	}
	if len([]rune(fullName)) > 255 {
		fields = append(fields, apperror.FieldError{Field: "fullName", Message: "maksimal 255 karakter"})
	}
	if len([]rune(phone)) > 30 {
		fields = append(fields, apperror.FieldError{Field: "phone", Message: "maksimal 30 karakter"})
	}
	if len([]rune(title)) > 100 {
		fields = append(fields, apperror.FieldError{Field: "displayTitle", Message: "maksimal 100 karakter"})
	}
	if len(fields) > 0 {
		return apperror.Validation("", fields)
	}
	return nil
}

// reservedRolesOf returns the reserved roles the user already holds (empty
// for every ordinary account). A holder is the hidden developer account: it
// keeps exactly these roles through UpdateUser (legacy assignRoles parity,
// F1-7), so the API can neither strip nor extend them. Identification is by
// role, never by user id.
func (s *Service) reservedRolesOf(ctx context.Context, userID int64) ([]int64, error) {
	ids, err := s.repo.GetRoleIDs(ctx, userID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var reserved []int64
	for _, id := range ids {
		role, err := s.repo.RoleByID(ctx, id)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if role != nil && role.IsReserved {
			reserved = append(reserved, id)
		}
	}
	return reserved, nil
}

// isLockedRole reports owner/superadmin: permission sets managed by the
// bootstrap, never through the API (F1-7).
func isLockedRole(code string) bool {
	for _, c := range domain.LockedRoleCodes() {
		if code == c {
			return true
		}
	}
	return false
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

// validatePassword enforces the length window on BYTES: bcrypt reads at most
// 72, and counting characters would let a long multibyte password through.
func validatePassword(password string) error {
	n := len([]byte(password))
	switch {
	case n < domain.MinPasswordLength:
		return apperror.Validation("", []apperror.FieldError{{Field: "password", Message: "minimal 8 karakter"}})
	case n > domain.MaxPasswordBytes:
		return apperror.Validation("", []apperror.FieldError{{Field: "password", Message: "maksimal 72 byte (sekitar 72 karakter; huruf non-Latin memakai lebih banyak byte)"}})
	}
	return nil
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), domain.BcryptCost)
	if err != nil {
		if errors.Is(err, bcrypt.ErrPasswordTooLong) {
			return "", validatePassword(password)
		}
		return "", err
	}
	return string(hash), nil
}

var (
	dummyHashOnce sync.Once
	dummyHash     []byte
)

// burnPasswordTime performs one bcrypt comparison against a fixed hash. It runs
// when the account does not exist or is inactive, so an attacker cannot tell
// "no such user" from "wrong password" by how long the response takes.
func burnPasswordTime(password string) {
	dummyHashOnce.Do(func() {
		dummyHash, _ = bcrypt.GenerateFromPassword([]byte("tidak-pernah-dipakai"), domain.BcryptCost)
	})
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
}

// GetByID resolves a user or returns nil when missing.
func (s *Service) GetByID(ctx context.Context, id int64) (*contracts.User, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return u, nil
}

// NamesByIDs implements contracts.UserClient.
func (s *Service) NamesByIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	names, err := s.repo.NamesByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return names, nil
}

// GetByLogin resolves a user by username OR email, or nil when missing.
func (s *Service) GetByLogin(ctx context.Context, identifier string) (*contracts.User, error) {
	u, err := s.repo.GetByLogin(ctx, identifier)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return u, nil
}

// TouchLogin stamps users.last_login_at.
func (s *Service) TouchLogin(ctx context.Context, userID int64) error {
	if err := s.repo.TouchLogin(ctx, userID); err != nil {
		return apperror.Internal(err)
	}
	return nil
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
func (s *Service) VerifyCredentials(ctx context.Context, identifier, password string) (*contracts.User, error) {
	u, err := s.repo.GetByLogin(ctx, strings.TrimSpace(identifier))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if u == nil || u.Status != "active" {
		burnPasswordTime(password)
		return nil, apperror.Unauthorized()
	}
	hash, err := s.repo.GetPasswordHash(ctx, u.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	// Over-long input is a wrong credential, never truncated and never a 500.
	if len([]byte(password)) > domain.MaxPasswordBytes {
		burnPasswordTime(password[:domain.MaxPasswordBytes])
		return nil, apperror.Unauthorized()
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, apperror.Unauthorized()
	}
	s.upgradeHashIfWeak(ctx, u.ID, hash, password)
	return u, nil
}

// upgradeHashIfWeak re-hashes at the current cost after a successful login when
// the stored hash is cheaper (accounts created at the old cost 10). It is best
// effort: a failure must never turn a correct login into an error. The update
// only applies if the stored hash is still the one just verified, so it cannot
// overwrite a password changed in between.
func (s *Service) upgradeHashIfWeak(ctx context.Context, userID int64, oldHash, password string) {
	cost, err := bcrypt.Cost([]byte(oldHash))
	if err != nil || cost >= domain.BcryptCost {
		return
	}
	fresh, err := hashPassword(password)
	if err != nil {
		return
	}
	_ = s.repo.UpgradePasswordHash(ctx, userID, oldHash, fresh)
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
	Phone    string
	// DisplayTitle is the optional job title shown next to the name.
	DisplayTitle string
	FullName     string
	Password     string
	RoleIDs      []int64
	Branches     []contracts.BranchAccess
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
	if err := validatePassword(in.Password); err != nil {
		return nil, err
	}
	if err := validateProfileFields(username, in.FullName, in.Phone, in.DisplayTitle); err != nil {
		return nil, err
	}
	if err := s.requireRolesExist(ctx, in.RoleIDs); err != nil {
		return nil, err
	}
	if err := s.rejectReservedRole(ctx, in.RoleIDs); err != nil {
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
	created := &contracts.User{
		Username: username, Email: email, FullName: strings.TrimSpace(in.FullName),
		Phone: strings.TrimSpace(in.Phone), DisplayTitle: strings.TrimSpace(in.DisplayTitle),
		Status: "active",
	}
	id, err := s.repo.CreateUser(ctx, created, hash, in.RoleIDs, branches, actorID,
		func(ctx context.Context, tx *sql.Tx, id int64) error {
			return s.logTx(ctx, tx, auditcontracts.Entry{
				Action: "users.create", Entity: "user", EntityID: id, ActorID: actorID, Note: username,
				After: userSnapshot(created, in.RoleIDs, branches),
			})
		})
	if err != nil {
		return nil, dberr.Map(err)
	}
	return s.GetByID(ctx, id)
}

// UpdateUserInput is the admin edit payload (password changes go through
// ChangePassword, never silently alongside profile edits).
type UpdateUserInput struct {
	Username     string
	Email        *string
	Phone        string
	DisplayTitle string
	FullName     string
	RoleIDs      []int64
	Branches     []contracts.BranchAccess
}

// UpdateUser rewrites profile, roles, and branch links in one transaction.
// Reserved holders are invisible to non-superadmin actors (404) and keep
// exactly their reserved roles; the write is refused when it would leave
// zero managers (F1-7).
func (s *Service) UpdateUser(ctx context.Context, actorID, userID int64, in UpdateUserInput) (*contracts.User, error) {
	if err := s.holderDenied(ctx, actorID, userID); err != nil {
		return nil, err
	}
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
	if err := validateProfileFields(username, in.FullName, in.Phone, in.DisplayTitle); err != nil {
		return nil, err
	}
	if err := s.requireRolesExist(ctx, in.RoleIDs); err != nil {
		return nil, err
	}
	// A reserved holder keeps exactly its reserved roles; every other user
	// gets the requested roles, which may never include a reserved one.
	roleIDs := in.RoleIDs
	reserved, err := s.reservedRolesOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(reserved) > 0 {
		roleIDs = reserved
	} else if err := s.rejectReservedRole(ctx, roleIDs); err != nil {
		return nil, err
	}
	branches, err := normalizeBranches(in.Branches)
	if err != nil {
		return nil, err
	}
	before, err := s.userSnapshotOf(ctx, existing)
	if err != nil {
		return nil, err
	}
	existing.Username = username
	existing.Email = email
	existing.FullName = strings.TrimSpace(in.FullName)
	existing.Phone = strings.TrimSpace(in.Phone)
	existing.DisplayTitle = strings.TrimSpace(in.DisplayTitle)
	updateErr := doclock.Lock(managerLockKey, func() error {
		return s.repo.UpdateUser(ctx, existing, roleIDs, branches, actorID, true,
			func(ctx context.Context, tx *sql.Tx) error {
				return s.logTx(ctx, tx, auditcontracts.Entry{
					Action: "users.update", Entity: "user", EntityID: userID, ActorID: actorID, Note: username,
					Before: before, After: userSnapshot(existing, roleIDs, branches),
				})
			})
	})
	if updateErr != nil {
		if errors.Is(updateErr, infrastructure.ErrNoManagerLeft) {
			return nil, noManagerLeft(updateErr)
		}
		return nil, dberr.Map(updateErr)
	}
	// Roles and branch access just changed for this user.
	s.invalidateUser(userID)
	return s.GetByID(ctx, userID)
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
// Reserved holders are invisible to non-superadmin actors (404); deactivating
// the last pengelola is refused with 409 (F1-7).
func (s *Service) SetStatus(ctx context.Context, actorID, userID int64, status string) error {
	if status != "active" && status != "inactive" {
		return apperror.Validation("", []apperror.FieldError{{Field: "status", Message: "harus active atau inactive"}})
	}
	if err := s.holderDenied(ctx, actorID, userID); err != nil {
		return err
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
	guard := status == "inactive"
	write := func() error {
		return s.repo.SetStatus(ctx, userID, status, actorID, guard,
			func(ctx context.Context, tx *sql.Tx) error {
				return s.logTx(ctx, tx, auditcontracts.Entry{
					Action: "users.status", Entity: "user", EntityID: userID, ActorID: actorID, Note: status,
					Before: map[string]any{"status": existing.Status}, After: map[string]any{"status": status},
				})
			})
	}
	var statusErr error
	if guard {
		statusErr = doclock.Lock(managerLockKey, write)
	} else {
		statusErr = write()
	}
	if statusErr != nil {
		if errors.Is(statusErr, infrastructure.ErrNoManagerLeft) {
			return noManagerLeft(statusErr)
		}
		return apperror.Internal(statusErr)
	}
	// A deactivated user must stop passing authentication immediately, and a
	// reactivated user must not resurrect pre-deactivation sessions: revoke
	// every session in both directions (F1-4). Cache invalidation alone only
	// retires the resolved copy; the rows stay valid for Verify otherwise.
	s.invalidateUser(userID)
	if s.revoker != nil {
		if _, err := s.revoker.RevokeUserSessions(ctx, userID, 0); err != nil {
			return apperror.Internal(err)
		}
	}
	return nil
}

// ChangePassword replaces the credential after a length check. Every session
// of the target dies, except the actor's own when changing their own
// password (legacy retainedCurrentSession parity, F1-4): a stolen session
// must not survive a password change.
func (s *Service) ChangePassword(ctx context.Context, actorID, userID int64, password string) error {
	if err := validatePassword(password); err != nil {
		return err
	}
	existing, err := s.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if existing == nil {
		return apperror.NotFound("Pengguna")
	}
	if err := s.holderDenied(ctx, actorID, userID); err != nil {
		return err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return apperror.Internal(err)
	}
	// The trail never carries the credential or its hash (F3b-8).
	if err := s.repo.SetPasswordHash(ctx, userID, hash, actorID, func(ctx context.Context, tx *sql.Tx) error {
		return s.logTx(ctx, tx, auditcontracts.Entry{
			Action: "users.password", Entity: "user", EntityID: userID, ActorID: actorID,
			Metadata: map[string]any{"self": actorID == userID},
		})
	}); err != nil {
		return apperror.Internal(err)
	}
	s.invalidateUser(userID)
	if s.revoker != nil {
		var except int64
		if actorID == userID {
			if sess := authcontracts.SessionFromContext(ctx); sess != nil {
				except = sess.ID
			}
		}
		if _, err := s.revoker.RevokeUserSessions(ctx, userID, except); err != nil {
			return apperror.Internal(err)
		}
	}
	return nil
}

// RoleRef is a role label shown on a listed user.
type RoleRef struct {
	ID   int64
	Name string
}

// BranchRef is a branch label shown on a listed user.
type BranchRef struct {
	ID        int64
	Name      string
	IsDefault bool
}

// UserListMeta decorates one listed user with roles and branches.
type UserListMeta struct {
	Roles    []RoleRef
	Branches []BranchRef
}

// ListUsersResult is a paginated user page.
type ListUsersResult struct {
	Users []*contracts.User
	Total int64
	// Meta is keyed by user id; every listed user has an entry.
	Meta map[int64]*UserListMeta
}

// ListUsers searches per word across username/full_name/email (KI-25:
// word-based like the other modules, not whole-phrase). Holders of the
// superadmin role are listed only for superadmin actors (F1-7).
//
// Roles/branches decorate the page in a constant number of reads (U-01): one
// UNION query over the link tables plus one branch.GetByIDs batch — never one
// lookup per row.
func (s *Service) ListUsers(ctx context.Context, actorID int64, search, status string, page, limit int) (*ListUsersResult, error) {
	super, err := s.isSuperadminActor(ctx, actorID)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountUsers(ctx, search, status, !super)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	users, err := s.repo.ListUsers(ctx, search, status, limit, (page-1)*limit, !super)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if users == nil {
		users = []*contracts.User{}
	}
	meta, err := s.decorateUsers(ctx, users)
	if err != nil {
		return nil, err
	}
	return &ListUsersResult{Users: users, Total: total, Meta: meta}, nil
}

func (s *Service) decorateUsers(ctx context.Context, users []*contracts.User) (map[int64]*UserListMeta, error) {
	meta := make(map[int64]*UserListMeta, len(users))
	ids := make([]int64, 0, len(users))
	for _, u := range users {
		meta[u.ID] = &UserListMeta{Roles: []RoleRef{}, Branches: []BranchRef{}}
		ids = append(ids, u.ID)
	}
	rows, err := s.repo.ListMetaByUsers(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	branchIDs := make([]int64, 0)
	seen := map[int64]bool{}
	for _, r := range rows {
		m := meta[r.UserID]
		if m == nil {
			continue
		}
		if r.Kind == "role" {
			m.Roles = append(m.Roles, RoleRef{ID: r.RefID, Name: r.RoleName})
			continue
		}
		m.Branches = append(m.Branches, BranchRef{ID: r.RefID, IsDefault: r.IsDefault})
		if !seen[r.RefID] {
			seen[r.RefID] = true
			branchIDs = append(branchIDs, r.RefID)
		}
	}
	if s.branches == nil || len(branchIDs) == 0 {
		return meta, nil
	}
	// Names are decoration: a branch lookup failure must not break the list.
	found, err := s.branches.GetByIDs(ctx, branchIDs)
	if err != nil {
		return meta, nil
	}
	names := make(map[int64]string, len(found))
	for _, b := range found {
		names[b.ID] = b.Name
	}
	for _, m := range meta {
		kept := m.Branches[:0]
		for _, b := range m.Branches {
			name, ok := names[b.ID]
			if !ok {
				continue // orphan access row: not offered (as in login)
			}
			b.Name = name
			kept = append(kept, b)
		}
		m.Branches = kept
	}
	return meta, nil
}

// GetUser resolves one account for the admin UI, hiding reserved holders
// from non-superadmin actors (F1-7). Cross-module callers keep using the
// unfiltered UserClient.GetByID (auth must resolve every session).
func (s *Service) GetUser(ctx context.Context, actorID, userID int64) (*contracts.User, error) {
	if err := s.holderDenied(ctx, actorID, userID); err != nil {
		return nil, err
	}
	u, err := s.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, apperror.NotFound("Pengguna")
	}
	return u, nil
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
	return contracts.PermissionCatalog()
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

// ListRoles is the admin-UI role list: the superadmin role is hidden from
// non-superadmin sessions (F1-7). The contractual List stays unfiltered.
func (s *Service) ListRoles(ctx context.Context, actorID int64) ([]*contracts.Role, error) {
	roles, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	super, err := s.isSuperadminActor(ctx, actorID)
	if err != nil {
		return nil, err
	}
	if super {
		return roles, nil
	}
	out := roles[:0]
	for _, r := range roles {
		if !r.IsReserved {
			out = append(out, r)
		}
	}
	return out, nil
}

// RoleDetail is one role with its permission codes (F-02).
type RoleDetail struct {
	Role        *contracts.Role
	Permissions []string
	// Locked marks owner/superadmin: their set is bootstrap-managed, read-only here.
	Locked bool
}

// GetRoleDetail returns a role and its current permission codes. A reserved
// role is invisible (404) to actors who do not hold one, as in ListRoles.
func (s *Service) GetRoleDetail(ctx context.Context, actorID, id int64) (*RoleDetail, error) {
	role, err := s.GetRole(ctx, id)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, apperror.NotFound("Role")
	}
	if err := s.roleHiddenFrom(ctx, actorID, role); err != nil {
		return nil, err
	}
	codes, err := s.GetPermissionCodes(ctx, []int64{id})
	if err != nil {
		return nil, err
	}
	sort.Strings(codes)
	return &RoleDetail{Role: role, Permissions: codes, Locked: isLockedRole(role.Code)}, nil
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
	// The reserved code can never be taken through the API, even on a fresh
	// database where it does not exist yet (F1-7).
	if code == domain.ReservedSuperadminRole {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "code", Message: "Role Super Admin hanya untuk akun developer default"}})
	}
	if existing, err := s.repo.RoleByCode(ctx, code); err != nil {
		return nil, apperror.Internal(err)
	} else if existing != nil {
		return nil, apperror.Conflict("Kode role '" + code + "' sudah digunakan")
	}
	newRole := &contracts.Role{Code: code, Name: strings.TrimSpace(in.Name), Description: in.Description}
	id, err := s.repo.CreateRole(ctx, newRole, func(ctx context.Context, tx *sql.Tx, id int64) error {
		return s.logTx(ctx, tx, auditcontracts.Entry{
			Action: "roles.create", Entity: "role", EntityID: id, ActorID: actorID, Note: code,
			After: roleSnapshot(newRole),
		})
	})
	if err != nil {
		return nil, dberr.Map(err)
	}
	return s.GetRole(ctx, id)
}

// UpdateRole rewrites name/description. Code is immutable. System roles
// cannot be edited through the API (F1-7: seeded names stay canonical).
func (s *Service) UpdateRole(ctx context.Context, actorID, id int64, name, description string) (*contracts.Role, error) {
	role, err := s.GetRole(ctx, id)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, apperror.NotFound("Role")
	}
	if err := s.roleHiddenFrom(ctx, actorID, role); err != nil {
		return nil, err
	}
	if role.IsSystem {
		return nil, apperror.Forbidden()
	}
	beforeRole := roleSnapshot(role)
	role.Name = strings.TrimSpace(name)
	role.Description = description
	if err := s.repo.UpdateRole(ctx, role, func(ctx context.Context, tx *sql.Tx) error {
		return s.logTx(ctx, tx, auditcontracts.Entry{
			Action: "roles.update", Entity: "role", EntityID: id, ActorID: actorID, Note: role.Code,
			Before: beforeRole, After: roleSnapshot(role),
		})
	}); err != nil {
		return nil, apperror.Internal(err)
	}
	return s.GetRole(ctx, id)
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
	if err := s.roleHiddenFrom(ctx, actorID, role); err != nil {
		return err
	}
	if role.IsSystem {
		return apperror.Forbidden()
	}
	if inUse, err := s.repo.RoleInUse(ctx, id); err != nil {
		return apperror.Internal(err)
	} else if inUse {
		return apperror.Conflict("Role masih dipakai oleh pengguna dan tidak dapat dihapus")
	}
	if err := s.repo.DeleteRole(ctx, id, func(ctx context.Context, tx *sql.Tx) error {
		return s.logTx(ctx, tx, auditcontracts.Entry{
			Action: "roles.delete", Entity: "role", EntityID: id, ActorID: actorID, Note: role.Code,
			Before: roleSnapshot(role),
		})
	}); err != nil {
		return apperror.Internal(err)
	}
	s.invalidateAll()
	return nil
}

// SetRolePermissions replaces the role's permission set. Unknown codes are
// rejected before anything is deleted (KI-27 class: no stranded empty role).
// Owner/superadmin sets are bootstrap-managed (403 here); the replacement is
// refused when it would leave zero managers (F1-7, 409).
func (s *Service) SetRolePermissions(ctx context.Context, actorID, roleID int64, codes []string) error {
	role, err := s.GetRole(ctx, roleID)
	if err != nil {
		return err
	}
	if role == nil {
		return apperror.NotFound("Role")
	}
	if err := s.roleHiddenFrom(ctx, actorID, role); err != nil {
		return err
	}
	if isLockedRole(role.Code) {
		return apperror.Forbidden()
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
	oldCodes, err := s.repo.GetPermissionCodes(ctx, []int64{roleID})
	if err != nil {
		return apperror.Internal(err)
	}
	permErr := doclock.Lock(managerLockKey, func() error {
		return s.repo.SetRolePermissions(ctx, roleID, ids, true, func(ctx context.Context, tx *sql.Tx) error {
			return s.logTx(ctx, tx, auditcontracts.Entry{
				Action: "roles.permissions", Entity: "role", EntityID: roleID, ActorID: actorID, Note: role.Code,
				Metadata: permissionDiff(oldCodes, codes),
			})
		})
	})
	if permErr != nil {
		if errors.Is(permErr, infrastructure.ErrNoManagerLeft) {
			return noManagerLeft(permErr)
		}
		return apperror.Internal(permErr)
	}
	// Every session holding this role has different permissions now.
	s.invalidateAll()
	return nil
}

// SyncRolePermissions replaces the set without the API guards (F1-6:
// composition-root sync of owner/superadmin to the full catalog). It still
// serializes against manager-changing writes; callers must be the bootstrap
// or the seed, never an HTTP handler.
func (s *Service) SyncRolePermissions(ctx context.Context, actorID, roleID int64, codes []string) error {
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
	oldCodes, err := s.repo.GetPermissionCodes(ctx, []int64{roleID})
	if err != nil {
		return apperror.Internal(err)
	}
	permErr := doclock.Lock(managerLockKey, func() error {
		return s.repo.SetRolePermissions(ctx, roleID, ids, false, func(ctx context.Context, tx *sql.Tx) error {
			return s.logTx(ctx, tx, auditcontracts.Entry{
				Action: "roles.permissions", Entity: "role", EntityID: roleID, ActorID: actorID, Note: role.Code,
				Metadata: permissionDiff(oldCodes, codes),
			})
		})
	})
	if permErr != nil {
		return apperror.Internal(permErr)
	}
	s.invalidateAll()
	return nil
}

// EnsureSystemRole creates the role with is_system=1 when missing, or flips
// an existing row to system (F1-6: upgrades roles from the pre-bootstrap
// seed). Never called from HTTP handlers — API-created roles stay custom.
func (s *Service) EnsureSystemRole(ctx context.Context, code, name, description string) (*contracts.Role, error) {
	role, err := s.repo.RoleByCode(ctx, code)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if role == nil {
		id, err := s.repo.CreateRole(ctx, &contracts.Role{
			Code: code, Name: name, Description: description, IsSystem: true,
			IsReserved: code == domain.ReservedSuperadminRole,
		})
		if err != nil {
			return nil, dberr.Map(err)
		}
		return s.GetRole(ctx, id)
	}
	if !role.IsSystem {
		if err := s.repo.SetRoleSystem(ctx, code); err != nil {
			return nil, apperror.Internal(err)
		}
		role.IsSystem = true
	}
	if code == domain.ReservedSuperadminRole && !role.IsReserved {
		if err := s.repo.SetRoleReserved(ctx, code); err != nil {
			return nil, apperror.Internal(err)
		}
		role.IsReserved = true
	}
	return role, nil
}

// AddRolePermissions attaches codes without removing existing ones (F1-6
// grant-once defaults). Unknown codes are rejected before any insert.
func (s *Service) AddRolePermissions(ctx context.Context, actorID, roleID int64, codes []string) error {
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
	if err := s.repo.AddRolePermissions(ctx, roleID, ids); err != nil {
		return apperror.Internal(err)
	}
	s.invalidateAll()
	return nil
}

// SeedGranted reports which grant-once defaults a role already received.
func (s *Service) SeedGranted(ctx context.Context, roleCode string) (map[string]bool, error) {
	out, err := s.repo.SeedGranted(ctx, roleCode)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if out == nil {
		out = map[string]bool{}
	}
	return out, nil
}

// MarkSeedGranted records one grant-once default as applied.
func (s *Service) MarkSeedGranted(ctx context.Context, roleCode, permCode string) error {
	if err := s.repo.MarkSeedGranted(ctx, roleCode, permCode); err != nil {
		return apperror.Internal(err)
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
