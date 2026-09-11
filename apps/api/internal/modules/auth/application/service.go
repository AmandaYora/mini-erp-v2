package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"log/slog"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/auth/infrastructure"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	companycontracts "mini-erp/internal/modules/company/contracts"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/apperror"
)

// AbsoluteSessionLifetime caps every session at 30 days of wall-clock life
// from creation (OQ-A02): sliding refresh never extends past this point.
const AbsoluteSessionLifetime = 30 * 24 * time.Hour

// loginAttemptsPerMinute bounds password guessing per IP+username (OQ-A04).
const loginAttemptsPerMinute = 10

// Service implements sessions, JWT issuance, and permission resolution.
type Service struct {
	repo     *infrastructure.Repository
	users    usercontracts.UserClient
	roles    usercontracts.RoleClient
	branches branchcontracts.BranchClient
	company  companycontracts.CompanyClient
	jwtKey   []byte
	access   time.Duration
	refresh  time.Duration
	log      *slog.Logger
	limiter  *rateLimiter
	audit    auditcontracts.AuditClient
}

// NewService wires auth use cases. All cross-module links are contracts.
func NewService(
	repo *infrastructure.Repository,
	users usercontracts.UserClient,
	roles usercontracts.RoleClient,
	branches branchcontracts.BranchClient,
	company companycontracts.CompanyClient,
	jwtSecret string,
	accessTTL, refreshTTL time.Duration,
	log *slog.Logger,
	audit auditcontracts.AuditClient,
) *Service {
	return &Service{
		repo: repo, users: users, roles: roles, branches: branches, company: company,
		jwtKey: []byte(jwtSecret), access: accessTTL, refresh: refreshTTL,
		log: log, limiter: newRateLimiter(), audit: audit,
	}
}

// appErr passes *apperror.AppError through and wraps any foreign error as
// internal, so contract boundaries can never panic on type assertion.
func appErr(err error) *apperror.AppError {
	if err == nil {
		return nil
	}
	if e, ok := err.(*apperror.AppError); ok {
		return e
	}
	return apperror.Internal(err)
}

// --- rate limiter (in-memory, per instance) --------------------------------

type bucket struct {
	count int
	reset time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{buckets: map[string]*bucket{}}
}

// Allow reports whether key may proceed (10 events per rolling minute).
// The map resets past 10k entries so a flood of distinct keys cannot grow
// memory without bound.
func (l *rateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.buckets) > 10000 {
		l.buckets = map[string]*bucket{}
	}
	now := time.Now()
	b, ok := l.buckets[key]
	if !ok || now.After(b.reset) {
		l.buckets[key] = &bucket{count: 1, reset: now.Add(time.Minute)}
		return true
	}
	b.count++
	return b.count <= loginAttemptsPerMinute
}

// --- tokens ----------------------------------------------------------------

func (s *Service) signAccess(userID, sessionID int64) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"sid": sessionID,
		"exp": time.Now().Add(s.access).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtKey)
}

func (s *Service) parseAccess(token string) (userID, sessionID int64, err error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return s.jwtKey, nil
	})
	if err != nil || !parsed.Valid {
		return 0, 0, jwt.ErrSignatureInvalid
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return 0, 0, jwt.ErrSignatureInvalid
	}
	sub, _ := claims["sub"].(float64)
	sid, _ := claims["sid"].(float64)
	if sub == 0 || sid == 0 {
		return 0, 0, jwt.ErrSignatureInvalid
	}
	return int64(sub), int64(sid), nil
}

func newRefreshToken() (token, hash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token = hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	return token, hex.EncodeToString(sum[:]), nil
}

func hashOf(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// --- login -----------------------------------------------------------------

// BranchOption is one selectable branch at login.
type BranchOption struct {
	ID     int64  `json:"id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// LoginResult is the login response payload.
type LoginResult struct {
	AccessToken          string         `json:"accessToken"`
	RefreshToken         string         `json:"refreshToken"`
	User                 map[string]any `json:"user"`
	Branch               any            `json:"branch"`
	RequiresBranchSelect bool           `json:"requiresBranchSelection"`
	Branches             []BranchOption `json:"branches,omitempty"`
}

// Login authenticates and opens a session. Branch resolution: exactly one
// accessible active branch auto-selects; several with a marked default pick
// the default; otherwise the client must call switch-branch (no dead ends:
// zero branches is a clear 403, KI-01).
func (s *Service) Login(ctx context.Context, username, password, ip string) (*LoginResult, *apperror.AppError) {
	if !s.limiter.Allow(ip + "|" + username) {
		return nil, apperror.RateLimited()
	}
	u, err := s.users.VerifyCredentials(ctx, username, password)
	if err != nil {
		// Failed attempts are logged for intrusion review (OQ-A03); the
		// persistent trail arrives with the audit module.
		s.log.Warn("login failed", "username", username, "ip", ip)
		if s.audit != nil {
			_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "auth.login_failed", Entity: "user", BranchID: 0, ActorID: 0, Note: username})
		}
		return nil, appErr(err)
	}
	roleIDs, err := s.users.GetRoleIDs(ctx, u.ID)
	if err != nil {
		return nil, appErr(err)
	}
	if len(roleIDs) == 0 {
		return nil, apperror.Unauthorized()
	}
	access, err := s.users.GetBranchAccess(ctx, u.ID)
	if err != nil {
		return nil, appErr(err)
	}
	ids := make([]int64, 0, len(access))
	defID := int64(0)
	for _, a := range access {
		ids = append(ids, a.BranchID)
		if a.IsDefault {
			defID = a.BranchID
		}
	}
	branches, err := s.branches.GetByIDs(ctx, ids)
	if err != nil {
		return nil, appErr(err)
	}
	active := branches[:0]
	for _, b := range branches {
		if b.Status == "active" {
			active = append(active, b)
		}
	}
	if len(active) == 0 {
		return nil, &apperror.AppError{
			Code:    apperror.CodeForbidden,
			Message: "Akun Anda belum memiliki akses cabang aktif. Hubungi admin.",
		}
	}

	var branchID int64
	needsSelect := false
	switch {
	case len(active) == 1:
		branchID = active[0].ID
	default:
		found := false
		for _, b := range active {
			if b.ID == defID {
				branchID, found = b.ID, true
				break
			}
		}
		if !found {
			needsSelect = true
		}
	}

	now := time.Now().UTC()
	token, hash, err := newRefreshToken()
	if err != nil {
		return nil, apperror.Internal(err)
	}
	roleID := roleIDs[0]
	sessionID, err := s.repo.CreateSession(ctx, &infrastructure.SessionRecord{
		UserID:            u.ID,
		RefreshHash:       hash,
		ActiveBranchID:    toNullInt(branchID),
		ActiveRoleID:      toNullInt(roleID),
		ExpiresAt:         now.Add(s.refresh),
		AbsoluteExpiresAt: now.Add(AbsoluteSessionLifetime),
	})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	signed, err := s.signAccess(u.ID, sessionID)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	res := &LoginResult{
		AccessToken:          signed,
		RefreshToken:         token,
		User:                 userView(u, roleIDs),
		RequiresBranchSelect: needsSelect,
	}
	for _, b := range active {
		opt := BranchOption{ID: b.ID, Code: b.Code, Name: b.Name, Status: b.Status}
		res.Branches = append(res.Branches, opt)
		if b.ID == branchID {
			res.Branch = opt
		}
	}
	if res.Branches == nil {
		res.Branches = []BranchOption{}
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "auth.login", Entity: "user", EntityID: u.ID, BranchID: branchID, ActorID: u.ID, Note: username})
	}
	return res, nil
}

func toNullInt(v int64) sql.NullInt64 {
	if v == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: v, Valid: true}
}

func userView(u *usercontracts.User, roleIDs []int64) map[string]any {
	var email any
	if u.Email != nil {
		email = *u.Email
	}
	return map[string]any{
		"id": u.ID, "username": u.Username, "email": email,
		"fullName": u.FullName, "roleIds": roleIDs,
	}
}

// --- refresh / logout ------------------------------------------------------

// RefreshResult carries the rotated token pair.
type RefreshResult struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// Refresh rotates the refresh token. A reused (previous) hash means replay:
// the whole session dies and every sibling session is revoked.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*RefreshResult, *apperror.AppError) {
	rec, err := s.repo.GetSessionByRefreshHash(ctx, hashOf(refreshToken))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	now := time.Now().UTC()
	if rec == nil {
		// Unknown hash: it may be a rotated-out token replayed by an
		// attacker holding a stolen copy. Kill the session either way.
		if prev, err := s.repo.GetSessionByPrevHash(ctx, hashOf(refreshToken)); err != nil {
			return nil, apperror.Internal(err)
		} else if prev != nil && !prev.RevokedAt.Valid {
			s.log.Warn("refresh replay detected", "user", prev.UserID, "session", prev.ID)
			_ = s.repo.RevokeUserSessions(ctx, prev.UserID)
		}
		return nil, apperror.Unauthorized()
	}
	if rec.RevokedAt.Valid || now.After(rec.ExpiresAt) {
		return nil, apperror.Unauthorized()
	}
	if now.After(rec.AbsoluteExpiresAt) {
		_ = s.repo.RevokeSession(ctx, rec.ID)
		return nil, apperror.Unauthorized()
	}
	token, hash, err := newRefreshToken()
	if err != nil {
		return nil, apperror.Internal(err)
	}
	expiry := now.Add(s.refresh)
	if expiry.After(rec.AbsoluteExpiresAt) {
		expiry = rec.AbsoluteExpiresAt
	}
	if err := s.repo.Rotate(ctx, rec.ID, hash, expiry); err != nil {
		return nil, apperror.Internal(err)
	}
	signed, err := s.signAccess(rec.UserID, rec.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &RefreshResult{AccessToken: signed, RefreshToken: token}, nil
}

// Logout revokes the caller's session.
func (s *Service) Logout(ctx context.Context, sess *contracts.Session) *apperror.AppError {
	if err := s.repo.RevokeSession(ctx, sess.ID); err != nil {
		return apperror.Internal(err)
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "auth.logout", Entity: "user", EntityID: sess.UserID, BranchID: sess.BranchID, ActorID: sess.UserID})
	}
	return nil
}

// --- me / switch -----------------------------------------------------------

// Me resolves the full identity: user, active role, active branch (may be
// unselected), fresh permissions, and the company profile.
func (s *Service) Me(ctx context.Context, sess *contracts.Session) (map[string]any, *apperror.AppError) {
	u, err := s.users.GetByID(ctx, sess.UserID)
	if err != nil {
		return nil, appErr(err)
	}
	if u == nil || u.Status != "active" {
		return nil, apperror.Unauthorized()
	}
	roleIDs, err := s.users.GetRoleIDs(ctx, u.ID)
	if err != nil {
		return nil, appErr(err)
	}
	perms, err := s.roles.GetPermissionCodes(ctx, roleIDs)
	if err != nil {
		return nil, appErr(err)
	}
	var branch any
	if sess.BranchID != 0 {
		if b, err := s.branches.GetByID(ctx, sess.BranchID); err != nil {
			return nil, appErr(err)
		} else if b != nil {
			branch = BranchOption{ID: b.ID, Code: b.Code, Name: b.Name, Status: b.Status}
		}
	}
	profile, err := s.company.GetProfile(ctx)
	if err != nil {
		return nil, appErr(err)
	}
	var company any
	if profile != nil {
		company = map[string]any{"name": profile.Name, "city": profile.City, "phone": profile.Phone}
	}
	return map[string]any{
		"user":        userView(u, roleIDs),
		"branch":      branch,
		"permissions": perms,
		"company":     company,
	}, nil
}

// SwitchBranch moves the session to another accessible, active branch.
// Inactive or unlisted branches are a business 403 with a clear message
// (KI-02), never a 500.
func (s *Service) SwitchBranch(ctx context.Context, sess *contracts.Session, branchID int64) (string, *apperror.AppError) {
	access, err := s.users.GetBranchAccess(ctx, sess.UserID)
	if err != nil {
		return "", appErr(err)
	}
	allowed := false
	for _, a := range access {
		if a.BranchID == branchID {
			allowed = true
			break
		}
	}
	b, err := s.branches.GetByID(ctx, branchID)
	if err != nil {
		return "", appErr(err)
	}
	if !allowed || b == nil {
		return "", &apperror.AppError{Code: apperror.CodeForbidden, Message: "Cabang tidak dapat diakses"}
	}
	if b.Status != "active" {
		return "", &apperror.AppError{Code: apperror.CodeForbidden, Message: "Cabang " + b.Name + " sudah tidak aktif"}
	}
	if err := s.repo.UpdateBranchRole(ctx, sess.ID, branchID, sess.RoleID); err != nil {
		return "", apperror.Internal(err)
	}
	signed, err := s.signAccess(sess.UserID, sess.ID)
	if err != nil {
		return "", apperror.Internal(err)
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "auth.switch_branch", Entity: "branch", EntityID: branchID, BranchID: branchID, ActorID: sess.UserID})
	}
	return signed, nil
}

// SwitchRole moves the session to another of the user's own roles.
func (s *Service) SwitchRole(ctx context.Context, sess *contracts.Session, roleID int64) (string, *apperror.AppError) {
	roleIDs, err := s.users.GetRoleIDs(ctx, sess.UserID)
	if err != nil {
		return "", appErr(err)
	}
	held := false
	for _, id := range roleIDs {
		if id == roleID {
			held = true
			break
		}
	}
	if !held {
		return "", &apperror.AppError{Code: apperror.CodeForbidden, Message: "Role tidak dapat dipakai"}
	}
	if err := s.repo.UpdateBranchRole(ctx, sess.ID, sess.BranchID, roleID); err != nil {
		return "", apperror.Internal(err)
	}
	signed, err := s.signAccess(sess.UserID, sess.ID)
	if err != nil {
		return "", apperror.Internal(err)
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "auth.switch_role", Entity: "role", EntityID: roleID, BranchID: sess.BranchID, ActorID: sess.UserID})
	}
	return signed, nil
}

// --- middleware support ----------------------------------------------------

// Verify implements contracts.SessionProvider: token → row → live user →
// fresh permissions. Role drift (active role revoked mid-session, KI-05b)
// falls back to the first current role instead of stranding the session.
func (s *Service) Verify(ctx context.Context, accessToken string) (*contracts.Session, error) {
	userID, sessionID, err := s.parseAccess(accessToken)
	if err != nil {
		return nil, contracts.ErrSessionInvalid
	}
	now := time.Now().UTC()
	rec, err := s.repo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if rec == nil || rec.UserID != userID ||
		rec.RevokedAt.Valid || now.After(rec.ExpiresAt) || now.After(rec.AbsoluteExpiresAt) {
		return nil, contracts.ErrSessionInvalid
	}
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u == nil || u.Status != "active" {
		return nil, contracts.ErrSessionInvalid
	}
	roleIDs, err := s.users.GetRoleIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(roleIDs) == 0 {
		return nil, contracts.ErrSessionInvalid
	}
	roleID := rec.ActiveRoleID.Int64
	held := false
	for _, id := range roleIDs {
		if id == roleID {
			held = true
			break
		}
	}
	if !held {
		roleID = roleIDs[0]
	}
	perms, err := s.roles.GetPermissionCodes(ctx, roleIDs)
	if err != nil {
		return nil, err
	}
	role, err := s.roles.GetRole(ctx, roleID)
	if err != nil {
		return nil, err
	}
	code := ""
	if role != nil {
		code = role.Code
	}
	branchActive := true
	if branchID := rec.ActiveBranchID.Int64; branchID != 0 {
		// A branch closed after login stops authorizing business routes
		// (switch-branch stays open as the way out).
		if b, err := s.branches.GetByID(ctx, branchID); err != nil {
			return nil, err
		} else if b == nil || b.Status != "active" {
			branchActive = false
		}
	}
	return &contracts.Session{
		ID: sessionID, UserID: userID,
		BranchID: rec.ActiveBranchID.Int64, BranchActive: branchActive,
		RoleID: roleID, RoleCode: code, Permissions: perms,
	}, nil
}

// HasPermission implements contracts.PermissionChecker against the session
// already in context.
func (s *Service) HasPermission(ctx context.Context, perm string) bool {
	return contracts.SessionFromContext(ctx).HasPermission(perm)
}
