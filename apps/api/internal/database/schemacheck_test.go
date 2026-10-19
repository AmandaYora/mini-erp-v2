package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestMaxMigrationVersion(t *testing.T) {
	dir := t.TempDir()
	files := []string{"000001_user.up.sql", "000018_assistant.up.sql", "000018_assistant.down.sql", "README.md"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("-- test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := MaxMigrationVersion(dir)
	if err != nil {
		t.Fatalf("MaxMigrationVersion: %v", err)
	}
	if got != 18 {
		t.Fatalf("got %d, want 18", got)
	}
	if _, err := MaxMigrationVersion(t.TempDir()); err == nil {
		t.Fatal("empty dir should error")
	}
}

func TestCheckMigrationVersionNoTable(t *testing.T) {
	// sqlmock tidak dipakai di repo ini; cukup pastikan query ke DB tertutup
	// gagal dengan cara yang dikenali (bukan panic) dan non-production skip.
	db, err := sql.Open("mysql", "root:@tcp(127.0.0.1:1)/x?timeout=1s")
	if err != nil {
		t.Skipf("open: %v", err)
	}
	defer func() { _ = db.Close() }()
	_, skipped, err := CheckMigrationVersion(context.Background(), db, 18, false)
	if err != nil || !skipped {
		t.Fatalf("non-production tanpa tabel harus skip tanpa error, got skipped=%v err=%v", skipped, err)
	}
	if _, _, err := CheckMigrationVersion(context.Background(), db, 18, true); err == nil {
		t.Fatal("production tanpa tabel harus error")
	}
}
