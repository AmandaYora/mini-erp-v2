package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func layoutConfig(root string) *Config {
	return &Config{
		StorageDir: root,
		SessionDir: filepath.Join(root, "_private"),
		MediaDir:   filepath.Join(root, "media"),
	}
}

// The old layout served the whole storage root. After migration the WhatsApp
// session must sit outside the served folder and uploads inside it.
func TestPrepareStorageLayoutMovesLegacyFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "whatsapp_session.db"), "secret")
	writeFile(t, filepath.Join(root, "whatsapp_session.db-wal"), "wal")
	writeFile(t, filepath.Join(root, "product", "2026", "09", "a.jpg"), "img")
	writeFile(t, filepath.Join(root, ".gitkeep"), "")
	cfg := layoutConfig(root)

	moved, skipped, err := PrepareStorageLayout(cfg)
	if err != nil {
		t.Fatalf("PrepareStorageLayout: %v", err)
	}
	if moved != 3 || len(skipped) != 0 {
		t.Fatalf("moved=%d skipped=%v, want 3 and none", moved, skipped)
	}
	for _, p := range []string{
		filepath.Join(cfg.SessionDir, "whatsapp_session.db"),
		filepath.Join(cfg.SessionDir, "whatsapp_session.db-wal"),
		filepath.Join(cfg.MediaDir, "product", "2026", "09", "a.jpg"),
		filepath.Join(root, ".gitkeep"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("harus ada: %s (%v)", p, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "whatsapp_session.db")); err == nil {
		t.Error("sesi WA tidak boleh tertinggal di root storage")
	}
	if _, err := os.Stat(filepath.Join(cfg.MediaDir, "whatsapp_session.db")); err == nil {
		t.Error("sesi WA tidak boleh masuk folder media")
	}
}

func TestPrepareStorageLayoutIsIdempotentAndNeverOverwrites(t *testing.T) {
	root := t.TempDir()
	cfg := layoutConfig(root)
	writeFile(t, filepath.Join(cfg.SessionDir, "whatsapp_session.db"), "already-here")
	writeFile(t, filepath.Join(root, "whatsapp_session.db"), "stale-copy")

	moved, skipped, err := PrepareStorageLayout(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if moved != 0 || len(skipped) != 1 || skipped[0] != "whatsapp_session.db" {
		t.Fatalf("moved=%d skipped=%v", moved, skipped)
	}
	got, _ := os.ReadFile(filepath.Join(cfg.SessionDir, "whatsapp_session.db"))
	if string(got) != "already-here" {
		t.Fatalf("berkas tujuan tertimpa: %q", got)
	}
	// Second run: nothing left to do beyond the skipped duplicate.
	moved, _, err = PrepareStorageLayout(cfg)
	if err != nil || moved != 0 {
		t.Fatalf("run kedua moved=%d err=%v", moved, err)
	}
}

func TestPrepareStorageLayoutCustomSessionDirOnlyCreates(t *testing.T) {
	root := t.TempDir()
	custom := filepath.Join(t.TempDir(), "sessions")
	writeFile(t, filepath.Join(root, "product", "a.jpg"), "img")
	cfg := &Config{StorageDir: root, SessionDir: custom, MediaDir: filepath.Join(root, "media")}
	moved, _, err := PrepareStorageLayout(cfg)
	if err != nil || moved != 0 {
		t.Fatalf("moved=%d err=%v; tata letak kustom tidak boleh memindahkan apa pun", moved, err)
	}
	if _, err := os.Stat(custom); err != nil {
		t.Fatalf("SESSION_DIR kustom harus dibuat: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "product", "a.jpg")); err != nil {
		t.Fatal("berkas tidak boleh dipindah pada tata letak kustom")
	}
}

func TestCheckDirsDisjoint(t *testing.T) {
	base := t.TempDir()
	media := filepath.Join(base, "media")
	cases := []struct {
		name    string
		session string
		wantErr bool
	}{
		{"saudara", filepath.Join(base, "_private"), false},
		{"di luar sama sekali", filepath.Join(t.TempDir(), "s"), false},
		{"sesi di dalam media", filepath.Join(media, "wa"), true},
		{"sesi sama dengan media", media, true},
		{"media di dalam sesi", base, true},
	}
	for _, c := range cases {
		err := checkDirsDisjoint(c.session, media)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err=%v wantErr=%v", c.name, err, c.wantErr)
		}
	}
}

func TestLoadRejectsSessionDirInsideMedia(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APP_ENV", "development")
	t.Setenv("JWT_SECRET", "change-me")
	t.Setenv("STORAGE_DIR", root)
	t.Setenv("SESSION_DIR", filepath.Join(root, "media", "wa"))
	if _, err := Load(); err == nil {
		t.Fatal("Load harus menolak SESSION_DIR di dalam folder media")
	}
}

func TestLoadDefaultsKeepSessionOutsideMedia(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APP_ENV", "development")
	t.Setenv("JWT_SECRET", "change-me")
	t.Setenv("STORAGE_DIR", root)
	t.Setenv("SESSION_DIR", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SessionDir != filepath.Join(root, "_private") || cfg.MediaDir != filepath.Join(root, "media") {
		t.Fatalf("SessionDir=%q MediaDir=%q", cfg.SessionDir, cfg.MediaDir)
	}
	if len(cfg.MediaSigningKey) != 32 {
		t.Fatalf("kunci tanda tangan turunan harus 32 byte, got %d", len(cfg.MediaSigningKey))
	}
}
