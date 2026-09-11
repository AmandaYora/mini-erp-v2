package integration

import (
	"testing"

	userapp "mini-erp/internal/modules/user/application"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/apperror"
)

// TestRoleDeleteGuards (A2): roles in use cannot be deleted (conflict),
// unused roles delete cleanly, and permission grants round-trip.
func TestRoleDeleteGuards(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	role, err := m.user.Service().CreateRole(ctx(), 0, userapp.CreateRoleInput{
		Code: "UJI", Name: "Peran Uji",
	})
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	if _, err := m.user.Service().CreateUser(ctx(), 0, userapp.CreateUserInput{
		Username: "uji_coba", Password: "rahasia-kuat-1", RoleIDs: []int64{role.ID},
		Branches: []usercontracts.BranchAccess{{BranchID: fx.branchID, IsDefault: true}},
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := m.user.Service().DeleteRole(ctx(), 0, role.ID); mustAppErr(t, err).Code != apperror.CodeConflict {
		t.Fatalf("delete in-use role: want conflict, got %v", err)
	}

	free, err := m.user.Service().CreateRole(ctx(), 0, userapp.CreateRoleInput{
		Code: "BEBAS", Name: "Peran Bebas",
	})
	if err != nil {
		t.Fatalf("create free role: %v", err)
	}
	if err := m.user.Service().EnsurePermissions(ctx(), []usercontracts.PermissionSeed{
		{Code: "stock.view", Name: "Stok: Lihat"},
	}); err != nil {
		t.Fatalf("ensure permissions: %v", err)
	}
	if err := m.user.Service().SetRolePermissions(ctx(), 0, free.ID, []string{"stock.view"}); err != nil {
		t.Fatalf("set permissions: %v", err)
	}
	codes, err := m.user.Service().GetPermissionCodes(ctx(), []int64{free.ID})
	if err != nil {
		t.Fatalf("permission codes: %v", err)
	}
	if len(codes) != 1 || codes[0] != "stock.view" {
		t.Fatalf("codes = %v, want [stock.view]", codes)
	}
	if err := m.user.Service().DeleteRole(ctx(), 0, free.ID); err != nil {
		t.Fatalf("delete unused role: %v", err)
	}
}
