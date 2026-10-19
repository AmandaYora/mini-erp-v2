package integration

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	userapp "mini-erp/internal/modules/user/application"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/apperror"
)

func storedHash(t *testing.T, username string) string {
	t.Helper()
	var h string
	if err := testMods.db.QueryRow("SELECT password_hash FROM users WHERE username = ?", username).Scan(&h); err != nil {
		t.Fatalf("read hash: %v", err)
	}
	return h
}

func makeUser(t *testing.T, branchID int64, username, password string) int64 {
	t.Helper()
	role, err := testMods.user.Service().CreateRole(ctx(), 0, userapp.CreateRoleInput{Code: "R-" + username, Name: "Peran " + username})
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	u, err := testMods.user.Service().CreateUser(ctx(), 0, userapp.CreateUserInput{
		Username: username, Password: password, RoleIDs: []int64{role.ID},
		Branches: []usercontracts.BranchAccess{{BranchID: branchID, IsDefault: true}},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u.ID
}

func TestPasswordPolicyEndToEnd(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.user.Service()

	// New accounts are hashed at the legacy cost.
	makeUser(t, fx.branchID, "kasir1", "rahasia-kuat-1")
	if c, _ := bcrypt.Cost([]byte(storedHash(t, "kasir1"))); c != 12 {
		t.Fatalf("cost = %d, want 12", c)
	}

	// Length window counts bytes: 73 bytes is a validation error, not a 500.
	tooLong := strings.Repeat("a", 73)
	_, err := svc.CreateUser(ctx(), 0, userapp.CreateUserInput{Username: "panjang", Password: tooLong})
	if mustAppErr(t, err).Code != apperror.CodeValidation {
		t.Fatalf("73 byte saat buat user: %v", err)
	}
	if err := svc.ChangePassword(ctx(), 0, 1, tooLong); mustAppErr(t, err).Code != apperror.CodeValidation {
		t.Fatalf("73 byte saat ganti sandi: %v", err)
	}

	// Login: right password ok; wrong and over-long are plain "unauthorized".
	if _, err := svc.VerifyCredentials(ctx(), "kasir1", "rahasia-kuat-1"); err != nil {
		t.Fatalf("login benar: %v", err)
	}
	for name, pw := range map[string]string{"salah": "salah-sekali-1", "terlalu panjang": tooLong} {
		if _, err := svc.VerifyCredentials(ctx(), "kasir1", pw); mustAppErr(t, err).Code != apperror.CodeUnauthorized {
			t.Errorf("%s: %v", name, err)
		}
	}
	// Unknown and inactive accounts answer exactly like a wrong password.
	if _, err := svc.VerifyCredentials(ctx(), "tidak-ada", "rahasia-kuat-1"); mustAppErr(t, err).Code != apperror.CodeUnauthorized {
		t.Fatalf("user tak dikenal: %v", err)
	}
}

// Accounts created at the old cost (10) are upgraded on the next good login,
// and a wrong password never touches the stored hash.
func TestWeakHashIsUpgradedOnLogin(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.user.Service()
	makeUser(t, fx.branchID, "lama", "rahasia-kuat-1")

	weak, err := bcrypt.GenerateFromPassword([]byte("rahasia-kuat-1"), 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testMods.db.Exec("UPDATE users SET password_hash = ? WHERE username = 'lama'", string(weak)); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.VerifyCredentials(ctx(), "lama", "salah-sekali-1"); err == nil {
		t.Fatal("sandi salah harus ditolak")
	}
	if got := storedHash(t, "lama"); got != string(weak) {
		t.Fatal("login gagal tidak boleh mengubah hash")
	}

	if _, err := svc.VerifyCredentials(ctx(), "lama", "rahasia-kuat-1"); err != nil {
		t.Fatalf("login dengan hash cost 10 harus berhasil: %v", err)
	}
	upgraded := storedHash(t, "lama")
	if c, _ := bcrypt.Cost([]byte(upgraded)); c != 12 {
		t.Fatalf("setelah login cost = %d, want 12", c)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(upgraded), []byte("rahasia-kuat-1")); err != nil {
		t.Fatalf("hash baru harus memverifikasi sandi yang sama: %v", err)
	}
	// Second login leaves the (already strong) hash alone.
	if _, err := svc.VerifyCredentials(ctx(), "lama", "rahasia-kuat-1"); err != nil {
		t.Fatal(err)
	}
	if storedHash(t, "lama") != upgraded {
		t.Fatal("hash yang sudah kuat tidak boleh diganti lagi")
	}
}
