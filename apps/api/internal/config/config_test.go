package config

import (
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
