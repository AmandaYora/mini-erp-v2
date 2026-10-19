package contracts

import (
	"context"
	"errors"
	"testing"
)

type fakeInv struct{ all, user int }

func (f *fakeInv) InvalidateAll()       { f.all++ }
func (f *fakeInv) InvalidateUser(int64) { f.user++ }

type fakeRev struct{ got [2]int64 }

func (f *fakeRev) RevokeUserSessions(_ context.Context, u, e int64) (int64, error) {
	f.got = [2]int64{u, e}
	return 3, nil
}

func TestHubFanOut(t *testing.T) {
	h := NewAuthzHub()
	a, b := &fakeInv{}, &fakeInv{}
	h.Subscribe(a)
	h.Subscribe(b)
	h.Subscribe(nil) // ignored
	h.InvalidateAll()
	h.InvalidateUser(5)
	if a.all != 1 || b.all != 1 || a.user != 1 || b.user != 1 {
		t.Fatalf("fan-out: %+v %+v", a, b)
	}
}

func TestHubNoSubscribersIsNoop(t *testing.T) {
	h := NewAuthzHub()
	h.InvalidateAll()
	h.InvalidateUser(1)
}

func TestHubRevokerAndVerify(t *testing.T) {
	h := NewAuthzHub()
	if _, err := h.RevokeUserSessions(context.Background(), 1, 0); !errors.Is(err, ErrHubNotAttached) {
		t.Fatalf("want ErrHubNotAttached, got %v", err)
	}
	if h.Verify() == nil {
		t.Fatal("empty hub must fail Verify")
	}
	h.Subscribe(&fakeInv{})
	if !errors.Is(h.Verify(), ErrHubNotAttached) {
		t.Fatal("revoker still missing")
	}
	r := &fakeRev{}
	h.AttachRevoker(r)
	if err := h.Verify(); err != nil {
		t.Fatal(err)
	}
	n, err := h.RevokeUserSessions(context.Background(), 7, 9)
	if err != nil || n != 3 || r.got != [2]int64{7, 9} {
		t.Fatalf("delegate: n=%d err=%v got=%v", n, err, r.got)
	}
}
