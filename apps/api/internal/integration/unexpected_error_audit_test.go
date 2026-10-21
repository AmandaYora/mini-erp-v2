package integration

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mini-erp/internal/config"
	auditapp "mini-erp/internal/modules/audit/application"
	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/server"
)

// OP-06: panics and unexpected 5xx responses land in the audit trail exactly
// once per (route, status, message), within a per-minute budget, and without any
// request data.
func TestUnexpectedErrorsReachAuditTrailDedupedAndBounded(t *testing.T) {
	freshDB(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := server.New(&config.Config{}, log)
	srv.SetErrorReporter(func(c context.Context, e server.UnexpectedError) {
		testMods.audit.Service().RecordUnexpected(c, auditapp.Unexpected{Route: e.Route, Status: e.Status, Panic: e.Panic,
			Message: e.Message, RequestID: e.RequestID, UserID: e.UserID, BranchID: e.BranchID})
	})
	mux := srv.Mux()
	mux.HandleFunc("POST /api/v1/optest/panic/{id}", func(w http.ResponseWriter, r *http.Request) {
		panic("koneksi gagal password=hunter2")
	})
	mux.HandleFunc("GET /api/v1/optest/fail", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	mux.HandleFunc("GET /api/v1/optest/notfound", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })
	for i := 0; i < 30; i++ {
		mux.HandleFunc(fmt.Sprintf("GET /api/v1/optest/burst%d", i), func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	}
	h := srv.Handler()
	count := func() int {
		var n int
		if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action_key = 'error.unexpected'`).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}
	do := func(method, path string) int {
		req := httptest.NewRequest(method, path, strings.NewReader(`{"password":"rahasia-body"}`))
		req.Header.Set("Authorization", "Bearer rahasia-token")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	// (1) one panic -> one row, and the client still gets the 500 envelope.
	if code := do("POST", "/api/v1/optest/panic/77?token=rahasia-query"); code != 500 {
		t.Fatalf("panic code = %d, want 500", code)
	}
	if n := count(); n != 1 {
		t.Fatalf("rows after one panic = %d, want 1", n)
	}
	// (2) the same failure again (even for another id) does not add rows.
	do("POST", "/api/v1/optest/panic/78")
	do("POST", "/api/v1/optest/panic/79")
	if n := count(); n != 1 {
		t.Fatalf("rows after duplicates = %d, want 1 (deduped)", n)
	}
	// (3) a plain 5xx is recorded; 4xx is not.
	do("GET", "/api/v1/optest/fail")
	do("GET", "/api/v1/optest/notfound")
	if n := count(); n != 2 {
		t.Fatalf("rows after a 500 and a 404 = %d, want 2", n)
	}
	// (4) nothing from the request is stored; the route is the pattern.
	var note, meta string
	if err := testMods.db.QueryRow(`SELECT a.note, CAST(d.metadata_json AS CHAR) FROM audit_logs a JOIN audit_log_details d ON d.log_id = a.id
		WHERE a.action_key = 'error.unexpected' AND a.note LIKE '%panic%'`).Scan(&note, &meta); err != nil {
		t.Fatalf("read row: %v", err)
	}
	// The concrete path, not a bare "77": the row's random request id may
	// contain those digits.
	for _, leak := range []string{"hunter2", "rahasia", "panic/77", "Bearer"} {
		if strings.Contains(note+meta, leak) {
			t.Fatalf("audit row leaks %q: note=%q meta=%s", leak, note, meta)
		}
	}
	if !strings.Contains(note, "/api/v1/optest/panic/{id}") || !strings.Contains(meta, `"panic": true`) {
		t.Fatalf("row = note %q meta %s, want the route pattern and the panic flag", note, meta)
	}
	// (5) a storm across many routes is capped by the per-minute budget.
	for i := 0; i < 30; i++ {
		do("GET", fmt.Sprintf("/api/v1/optest/burst%d", i))
	}
	if n := count(); n > 20 {
		t.Fatalf("rows after a 30-route storm = %d, want at most the budget of 20", n)
	}
	// (6) the audit page reads them.
	recs, total, err := testMods.audit.Service().Query(context.Background(), auditcontracts.ListFilter{AllBranches: true, Action: "error.", Page: 1, Limit: 50})
	if err != nil || total == 0 || len(recs) == 0 || recs[0].Entity != "error" {
		t.Fatalf("audit query = %d rows err=%v, want the error rows", total, err)
	}
}
