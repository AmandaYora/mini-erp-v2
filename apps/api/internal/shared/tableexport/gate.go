package tableexport

import (
	"context"
	"net/http"
	"time"

	"mini-erp/internal/shared/apperror"
)

// MaxConcurrent bounds exports running at once: each holds one DB connection
// while it pages, and the pool is shared with every interactive request.
const MaxConcurrent = 2

// gateWait is how long a third export waits before it is refused.
const gateWait = 10 * time.Second

// WriteWindow is the write deadline an export request is granted (the
// server-wide WriteTimeout is shorter, see F4-11).
const WriteWindow = 120 * time.Second

var gate = make(chan struct{}, MaxConcurrent)

// Acquire takes an export slot; the returned release must be called once. A
// request that waits longer than gateWait gets RateLimited.
func Acquire(ctx context.Context) (release func(), err error) {
	t := time.NewTimer(gateWait)
	defer t.Stop()
	select {
	case gate <- struct{}{}:
		return func() { <-gate }, nil
	case <-t.C:
		return nil, apperror.RateLimited()
	case <-ctx.Done():
		return nil, apperror.RateLimited()
	}
}

// ExtendWriteDeadline lifts the per-connection write deadline for a long
// download. Failure (writer without deadline support) is ignored: the global
// timeout then simply applies.
func ExtendWriteDeadline(w http.ResponseWriter) {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(WriteWindow))
}

// Stream sends the file with download headers.
func Stream(w http.ResponseWriter, f *File) {
	defer f.Close()
	w.Header().Set("Content-Type", f.ContentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+f.Name+`"`)
	w.WriteHeader(http.StatusOK)
	_ = f.Send(w)
}
