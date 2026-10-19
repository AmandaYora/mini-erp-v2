package integration

import (
	"errors"
	"testing"

	authcontracts "mini-erp/internal/modules/auth/contracts"
)

func login(t *testing.T, username, password string) string {
	t.Helper()
	res, err := testMods.auth.Service().Login(ctx(), username, password, "127.0.0.1")
	if err != nil {
		t.Fatalf("login %s: %v", username, err)
	}
	return res.AccessToken
}

func mustVerify(t *testing.T, token string) {
	t.Helper()
	if _, err := testMods.auth.Service().Verify(ctx(), token); err != nil {
		t.Fatalf("verify: %v, want valid", err)
	}
}

func mustInvalid(t *testing.T, token string) {
	t.Helper()
	if _, err := testMods.auth.Service().Verify(ctx(), token); !errors.Is(err, authcontracts.ErrSessionInvalid) {
		t.Fatalf("verify: err=%v, want session invalid", err)
	}
}

// TestDeactivateRevokesSessions (F1-4): nonaktif mencabut sesi DB; reaktivasi
// tidak menghidupkan sesi lama.
func TestDeactivateRevokesSessions(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	targetID := makeUser(t, fx.branchID, "target", "rahasia-kuat-1")

	tok := login(t, "target", "rahasia-kuat-1")
	mustVerify(t, tok)

	if err := testMods.user.Service().SetStatus(ctx(), 0, targetID, "inactive"); err != nil {
		t.Fatalf("set inactive: %v", err)
	}
	mustInvalid(t, tok)

	if err := testMods.user.Service().SetStatus(ctx(), 0, targetID, "active"); err != nil {
		t.Fatalf("set active: %v", err)
	}
	mustInvalid(t, tok)

	tok2 := login(t, "target", "rahasia-kuat-1")
	mustVerify(t, tok2)
}

// TestAdminPasswordChangeKillsTargetSessions (F1-4): ganti sandi oleh admin
// mencabut semua sesi target; sesi admin tetap hidup.
func TestAdminPasswordChangeKillsTargetSessions(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	adminID := makeUser(t, fx.branchID, "admin", "rahasia-kuat-1")
	targetID := makeUser(t, fx.branchID, "target", "rahasia-kuat-1")

	tokA := login(t, "target", "rahasia-kuat-1")
	tokB := login(t, "target", "rahasia-kuat-1")
	tokAdmin := login(t, "admin", "rahasia-kuat-1")

	if err := testMods.user.Service().ChangePassword(ctx(), adminID, targetID, "baru-kuat-22"); err != nil {
		t.Fatalf("change password: %v", err)
	}
	mustInvalid(t, tokA)
	mustInvalid(t, tokB)
	mustVerify(t, tokAdmin)

	tokC := login(t, "target", "baru-kuat-22")
	mustVerify(t, tokC)
}

// TestSelfPasswordChangeKeepsOwnSession (F1-4): ganti sandi sendiri
// mempertahankan sesi pemohon, mencabut sesi lain (paritas legacy).
func TestSelfPasswordChangeKeepsOwnSession(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	targetID := makeUser(t, fx.branchID, "target", "rahasia-kuat-1")

	tokA := login(t, "target", "rahasia-kuat-1")
	tokB := login(t, "target", "rahasia-kuat-1")
	sessA, err := testMods.auth.Service().Verify(ctx(), tokA)
	if err != nil {
		t.Fatalf("verify A: %v", err)
	}
	ownCtx := authcontracts.WithSession(ctx(), sessA)
	if err := testMods.user.Service().ChangePassword(ownCtx, targetID, targetID, "lagi-kuat-33"); err != nil {
		t.Fatalf("self change password: %v", err)
	}
	mustVerify(t, tokA)
	mustInvalid(t, tokB)
}

// TestRefreshAfterDeactivate (F1-4): refresh token user nonaktif ditolak dan
// sesinya dicabut.
func TestRefreshAfterDeactivate(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	targetID := makeUser(t, fx.branchID, "target", "rahasia-kuat-1")

	res, err := testMods.auth.Service().Login(ctx(), "target", "rahasia-kuat-1", "127.0.0.1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if err := testMods.user.Service().SetStatus(ctx(), 0, targetID, "inactive"); err != nil {
		t.Fatalf("set inactive: %v", err)
	}
	if _, rerr := testMods.auth.Service().Refresh(ctx(), res.RefreshToken); rerr == nil {
		t.Fatal("refresh setelah nonaktif: want 401")
	}
	mustInvalid(t, res.AccessToken)
}
