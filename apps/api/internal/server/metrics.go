package server

import (
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics are hand-rolled (no new dependency): per-route counters, duration
// buckets, in-flight gauge, and DB pool gauges filled by the composition
// root. Cardinality stays low because routes use mux patterns (F6-5).
var (
	mu       sync.Mutex
	counts   = map[string]*int64{}
	hists    = map[string]*[6]int64{}
	inFlight int64
	buckets  = []time.Duration{5 * time.Millisecond, 25 * time.Millisecond, 100 * time.Millisecond, 250 * time.Millisecond, time.Second, 5 * time.Second}
	dbOpen   int64
	dbInUse  int64
	dbIdle   int64
	dbWait   int64
	started  = time.Now()
)

// Observe records one completed request.
func Observe(route, method string, status int, d time.Duration) {
	key := route + "\x00" + method + "\x00" + statusClass(status)
	mu.Lock()
	c, ok := counts[key]
	if !ok {
		var v int64
		counts[key] = &v
		c = &v
		h := &[6]int64{}
		hists[key] = h
	}
	h := hists[key]
	mu.Unlock()
	atomic.AddInt64(c, 1)
	for i, b := range buckets {
		if d <= b {
			atomic.AddInt64(&h[i], 1)
			break
		}
		if i == len(buckets)-1 {
			atomic.AddInt64(&h[i], 1)
		}
	}
}

func statusClass(status int) string {
	switch {
	case status < 200:
		return "1xx"
	case status < 300:
		return "2xx"
	case status < 400:
		return "3xx"
	case status < 500:
		return "4xx"
	default:
		return "5xx"
	}
}

// TrackInFlight wraps the handler to expose concurrent requests.
func TrackInFlight(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&inFlight, 1)
		defer atomic.AddInt64(&inFlight, -1)
		next.ServeHTTP(w, r)
	})
}

// SetDBPool publishes database/sql pool stats for /metrics.
func SetDBPool(open, inUse, idle int, waitCount int64) {
	atomic.StoreInt64(&dbOpen, int64(open))
	atomic.StoreInt64(&dbInUse, int64(inUse))
	atomic.StoreInt64(&dbIdle, int64(idle))
	atomic.StoreInt64(&dbWait, waitCount)
}

// MetricsHandler serves Prometheus text format. Empty token disables it (404).
func MetricsHandler(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			http.NotFound(w, r)
			return
		}
		const prefix = "Bearer "
		auth := r.Header.Get("Authorization")
		if len(auth) <= len(prefix) || auth[:len(prefix)] != prefix || auth[len(prefix):] != token {
			http.NotFound(w, r)
			return
		}
		var sb strings.Builder
		mu.Lock()
		keys := make([]string, 0, len(counts))
		for k := range counts {
			keys = append(keys, k)
		}
		mu.Unlock()
		sort.Strings(keys)
		fmt.Fprintln(&sb, "# TYPE http_requests_total counter")
		for _, k := range keys {
			parts := strings.Split(k, "\x00")
			fmt.Fprintf(&sb, "http_requests_total{route=%q,method=%q,class=%q} %d\n",
				parts[0], parts[1], parts[2], atomic.LoadInt64(counts[k]))
		}
		fmt.Fprintln(&sb, "# TYPE http_in_flight gauge")
		fmt.Fprintf(&sb, "http_in_flight %d\n", atomic.LoadInt64(&inFlight))
		fmt.Fprintln(&sb, "# TYPE db_pool_open gauge")
		fmt.Fprintf(&sb, "db_pool_open %d\ndb_pool_in_use %d\ndb_pool_idle %d\ndb_pool_wait_count %d\n",
			atomic.LoadInt64(&dbOpen), atomic.LoadInt64(&dbInUse), atomic.LoadInt64(&dbIdle), atomic.LoadInt64(&dbWait))
		fmt.Fprintln(&sb, "# TYPE go_goroutines gauge")
		fmt.Fprintf(&sb, "go_goroutines %d\n", runtime.NumGoroutine())
		fmt.Fprintf(&sb, "process_start_time %d\n", started.Unix())
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(sb.String()))
	}
}
