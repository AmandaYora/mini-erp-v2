package response

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The four fixed-wording envelopes that used to be hand-written literals in
// auth/contracts and server must keep their exact JSON (F8-9 golden).
func TestFixedEnvelopesGolden(t *testing.T) {
	cases := []struct {
		name   string
		write  func(http.ResponseWriter)
		status int
		body   string
	}{
		{"session expired", SessionExpired, 401, `{"success":false,"message":"Sesi berakhir, silakan login kembali"}`},
		{"forbidden", Forbidden, 403, `{"success":false,"message":"Anda tidak memiliki akses"}`},
		{"internal", Internal, 500, `{"success":false,"message":"Terjadi kesalahan internal"}`},
		{"forbidden msg", func(w http.ResponseWriter) { ForbiddenMsg(w, "Cabang ditutup") }, 403, `{"success":false,"message":"Cabang ditutup"}`},
		{"unavailable", func(w http.ResponseWriter) { Unavailable(w, "Layanan belum siap") }, 503, `{"success":false,"message":"Layanan belum siap"}`},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		c.write(rec)
		if rec.Code != c.status {
			t.Errorf("%s: status %d want %d", c.name, rec.Code, c.status)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("%s: content-type %q", c.name, ct)
		}
		if got := strings.TrimSpace(rec.Body.String()); got != c.body {
			t.Errorf("%s: body %s want %s", c.name, got, c.body)
		}
	}
}

func TestMessageIsJSONEscaped(t *testing.T) {
	rec := httptest.NewRecorder()
	ForbiddenMsg(rec, `bad "quote"`)
	if !strings.Contains(rec.Body.String(), `bad \"quote\"`) {
		t.Fatalf("message not escaped: %s", rec.Body.String())
	}
}

func TestPaginatedMetaKeys(t *testing.T) {
	rec := httptest.NewRecorder()
	Paginated(rec, "ok", []int{}, Meta{Page: 1, Limit: 20, Total: 3, TotalPages: 1})
	if !strings.Contains(rec.Body.String(), `"meta":{"page":1,"limit":20,"total":3,"totalPages":1}`) {
		t.Fatalf("meta shape: %s", rec.Body.String())
	}
}
