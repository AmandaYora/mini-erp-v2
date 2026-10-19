package integration

import (
	"testing"

	"mini-erp/internal/bootstrap"
)

// TestEnsureAuthzIdempotent (F1-6): dua kali jalan di DB kosong menghasilkan
// katalog + role yang sama; default admin yang dicabut tidak diberikan ulang;
// kode katalog baru ikut tersinkron ke owner.
func TestEnsureAuthzIdempotent(t *testing.T) {
	freshDB(t)
	users := testMods.user.Service()

	if err := bootstrap.EnsureAuthz(ctx(), users); err != nil {
		t.Fatalf("ensure 1: %v", err)
	}
	countRoles := func() int {
		roles, err := users.List(ctx())
		if err != nil {
			t.Fatalf("list roles: %v", err)
		}
		return len(roles)
	}
	countPerms := func() int {
		list, err := users.ListPermissions(ctx())
		if err != nil {
			t.Fatalf("list permissions: %v", err)
		}
		return len(list)
	}
	roles1, perms1 := countRoles(), countPerms()
	if roles1 != 5 {
		t.Fatalf("roles = %d, want 5 (superadmin owner admin staff kasir)", roles1)
	}
	if perms1 != len(bootstrap.AllPermissionSeeds()) {
		t.Fatalf("permissions = %d, want %d", perms1, len(bootstrap.AllPermissionSeeds()))
	}

	owner, err := users.GetRoleByCode(ctx(), "owner")
	if err != nil || owner == nil {
		t.Fatalf("owner: %v %v", owner, err)
	}
	if !owner.IsSystem {
		t.Fatal("owner harus is_system")
	}
	kasir, err := users.GetRoleByCode(ctx(), "kasir")
	if err != nil || kasir == nil {
		t.Fatalf("kasir: %v %v", kasir, err)
	}
	if kasir.IsSystem {
		t.Fatal("kasir warisan legacy bukan system role")
	}
	ownerCodes, err := users.GetPermissionCodes(ctx(), []int64{owner.ID})
	if err != nil {
		t.Fatalf("owner codes: %v", err)
	}
	if len(ownerCodes) != perms1 {
		t.Fatalf("owner memegang %d kode, want semua %d", len(ownerCodes), perms1)
	}

	// Admin mencabut satu default, lalu seed jalan lagi: tidak diberikan ulang.
	admin, err := users.GetRoleByCode(ctx(), "admin")
	if err != nil || admin == nil {
		t.Fatalf("admin: %v %v", admin, err)
	}
	adminCodes, err := users.GetPermissionCodes(ctx(), []int64{admin.ID})
	if err != nil || len(adminCodes) == 0 {
		t.Fatalf("admin codes: %v %d", err, len(adminCodes))
	}
	revoked := adminCodes[0]
	rest := adminCodes[1:]
	if err := users.SetRolePermissions(ctx(), 0, admin.ID, rest); err != nil {
		t.Fatalf("cabut default admin: %v", err)
	}
	if err := bootstrap.EnsureAuthz(ctx(), users); err != nil {
		t.Fatalf("ensure 2: %v", err)
	}
	if roles1 != countRoles() || perms1 != countPerms() {
		t.Fatalf("jalan kedua tidak idempoten: roles %d->%d perms %d->%d",
			roles1, countRoles(), perms1, countPerms())
	}
	after, err := users.GetPermissionCodes(ctx(), []int64{admin.ID})
	if err != nil {
		t.Fatalf("admin codes 2: %v", err)
	}
	for _, c := range after {
		if c == revoked {
			t.Fatalf("kode %q yang dicabut admin diberikan ulang oleh seed", revoked)
		}
	}
	if len(after) != len(rest) {
		t.Fatalf("admin codes = %d, want %d (hanya yang dicabut hilang)", len(after), len(rest))
	}
}
