package integration

import (
	"fmt"
	"testing"

	branchapp "mini-erp/internal/modules/branch/application"
	userapp "mini-erp/internal/modules/user/application"
	usercontracts "mini-erp/internal/modules/user/contracts"
)

// TestUserListCarriesRolesBranchesLastLogin (U-01): the admin list shows each
// user's roles, branch names and last login without opening the user, and the
// statement count stays constant as the page grows (no N+1).
func TestUserListCarriesRolesBranchesLastLogin(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	f3bSeed(t)
	cm, ctr := countedMods(t)

	roleA := f3bRole(t, "peran-a", "users.view")
	roleB := f3bRole(t, "peran-b", "users.view")
	br2, err := testMods.branch.Service().CreateBranch(ctx(), 0, branchapp.CreateBranchInput{
		Code: "BDG", Name: "Cabang Bandung",
	})
	if err != nil {
		t.Fatalf("create branch: %v", err)
	}

	mk := func(name string, roles []int64, branches []usercontracts.BranchAccess) int64 {
		u, err := testMods.user.Service().CreateUser(ctx(), 0, userapp.CreateUserInput{
			Username: name, FullName: name, Password: "rahasia-kuat-1", RoleIDs: roles, Branches: branches,
		})
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		return u.ID
	}
	both := []usercontracts.BranchAccess{{BranchID: fx.branchID, IsDefault: true}, {BranchID: br2.ID}}
	first := mk("aaa-multi", []int64{roleA, roleB}, both)
	solo := mk("bbb-solo", []int64{roleA}, []usercontracts.BranchAccess{{BranchID: br2.ID, IsDefault: true}})
	bare := mk("ccc-bare", []int64{roleB}, []usercontracts.BranchAccess{{BranchID: fx.branchID, IsDefault: true}})
	if err := testMods.user.Service().TouchLogin(ctx(), first); err != nil {
		t.Fatal(err)
	}

	res, err := testMods.user.Service().ListUsers(ctx(), 0, "", "", 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[int64]*usercontracts.User{}
	for _, u := range res.Users {
		byID[u.ID] = u
	}
	if byID[first].LastLoginAt == nil {
		t.Error("lastLoginAt pengguna yang pernah login harus terisi")
	}
	if byID[solo].LastLoginAt != nil {
		t.Error("lastLoginAt pengguna yang belum pernah login harus nil")
	}
	if m := res.Meta[first]; len(m.Roles) != 2 || len(m.Branches) != 2 {
		t.Errorf("multi: roles=%v branches=%v, want 2 & 2", m.Roles, m.Branches)
	} else {
		names := map[string]bool{m.Branches[0].Name: true, m.Branches[1].Name: true}
		if !names["Cabang Bandung"] || m.Roles[0].Name == "" {
			t.Errorf("nama cabang/peran tidak ter-resolve: %+v", m)
		}
	}
	if m := res.Meta[solo]; len(m.Roles) != 1 || len(m.Branches) != 1 || m.Branches[0].Name != "Cabang Bandung" || !m.Branches[0].IsDefault {
		t.Errorf("solo = %+v", m)
	}
	if m := res.Meta[bare]; m == nil || len(m.Roles) != 1 || len(m.Branches) != 1 || m.Branches[0].ID != fx.branchID {
		t.Errorf("bare = %+v", m)
	}

	// Constant statements: 3 users vs 40 users on one page.
	measureList := func() int {
		return measure(t, ctr, "list users", func() error {
			_, err := cm.user.Service().ListUsers(ctx(), 0, "", "", 1, 100)
			return err
		})
	}
	small := measureList()
	for i := 0; i < 37; i++ {
		mk(fmt.Sprintf("zz-user-%02d", i), []int64{roleA, roleB}, both)
	}
	large := measureList()
	requireConstant(t, "ListUsers", small, large)
}
