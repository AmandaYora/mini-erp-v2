package integration

import (
	"reflect"
	"testing"

	"mini-erp/internal/shared/apperror"
)

// TestRoleDetailReadsRealPermissions (F-02/K-02): the matrix UI seeds its
// checkboxes from GetRoleDetail, so saving without touching them cannot wipe
// permissions; owner is flagged locked; the reserved superadmin role is a 404
// for actors who do not hold it.
func TestRoleDetailReadsRealPermissions(t *testing.T) {
	freshDB(t)
	newShop(t, ctx())
	ensureManagerCodes(t)
	svc := testMods.user.Service()

	id := makeRole(t, "gudang-x", "Gudang X")
	if err := svc.SetRolePermissions(ctx(), 0, id, []string{"users.update", "roles.manage"}); err != nil {
		t.Fatalf("set perms: %v", err)
	}
	d, err := svc.GetRoleDetail(ctx(), 0, id)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if want := []string{"roles.manage", "users.update"}; !reflect.DeepEqual(d.Permissions, want) || d.Locked {
		t.Fatalf("detail perms=%v locked=%v, want %v unlocked", d.Permissions, d.Locked, want)
	}

	// Round-trip: PUT exactly what GET returned changes nothing.
	if err := svc.SetRolePermissions(ctx(), 0, id, d.Permissions); err != nil {
		t.Fatalf("resave: %v", err)
	}
	again, _ := svc.GetRoleDetail(ctx(), 0, id)
	if !reflect.DeepEqual(again.Permissions, d.Permissions) {
		t.Fatalf("resave changed perms: %v → %v", d.Permissions, again.Permissions)
	}

	// Empty role reads as an empty (non-nil) list.
	empty, err := svc.GetRoleDetail(ctx(), 0, makeRole(t, "kosong", "Kosong"))
	if err != nil || empty.Permissions == nil || len(empty.Permissions) != 0 {
		t.Fatalf("empty role perms=%v err=%v", empty, err)
	}

	if _, err := svc.GetRoleDetail(ctx(), 0, 999999); mustAppErr(t, err).Code != apperror.CodeNotFound {
		t.Fatalf("unknown role: want 404, got %v", err)
	}

	if owner, err := svc.GetRoleByCode(ctx(), "owner"); err == nil && owner != nil {
		od, err := svc.GetRoleDetail(ctx(), 0, owner.ID)
		if err != nil || !od.Locked {
			t.Fatalf("owner detail=%v err=%v, want locked", od, err)
		}
	}
}
