package integration

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mini-erp/internal/config"
	"mini-erp/internal/modules/media"
	mediacontracts "mini-erp/internal/modules/media/contracts"
	"mini-erp/internal/server"
	"mini-erp/internal/shared/signedurl"
)

// pngBytes is a valid 1x1 PNG so MIME sniffing accepts it as an image.
var pngBytes = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0xf8, 0xff, 0xff, 0x3f,
	0x00, 0x05, 0xfe, 0x02, 0xfe, 0xa7, 0x35, 0x81, 0x84, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
	0x44, 0xae, 0x42, 0x60, 0x82,
}

// TestUploadedFileIsServedOnlyThroughItsSignedURL runs the real path end to
// end: upload through the media module, take the URL the API would hand to the
// browser, fetch it through the server, and prove the neighbouring session
// credential is unreachable.
func TestUploadedFileIsServedOnlyThroughItsSignedURL(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	mediaDir := filepath.Join(root, "media")
	secret := filepath.Join(root, "_private", "whatsapp_session.db")
	if err := os.MkdirAll(filepath.Dir(secret), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secret, []byte("SESSION-CREDENTIAL"), 0o600); err != nil {
		t.Fatal(err)
	}

	signer := signedurl.New([]byte("e2e-key"))
	mm, err := media.NewModule(testMods.db, config.Storage{Driver: "local", SignedURLTTLSecs: 300},
		mediaDir, signer, testMods.audit.Audit())
	if err != nil {
		t.Fatal(err)
	}
	freshDB(t)
	mf, err := mm.Media().Upload(ctx, mediacontracts.OwnerProduct, 1, mediacontracts.Upload{
		OriginalName: "foto.png", Content: bytes.NewReader(pngBytes), Accept: []string{"image/"},
	}, 0)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	url, err := mm.Media().GetURL(ctx, mf)
	if err != nil {
		t.Fatalf("GetURL: %v", err)
	}
	if !strings.HasPrefix(url, "/media/") || !strings.Contains(url, "sig=") {
		t.Fatalf("URL harus bertanda tangan di bawah /media/: %q", url)
	}

	srv := server.New(&config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	srv.MountMedia(mediaDir, signer)
	h := srv.Handler()

	fetch := func(target string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		return rec
	}
	if rec := fetch(url); rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), pngBytes) {
		t.Fatalf("URL bertanda tangan: code=%d, byte sama=%v", rec.Code, bytes.Equal(rec.Body.Bytes(), pngBytes))
	}
	// The same path without its signature is refused.
	if rec := fetch(strings.SplitN(url, "?", 2)[0]); rec.Code != http.StatusForbidden {
		t.Fatalf("tanpa tanda tangan: code=%d, want 403", rec.Code)
	}
	for _, target := range []string{"/uploads/", "/uploads/whatsapp_session.db", "/media/_private/whatsapp_session.db", "/media/"} {
		rec := fetch(target)
		if rec.Code == http.StatusOK || strings.Contains(rec.Body.String(), "SESSION-CREDENTIAL") {
			t.Errorf("%s: code=%d, kredensial atau listing bocor", target, rec.Code)
		}
	}
}
