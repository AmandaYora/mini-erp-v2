// Package signedurl signs and verifies short-lived URLs for files served from
// local disk. It is the local-driver counterpart of an S3 presigned URL: an
// <img src> or <a href> cannot carry an Authorization header, so the proof of
// access travels in the URL itself.
//
// A signature binds exactly one path to one expiry. Changing either invalidates
// it. The clock and key are injected so behaviour is testable.
package signedurl

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"time"
)

var (
	// ErrBadSignature means the signature is missing, malformed, or does not match.
	ErrBadSignature = errors.New("tanda tangan URL tidak valid")
	// ErrExpired means the signature was valid but its expiry has passed.
	ErrExpired = errors.New("tautan sudah kedaluwarsa")
)

// Signer issues and checks signatures with one secret key.
type Signer struct {
	key []byte
	now func() time.Time
}

// New builds a Signer. An empty key is rejected by callers (config derives one);
// New itself accepts any non-empty key.
func New(key []byte) *Signer {
	return &Signer{key: append([]byte(nil), key...), now: time.Now}
}

// WithClock returns a copy that reads time from now (tests).
func (s *Signer) WithClock(now func() time.Time) *Signer {
	c := *s
	c.now = now
	return &c
}

// Derive returns a key derived from base and a purpose label, so one configured
// secret can serve several unrelated uses without sharing raw key material.
func Derive(base, purpose string) []byte {
	m := hmac.New(sha256.New, []byte(base))
	m.Write([]byte(purpose))
	return m.Sum(nil)
}

func (s *Signer) mac(path string, exp int64) []byte {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte("GET\n"))
	m.Write([]byte(path))
	m.Write([]byte("\n"))
	m.Write([]byte(strconv.FormatInt(exp, 10)))
	return m.Sum(nil)
}

// Sign returns the expiry (unix seconds) and signature for path. The expiry is
// rounded up to a window boundary plus ttl, so every URL for the same file
// issued within one window is identical — which keeps browser caching useful
// instead of producing a new URL on every API response.
func (s *Signer) Sign(path string, window, ttl time.Duration) (exp int64, sig string) {
	now := s.now()
	if window <= 0 {
		window = time.Hour
	}
	boundary := now.Truncate(window).Add(window)
	exp = boundary.Add(ttl).Unix()
	return exp, base64.RawURLEncoding.EncodeToString(s.mac(path, exp))
}

// Verify checks sig for path and exp, then the clock. The signature is compared
// in constant time before expiry so a forged request learns nothing about expiry.
func (s *Signer) Verify(path string, exp int64, sig string) error {
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || len(got) == 0 {
		return ErrBadSignature
	}
	if !hmac.Equal(got, s.mac(path, exp)) {
		return ErrBadSignature
	}
	if s.now().Unix() > exp {
		return ErrExpired
	}
	return nil
}

// Remaining reports how long a (already verified) expiry has left.
func (s *Signer) Remaining(exp int64) time.Duration {
	d := time.Unix(exp, 0).Sub(s.now())
	if d < 0 {
		return 0
	}
	return d
}
