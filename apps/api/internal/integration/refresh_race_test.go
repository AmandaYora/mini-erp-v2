package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"testing"

	"mini-erp/internal/shared/apperror"
)

// TestRefreshRaceWithinGrace (F1-10, A-21): memakai token lama di dalam
// jendela grace = race antar-tab yang sah: 401 refresh_raced TANPA mencabut
// sesi; token baru tetap bisa dipakai.
func TestRefreshRaceWithinGrace(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	makeUser(t, fx.branchID, "target", "rahasia-kuat-1")

	first, err := testMods.auth.Service().Login(ctx(), "target", "rahasia-kuat-1", testIP())
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	second, err := testMods.auth.Service().Refresh(ctx(), first.RefreshToken)
	if err != nil {
		t.Fatalf("refresh pertama: %v", err)
	}
	// Tab yang kalah memakai token lama dalam grace.
	if _, rerr := testMods.auth.Service().Refresh(ctx(), first.RefreshToken); rerr == nil || rerr.Code != apperror.CodeRefreshRaced {
		t.Fatalf("refresh lama dalam grace: err=%v, want refresh_raced", rerr)
	}
	// Sesi tetap hidup: token baru bisa dipakai, dan access valid.
	third, err := testMods.auth.Service().Refresh(ctx(), second.RefreshToken)
	if err != nil {
		t.Fatalf("refresh dengan token baru: %v, sesi harus hidup", err)
	}
	mustVerify(t, third.AccessToken)
}

// TestRefreshReplayBeyondGrace (F1-10): token lama di luar jendela grace =
// replay sungguhan: semua sesi user dicabut.
func TestRefreshReplayBeyondGrace(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	makeUser(t, fx.branchID, "target", "rahasia-kuat-1")

	first, err := testMods.auth.Service().Login(ctx(), "target", "rahasia-kuat-1", testIP())
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	second, err := testMods.auth.Service().Refresh(ctx(), first.RefreshToken)
	if err != nil {
		t.Fatalf("refresh pertama: %v", err)
	}
	// Tuakan rotasi melampaui grace (30 dtk di kode): hash dihitung seperti
	// hashOf di auth (hex sha256), bukan SHA2 SQL.
	sum := sha256.Sum256([]byte(second.RefreshToken))
	if _, err := testMods.db.Exec(
		"UPDATE user_sessions SET rotated_at = DATE_SUB(UTC_TIMESTAMP(6), INTERVAL 60 SECOND) WHERE refresh_hash = ?",
		hex.EncodeToString(sum[:])); err != nil {
		t.Fatalf("tuakan rotated_at: %v", err)
	}
	_ = first
	if _, rerr := testMods.auth.Service().Refresh(ctx(), first.RefreshToken); rerr == nil {
		t.Fatal("replay di luar grace: want 401")
	}
	// Semua sesi mati, termasuk token baru.
	if _, rerr := testMods.auth.Service().Refresh(ctx(), second.RefreshToken); rerr == nil {
		t.Fatal("token baru setelah replay: want 401 (sesi dicabut)")
	}
	mustInvalid(t, second.AccessToken)
}

// TestConcurrentRefreshSingleWinner (F1-10): dua Refresh bersamaan dengan
// token sama menghasilkan tepat satu sukses; sesi tetap hidup.
func TestConcurrentRefreshSingleWinner(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	makeUser(t, fx.branchID, "target", "rahasia-kuat-1")

	first, err := testMods.auth.Service().Login(ctx(), "target", "rahasia-kuat-1", testIP())
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	var wg sync.WaitGroup
	results := make([]string, 2)
	var mu sync.Mutex
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res, rerr := testMods.auth.Service().Refresh(ctx(), first.RefreshToken)
			mu.Lock()
			defer mu.Unlock()
			if rerr == nil {
				results[i] = res.RefreshToken
			} else {
				results[i] = "gagal:" + string(rerr.Code)
			}
		}(i)
	}
	wg.Wait()
	wins := 0
	var winner string
	for _, r := range results {
		if len(r) > 0 && r[:6] != "gagal:" {
			wins++
			winner = r
		}
	}
	if wins != 1 {
		t.Fatalf("pemenang = %d, want tepat 1 (hasil: %v)", wins, results)
	}
	// Sesi hidup: token pemenang bisa dipakai lagi.
	if _, err := testMods.auth.Service().Refresh(ctx(), winner); err != nil {
		t.Fatalf("refresh lanjutan: %v, sesi harus hidup", err)
	}
}
