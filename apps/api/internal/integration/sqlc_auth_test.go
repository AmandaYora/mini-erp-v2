package integration

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	authinfra "mini-erp/internal/modules/auth/infrastructure"
)

// Characterization tests for the auth session Repository (F8-18).
func TestSqlcAuthSessionRepository(t *testing.T) {
	c := context.Background()
	freshDB(t)
	repo := authinfra.NewRepository(testMods.db)
	h := func(ch string) string { return strings.Repeat(ch, 64) }
	exp := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	abs := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)

	// Create: NULL branch/role/agent/ip round trip.
	id1, err := repo.CreateSession(c, &authinfra.SessionRecord{UserID: 1, RefreshHash: h("a"), ExpiresAt: exp, AbsoluteExpiresAt: abs})
	if err != nil || id1 == 0 {
		t.Fatalf("create: %d %v", id1, err)
	}
	id2, err := repo.CreateSession(c, &authinfra.SessionRecord{UserID: 1, RefreshHash: h("b"),
		ActiveBranchID: sql.NullInt64{Int64: 3, Valid: true}, ActiveRoleID: sql.NullInt64{Int64: 4, Valid: true},
		UserAgent: "UA", IPAddress: "10.0.0.1", ExpiresAt: exp, AbsoluteExpiresAt: abs})
	if err != nil {
		t.Fatal(err)
	}
	id3, _ := repo.CreateSession(c, &authinfra.SessionRecord{UserID: 2, RefreshHash: h("c"), ExpiresAt: exp, AbsoluteExpiresAt: abs})
	if _, err := repo.CreateSession(c, &authinfra.SessionRecord{UserID: 2, RefreshHash: h("c"), ExpiresAt: exp, AbsoluteExpiresAt: abs}); err == nil {
		t.Fatal("duplicate refresh hash must fail")
	}

	g, err := repo.GetSessionByID(c, id1)
	if err != nil || g == nil || g.UserID != 1 || g.RefreshHash != h("a") || g.PrevRefreshHash.Valid || g.RotatedAt.Valid ||
		g.ActiveBranchID.Valid || g.ActiveRoleID.Valid || g.RevokedAt.Valid ||
		!g.ExpiresAt.Equal(exp) || !g.AbsoluteExpiresAt.Equal(abs) {
		t.Fatalf("get id1: %+v %v", g, err)
	}
	g, _ = repo.GetSessionByID(c, id2)
	if g.ActiveBranchID.Int64 != 3 || g.ActiveRoleID.Int64 != 4 || !g.ActiveBranchID.Valid {
		t.Fatalf("get id2: %+v", g)
	}
	var ua, ip sql.NullString
	var last time.Time
	_ = testMods.db.QueryRow("SELECT user_agent, ip_address, last_activity_at FROM user_sessions WHERE id=?", id2).Scan(&ua, &ip, &last)
	if ua.String != "UA" || ip.String != "10.0.0.1" || !last.Equal(exp) {
		t.Fatalf("raw: %v %v %v", ua, ip, last)
	}
	_ = testMods.db.QueryRow("SELECT user_agent, ip_address FROM user_sessions WHERE id=?", id1).Scan(&ua, &ip)
	if ua.Valid || ip.Valid {
		t.Fatalf("empty ua/ip must be NULL: %v %v", ua, ip)
	}
	if g, err := repo.GetSessionByID(c, 999999); err != nil || g != nil {
		t.Fatalf("missing id: %v %v", g, err)
	}
	if g, err := repo.GetSessionByRefreshHash(c, h("b")); err != nil || g == nil || g.ID != id2 {
		t.Fatalf("by hash: %v %v", g, err)
	}
	if g, err := repo.GetSessionByRefreshHash(c, h("z")); err != nil || g != nil {
		t.Fatalf("by hash missing: %v %v", g, err)
	}

	// Rotate (CAS): winner, loser (stale hash), revoked, wrong id.
	newExp := exp.Add(time.Hour)
	n, err := repo.Rotate(c, id1, h("a"), h("d"), newExp)
	if err != nil || n != 1 {
		t.Fatalf("rotate: %d %v", n, err)
	}
	g, _ = repo.GetSessionByID(c, id1)
	if g.RefreshHash != h("d") || g.PrevRefreshHash.String != h("a") || !g.RotatedAt.Valid || !g.ExpiresAt.Equal(newExp) {
		t.Fatalf("after rotate: %+v", g)
	}
	if time.Since(g.RotatedAt.Time) > time.Minute || time.Since(g.RotatedAt.Time) < -time.Minute {
		t.Fatalf("rotated_at not now: %v", g.RotatedAt.Time)
	}
	if n, err := repo.Rotate(c, id1, h("a"), h("e"), newExp); err != nil || n != 0 {
		t.Fatalf("stale rotate must lose: %d %v", n, err)
	}
	if g, err := repo.GetSessionByPrevHash(c, h("a")); err != nil || g == nil || g.ID != id1 {
		t.Fatalf("by prev: %v %v", g, err)
	}
	if g, err := repo.GetSessionByPrevHash(c, h("q")); err != nil || g != nil {
		t.Fatalf("by prev missing: %v %v", g, err)
	}
	if n, _ := repo.Rotate(c, 999999, h("a"), h("x"), newExp); n != 0 {
		t.Fatalf("rotate unknown id: %d", n)
	}

	// UpdateBranchRole: set, and branch 0 -> NULL.
	if err := repo.UpdateBranchRole(c, id1, 9, 8); err != nil {
		t.Fatal(err)
	}
	g, _ = repo.GetSessionByID(c, id1)
	if g.ActiveBranchID.Int64 != 9 || g.ActiveRoleID.Int64 != 8 {
		t.Fatalf("branch/role: %+v", g)
	}
	if err := repo.UpdateBranchRole(c, id1, 0, 5); err != nil {
		t.Fatal(err)
	}
	g, _ = repo.GetSessionByID(c, id1)
	if g.ActiveBranchID.Valid || g.ActiveRoleID.Int64 != 5 {
		t.Fatalf("branch NULL: %+v", g)
	}

	// Rotate on a revoked session loses.
	if err := repo.RevokeSession(c, id3); err != nil {
		t.Fatal(err)
	}
	g, _ = repo.GetSessionByID(c, id3)
	if !g.RevokedAt.Valid {
		t.Fatal("revoke did not set revoked_at")
	}
	if n, _ := repo.Rotate(c, id3, h("c"), h("y"), newExp); n != 0 {
		t.Fatalf("rotate revoked: %d", n)
	}
	firstRevoke := g.RevokedAt.Time
	time.Sleep(20 * time.Millisecond)
	_ = repo.RevokeSession(c, id3) // overwrites revoked_at (previous behaviour)
	g, _ = repo.GetSessionByID(c, id3)
	if !g.RevokedAt.Time.After(firstRevoke) {
		t.Fatalf("RevokeSession re-stamps: %v vs %v", g.RevokedAt.Time, firstRevoke)
	}

	// RevokeUserSessions: only that user's still-live sessions.
	id4, _ := repo.CreateSession(c, &authinfra.SessionRecord{UserID: 1, RefreshHash: h("f"), ExpiresAt: exp, AbsoluteExpiresAt: abs})
	if err := repo.RevokeUserSessions(c, 1); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{id1, id2, id4} {
		if g, _ := repo.GetSessionByID(c, id); !g.RevokedAt.Valid {
			t.Fatalf("session %d should be revoked", id)
		}
	}
	// user 2's session was revoked individually earlier; make a live one.
	id5, _ := repo.CreateSession(c, &authinfra.SessionRecord{UserID: 2, RefreshHash: h("g"), ExpiresAt: exp, AbsoluteExpiresAt: abs})
	if g, _ := repo.GetSessionByID(c, id5); g.RevokedAt.Valid {
		t.Fatal("other user's session must stay live")
	}

	// RevokeUserSessionsExcept: keeps the excepted one, counts affected; 0 revokes all.
	id6, _ := repo.CreateSession(c, &authinfra.SessionRecord{UserID: 3, RefreshHash: h("h"), ExpiresAt: exp, AbsoluteExpiresAt: abs})
	id7, _ := repo.CreateSession(c, &authinfra.SessionRecord{UserID: 3, RefreshHash: h("i"), ExpiresAt: exp, AbsoluteExpiresAt: abs})
	id8, _ := repo.CreateSession(c, &authinfra.SessionRecord{UserID: 3, RefreshHash: h("j"), ExpiresAt: exp, AbsoluteExpiresAt: abs})
	n, err = repo.RevokeUserSessionsExcept(c, 3, id7)
	if err != nil || n != 2 {
		t.Fatalf("except: %d %v", n, err)
	}
	if g, _ := repo.GetSessionByID(c, id7); g.RevokedAt.Valid {
		t.Fatal("excepted session revoked")
	}
	for _, id := range []int64{id6, id8} {
		if g, _ := repo.GetSessionByID(c, id); !g.RevokedAt.Valid {
			t.Fatalf("session %d should be revoked", id)
		}
	}
	if n, _ := repo.RevokeUserSessionsExcept(c, 3, 0); n != 1 {
		t.Fatalf("except 0 revokes the remaining live one: %d", n)
	}
	if n, _ := repo.RevokeUserSessionsExcept(c, 3, 0); n != 0 {
		t.Fatalf("idempotent: %d", n)
	}
	if g, _ := repo.GetSessionByID(c, id5); g.RevokedAt.Valid {
		t.Fatal("user 2 untouched")
	}

	// PurgeSessions: revoked-before-cutoff OR absolute expiry before cutoff; bounded.
	old := time.Now().UTC().Add(-48 * time.Hour)
	if _, err := testMods.db.Exec("UPDATE user_sessions SET revoked_at = ? WHERE id IN (?, ?, ?)", old, id1, id2, id4); err != nil {
		t.Fatal(err)
	}
	if _, err := testMods.db.Exec("UPDATE user_sessions SET absolute_expires_at = ? WHERE id = ?", old, id5); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	gone, err := repo.PurgeSessions(c, cutoff, 2)
	if err != nil || gone != 2 {
		t.Fatalf("purge bounded: %d %v", gone, err)
	}
	gone, err = repo.PurgeSessions(c, cutoff, 100)
	if err != nil || gone != 2 { // 4 eligible in total: id1,id2,id4,id5
		t.Fatalf("purge rest: %d %v", gone, err)
	}
	if gone, _ := repo.PurgeSessions(c, cutoff, 100); gone != 0 {
		t.Fatalf("purge idempotent: %d", gone)
	}
	// id3 (revoked recently), id6,id8 (revoked recently) and live id7 survive.
	var left int
	_ = testMods.db.QueryRow("SELECT COUNT(*) FROM user_sessions").Scan(&left)
	if left != 4 {
		t.Fatalf("survivors = %d, want 4", left)
	}
}
