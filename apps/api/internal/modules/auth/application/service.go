package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/auth/infrastructure"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	companycontracts "mini-erp/internal/modules/company/contracts"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/ratelimit"
	"mini-erp/internal/shared/reqctx"
)

// AbsoluteSessionLifetime caps every session at 30 days of wall-clock life
// from creation (OQ-A02): sliding refresh never extends past this point.
const AbsoluteSessionLifetime = 30 * 24 * time.Hour

// RefreshReuseGrace tolerates concurrent refresh from two tabs (F1-10, A-21):
// reuse of the previous hash within this window is a race, not an attack.
const RefreshReuseGrace = 30 * time.Second

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
	limiter  *ratelimit.Limiter
	userLim  *ratelimit.Limiter
	audit    auditcontracts.AuditClient
	cache    *sessionCache
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
		log: log, limiter: ratelimit.New(10, time.Minute), userLim: ratelimit.New(30, time.Minute), audit: audit,
		cache: newSessionCache(sessionCacheTTL),
	}
}

// InvalidateAll implements contracts.AuthzInvalidator: retire every resolved
// session after a write whose blast radius is all of them.
func (s *Service) InvalidateAll() { s.cache.bump() }

// InvalidateUser implements contracts.AuthzInvalidator: retire one user's
// resolved sessions after a write scoped to that user.
func (s *Service) InvalidateUser(userID int64) { s.cache.dropUser(userID) }

// RevokeUserSessions implements contracts.SessionRevoker (F1-4).
func (s *Service) RevokeUserSessions(ctx context.Context, userID, exceptSessionID int64) (int64, error) {
	n, err := s.repo.RevokeUserSessionsExcept(ctx, userID, exceptSessionID)
	if err != nil {
		return 0, err
	}
	s.cache.dropUser(userID)
	return n, nil
}

// Revoker exposes the SessionRevoker for the user module via main.go.
func (s *Service) Revoker() contracts.SessionRevoker { return s }

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
	ActiveRole           *RoleOption    `json:"activeRole,omitempty"`
	AvailableRoles       []RoleOption   `json:"availableRoles"`
}

// RoleOption is one role the user holds (F3b-1: switch-role UI).
type RoleOption struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// LoginMeta carries request facts recorded in the login trail.
type LoginMeta struct {
	IP        string
	UserAgent string
}

// roleOptions resolves the user's held roles in one grouped read, id-asc.
func (s *Service) roleOptions(ctx context.Context, roleIDs []int64) ([]RoleOption, error) {
	all, err := s.roles.List(ctx)
	if err != nil {
		return nil, err
	}
	held := make(map[int64]bool, len(roleIDs))
	for _, id := range roleIDs {
		held[id] = true
	}
	out := []RoleOption{}
	for _, r := range all {
		if held[r.ID] {
			out = append(out, RoleOption{ID: r.ID, Code: r.Code, Name: r.Name})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func pickRole(opts []RoleOption, id int64) *RoleOption {
	for i := range opts {
		if opts[i].ID == id {
			return &opts[i]
		}
	}
	return nil
}

func truncate(v string, n int) string {
	if len(v) <= n {
		return v
	}
	return strings.ToValidUTF8(v[:n], "")
}

// Login authenticates and opens a session. Branch resolution: exactly one
// accessible active branch auto-selects; several with a marked default pick
// the default; otherwise the client must call switch-branch (no dead ends:
// zero branches is a clear 403, KI-01).
func (s *Service) Login(ctx context.Context, username, password, ip string) (*LoginResult, *apperror.AppError) {
	return s.LoginWithMeta(ctx, username, password, LoginMeta{IP: ip})
}

// LoginWithMeta is Login with the login trail (user agent + IP). The
// identifier is a username OR an email (F3b-1); the failure answer is the
// same either way so nothing leaks which one matched.
func (s *Service) LoginWithMeta(ctx context.Context, identifier, password string, meta LoginMeta) (*LoginResult, *apperror.AppError) {
	username, ip := strings.TrimSpace(identifier), meta.IP
	if !s.limiter.Allow(ip+"|"+strings.ToLower(username)) || !s.userLim.Allow("u|"+strings.ToLower(username)) {
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
	roleOpts, err := s.roleOptions(ctx, roleIDs)
	if err != nil {
		return nil, appErr(err)
	}
	sessionID, err := s.repo.CreateSession(ctx, &infrastructure.SessionRecord{
		UserAgent:         truncate(meta.UserAgent, 255),
		IPAddress:         truncate(ip, 64),
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
		ActiveRole:           pickRole(roleOpts, roleID),
		AvailableRoles:       roleOpts,
	}
	// Best effort: a failed stamp must never turn a correct login into an error.
	if err := s.users.TouchLogin(ctx, u.ID); err != nil {
		s.log.Warn("touch last_login_at", "user", u.ID, "error", err)
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
		"phone": u.Phone, "displayTitle": u.DisplayTitle,
	}
}

// --- refresh / logout ------------------------------------------------------

// RefreshResult carries the rotated token pair.
type RefreshResult struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// Refresh rotates the refresh token with CAS + grace (F1-10, A-21). Reuse of
// the previous hash inside RefreshReuseGrace is a concurrent-tab race: answer
// 401 refresh_raced without killing sessions. Outside the window it is a real
// replay: revoke all user sessions.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*RefreshResult, *apperror.AppError) {
	h := hashOf(refreshToken)
	rec, err := s.repo.GetSessionByRefreshHash(ctx, h)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	now := time.Now().UTC()
	if rec == nil {
		if prev, err := s.repo.GetSessionByPrevHash(ctx, h); err != nil {
			return nil, apperror.Internal(err)
		} else if prev != nil && !prev.RevokedAt.Valid {
			if prev.RotatedAt.Valid && now.Sub(prev.RotatedAt.Time) <= RefreshReuseGrace {
				return nil, &apperror.AppError{Code: apperror.CodeRefreshRaced, Message: "Sesi disegarkan di tab lain, coba lagi"}
			}
			ip, ua := "", ""
			if info := reqctx.From(ctx); info != nil {
				ip = info.IP
			}
			s.log.Warn("refresh replay detected", "user", prev.UserID, "session", prev.ID, "ip", ip, "ua", ua)
			_ = s.repo.RevokeUserSessions(ctx, prev.UserID)
			s.cache.dropUser(prev.UserID)
			if s.audit != nil {
				_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "auth.refresh_replay", Entity: "user", EntityID: prev.UserID, BranchID: 0, ActorID: prev.UserID})
			}
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
	// Revoked users / removed roles cannot refresh (F1-4).
	if u, err := s.users.GetByID(ctx, rec.UserID); err != nil {
		return nil, apperror.Internal(err)
	} else if u == nil || u.Status != "active" {
		_ = s.repo.RevokeSession(ctx, rec.ID)
		s.cache.drop(rec.ID)
		return nil, apperror.Unauthorized()
	} else if roleIDs, err := s.users.GetRoleIDs(ctx, u.ID); err != nil {
		return nil, apperror.Internal(err)
	} else if len(roleIDs) == 0 {
		_ = s.repo.RevokeSession(ctx, rec.ID)
		s.cache.drop(rec.ID)
		return nil, apperror.Unauthorized()
	}
	if err := s.reconcileBranch(ctx, rec); err != nil {
		return nil, apperror.Internal(err)
	}
	token, hash, err := newRefreshToken()
	if err != nil {
		return nil, apperror.Internal(err)
	}
	expiry := now.Add(s.refresh)
	if expiry.After(rec.AbsoluteExpiresAt) {
		expiry = rec.AbsoluteExpiresAt
	}
	rows, err := s.repo.Rotate(ctx, rec.ID, h, hash, expiry)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if rows == 0 {
		// Lost the CAS race: reread; inside grace it is the sibling tab.
		if cur, err := s.repo.GetSessionByID(ctx, rec.ID); err != nil {
			return nil, apperror.Internal(err)
		} else if cur != nil && cur.PrevRefreshHash.Valid && cur.PrevRefreshHash.String == h &&
			cur.RotatedAt.Valid && now.Sub(cur.RotatedAt.Time) <= RefreshReuseGrace {
			return nil, &apperror.AppError{Code: apperror.CodeRefreshRaced, Message: "Sesi disegarkan di tab lain, coba lagi"}
		}
		return nil, apperror.Unauthorized()
	}
	signed, err := s.signAccess(rec.UserID, rec.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &RefreshResult{AccessToken: signed, RefreshToken: token}, nil
}

// reconcileBranch repairs a session whose stored active branch is no longer in
// the user's access list (revoked mid-session, F-03): it moves to the user's
// default branch when that one is still usable, otherwise to none so the
// client must pick. A branch the user still holds is left alone.
func (s *Service) reconcileBranch(ctx context.Context, rec *infrastructure.SessionRecord) error {
	if !rec.ActiveBranchID.Valid || rec.ActiveBranchID.Int64 == 0 {
		return nil
	}
	access, err := s.users.GetBranchAccess(ctx, rec.UserID)
	if err != nil {
		return err
	}
	if hasBranchAccess(access, rec.ActiveBranchID.Int64) {
		return nil
	}
	var next int64
	for _, a := range access {
		if !a.IsDefault {
			continue
		}
		if b, err := s.branches.GetByID(ctx, a.BranchID); err != nil {
			return err
		} else if b != nil && b.Status == "active" {
			next = a.BranchID
		}
	}
	if err := s.repo.UpdateBranchRole(ctx, rec.ID, next, rec.ActiveRoleID.Int64); err != nil {
		return err
	}
	s.cache.drop(rec.ID)
	return nil
}

// Logout revokes the caller's session.
func (s *Service) Logout(ctx context.Context, sess *contracts.Session) *apperror.AppError {
	if err := s.repo.RevokeSession(ctx, sess.ID); err != nil {
		return apperror.Internal(err)
	}
	// The revoked row must not keep answering from cache.
	s.cache.drop(sess.ID)
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
	// Permissions come from the ACTIVE role only (legacy parity, F3b-1).
	activeRoleID := sess.RoleID
	perms, err := s.roles.GetPermissionCodes(ctx, []int64{activeRoleID})
	if err != nil {
		return nil, appErr(err)
	}
	roleOpts, err := s.roleOptions(ctx, roleIDs)
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
		"user":           userView(u, roleIDs),
		"branch":         branch,
		"permissions":    perms,
		"company":        company,
		"activeRole":     pickRole(roleOpts, activeRoleID),
		"availableRoles": roleOpts,
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
	// The session now points at a different branch — rebuild on next request.
	s.cache.drop(sess.ID)
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
	// The session now carries a different role, hence different permissions.
	s.cache.drop(sess.ID)
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
//
// The six reads below run on a cache miss only. Every write that changes
// their outcome invalidates first (see sessionCache), so a hit is not a
// stale read — it is the same answer, not fetched twice.
func (s *Service) Verify(ctx context.Context, accessToken string) (*contracts.Session, error) {
	userID, sessionID, err := s.parseAccess(accessToken)
	if err != nil {
		return nil, contracts.ErrSessionInvalid
	}
	now := time.Now().UTC()
	if cached, ok := s.cache.get(sessionID, now); ok {
		// The token carries the user; a cached session built for a different
		// one means a forged or recycled sid. Never serve it.
		if cached.UserID != userID {
			return nil, contracts.ErrSessionInvalid
		}
		if info := reqctx.From(ctx); info != nil {
			info.UserID = cached.UserID
			info.BranchID = cached.BranchID
		}
		return &cached, nil
	}
	mark := s.cache.mark() // before the reads: see sessionCache.putIf
	sess, err := s.resolveSession(ctx, userID, sessionID, now)
	if err != nil {
		return nil, err
	}
	s.cache.putIf(sessionID, *sess, now, mark)
	if info := reqctx.From(ctx); info != nil {
		info.UserID = sess.UserID
		info.BranchID = sess.BranchID
	}
	// Copy: the cache owns the stored value, the caller owns this one.
	out := *sess
	return &out, nil
}

// resolveSession reads the session's authorization state from the database.
func (s *Service) resolveSession(ctx context.Context, userID, sessionID int64, now time.Time) (*contracts.Session, error) {
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
	// Only the active role grants permissions (F3b-1).
	perms, err := s.roles.GetPermissionCodes(ctx, []int64{roleID})
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
	activeBranch := rec.ActiveBranchID.Int64
	if activeBranch != 0 {
		// Access revoked after login (F-03): the session keeps no branch, so
		// branch routes answer "pilih cabang" until the user switches to one
		// they may still use. Runs on a cache miss only; UpdateUser drops the
		// user's cached sessions, so the revocation takes effect next request.
		access, err := s.users.GetBranchAccess(ctx, userID)
		if err != nil {
			return nil, err
		}
		if !hasBranchAccess(access, activeBranch) {
			activeBranch = 0
		}
	}
	if activeBranch != 0 {
		// A branch closed after login stops authorizing business routes
		// (switch-branch stays open as the way out).
		if b, err := s.branches.GetByID(ctx, activeBranch); err != nil {
			return nil, err
		} else if b == nil || b.Status != "active" {
			branchActive = false
		}
	}
	return &contracts.Session{
		ID: sessionID, UserID: userID,
		BranchID: activeBranch, BranchActive: branchActive,
		RoleID: roleID, RoleCode: code, Permissions: perms,
	}, nil
}

func hasBranchAccess(access []usercontracts.BranchAccess, branchID int64) bool {
	for _, a := range access {
		if a.BranchID == branchID {
			return true
		}
	}
	return false
}

// HasPermission implements contracts.PermissionChecker against the session
// already in context.
func (s *Service) HasPermission(ctx context.Context, perm string) bool {
	return contracts.SessionFromContext(ctx).HasPermission(perm)
}
