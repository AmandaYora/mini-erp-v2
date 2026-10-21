package integration

import (
	"testing"

	branchapp "mini-erp/internal/modules/branch/application"
	userapp "mini-erp/internal/modules/user/application"
	usercontracts "mini-erp/internal/modules/user/contracts"
)

// TestRevokedBranchDropsActiveBranch (F-03/K-01): a branch whose access is
// revoked while a session is live stops being the session's branch on the very
// next request, and refresh moves to the default branch the user still holds.
func TestRevokedBranchDropsActiveBranch(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	second, err := testMods.branch.Service().CreateBranch(ctx(), 0, branchapp.CreateBranchInput{Code: "CB2", Name: "Cabang Dua"})
	if err != nil {
		t.Fatalf("create branch: %v", err)
	}
	role, err := testMods.user.Service().CreateRole(ctx(), 0, userapp.CreateRoleInput{Code: "R-multi", Name: "Peran multi"})
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	u, err := testMods.user.Service().CreateUser(ctx(), 0, userapp.CreateUserInput{
		Username: "multi", Password: "rahasia-kuat-1", RoleIDs: []int64{role.ID},
		Branches: []usercontracts.BranchAccess{{BranchID: fx.branchID, IsDefault: true}, {BranchID: second.ID}},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	update := func(branches []usercontracts.BranchAccess) {
		t.Helper()
		if _, err := testMods.user.Service().UpdateUser(ctx(), 0, u.ID, userapp.UpdateUserInput{
			Username: "multi", FullName: "Multi", RoleIDs: []int64{role.ID}, Branches: branches,
		}); err != nil {
			t.Fatalf("update user: %v", err)
		}
	}

	res, lerr := testMods.auth.Service().Login(ctx(), "multi", "rahasia-kuat-1", "127.0.0.1")
	if lerr != nil {
		t.Fatalf("login: %v", lerr)
	}
	sess, err := testMods.auth.Service().Verify(ctx(), res.AccessToken)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	// Move the live session to the second branch, then revoke it.
	tok, aerr := testMods.auth.Service().SwitchBranch(ctx(), sess, second.ID)
	if aerr != nil {
		t.Fatalf("switch branch: %v", aerr)
	}
	if s, err := testMods.auth.Service().Verify(ctx(), tok); err != nil || s.BranchID != second.ID {
		t.Fatalf("before revoke: branch=%v err=%v, want %d", s, err, second.ID)
	}

	update([]usercontracts.BranchAccess{{BranchID: fx.branchID, IsDefault: true}})
	s, err := testMods.auth.Service().Verify(ctx(), tok)
	if err != nil {
		t.Fatalf("verify after revoke: %v", err)
	}
	if s.HasBranch() {
		t.Fatalf("session still holds branch %d after revoke", s.BranchID)
	}

	// Refresh lands on the remaining default branch.
	if _, rerr := testMods.auth.Service().Refresh(ctx(), res.RefreshToken); rerr != nil {
		t.Fatalf("refresh: %v", rerr)
	}
	s, err = testMods.auth.Service().Verify(ctx(), tok)
	if err != nil || s.BranchID != fx.branchID {
		t.Fatalf("after refresh: branch=%v err=%v, want %d", s, err, fx.branchID)
	}

	// Untouched access keeps the branch (no false positives).
	update([]usercontracts.BranchAccess{{BranchID: fx.branchID, IsDefault: true}})
	if s, err := testMods.auth.Service().Verify(ctx(), tok); err != nil || s.BranchID != fx.branchID {
		t.Fatalf("after no-op update: branch=%v err=%v", s, err)
	}
}
