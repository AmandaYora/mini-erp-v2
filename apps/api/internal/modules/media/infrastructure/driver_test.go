package infrastructure

import (
	"bytes"
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"mini-erp/internal/shared/signedurl"
)

func newDriver(t *testing.T) (*LocalDriver, *signedurl.Signer, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "media")
	signer := signedurl.New([]byte("k"))
	d, err := NewLocalDriver(dir, signer, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return d, signer, dir
}

func TestLocalDriverStoreURLRemove(t *testing.T) {
	d, signer, dir := newDriver(t)
	ctx := context.Background()
	key := "product/2026/09/abc.jpg"
	if err := d.Store(ctx, key, bytes.NewReader([]byte("bytes")), "image/jpeg", "a.jpg"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "product", "2026", "09", "abc.jpg"))
	if err != nil || string(got) != "bytes" {
		t.Fatalf("berkas tidak tersimpan di folder media: %v %q", err, got)
	}
	raw, err := d.URLFor(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(raw)
	if err != nil || !strings.HasPrefix(u.Path, "/media/") {
		t.Fatalf("URL harus di bawah /media/: %q (%v)", raw, err)
	}
	rel := strings.TrimPrefix(u.Path, "/media/")
	exp, _ := strconv.ParseInt(u.Query().Get("exp"), 10, 64)
	if err := signer.Verify(rel, exp, u.Query().Get("sig")); err != nil {
		t.Fatalf("URL yang diterbitkan harus lolos verifikasi: %v", err)
	}
	d.Remove(ctx, key)
	if _, err := os.Stat(filepath.Join(dir, "product", "2026", "09", "abc.jpg")); err == nil {
		t.Fatal("Remove harus menghapus berkas")
	}
	d.Remove(ctx, key) // missing key is not an error
}

func TestLocalDriverRefusesKeysOutsideMediaDir(t *testing.T) {
	d, _, dir := newDriver(t)
	ctx := context.Background()
	outside := filepath.Join(filepath.Dir(dir), "escaped.jpg")
	for _, key := range []string{"../escaped.jpg", "product/../../escaped.jpg", "", "a\\..\\b.jpg", "x\x00y.jpg", ".", ".."} {
		if err := d.Store(ctx, key, bytes.NewReader([]byte("x")), "", ""); err == nil {
			t.Errorf("Store(%q) harus ditolak", key)
		}
		if _, err := d.URLFor(ctx, key); err == nil {
			t.Errorf("URLFor(%q) harus ditolak", key)
		}
	}
	if _, err := os.Stat(outside); err == nil {
		t.Fatal("berkas keluar dari folder media")
	}
}

func TestLocalDriverRejectsEmptyFile(t *testing.T) {
	d, _, dir := newDriver(t)
	if err := d.Store(context.Background(), "a.jpg", bytes.NewReader(nil), "", ""); err == nil {
		t.Fatal("file kosong harus ditolak")
	}
	if _, err := os.Stat(filepath.Join(dir, "a.jpg")); err == nil {
		t.Fatal("file kosong tidak boleh tertinggal")
	}
}

func TestSameKeyInSameWindowGivesSameURL(t *testing.T) {
	d, _, _ := newDriver(t)
	a, _ := d.URLFor(context.Background(), "p/a.jpg")
	b, _ := d.URLFor(context.Background(), "p/a.jpg")
	if a != b {
		t.Fatalf("URL dalam jendela yang sama harus identik: %q vs %q", a, b)
	}
}

func TestNewLocalDriverNeedsSigner(t *testing.T) {
	if _, err := NewLocalDriver(t.TempDir(), nil, time.Minute); err == nil {
		t.Fatal("tanpa signer harus gagal")
	}
}
