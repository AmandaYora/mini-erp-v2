package clientip

import (
	"net"
	"net/http"
	"strings"
)

// Resolve returns the client IP for r. RemoteAddr's peer is trusted by
// default; the forwarding header is honoured only when the peer itself is in
// trusted (e.g. loopback / docker network when behind nginx). For
// X-Forwarded-For the chain is scanned from the right, skipping trusted
// proxies, so a spoofed left entry can never override the real client.
func Resolve(r *http.Request, trusted []*net.IPNet, header string) string {
	peer := remoteIP(r.RemoteAddr)
	if len(trusted) == 0 || !inNet(peer, trusted) {
		return peer
	}
	h := header
	if h == "" {
		h = "X-Forwarded-For"
	}
	raw := r.Header.Get(h)
	if h == "X-Forwarded-For" {
		parts := strings.Split(raw, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			cand := strings.TrimSpace(parts[i])
			if cand == "" {
				continue
			}
			ip := net.ParseIP(strings.Trim(cand, "[]"))
			if ip == nil {
				continue
			}
			if inNet(cand, trusted) {
				continue
			}
			return cand
		}
		return peer
	}
	if cand := strings.TrimSpace(raw); cand != "" {
		if ip := net.ParseIP(strings.Trim(cand, "[]")); ip != nil {
			return cand
		}
	}
	return peer
}

func remoteIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

func inNet(ipStr string, nets []*net.IPNet) bool {
	ip := net.ParseIP(strings.Trim(ipStr, "[]"))
	if ip == nil {
		return false
	}
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// ParseTrusted parses comma-separated CIDR/IP entries. Bare IPs become /32
// (/128 for IPv6). Empty input yields no trusted networks.
func ParseTrusted(s string) ([]*net.IPNet, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	var out []*net.IPNet
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, n, err := net.ParseCIDR(part); err == nil {
			out = append(out, n)
			continue
		}
		ip := net.ParseIP(strings.Trim(part, "[]"))
		if ip == nil {
			continue
		}
		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		_, n, _ := net.ParseCIDR(ip.String() + "/" + itoa(bits))
		out = append(out, n)
	}
	return out, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
