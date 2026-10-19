package bootstrap

import (
	"database/sql"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"mini-erp/internal/config"
	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/product"
	"mini-erp/internal/shared/signedurl"
)

// wireForTest builds the real module graph over a lazy (never-dialled) DB
// handle: constructors only store it, so no database is needed to prove the
// wiring itself.
func wireForTest(t *testing.T) (*Modules, *sql.DB) {
	t.Helper()
	db, err := sql.Open("mysql", "u:p@tcp(127.0.0.1:1)/none")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	m, err := Wire(db, Options{
		Log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		JWTSecret: "wire-test-jwt-secret-minimal-32-karakter",
		AccessTTL: 15 * time.Minute, RefreshTTL: 24 * time.Hour,
		Storage:  config.Storage{Driver: "local", SignedURLTTLSecs: 300},
		MediaDir: t.TempDir(), Signer: signedurl.New([]byte("wire-test-key")),
		SessionDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Wire must succeed and verify itself: %v", err)
	}
	return m, db
}

// TestWireVerifiesEveryLateBoundDependency: Wire already ran Verify; a second
// explicit call stays green, and the hub really got connected.
func TestWireVerifiesEveryLateBoundDependency(t *testing.T) {
	m, _ := wireForTest(t)
	if err := m.Verify(); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if err := m.AuthzHub.Verify(); err != nil {
		t.Fatalf("authz hub: %v", err)
	}
}

// A forgotten setter must be reported by name, not discovered at the first
// request that needs the dependency.
func TestVerifyCatchesForgottenSetter(t *testing.T) {
	m, db := wireForTest(t)
	// A product module that never received SetStockState.
	m.Product = product.NewModule(db, m.Audit.Audit())
	err := m.Verify()
	if err == nil {
		t.Fatal("Verify must fail when product.StockState was never set")
	}
	if !strings.Contains(err.Error(), "product") || !strings.Contains(err.Error(), "stockState") {
		t.Fatalf("error should name module and dependency, got: %v", err)
	}
}

func TestVerifyCatchesUnattachedHubAndMissingModule(t *testing.T) {
	m, _ := wireForTest(t)
	m.AuthzHub = authcontracts.NewAuthzHub() // auth never subscribed to this one
	m.Finance = nil
	err := m.Verify()
	if err == nil {
		t.Fatal("expected failure")
	}
	for _, want := range []string{"authz hub", "finance: module not constructed"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q: %v", want, err)
		}
	}
}
