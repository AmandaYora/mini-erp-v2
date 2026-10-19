package signedurl

import (
	"testing"
	"time"
)

func fixed(t time.Time) func() time.Time { return func() time.Time { return t } }

var base = time.Date(2026, 10, 8, 10, 30, 0, 0, time.UTC)

func TestSignVerifyRoundTrip(t *testing.T) {
	s := New([]byte("k")).WithClock(fixed(base))
	exp, sig := s.Sign("product/2026/09/a.jpg", time.Hour, 5*time.Minute)
	if err := s.Verify("product/2026/09/a.jpg", exp, sig); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	s := New([]byte("k")).WithClock(fixed(base))
	exp, sig := s.Sign("a.jpg", time.Hour, time.Minute)
	cases := map[string]func() error{
		"path lain":        func() error { return s.Verify("b.jpg", exp, sig) },
		"exp diubah":       func() error { return s.Verify("a.jpg", exp+1, sig) },
		"sig kosong":       func() error { return s.Verify("a.jpg", exp, "") },
		"sig bukan b64":    func() error { return s.Verify("a.jpg", exp, "!!!") },
		"sig lebih pendek": func() error { return s.Verify("a.jpg", exp, sig[:len(sig)-4]) },
		"kunci lain":       func() error { return New([]byte("x")).WithClock(fixed(base)).Verify("a.jpg", exp, sig) },
	}
	for name, f := range cases {
		if err := f(); err != ErrBadSignature {
			t.Errorf("%s: err=%v, want ErrBadSignature", name, err)
		}
	}
}

func TestVerifyExpiry(t *testing.T) {
	s := New([]byte("k")).WithClock(fixed(base))
	exp, sig := s.Sign("a.jpg", time.Hour, 5*time.Minute)
	// Window boundary is 11:00; ttl 5 minutes => 11:05.
	if want := time.Date(2026, 10, 8, 11, 5, 0, 0, time.UTC).Unix(); exp != want {
		t.Fatalf("exp=%d want %d", exp, want)
	}
	later := New([]byte("k")).WithClock(fixed(time.Unix(exp, 0).Add(2 * time.Second)))
	if err := later.Verify("a.jpg", exp, sig); err != ErrExpired {
		t.Fatalf("err=%v, want ErrExpired", err)
	}
	atLimit := New([]byte("k")).WithClock(fixed(time.Unix(exp, 0)))
	if err := atLimit.Verify("a.jpg", exp, sig); err != nil {
		t.Fatalf("pada batas exp masih sah: %v", err)
	}
}

func TestSameWindowSameURL(t *testing.T) {
	a := New([]byte("k")).WithClock(fixed(base))
	b := New([]byte("k")).WithClock(fixed(base.Add(20 * time.Minute)))
	e1, s1 := a.Sign("a.jpg", time.Hour, 5*time.Minute)
	e2, s2 := b.Sign("a.jpg", time.Hour, 5*time.Minute)
	if e1 != e2 || s1 != s2 {
		t.Fatal("URL dalam satu jendela harus identik agar cache browser bekerja")
	}
	c := New([]byte("k")).WithClock(fixed(base.Add(40 * time.Minute)))
	e3, _ := c.Sign("a.jpg", time.Hour, 5*time.Minute)
	if e3 == e1 {
		t.Fatal("jendela berikutnya harus menghasilkan exp baru")
	}
}

func TestClockGoingBackwardsNeverExtends(t *testing.T) {
	s := New([]byte("k")).WithClock(fixed(base))
	exp, sig := s.Sign("a.jpg", time.Hour, time.Minute)
	earlier := New([]byte("k")).WithClock(fixed(base.Add(-48 * time.Hour)))
	// An earlier clock cannot make a signature invalid-by-expiry, but the
	// signature must still be bound to its own exp.
	if err := earlier.Verify("a.jpg", exp+3600, sig); err != ErrBadSignature {
		t.Fatalf("exp yang digeser harus ditolak: %v", err)
	}
}

func TestDerive(t *testing.T) {
	a := Derive("secret", "media-url-v1")
	b := Derive("secret", "other")
	if string(a) == string(b) || len(a) != 32 {
		t.Fatal("Derive harus menghasilkan kunci berbeda per tujuan, 32 byte")
	}
}
