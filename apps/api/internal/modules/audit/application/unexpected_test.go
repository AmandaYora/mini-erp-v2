package application

import (
	"strings"
	"testing"
	"time"
)

func TestScrubMessageRemovesCredentialsAndBoundsLength(t *testing.T) {
	got := scrubMessage("db error password=hunter2 token: abc.def.ghi Authorization: Bearer xyz")
	for _, leak := range []string{"hunter2", "abc.def.ghi", "xyz"} {
		if strings.Contains(got, leak) {
			t.Fatalf("scrubbed message %q still leaks %q", got, leak)
		}
	}
	if long := scrubMessage(strings.Repeat("x", 1000)); len([]rune(long)) != unexpectedMessageMax {
		t.Fatalf("long message length = %d, want %d", len([]rune(long)), unexpectedMessageMax)
	}
}

func TestMaskRouteHidesIDsAndQuery(t *testing.T) {
	if got := maskRoute("/api/v1/sales/orders/123/items/9?token=abc"); got != "/api/v1/sales/orders/{id}/items/{id}" {
		t.Fatalf("maskRoute = %q", got)
	}
	if got := maskRoute("GET /api/v1/sales/orders/{id}"); got != "GET /api/v1/sales/orders/{id}" {
		t.Fatalf("registered pattern changed: %q", got)
	}
}

func TestAdmitDedupesAndRateLimits(t *testing.T) {
	var u unexpectedState
	t0 := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	if !u.admit("a", t0) {
		t.Fatal("first occurrence must be admitted")
	}
	if u.admit("a", t0.Add(10*time.Second)) {
		t.Fatal("duplicate inside the window must be dropped")
	}
	if !u.admit("a", t0.Add(unexpectedDedupeWindow+time.Second)) {
		t.Fatal("duplicate after the window must be admitted again")
	}
	// Budget: distinct keys inside one minute stop at unexpectedMaxPerMinute.
	var v unexpectedState
	admitted := 0
	for i := 0; i < unexpectedMaxPerMinute*3; i++ {
		if v.admit(string(rune('A'+i%26))+strings.Repeat("k", i), t0.Add(time.Duration(i)*time.Millisecond)) {
			admitted++
		}
	}
	if admitted != unexpectedMaxPerMinute {
		t.Fatalf("admitted %d within a minute, want the budget %d", admitted, unexpectedMaxPerMinute)
	}
	if !v.admit("fresh", t0.Add(61*time.Second)) {
		t.Fatal("a new minute must reset the budget")
	}
}
