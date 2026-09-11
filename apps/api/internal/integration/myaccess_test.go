package integration

import (
	"testing"

	branchapp "mini-erp/internal/modules/branch/application"
	userapp "mini-erp/internal/modules/user/application"
	usercontracts "mini-erp/internal/modules/user/contracts"
)

// mustTestRole returns one reusable role ID for user fixtures in this file.
// The integration DB is reused across runs, so an existing code is reused
// instead of failing on the unique key.
func mustTestRole(t *testing.T) int64 {
	t.Helper()
	if existing, err := testMods.user.Service().GetRoleByCode(ctx(), "uji-myaccess"); err == nil && existing != nil {
		return existing.ID
	}
	role, err := testMods.user.Service().CreateRole(ctx(), 0, userapp.CreateRoleInput{
		Code: "uji-myaccess", Name: "Uji MyAccess",
	})
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	return role.ID
}

// TestBranchMyAccessOnlyActiveWithDefault guards J6 (legacy F-05): the
// caller's own ACTIVE branches only, default marked, login suffices (no
// branches.view involved at the service layer). Inactive branches are
// dropped, unknown access rows never fatal, empty access reads as empty.
func TestBranchMyAccessOnlyActiveWithDefault(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	br2, err := m.branch.Service().CreateBranch(ctx(), 0, branchapp.CreateBranchInput{
		Code: "TST2", Name: "Cabang Kedua",
	})
	if err != nil {
		t.Fatalf("create branch 2: %v", err)
	}
	if _, err := m.branch.Service().UpdateBranch(ctx(), 0, br2.ID, branchapp.UpdateBranchInput{
		Code: br2.Code, Name: br2.Name, Status: "inactive",
	}); err != nil {
		t.Fatalf("close branch 2: %v", err)
	}

	u, err := m.user.Service().CreateUser(ctx(), 0, userapp.CreateUserInput{
		Username: "kasir-uji", FullName: "Kasir Uji", Password: "rahasia-123",
		RoleIDs:  []int64{mustTestRole(t)},
		Branches: []usercontracts.BranchAccess{
			{BranchID: fx.branchID, IsDefault: true},
			{BranchID: br2.ID},
		},
	})
	if err != nil {
		t.Fatalf("create user: %#v", err)
	}

	rows, err := m.branch.Service().MyAccess(ctx(), u.ID)
	if err != nil {
		t.Fatalf("my access: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("my access rows = %d, want 1 (inactive dropped)", len(rows))
	}
	if rows[0].Branch.ID != fx.branchID {
		t.Fatalf("my access branch = %d, want %d", rows[0].Branch.ID, fx.branchID)
	}
	if !rows[0].IsDefault {
		t.Fatal("my access default mark lost")
	}

	plain, err := m.user.Service().CreateUser(ctx(), 0, userapp.CreateUserInput{
		Username: "tanpa-akses", FullName: "Tanpa Akses", Password: "rahasia-123",
		RoleIDs:  []int64{mustTestRole(t)},
	})
	if err != nil {
		t.Fatalf("create plain user: %v", err)
	}
	empty, err := m.branch.Service().MyAccess(ctx(), plain.ID)
	if err != nil {
		t.Fatalf("my access empty: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("my access without links = %d rows, want 0", len(empty))
	}
}
