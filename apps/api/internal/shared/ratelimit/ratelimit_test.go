package ratelimit

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestAllowEnforcesLimit: the (limit+1)-th event in one window is refused,
// and the next window admits again.
func TestAllowEnforcesLimit(t *testing.T) {
	l := New(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !l.Allow("k") {
			t.Fatalf("attempt %d: want allow", i+1)
		}
	}
	if l.Allow("k") {
		t.Fatal("attempt 4 in window: want refuse")
	}
}

// TestWindowReset uses an injected clock: after the interval the same key is
// admitted again.
func TestWindowReset(t *testing.T) {
	l := New(1, time.Minute)
	now := time.Now()
	l.now = func() time.Time { return now }
	if !l.Allow("k") {
		t.Fatal("first: want allow")
	}
	if l.Allow("k") {
		t.Fatal("second in window: want refuse")
	}
	now = now.Add(61 * time.Second)
	if !l.Allow("k") {
		t.Fatal("after window: want allow")
	}
}

// TestKeyFloodSweepsExpired (F1-3): flooding with fresh keys must sweep
// expired buckets instead of wiping active counters (the old "reset total di
// 10k" bug): after the flood expires, newcomers are admitted and the map
// stays bounded.
func TestKeyFloodSweepsExpired(t *testing.T) {
	l := New(2, time.Minute)
	now := time.Now()
	l.now = func() time.Time { return now }
	if !l.Allow("korban") {
		t.Fatal("setup: want allow")
	}
	for i := 0; i < 5000; i++ {
		l.Allow(fmt.Sprintf("banjir-%d", i))
	}
	now = now.Add(61 * time.Second)
	admitted := 0
	for i := 0; i < 3000; i++ {
		if l.Allow(fmt.Sprintf("baru-%d", i)) {
			admitted++
		}
	}
	if admitted != 3000 {
		t.Fatalf("admitted = %d, want 3000 (sweep harus membuang bucket kadaluarsa)", admitted)
	}
	if !l.Allow("korban") {
		t.Fatal("after expiry: want allow")
	}
	if n := len(l.buckets); n > 4096 {
		t.Fatalf("map = %d, want bounded (sweep tidak jalan)", n)
	}
}

// TestHardCapRefusesNewKeys (F1-3): with a full map of live buckets, a brand
// new key is refused while a known key still counts normally.
func TestHardCapRefusesNewKeys(t *testing.T) {
	l := New(1000000, time.Hour)
	l.Allow("penuh")
	for i := 0; i < 21000; i++ {
		k := fmt.Sprintf("penuh-%d", i)
		l.buckets[k] = &bucket{count: 1, reset: time.Now().Add(time.Hour)}
	}
	if l.Allow("kunci-sama-sekali-baru") {
		t.Fatal("map penuh: kunci baru harus ditolak")
	}
	if !l.Allow("penuh") {
		t.Fatal("kunci lama yang hidup harus tetap dihitung normal")
	}
}

// TestConcurrentAllow is a smoke test for the mutex: parallel Allow calls
// must neither panic nor admit more than limit per key.
func TestConcurrentAllow(t *testing.T) {
	l := New(50, time.Minute)
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n := 0
			for j := 0; j < 25; j++ {
				if l.Allow("bersama") {
					n++
				}
			}
			mu.Lock()
			allowed += n
			mu.Unlock()
		}()
	}
	wg.Wait()
	if allowed != 50 {
		t.Fatalf("allowed = %d, want exactly 50", allowed)
	}
}
