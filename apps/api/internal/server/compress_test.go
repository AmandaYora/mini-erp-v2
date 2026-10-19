package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serve(t *testing.T, acceptEncoding string, h http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/thing", nil)
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	rec := httptest.NewRecorder()
	compress(h).ServeHTTP(rec, req)
	return rec
}

func jsonBody(size int) string {
	return `{"success":true,"message":"ok","data":"` + strings.Repeat("a", size) + `"}`
}

// A report-sized JSON body is the case compression exists for.
func TestCompressLargeJSON(t *testing.T) {
	body := jsonBody(8000)
	rec := serve(t, "gzip", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	})
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if rec.Header().Get("Vary") != "Accept-Encoding" {
		t.Fatalf("Vary = %q, want Accept-Encoding", rec.Header().Get("Vary"))
	}
	if rec.Body.Len() >= len(body) {
		t.Fatalf("compressed size %d >= raw %d", rec.Body.Len(), len(body))
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Fatalf("round trip changed the body")
	}
}

// Small envelopes are the common case and must not pay for a gzip header.
func TestCompressSkipsSmallBody(t *testing.T) {
	body := `{"success":true,"message":"ok","data":null}`
	rec := serve(t, "gzip", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	})
	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatalf("small body was compressed")
	}
	if rec.Body.String() != body {
		t.Fatalf("body = %q, want %q", rec.Body.String(), body)
	}
}

// Already-compressed payloads must pass through untouched.
func TestCompressSkipsBinary(t *testing.T) {
	body := strings.Repeat("x", 8000)
	rec := serve(t, "gzip", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = io.WriteString(w, body)
	})
	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatalf("png was compressed")
	}
	if rec.Body.Len() != len(body) {
		t.Fatalf("body length = %d, want %d", rec.Body.Len(), len(body))
	}
}

// A client that does not advertise gzip gets bytes exactly as written.
func TestCompressSkipsWithoutAcceptEncoding(t *testing.T) {
	body := jsonBody(8000)
	rec := serve(t, "", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	})
	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatalf("compressed without Accept-Encoding")
	}
	if rec.Body.String() != body {
		t.Fatalf("body changed")
	}
}

// The status a handler writes must survive the buffering, including errors
// carrying a large body.
func TestCompressPreservesStatus(t *testing.T) {
	rec := serve(t, "gzip", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, jsonBody(4000))
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("large error body was not compressed")
	}
}

// A handler that writes nothing (204) must still finish cleanly.
func TestCompressEmptyBody(t *testing.T) {
	rec := serve(t, "gzip", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", rec.Body.String())
	}
}

// Many small writes that add up past the threshold must still compress, and
// must reassemble in order.
func TestCompressAccumulatesChunkedWrites(t *testing.T) {
	chunk := strings.Repeat("b", 200)
	rec := serve(t, "gzip", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		for i := 0; i < 20; i++ {
			_, _ = io.WriteString(w, chunk)
		}
	})
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("accumulated body was not compressed")
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != strings.Repeat(chunk, 20) {
		t.Fatalf("chunked body did not round trip")
	}
}

// A range response must pass through untouched: its bytes are an offset into
// the identity-coded file, and Content-Range describes them.
func TestCompressSkipsPartialContent(t *testing.T) {
	body := jsonBody(8000)
	rec := serve(t, "gzip", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Range", "bytes 0-7999/20000")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = io.WriteString(w, body)
	})
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", rec.Code)
	}
	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatalf("206 was compressed")
	}
	if rec.Body.String() != body {
		t.Fatalf("range body changed")
	}
}

// A large incompressible body streams past the decision point. The wrapper
// must not stamp a Content-Length taken from the threshold-sized buffer.
func TestCompressStreamedBinaryHasNoTruncatedLength(t *testing.T) {
	body := strings.Repeat("y", 50000)
	rec := serve(t, "gzip", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		for i := 0; i < 50; i++ {
			_, _ = io.WriteString(w, body[i*1000:(i+1)*1000])
		}
	})
	if got := rec.Header().Get("Content-Length"); got != "" && got != "50000" {
		t.Fatalf("Content-Length = %q, want empty or 50000 — not the buffer threshold", got)
	}
	if rec.Body.Len() != 50000 {
		t.Fatalf("body length = %d, want 50000", rec.Body.Len())
	}
}
