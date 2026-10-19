package clientip

import (
	"net"
	"net/http/httptest"
	"testing"
)

func mustNets(t *testing.T, s string) []*net.IPNet {
	t.Helper()
	nets, err := ParseTrusted(s)
	if err != nil {
		t.Fatal(err)
	}
	return nets
}

// TestResolveIgnoresHeaderWithoutProxy (F1-3): a forged X-Forwarded-For must
// never override the peer when no proxy is trusted.
func TestResolveIgnoresHeaderWithoutProxy(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.7:1234"
	r.Header.Set("X-Forwarded-For", "10.9.9.9")
	if got := Resolve(r, nil, ""); got != "203.0.113.7" {
		t.Fatalf("got %q, want peer 203.0.113.7", got)
	}
}

// TestResolveChainFromRight (F1-3): with a trusted proxy, the chain is
// scanned from the right skipping trusted entries, so a spoofed left entry
// can never override the real client.
func TestResolveChainFromRight(t *testing.T) {
	trusted := mustNets(t, "127.0.0.1/32")
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:4321"
	// Leftmost entry is attacker-spoofed; the trusted proxy appended the
	// real client (10.1.2.3) before its own address. Scanning from the
	// right yields 10.1.2.3 — the spoof can never win.
	r.Header.Set("X-Forwarded-For", "9.9.9.9, 10.1.2.3, 127.0.0.1")
	if got := Resolve(r, trusted, ""); got != "10.1.2.3" {
		t.Fatalf("got %q, want 10.1.2.3", got)
	}
}

// TestResolveSkipsUntrustedPeer (F1-3): header is honoured only when the
// direct peer itself is trusted; otherwise the peer wins.
func TestResolveSkipsUntrustedPeer(t *testing.T) {
	trusted := mustNets(t, "127.0.0.1/32")
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "198.51.100.9:1234"
	r.Header.Set("X-Forwarded-For", "10.1.2.3")
	if got := Resolve(r, trusted, ""); got != "198.51.100.9" {
		t.Fatalf("got %q, want peer 198.51.100.9", got)
	}
}

// TestResolveIPv6 (F1-3): bracketed IPv6 peers parse via SplitHostPort and
// bare IPv6 entries become /128 trust.
func TestResolveIPv6(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "[::1]:1234"
	if got := Resolve(r, nil, ""); got != "::1" {
		t.Fatalf("got %q, want ::1", got)
	}
	trusted := mustNets(t, "::1")
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.RemoteAddr = "[::1]:5678"
	r2.Header.Set("X-Forwarded-For", "2001:db8::5")
	if got := Resolve(r2, trusted, ""); got != "2001:db8::5" {
		t.Fatalf("got %q, want 2001:db8::5", got)
	}
}

// TestResolveGarbageHeader (F1-3): an unparseable header falls back to peer,
// never to an empty string.
func TestResolveGarbageHeader(t *testing.T) {
	trusted := mustNets(t, "127.0.0.1/32")
	for _, h := range []string{"", "sampah", "999.999.999.999", ", ,"} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "127.0.0.1:1234"
		r.Header.Set("X-Forwarded-For", h)
		if got := Resolve(r, trusted, ""); got != "127.0.0.1" {
			t.Fatalf("header %q: got %q, want peer", h, got)
		}
	}
}

// TestParseTrustedShapes (F1-3): CIDR kept, bare IP becomes /32 (/128 v6),
// blanks skipped, garbage skipped without error.
func TestParseTrustedShapes(t *testing.T) {
	nets := mustNets(t, "127.0.0.1, 10.0.0.0/8, ::1, , sampah")
	if len(nets) != 3 {
		t.Fatalf("got %d nets, want 3", len(nets))
	}
	if ones, _ := nets[0].Mask.Size(); ones != 32 {
		t.Fatalf("bare IPv4 mask = /%d, want /32", ones)
	}
	if ones, _ := nets[2].Mask.Size(); ones != 128 {
		t.Fatalf("bare IPv6 mask = /%d, want /128", ones)
	}
	if nets := mustNets(t, ""); len(nets) != 0 {
		t.Fatalf("empty input must yield no nets, got %d", len(nets))
	}
}
