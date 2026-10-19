package database

import (
	"strings"
	"testing"
)

// F4-12: the DSN carries the driver parameters production relies on.
func TestEnsureParams(t *testing.T) {
	got := ensureParams("u:p@tcp(h:3306)/db")
	for _, want := range []string{"parseTime=true", "loc=UTC", "timeout=5s", "readTimeout=30s", "writeTimeout=30s"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q missing %s", got, want)
		}
	}
	if !strings.Contains(got, "?") || strings.Count(got, "?") != 1 {
		t.Errorf("one query separator expected: %q", got)
	}
	// An operator override is never duplicated.
	got = ensureParams("u:p@tcp(h)/db?readTimeout=2m&parseTime=true")
	if strings.Count(got, "readTimeout=") != 1 || !strings.Contains(got, "readTimeout=2m") || strings.Count(got, "parseTime=") != 1 {
		t.Errorf("override duplicated or lost: %q", got)
	}
}

// Session attributes are applied per connection, quoted (never as DSN system vars).
func TestSessionInitIsIsolationAndUTC(t *testing.T) {
	joined := strings.Join(sessionInit, ";")
	if !strings.Contains(joined, "time_zone = '+00:00'") || !strings.Contains(joined, "READ COMMITTED") {
		t.Fatalf("sessionInit = %v", sessionInit)
	}
}
