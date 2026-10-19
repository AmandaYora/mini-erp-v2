package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mini-erp/internal/bootstrap"
	authapp "mini-erp/internal/modules/auth/application"
	userapp "mini-erp/internal/modules/user/application"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/apperror"
)

func f3bMeta(ip, ua string) authapp.LoginMeta { return authapp.LoginMeta{IP: ip, UserAgent: ua} }

func f3bReq(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "203.0.113.10:5000"
	req.Header.Set("Content-Type", "application/json")
	return req
}

func f3bServe(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func f3bStr(s string) *string { return &s }

func f3bRole(t *testing.T, code string, perms ...string) int64 {
	t.Helper()
	role, err := testMods.user.Service().CreateRole(ctx(), 0, userapp.CreateRoleInput{Code: code, Name: "Peran " + code})
	if err != nil {
		t.Fatalf("create role %s: %v", code, err)
	}
	if len(perms) > 0 {
		if err := testMods.user.Service().SyncRolePermissions(ctx(), 0, role.ID, perms); err != nil {
			t.Fatalf("perms %s: %v", code, err)
		}
	}
	return role.ID
}

func f3bUser(t *testing.T, branchID int64, in userapp.CreateUserInput) int64 {
	t.Helper()
	in.Branches = []usercontracts.BranchAccess{{BranchID: branchID, IsDefault: true}}
	if in.Password == "" {
		in.Password = "rahasia-kuat-1"
	}
	u, err := testMods.user.Service().CreateUser(ctx(), 0, in)
	if err != nil {
		t.Fatalf("create user %s: %v", in.Username, err)
	}
	return u.ID
}

func f3bSeed(t *testing.T) {
	t.Helper()
	if err := testMods.user.Service().EnsurePermissions(ctx(), bootstrap.AllPermissionSeeds()); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}
}

// TestLoginByUsernameOrEmail (F3b-1): the same credentials work through the
// username, the email (any case, padded), and the legacy `username` payload
// carrying an email; a wrong password and an unknown identifier answer the
// same 401; an empty identifier never matches an account without email.
func TestLoginByUsernameOrEmail(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	f3bSeed(t)
	role := f3bRole(t, "F3B-LOGIN", "users.view")
	f3bUser(t, fx.branchID, userapp.CreateUserInput{
		Username: "budi", Email: f3bStr("Budi@Toko.Test"), FullName: "Budi", RoleIDs: []int64{role}})
	f3bUser(t, fx.branchID, userapp.CreateUserInput{
		Username: "tanpa-email", FullName: "Tanpa Email", RoleIDs: []int64{role}})

	svc := testMods.auth.Service()
	for _, id := range []string{"budi", "budi@toko.test", "  BUDI@TOKO.TEST  "} {
		if _, err := svc.LoginWithMeta(ctx(), id, "rahasia-kuat-1", f3bMeta("10.1.1.1", "")); err != nil {
			t.Errorf("login %q: %v", id, err)
		}
	}
	for _, bad := range []string{"", "   ", "tidak-ada@toko.test"} {
		_, err := svc.LoginWithMeta(ctx(), bad, "rahasia-kuat-1", f3bMeta("10.1.1.2", ""))
		if err == nil || err.Code != apperror.CodeUnauthorized {
			t.Errorf("login %q: err=%v, want unauthorized", bad, err)
		}
	}
	if _, err := svc.LoginWithMeta(ctx(), "budi@toko.test", "salah-salah-1", f3bMeta("10.1.1.3", "")); err == nil || err.Code != apperror.CodeUnauthorized {
		t.Errorf("wrong password: err=%v, want unauthorized", err)
	}

	// HTTP payload compatibility: identifier, legacy username, email.
	h := routeServer(t).Handler()
	for _, body := range []string{
		`{"identifier":"budi@toko.test","password":"rahasia-kuat-1"}`,
		`{"username":"budi","password":"rahasia-kuat-1"}`,
		`{"username":"budi@toko.test","password":"rahasia-kuat-1"}`,
		`{"email":"budi@toko.test","password":"rahasia-kuat-1"}`,
	} {
		rec := doReq(h, http.MethodPost, "/api/v1/auth/login", "", body)
		if rec.Code != http.StatusOK {
			t.Errorf("POST login %s = %d %s", body, rec.Code, rec.Body.String())
		}
	}
	if rec := doReq(h, http.MethodPost, "/api/v1/auth/login", "", `{"password":"rahasia-kuat-1"}`); rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
		t.Errorf("login tanpa identifier = %d, want 400/422", rec.Code)
	}
}

// TestLoginUsernameBeatsEmail (F3b-1): when A's username equals B's email the
// username match wins, deterministically.
func TestLoginUsernameBeatsEmail(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	f3bSeed(t)
	role := f3bRole(t, "F3B-AMBIG", "users.view")
	idA := f3bUser(t, fx.branchID, userapp.CreateUserInput{
		Username: "kembar@x.test", FullName: "A", RoleIDs: []int64{role}, Password: "sandi-aaaa-1"})
	f3bUser(t, fx.branchID, userapp.CreateUserInput{
		Username: "orang-b", Email: f3bStr("kembar@x.test"), FullName: "B", RoleIDs: []int64{role}, Password: "sandi-bbbb-1"})
	u, err := testMods.user.Users().GetByLogin(ctx(), "kembar@x.test")
	if err != nil || u == nil || u.ID != idA {
		t.Fatalf("GetByLogin = %+v %v, want user A (%d)", u, err, idA)
	}
	if _, aerr := testMods.auth.Service().Login(ctx(), "kembar@x.test", "sandi-aaaa-1", "10.2.2.2"); aerr != nil {
		t.Fatalf("login as A: %v", aerr)
	}
}

// TestSessionPermissionsFollowActiveRole (F3b-1): permissions are those of
// the ACTIVE role only; switching role changes them on the next request;
// login and /auth/me report activeRole and availableRoles.
func TestSessionPermissionsFollowActiveRole(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	f3bSeed(t)
	r1 := f3bRole(t, "F3B-R1", "users.view")
	r2 := f3bRole(t, "F3B-R2", "roles.manage", "audit.view")
	f3bUser(t, fx.branchID, userapp.CreateUserInput{
		Username: "dua-role", FullName: "Dua Role", RoleIDs: []int64{r2, r1}})

	svc := testMods.auth.Service()
	res, aerr := svc.Login(ctx(), "dua-role", "rahasia-kuat-1", "10.3.3.3")
	if aerr != nil {
		t.Fatalf("login: %v", aerr)
	}
	if res.ActiveRole == nil || res.ActiveRole.ID != r1 {
		t.Fatalf("active role = %+v, want the lowest id (%d)", res.ActiveRole, r1)
	}
	if len(res.AvailableRoles) != 2 {
		t.Fatalf("availableRoles = %+v, want 2", res.AvailableRoles)
	}
	h0 := routeServer(t).Handler()
	if rec := doReq(h0, http.MethodGet, "/api/v1/audit-logs", res.AccessToken, ""); rec.Code != http.StatusForbidden {
		t.Errorf("audit-logs under r1 = %d, want 403", rec.Code)
	}
	sess, err := svc.Verify(ctx(), res.AccessToken)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got := strings.Join(sess.Permissions, ","); got != "users.view" {
		t.Fatalf("permissions = %q, want only the active role's users.view", got)
	}
	me, aerr := svc.Me(ctx(), sess)
	if aerr != nil {
		t.Fatalf("me: %v", aerr)
	}
	if perms := me["permissions"].([]string); len(perms) != 1 || perms[0] != "users.view" {
		t.Fatalf("me permissions = %v, want [users.view]", perms)
	}
	if len(me["availableRoles"].([]authapp.RoleOption)) != 2 {
		t.Fatalf("me availableRoles = %v", me["availableRoles"])
	}

	// Switch to r2: new token, other permissions; r1's are gone.
	tok, aerr := svc.SwitchRole(ctx(), sess, r2)
	if aerr != nil {
		t.Fatalf("switch role: %v", aerr)
	}
	sess2, err := svc.Verify(ctx(), tok)
	if err != nil {
		t.Fatalf("verify after switch: %v", err)
	}
	if sess2.RoleID != r2 || strings.Join(sess2.Permissions, ",") != "audit.view,roles.manage" {
		t.Fatalf("after switch: role=%d perms=%v, want r2 audit.view,roles.manage", sess2.RoleID, sess2.Permissions)
	}
	if _, aerr := svc.SwitchRole(ctx(), sess2, 999999); aerr == nil {
		t.Fatal("switching to a role the user does not hold must fail")
	}

	// HTTP: allowed under r2 (needs audit.view). The session is the same row,
	// so the old token follows the switch too.
	h := routeServer(t).Handler()
	if rec := doReq(h, http.MethodGet, "/api/v1/audit-logs", tok, ""); rec.Code != http.StatusOK {
		t.Errorf("audit-logs under r2 = %d %s, want 200", rec.Code, rec.Body.String())
	}
	rec := doReq(h, http.MethodGet, "/api/v1/auth/me", tok, "")
	var body struct {
		Data struct {
			ActiveRole     struct{ Id int64 }   `json:"activeRole"`
			AvailableRoles []struct{ Id int64 } `json:"availableRoles"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Data.ActiveRole.Id != r2 || len(body.Data.AvailableRoles) != 2 {
		t.Fatalf("GET /auth/me = %s", rec.Body.String())
	}
}

// TestLoginTrail (F3b-1): login stamps users.last_login_at and records the
// user agent and IP on the session; an overlong user agent is truncated.
func TestLoginTrail(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	f3bSeed(t)
	role := f3bRole(t, "F3B-TRAIL", "users.view")
	uid := f3bUser(t, fx.branchID, userapp.CreateUserInput{
		Username: "jejak", FullName: "Jejak", RoleIDs: []int64{role}})

	var last *string
	if err := testMods.db.QueryRow(`SELECT CAST(last_login_at AS CHAR) FROM users WHERE id = ?`, uid).Scan(&last); err != nil || last != nil {
		t.Fatalf("before login last_login_at = %v (%v), want NULL", last, err)
	}
	ua := "Mozilla/5.0 " + strings.Repeat("x", 400)
	h := routeServer(t).Handler()
	req := f3bReq(http.MethodPost, "/api/v1/auth/login", `{"identifier":"jejak","password":"rahasia-kuat-1"}`)
	req.Header.Set("User-Agent", ua)
	rec := f3bServe(h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login = %d %s", rec.Code, rec.Body.String())
	}
	if err := testMods.db.QueryRow(`SELECT CAST(last_login_at AS CHAR) FROM users WHERE id = ?`, uid).Scan(&last); err != nil || last == nil {
		t.Fatalf("after login last_login_at = %v (%v), want a timestamp", last, err)
	}
	var gotUA, gotIP string
	if err := testMods.db.QueryRow(`SELECT user_agent, ip_address FROM user_sessions WHERE user_id = ? ORDER BY id DESC LIMIT 1`, uid).Scan(&gotUA, &gotIP); err != nil {
		t.Fatalf("session trail: %v", err)
	}
	if !strings.HasPrefix(gotUA, "Mozilla/5.0 xxx") || len(gotUA) != 255 {
		t.Errorf("user_agent len=%d, want truncated to 255", len(gotUA))
	}
	if gotIP == "" {
		t.Error("ip_address must be recorded")
	}
}

// TestUserProfileFields (F3b-1/6): phone + displayTitle round-trip through
// create/update and the new widths hold (username 100, full name 255).
func TestUserProfileFields(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	f3bSeed(t)
	role := f3bRole(t, "F3B-PROF", "users.view")
	svc := testMods.user.Service()
	long := strings.Repeat("u", 100)
	u, err := svc.CreateUser(ctx(), 0, userapp.CreateUserInput{
		Username: long, FullName: strings.Repeat("n", 255), Phone: "0812-3456", DisplayTitle: "Kepala Gudang",
		Password: "rahasia-kuat-1", RoleIDs: []int64{role},
		Branches: []usercontracts.BranchAccess{{BranchID: fx.branchID, IsDefault: true}},
	})
	if err != nil {
		t.Fatalf("create with max widths: %v", err)
	}
	if u.Phone != "0812-3456" || u.DisplayTitle != "Kepala Gudang" {
		t.Fatalf("round trip = %+v", u)
	}
	if _, err := svc.CreateUser(ctx(), 0, userapp.CreateUserInput{
		Username: strings.Repeat("u", 101), FullName: "x", Password: "rahasia-kuat-1", RoleIDs: []int64{role},
	}); err == nil {
		t.Error("username of 101 chars must be rejected")
	}
	if _, err := svc.CreateUser(ctx(), 0, userapp.CreateUserInput{
		Username: "p", FullName: "x", Phone: strings.Repeat("1", 31), Password: "rahasia-kuat-1", RoleIDs: []int64{role},
	}); err == nil {
		t.Error("phone of 31 chars must be rejected")
	}
	upd, err := svc.UpdateUser(ctx(), 0, u.ID, userapp.UpdateUserInput{
		Username: long, FullName: "Baru", Phone: "", DisplayTitle: "Manajer", RoleIDs: []int64{role},
		Branches: []usercontracts.BranchAccess{{BranchID: fx.branchID, IsDefault: true}},
	})
	if err != nil || upd.Phone != "" || upd.DisplayTitle != "Manajer" {
		t.Fatalf("update = %+v %v", upd, err)
	}
}
