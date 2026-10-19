package application

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"mini-erp/internal/modules/audit/contracts"
)

// OP-06: unexpected server errors (panics and 5xx responses) reach the audit
// trail, so an operator sees them on the audit page without reading logs.
// The write path is deliberately small and bounded:
//
//   - dedupe: one row per (route, status, message) per unexpectedDedupeWindow,
//     so a crash loop on one endpoint is one row, not thousands;
//   - budget: at most unexpectedMaxPerMinute rows per minute overall, so an
//     outage that fans out over many routes cannot turn into a write storm;
//   - no sensitive data: never the request body, headers, query string or
//     tokens; the message is truncated and scrubbed of credential-looking
//     text; the route is the registered pattern (or the path with ids masked).
//
// It never fails the request it describes: every error is swallowed.

const (
	unexpectedDedupeWindow = 60 * time.Second
	unexpectedMaxPerMinute = 20
	unexpectedMaxKeys      = 500
	unexpectedMessageMax   = 200
)

// Unexpected describes one server error to record.
type Unexpected struct {
	Route     string // registered pattern, e.g. "POST /api/v1/sales/orders/{id}"
	Status    int
	Panic     bool
	Message   string // panic value or short description; scrubbed here
	RequestID string
	UserID    int64
	BranchID  int64
}

type unexpectedState struct {
	mu          sync.Mutex
	seen        map[string]time.Time
	windowStart time.Time
	windowCount int
}

var (
	secretPattern = regexp.MustCompile(`(?i)(password|passwd|pwd|secret|token|authorization|api[_-]?key|cookie|dsn)(["']?\s*[:=]?\s*)((?:bearer|basic)\s+)?(\S+)`)
	schemePattern = regexp.MustCompile(`(?i)(bearer|basic)\s+\S+`)
	numberPattern = regexp.MustCompile(`/\d+`)
)

// scrubMessage removes credential-looking text and bounds the length.
func scrubMessage(msg string) string {
	msg = strings.Join(strings.Fields(msg), " ")
	msg = secretPattern.ReplaceAllString(msg, "${1}${2}[disembunyikan]")
	msg = schemePattern.ReplaceAllString(msg, "${1} [disembunyikan]")
	if r := []rune(msg); len(r) > unexpectedMessageMax {
		msg = string(r[:unexpectedMessageMax])
	}
	return msg
}

// maskRoute keeps ids out of the route label when no pattern was registered.
func maskRoute(route string) string {
	if i := strings.Index(route, "?"); i >= 0 {
		route = route[:i]
	}
	return numberPattern.ReplaceAllString(route, "/{id}")
}

// RecordUnexpected writes one deduped, rate-limited error row. It reports
// whether a row was written (false: deduped, over budget, or storage failed).
func (s *Service) RecordUnexpected(ctx context.Context, e Unexpected) bool {
	defer func() { _ = recover() }() // logging must never take the request down
	now := s.clock()
	route := maskRoute(strings.TrimSpace(e.Route))
	if route == "" {
		route = "unknown"
	}
	message := scrubMessage(e.Message)
	key := route + "|" + strconv.Itoa(e.Status) + "|" + message
	if !s.unexpected.admit(key, now) {
		return false
	}
	actorType := "system"
	if e.UserID > 0 {
		actorType = "user"
	}
	note := strconv.Itoa(e.Status) + " " + route
	if message != "" {
		note += ": " + message
	}
	if r := []rune(note); len(r) > 480 {
		note = string(r[:480])
	}
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	err := s.repo.Log(wctx, contracts.Entry{
		Action: "error.unexpected", Entity: "error",
		BranchID: e.BranchID, ActorID: e.UserID, ActorType: actorType, Note: note,
		Metadata: map[string]any{"route": route, "status": e.Status, "panic": e.Panic, "requestId": e.RequestID},
	})
	if err != nil {
		s.log.Warn("catat error tak terduga ke audit gagal", "error", err)
		return false
	}
	return true
}

// admit applies the dedupe window and the per-minute budget.
func (u *unexpectedState) admit(key string, now time.Time) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.seen == nil {
		u.seen = map[string]time.Time{}
	}
	if now.Sub(u.windowStart) >= time.Minute {
		u.windowStart, u.windowCount = now, 0
	}
	if u.windowCount >= unexpectedMaxPerMinute {
		return false
	}
	if last, ok := u.seen[key]; ok && now.Sub(last) < unexpectedDedupeWindow {
		return false
	}
	if len(u.seen) >= unexpectedMaxKeys {
		for k, t := range u.seen {
			if now.Sub(t) >= unexpectedDedupeWindow {
				delete(u.seen, k)
			}
		}
		if len(u.seen) >= unexpectedMaxKeys {
			u.seen = map[string]time.Time{}
		}
	}
	u.seen[key] = now
	u.windowCount++
	return true
}

func (s *Service) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}
