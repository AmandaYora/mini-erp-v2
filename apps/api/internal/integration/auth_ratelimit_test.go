package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mini-erp/internal/config"
	"mini-erp/internal/modules/auth"
	authapp "mini-erp/internal/modules/auth/application"
	userapp "mini-erp/internal/modules/user/application"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/server"
	"mini-erp/internal/shared/apperror"
)

// TestLoginIgnoresSpoofedForwardedHeader (F1-3): without a trusted proxy the
// forwarding header is ignored, so rotating X-Forwarded-For across attempts
// still trips the IP+username login limiter on the 11th failed try
// (10 failures/minute).
func TestLoginIgnoresSpoofedForwardedHeader(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())

	role, err := testMods.user.Service().CreateRole(ctx(), 0, userapp.CreateRoleInput{
		Code: "KASIR", Name: "Kasir Uji",
	})
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	if _, err := testMods.user.Service().CreateUser(ctx(), 0, userapp.CreateUserInput{
		Username: "korban", Password: "rahasia-kuat-1", RoleIDs: []int64{role.ID},
		Branches: []usercontracts.BranchAccess{{BranchID: fx.branchID, IsDefault: true}},
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	authMod := auth.NewModule(testMods.db,
		testMods.user.Users(), testMods.user.Roles(),
		testMods.branch.Branches(), testMods.company.Company(),
		"uji-jwt-secret-minimal-32-karakter-aman", 15*time.Minute, 24*time.Hour,
		log, testMods.audit.Audit(), nil)
	// No TRUSTED_PROXIES: every forwarding header must be ignored.
	srv := server.New(&config.Config{}, log)
	authMod.RegisterRoutes(srv.Mux())
	h := srv.Handler()

	login := func(fwdFor string) int {
		body, _ := json.Marshal(map[string]string{"username": "korban", "password": "salah-salah"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		req.RemoteAddr = "203.0.113.7:4000"
		req.Header.Set("X-Forwarded-For", fwdFor)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	for i := 0; i < 10; i++ {
		if code := login(fmt.Sprintf("10.99.0.%d", i+1)); code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: code=%d, want 401", i+1, code)
		}
	}
	if code := login("10.99.0.99"); code != http.StatusTooManyRequests {
		t.Fatalf("attempt 11 dengan header berbeda: code=%d, want 429 (header harus diabaikan)", code)
	}
}

// TestLoginPerIPFailureLimit: password spraying (one IP, a different username
// each try) is capped per IP. After LoginFailuresPerIP failures that IP is
// refused even for a correct password, while other IPs are unaffected.
func TestLoginPerIPFailureLimit(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	makeUser(t, fx.branchID, "korban-semprot", "rahasia-kuat-1")
	svc := testMods.auth.Service()
	const ip = "198.51.100.21"

	for i := 0; i < authapp.LoginFailuresPerIP; i++ {
		_, err := svc.Login(ctx(), fmt.Sprintf("semprot-%d", i), "tebakan-salah-1", ip)
		if err == nil || err.Code != apperror.CodeUnauthorized {
			t.Fatalf("percobaan %d: %v, want 401", i+1, err)
		}
	}
	if _, err := svc.Login(ctx(), "semprot-lagi", "tebakan-salah-1", ip); err == nil || err.Code != apperror.CodeRateLimited {
		t.Fatalf("username baru dari IP yang sama: %v, want 429", err)
	}
	if _, err := svc.Login(ctx(), "korban-semprot", "rahasia-kuat-1", ip); err == nil || err.Code != apperror.CodeRateLimited {
		t.Fatalf("sandi benar dari IP terblokir: %v, want 429", err)
	}
	if _, err := svc.Login(ctx(), "korban-semprot", "rahasia-kuat-1", "198.51.100.22"); err != nil {
		t.Fatalf("IP lain tidak boleh ikut terblokir: %v", err)
	}
}

// TestLoginSuccessUsesNoAllowance: only failed logins are counted, so a user
// logging in correctly more often than the IP+username cap is never refused,
// and their earlier successes do not shrink the allowance for later typos.
func TestLoginSuccessUsesNoAllowance(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	makeUser(t, fx.branchID, "rajin-masuk", "rahasia-kuat-1")
	svc := testMods.auth.Service()
	const ip = "198.51.100.31"

	for i := 0; i <= authapp.LoginFailuresPerIPUser; i++ {
		if _, err := svc.Login(ctx(), "rajin-masuk", "rahasia-kuat-1", ip); err != nil {
			t.Fatalf("login benar ke-%d: %v", i+1, err)
		}
	}
	for i := 0; i < authapp.LoginFailuresPerIPUser-1; i++ {
		if _, err := svc.Login(ctx(), "rajin-masuk", "salah-ketik-1", ip); err == nil || err.Code != apperror.CodeUnauthorized {
			t.Fatalf("salah ketik ke-%d: %v, want 401", i+1, err)
		}
	}
	if _, err := svc.Login(ctx(), "rajin-masuk", "rahasia-kuat-1", ip); err != nil {
		t.Fatalf("login benar setelah %d salah ketik: %v", authapp.LoginFailuresPerIPUser-1, err)
	}
}
