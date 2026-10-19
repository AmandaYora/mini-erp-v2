package application

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"mini-erp/internal/modules/user/domain"
	"mini-erp/internal/shared/apperror"
)

func TestValidatePasswordCountsBytesNotCharacters(t *testing.T) {
	cases := []struct {
		name    string
		pw      string
		wantErr bool
	}{
		{"7 byte", "abcdefg", true},
		{"8 byte", "abcdefgh", false},
		{"72 byte", strings.Repeat("a", 72), false},
		{"73 byte", strings.Repeat("a", 73), true},
		// 36 x "é" = 72 bytes: allowed. 37 = 74 bytes although only 37 characters.
		{"36 huruf berbyte ganda = 72 byte", strings.Repeat("é", 36), false},
		{"37 huruf berbyte ganda = 74 byte", strings.Repeat("é", 37), true},
		// 40 CJK characters = 120 bytes.
		{"40 huruf CJK = 120 byte", strings.Repeat("漢", 40), true},
		{"kosong", "", true},
	}
	for _, c := range cases {
		err := validatePassword(c.pw)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err=%v wantErr=%v", c.name, err, c.wantErr)
			continue
		}
		if err != nil {
			if ae, ok := err.(*apperror.AppError); !ok || ae.Code != apperror.CodeValidation {
				t.Errorf("%s: harus apperror validation, got %T %v", c.name, err, err)
			}
		}
	}
}

func TestHashPasswordUsesLegacyCost(t *testing.T) {
	h, err := hashPassword("rahasia-kuat-1")
	if err != nil {
		t.Fatal(err)
	}
	if domain.BcryptCost != 12 {
		t.Fatalf("BcryptCost=%d, paritas legacy menuntut 12", domain.BcryptCost)
	}
	cost, err := bcrypt.Cost([]byte(h))
	if err != nil || cost != domain.BcryptCost {
		t.Fatalf("cost=%d err=%v, want %d", cost, err, domain.BcryptCost)
	}
}

// A hash produced by the legacy system (bcryptjs, $2a$/$2b$, cost 12) must be
// verifiable here without any reset — the cutover depends on it.
func TestLegacyShapedHashVerifies(t *testing.T) {
	for _, prefix := range []string{"$2a$", "$2b$", "$2y$"} {
		raw, err := bcrypt.GenerateFromPassword([]byte("sandi-legacy-1"), 12)
		if err != nil {
			t.Fatal(err)
		}
		h := prefix + string(raw[4:]) // same bytes, different version marker
		if err := bcrypt.CompareHashAndPassword([]byte(h), []byte("sandi-legacy-1")); err != nil {
			t.Errorf("%s: %v", prefix, err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(h), []byte("salah")); err == nil {
			t.Errorf("%s: sandi salah harus ditolak", prefix)
		}
	}
}

func TestBurnPasswordTimeDoesNotPanicOnFirstUse(t *testing.T) {
	burnPasswordTime("apa saja")
	burnPasswordTime(strings.Repeat("x", 200))
}
