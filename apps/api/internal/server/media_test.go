package server

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mini-erp/internal/config"
	"mini-erp/internal/shared/signedurl"
)

const sessionSecret = "WHATSAPP-SESSION-CREDENTIAL"

type mediaFixture struct {
	root   string
	media  string
	h      http.Handler
	signer *signedurl.Signer
}

// newMediaFixture reproduces the layout that leaked: the storage root holds a
// WhatsApp session credential next to the media folder.
func newMediaFixture(t *testing.T) mediaFixture {
	t.Helper()
	root := t.TempDir()
	media := filepath.Join(root, "media")
	private := filepath.Join(root, "_private")
	for p, content := range map[string]string{
		filepath.Join(media, "product", "2026", "09", "a.jpg"): "JPEGBYTES",
		filepath.Join(media, "payment", "proof.pdf"):           "PDFBYTES",
		filepath.Join(media, "payment", "notes.txt"):           "NOT-A-MEDIA-TYPE",
		filepath.Join(media, "payment", "dump.db"):             "DB-IN-MEDIA",
		filepath.Join(private, "whatsapp_session.db"):          sessionSecret,
		filepath.Join(root, "whatsapp_session.db"):             sessionSecret, // old layout leftover
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	signer := signedurl.New([]byte("test-key"))
	srv := New(&config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	srv.MountMedia(media, signer)
	return mediaFixture{root: root, media: media, h: srv.Handler(), signer: signer}
}

func (f mediaFixture) get(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}

func (f mediaFixture) signed(rel string) string {
	exp, sig := f.signer.Sign(rel, time.Hour, 5*time.Minute)
	return fmt.Sprintf("/media/%s?exp=%d&sig=%s", rel, exp, sig)
}

func TestMediaServesSignedFile(t *testing.T) {
	f := newMediaFixture(t)
	rec := f.get(t, f.signed("product/2026/09/a.jpg"))
	if rec.Code != http.StatusOK || rec.Body.String() != "JPEGBYTES" {
		t.Fatalf("code=%d body=%q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "image/jpeg" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("header salah: %v", rec.Header())
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.HasPrefix(cc, "private, max-age=") {
		t.Fatalf("Cache-Control=%q", cc)
	}
}

func TestMediaRejectsMissingOrBadSignature(t *testing.T) {
	f := newMediaFixture(t)
	exp, sig := f.signer.Sign("product/2026/09/a.jpg", time.Hour, 5*time.Minute)
	cases := map[string]string{
		"tanpa query":     "/media/product/2026/09/a.jpg",
		"tanpa sig":       fmt.Sprintf("/media/product/2026/09/a.jpg?exp=%d", exp),
		"exp bukan angka": "/media/product/2026/09/a.jpg?exp=abc&sig=" + sig,
		"sig palsu":       fmt.Sprintf("/media/product/2026/09/a.jpg?exp=%d&sig=AAAA", exp),
		"sig berkas lain": fmt.Sprintf("/media/payment/proof.pdf?exp=%d&sig=%s", exp, sig),
		"exp digeser":     fmt.Sprintf("/media/product/2026/09/a.jpg?exp=%d&sig=%s", exp+3600, sig),
	}
	for name, target := range cases {
		if rec := f.get(t, target); rec.Code != http.StatusForbidden {
			t.Errorf("%s: code=%d, want 403", name, rec.Code)
		}
	}
}

func TestMediaRejectsExpired(t *testing.T) {
	f := newMediaFixture(t)
	old := f.signer.WithClock(func() time.Time { return time.Now().Add(-3 * time.Hour) })
	exp, sig := old.Sign("product/2026/09/a.jpg", time.Hour, time.Minute)
	rec := f.get(t, fmt.Sprintf("/media/product/2026/09/a.jpg?exp=%d&sig=%s", exp, sig))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("code=%d, want 403 untuk tautan kedaluwarsa", rec.Code)
	}
}

// The leak this task closes: the session credential must not be reachable by
// any route, signed or not, under either the old or the new prefix.
func TestSessionCredentialIsNeverServed(t *testing.T) {
	f := newMediaFixture(t)
	targets := []string{
		"/uploads/whatsapp_session.db",
		"/uploads/",
		"/uploads",
		"/media/whatsapp_session.db",
		"/media/_private/whatsapp_session.db",
		"/media/../_private/whatsapp_session.db",
		"/media/..%2f_private%2fwhatsapp_session.db",
		"/media/..%2Fwhatsapp_session.db",
		"/media/%2e%2e/whatsapp_session.db",
		f.signed("whatsapp_session.db"),
	}
	for _, target := range targets {
		rec := f.get(t, target)
		if strings.Contains(rec.Body.String(), sessionSecret) || rec.Code == http.StatusOK {
			t.Errorf("%s -> code=%d, kredensial sesi bocor atau 200", target, rec.Code)
		}
	}
}

func TestMediaNoListingAndNoDirectories(t *testing.T) {
	f := newMediaFixture(t)
	for _, target := range []string{"/media/", "/media", "/media/product/", "/media/product/2026/"} {
		if rec := f.get(t, target); rec.Code == http.StatusOK {
			t.Errorf("%s -> 200, listing tidak boleh ada", target)
		}
	}
	// A directory with a valid signature is still not a file.
	if rec := f.get(t, f.signed("product")); rec.Code != http.StatusNotFound {
		t.Errorf("direktori bertanda tangan: code=%d, want 404", rec.Code)
	}
}

func TestMediaServesOnlyWhitelistedTypes(t *testing.T) {
	f := newMediaFixture(t)
	if rec := f.get(t, f.signed("payment/proof.pdf")); rec.Code != http.StatusOK {
		t.Fatalf("pdf: code=%d", rec.Code)
	}
	for _, rel := range []string{"payment/notes.txt", "payment/dump.db"} {
		if rec := f.get(t, f.signed(rel)); rec.Code != http.StatusNotFound {
			t.Errorf("%s: code=%d, want 404 (tipe bukan media)", rel, rec.Code)
		}
	}
}

// Even a correctly signed traversal path (as if the key were compromised) must
// be refused by the handler itself, not just by the router's path cleaning.
func TestServeMediaRefusesSignedTraversal(t *testing.T) {
	f := newMediaFixture(t)
	signer := f.signer
	for _, rel := range []string{"../_private/whatsapp_session.db", "product/../../_private/whatsapp_session.db", "a\\b.jpg"} {
		exp, sig := signer.Sign(rel, time.Hour, time.Minute)
		req := httptest.NewRequest(http.MethodGet, "/media/x?exp="+fmt.Sprint(exp)+"&sig="+sig, nil)
		req.URL.Path = "/media/" + rel
		rec := httptest.NewRecorder()
		serveMedia(rec, req, f.media, signer)
		if rec.Code == http.StatusOK || strings.Contains(rec.Body.String(), sessionSecret) {
			t.Errorf("%q -> code=%d, traversal lolos", rel, rec.Code)
		}
	}
}

func TestMediaOnlyGetAndHead(t *testing.T) {
	f := newMediaFixture(t)
	req := httptest.NewRequest(http.MethodPost, f.signed("product/2026/09/a.jpg"), nil)
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatal("POST ke /media tidak boleh 200")
	}
}
