package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// gzipMinSize is the smallest body worth compressing. Below roughly this
// size the gzip header and trailer cost more than the deflate saves, and the
// CPU is spent for nothing — most single-record API responses land here.
const gzipMinSize = 1024

// gzipWriterPool reuses compressors: a new gzip.Writer allocates a ~64KB
// window, and allocating one per response would trade database time for GC
// time.
var gzipWriterPool = sync.Pool{
	New: func() any { return gzip.NewWriter(io.Discard) },
}

// compressibleTypes are the content types worth compressing. Images, video,
// PDFs and fonts are already compressed — running deflate over them burns
// CPU to make the body marginally larger.
func compressible(contentType string) bool {
	ct := contentType
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	ct = strings.TrimSpace(strings.ToLower(ct))
	switch {
	case strings.HasPrefix(ct, "text/"):
		return true
	case ct == "application/json", ct == "application/javascript",
		ct == "application/xml", ct == "image/svg+xml",
		ct == "application/manifest+json":
		return true
	}
	return false
}

// gzipResponseWriter buffers the first write to decide, from the real
// Content-Type and size, whether compressing is worth it. Deciding up front
// from the request alone would either compress a 40-byte JSON envelope or
// skip a 2MB report — the answer is only knowable once the handler writes.
type gzipResponseWriter struct {
	http.ResponseWriter
	gz          *gzip.Writer
	buf         []byte
	wroteHead   bool
	status      int
	decided     bool
	compressing bool
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	// Held back until the body decides — Content-Encoding must be set before
	// the header goes out, and that needs the first bytes of the body.
	w.status = code
	w.wroteHead = true
}

// Unwrap exposes the underlying writer for http.ResponseController (F6-4).
func (w *gzipResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if w.decided {
		if w.compressing {
			return w.gz.Write(b)
		}
		return w.ResponseWriter.Write(b)
	}
	w.buf = append(w.buf, b...)
	if len(w.buf) < gzipMinSize {
		// Not enough yet to judge; hold and wait for more or for flush().
		return len(b), nil
	}
	w.decide(false)
	return len(b), nil
}

// decide commits to compressing or not, emits the header, and drains the
// buffer through the chosen path. final reports whether the buffer is the
// whole body — only then is its length the response's length.
func (w *gzipResponseWriter) decide(final bool) {
	w.decided = true
	header := w.ResponseWriter.Header()
	status := w.status
	if status == 0 {
		status = http.StatusOK
	}
	// A handler that set its own encoding owns the body. So does a range
	// response: 206 bytes are an offset into the identity-coded file, and
	// gzipping them would make Content-Range describe something else.
	alreadyEncoded := header.Get("Content-Encoding") != ""
	isPartial := status == http.StatusPartialContent
	w.compressing = !alreadyEncoded && !isPartial &&
		len(w.buf) >= gzipMinSize &&
		compressible(header.Get("Content-Type"))
	if w.compressing {
		header.Set("Content-Encoding", "gzip")
		header.Add("Vary", "Accept-Encoding")
		// Length is the uncompressed size; it no longer describes the body.
		header.Del("Content-Length")
	} else if final && !w.wroteHead && header.Get("Content-Length") == "" {
		// Only safe when the buffer IS the body. Deciding mid-stream means
		// more bytes are still coming, and a length taken here would be the
		// threshold, not the response.
		header.Set("Content-Length", strconv.Itoa(len(w.buf)))
	}
	w.ResponseWriter.WriteHeader(status)
	if len(w.buf) == 0 {
		return
	}
	if w.compressing {
		w.gz.Reset(w.ResponseWriter)
		_, _ = w.gz.Write(w.buf)
	} else {
		_, _ = w.ResponseWriter.Write(w.buf)
	}
	w.buf = nil
}

// flush finishes the response: small bodies never reached the threshold, so
// they are emitted here uncompressed.
func (w *gzipResponseWriter) flush() {
	if !w.decided {
		w.decide(true)
	}
	if w.compressing {
		_ = w.gz.Close()
	}
}

// Flush implements http.Flusher for streaming handlers. Deciding early is
// correct here: a handler that flushes wants bytes on the wire now.
func (w *gzipResponseWriter) Flush() {
	if !w.decided {
		w.decide(false)
	}
	if w.compressing {
		_ = w.gz.Flush()
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// compress gzips responses for clients that accept it.
//
// Report and list endpoints return JSON that compresses by roughly an order
// of magnitude — repeated keys, repeated codes, long runs of digits — and
// the SPA bundle likewise. Nothing in the stack did this before: neither the
// Go server nor the optional nginx in front of it, so every byte went raw.
func compress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		gz := gzipWriterPool.Get().(*gzip.Writer)
		defer gzipWriterPool.Put(gz)
		gw := &gzipResponseWriter{ResponseWriter: w, gz: gz}
		defer gw.flush()
		next.ServeHTTP(gw, r)
	})
}
