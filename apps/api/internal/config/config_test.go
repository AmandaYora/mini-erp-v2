package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestJWTSecretGuard (A5): example/short secrets pass only in development.
func TestJWTSecretGuard(t *testing.T) {
	cases := []struct {
		name    string
		env     string
		secret  string
		wantErr string
	}{
		{"dev example ok", "development", "change-me", ""},
		{"staging example rejected", "staging", "change-me", "example"},
		{"staging short rejected", "staging", "pendek", "32"},
		{"staging strong ok", "staging", "0123456789abcdef0123456789abcdef", ""},
		{"production example rejected", "production", "change-me", "example"},
		{"empty rejected everywhere", "development", "", "required"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("APP_ENV", c.env)
			if c.secret == "" {
				t.Setenv("JWT_SECRET", "")
			} else {
				t.Setenv("JWT_SECRET", c.secret)
			}
			_, err := Load()
			if c.wantErr == "" && err != nil {
				t.Fatalf("Load() = %v, want nil", err)
			}
			if c.wantErr != "" {
				if err == nil {
					t.Fatalf("Load() = nil, want error containing %q", c.wantErr)
				}
				if !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("Load() = %v, want containing %q", err, c.wantErr)
				}
			}
		})
	}
}

// prodBaseline sets a minimal valid production environment. Each production
// rule test starts from here and breaks exactly one variable.
func prodBaseline(t *testing.T) {
	t.Helper()
	storage := t.TempDir() // absolute, so the local-driver rule passes
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("DB_DSN", "mini_erp_app:s3cret@tcp(127.0.0.1:3306)/mini_erp_db")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://toko.example.com")
	t.Setenv("MEDIA_SIGNING_KEY", "media-key-terpisah-0123456789abcdef")
	t.Setenv("STORAGE_DRIVER", "local")
	t.Setenv("STORAGE_DIR", storage)
	t.Setenv("SESSION_DIR", filepath.Join(storage, "_private"))
	t.Setenv("STORAGE_UPLOAD_ROOT_PREFIX", "")
	t.Setenv("STORAGE_BUCKET", "")
	t.Setenv("STORAGE_ACCESS_KEY_ID", "")
	t.Setenv("STORAGE_SECRET_ACCESS_KEY", "")
}

// TestProductionRules (F1-2): every production-only guard in checkProduction.
func TestProductionRules(t *testing.T) {
	t.Run("baseline valid", func(t *testing.T) {
		prodBaseline(t)
		if _, err := Load(); err != nil {
			t.Fatalf("Load() baseline = %v, want nil", err)
		}
	})

	cases := []struct {
		name    string
		mutate  func(t *testing.T)
		wantErr string
	}{
		{"APP_ENV kosong ditolak", func(t *testing.T) { t.Setenv("APP_ENV", "") }, "APP_ENV wajib"},
		{"APP_ENV salah ketik ditolak", func(t *testing.T) { t.Setenv("APP_ENV", "prod") }, "APP_ENV wajib"},
		{"DB_DSN kosong ditolak", func(t *testing.T) { t.Setenv("DB_DSN", "") }, "DB_DSN wajib"},
		{"DB_DSN root tanpa password ditolak", func(t *testing.T) { t.Setenv("DB_DSN", "root:@tcp(localhost:3306)/mini_erp_db") }, "root tanpa password"},
		{"CORS kosong ditolak", func(t *testing.T) { t.Setenv("CORS_ALLOWED_ORIGINS", "") }, "CORS_ALLOWED_ORIGINS wajib"},
		{"CORS bintang ditolak", func(t *testing.T) { t.Setenv("CORS_ALLOWED_ORIGINS", "*") }, "tidak boleh '*'"},
		{"MEDIA_SIGNING_KEY kosong ditolak", func(t *testing.T) { t.Setenv("MEDIA_SIGNING_KEY", "") }, "MEDIA_SIGNING_KEY wajib"},
		{
			"MEDIA_SIGNING_KEY sama dengan JWT ditolak",
			func(t *testing.T) { t.Setenv("MEDIA_SIGNING_KEY", "0123456789abcdef0123456789abcdef") },
			"tidak boleh sama dengan JWT_SECRET",
		},
		{"STORAGE_DIR relatif ditolak untuk local", func(t *testing.T) { t.Setenv("STORAGE_DIR", "./storage") }, "path absolut"},
		{
			"s3 tanpa prefix eksplisit ditolak",
			func(t *testing.T) {
				t.Setenv("STORAGE_DRIVER", "s3")
				t.Setenv("STORAGE_DIR", t.TempDir())
				t.Setenv("STORAGE_BUCKET", "mini-erp")
				t.Setenv("STORAGE_ACCESS_KEY_ID", "key")
				t.Setenv("STORAGE_SECRET_ACCESS_KEY", "secret")
				t.Setenv("STORAGE_UPLOAD_ROOT_PREFIX", "")
			},
			"STORAGE_UPLOAD_ROOT_PREFIX wajib",
		},
		{
			"s3 dengan prefix eksplisit lolos",
			func(t *testing.T) {
				t.Setenv("STORAGE_DRIVER", "s3")
				t.Setenv("STORAGE_DIR", t.TempDir())
				t.Setenv("STORAGE_BUCKET", "mini-erp")
				t.Setenv("STORAGE_ACCESS_KEY_ID", "key")
				t.Setenv("STORAGE_SECRET_ACCESS_KEY", "secret")
				t.Setenv("STORAGE_UPLOAD_ROOT_PREFIX", "vioni/production-v2/upload")
			},
			"",
		},
		{
			"upload root sama dengan protected ditolak",
			func(t *testing.T) {
				t.Setenv("STORAGE_UPLOAD_ROOT_PREFIX", "vioni/production-v2/upload")
				t.Setenv("STORAGE_PROTECTED_PREFIXES", "vioni/production-v2/upload")
			},
			"bertumpuk",
		},
		{
			"upload root di dalam protected ditolak",
			func(t *testing.T) {
				t.Setenv("STORAGE_UPLOAD_ROOT_PREFIX", "vioni/production/upload/baru")
				t.Setenv("STORAGE_PROTECTED_PREFIXES", "vioni/production/upload")
			},
			"bertumpuk",
		},
		{
			"protected di dalam upload root ditolak",
			func(t *testing.T) {
				t.Setenv("STORAGE_UPLOAD_ROOT_PREFIX", "vioni/production-v2/upload")
				t.Setenv("STORAGE_PROTECTED_PREFIXES", "vioni/production-v2/upload/lama")
			},
			"bertumpuk",
		},
		{
			"protected tak bertumpuk lolos",
			func(t *testing.T) {
				t.Setenv("STORAGE_UPLOAD_ROOT_PREFIX", "vioni/production-v2/upload")
				t.Setenv("STORAGE_PROTECTED_PREFIXES", "vioni/production/upload")
			},
			"",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prodBaseline(t)
			c.mutate(t)
			_, err := Load()
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("Load() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Load() = nil, want error containing %q", c.wantErr)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("Load() = %v, want containing %q", err, c.wantErr)
			}
		})
	}
}

// TestProductionKeySeparation guards the regression that motivated moving
// checkProduction after the MediaSigningKey derivation: with the old order
// the equality check compared against a nil key and never fired.
func TestProductionKeySeparation(t *testing.T) {
	prodBaseline(t)
	t.Setenv("MEDIA_SIGNING_KEY", os.Getenv("JWT_SECRET"))
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "tidak boleh sama dengan JWT_SECRET") {
		t.Fatalf("Load() = %v, want MEDIA_SIGNING_KEY/JWT_SECRET separation error", err)
	}
}
