package doclock

import (
	"sync"
	"testing"
)

// The lock must actually serialize: two goroutines on one key never overlap.
func TestLockSerializesSameKey(t *testing.T) {
	var mu sync.Mutex
	inside, maxInside := 0, 0
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = Lock("po:1", func() error {
				mu.Lock()
				inside++
				if inside > maxInside {
					maxInside = inside
				}
				mu.Unlock()
				mu.Lock()
				inside--
				mu.Unlock()
				return nil
			})
		}()
	}
	wg.Wait()
	if maxInside != 1 {
		t.Fatalf("max concurrent holders = %d, want 1", maxInside)
	}
}

// Stripes are reference counted: once every holder is gone the key must not
// linger, or the map grows with every document the process ever touches.
func TestStripesReleasedAfterUse(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = Lock("so:"+string(rune('a'+n%26)), func() error { return nil })
		}(i)
	}
	wg.Wait()
	mu.Lock()
	size := len(stripes)
	mu.Unlock()
	if size != 0 {
		t.Fatalf("stripes retained %d entries after all holders released", size)
	}
}

// TestBypassRunsConcurrently proves SetBypass lets two callers of the same
// key overlap (so DB-level guards can be tested in isolation).
func TestBypassRunsConcurrently(t *testing.T) {
	SetBypass(true)
	defer SetBypass(false)
	inside := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		_ = Lock("k", func() error {
			close(inside)
			<-release
			return nil
		})
		close(done)
	}()
	<-inside
	ran := false
	_ = Lock("k", func() error { ran = true; return nil })
	if !ran {
		t.Fatal("second Lock on the same key should not wait under bypass")
	}
	close(release)
	<-done
}
