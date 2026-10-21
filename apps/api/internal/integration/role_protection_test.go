package integration

import (
	"testing"

	"mini-erp/internal/bootstrap"
	userapp "mini-erp/internal/modules/user/application"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/apperror"
)

func ensureManagerCodes(t *testing.T) {
	t.Helper()
	if err := testMods.user.Service().EnsurePermissions(ctx(), []usercontracts.PermissionSeed{
		{Code: "roles.manage", Name: "Role: Kelola"},
		{Code: "users.update", Name: "Pengguna: Ubah"},
	}); err != nil {
		t.Fatalf("ensure manager codes: %v", err)
	}
}

func makeRole(t *testing.T, code, name string) int64 {
	t.Helper()
	role, err := testMods.user.Service().CreateRole(ctx(), 0, userapp.CreateRoleInput{Code: code, Name: name})
	if err != nil {
		t.Fatalf("create role %s: %v", code, err)
	}
	return role.ID
}

func makeUserWithRole(t *testing.T, branchID int64, username string, roleID int64) int64 {
	t.Helper()
	u, err := testMods.user.Service().CreateUser(ctx(), 0, userapp.CreateUserInput{
		Username: username, Password: "rahasia-kuat-1", RoleIDs: []int64{roleID},
		Branches: []usercontracts.BranchAccess{{BranchID: branchID, IsDefault: true}},
	})
	if err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	return u.ID
}

func setUserRoles(t *testing.T, branchID, userID int64, roleIDs []int64) error {
	t.Helper()
	u, err := testMods.user.Service().GetByID(ctx(), userID)
	if err != nil || u == nil {
		t.Fatalf("get user: %v %v", u, err)
	}
	_, err = testMods.user.Service().UpdateUser(ctx(), 0, userID, userapp.UpdateUserInput{
		Username: u.Username, FullName: u.FullName, RoleIDs: roleIDs,
		Branches: []usercontracts.BranchAccess{{BranchID: branchID, IsDefault: true}},
	})
	return err
}

func makeManager(t *testing.T, branchID int64, username string) (roleID, userID int64) {
	t.Helper()
	ensureManagerCodes(t)
	roleID = makeRole(t, "MGR-"+username, "Pengelola "+username)
	if err := testMods.user.Service().SetRolePermissions(ctx(), 0, roleID, []string{"roles.manage", "users.update"}); err != nil {
		t.Fatalf("grant manager: %v", err)
	}
	return roleID, makeUserWithRole(t, branchID, username, roleID)
}

// TestLastManagerGuards (F1-7): operasi yang menghabiskan pengelola terakhir
// ditolak 409; dengan dua pengelola, operasi sama boleh.
func TestLastManagerGuards(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	mgrRole, boss := makeManager(t, fx.branchID, "bos")

	if err := testMods.user.Service().SetRolePermissions(ctx(), 0, mgrRole, []string{"users.update"}); mustAppErr(t, err).Code != apperror.CodeConflict {
		t.Fatalf("cabut roles.manage satu-satunya: want 409, got %v", err)
	}
	if err := testMods.user.Service().SetStatus(ctx(), 0, boss, "inactive"); mustAppErr(t, err).Code != apperror.CodeConflict {
		t.Fatalf("nonaktif pengelola terakhir: want 409, got %v", err)
	}

	_, boss2 := makeManager(t, fx.branchID, "bos2")
	if err := testMods.user.Service().SetStatus(ctx(), 0, boss, "inactive"); err != nil {
		t.Fatalf("nonaktif dengan dua pengelola: %v", err)
	}
	plain := makeRole(t, "POLOS", "Tanpa Kuasa")
	if err := setUserRoles(t, fx.branchID, boss2, []int64{plain}); mustAppErr(t, err).Code != apperror.CodeConflict {
		t.Fatalf("ganti role pengelola terakhir: want 409, got %v", err)
	}
}

// TestSystemRoleLocks (F1-7): role sistem tak bisa dihapus/rename; izin
// owner/superadmin terkunci; kode superadmin tak bisa dibuat via API.
func TestSystemRoleLocks(t *testing.T) {
	freshDB(t)
	if err := bootstrap.EnsureAuthz(ctx(), testMods.user.Service()); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	owner, err := testMods.user.Service().GetRoleByCode(ctx(), "owner")
	if err != nil || owner == nil {
		t.Fatalf("owner: %v %v", owner, err)
	}
	if err := testMods.user.Service().SetRolePermissions(ctx(), 0, owner.ID, []string{"roles.view"}); mustAppErr(t, err).Code != apperror.CodeForbidden {
		t.Fatalf("PUT permissions owner: want 403, got %v", err)
	}
	if _, err := testMods.user.Service().UpdateRole(ctx(), 0, owner.ID, "X", ""); mustAppErr(t, err).Code != apperror.CodeForbidden {
		t.Fatalf("rename owner: want 403, got %v", err)
	}
	if err := testMods.user.Service().DeleteRole(ctx(), 0, owner.ID); mustAppErr(t, err).Code != apperror.CodeForbidden {
		t.Fatalf("DELETE owner: want 403, got %v", err)
	}
	if _, err := testMods.user.Service().CreateRole(ctx(), 0, userapp.CreateRoleInput{Code: "superadmin", Name: "Palsu"}); mustAppErr(t, err).Code != apperror.CodeValidation {
		t.Fatalf("create superadmin: want 400, got %v", err)
	}
	// Role biasa tetap bisa diubah.
	free := makeRole(t, "BEBAS", "Bebas")
	if _, err := testMods.user.Service().UpdateRole(ctx(), 0, free, "Bebas 2", ""); err != nil {
		t.Fatalf("rename free: %v", err)
	}
}

// grantSuperadmin makes userID a holder of the superadmin role in SQL (the
// API refuses to grant it), the way the hidden developer account exists.
func grantSuperadmin(t *testing.T, userID, superRoleID int64) {
	t.Helper()
	if _, err := testMods.db.Exec(`DELETE FROM user_roles WHERE user_id = ?`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := testMods.db.Exec(`INSERT INTO user_roles (user_id, role_id) VALUES (?, ?)`, userID, superRoleID); err != nil {
		t.Fatal(err)
	}
}

// TestSuperadminHidden (F1-7): akun pemegang role superadmin tak terlihat oleh
// sesi non-superadmin (list tanpa, get/status/password 404); penetapan via
// API ditolak; UpdateUser tidak bisa mencabut role superadmin pemegangnya.
// Pemegang dikenali dari role-nya, bukan dari id (user id 1 di sini biasa).
func TestSuperadminHidden(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	if err := bootstrap.EnsureAuthz(ctx(), testMods.user.Service()); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	superRole, err := testMods.user.Service().GetRoleByCode(ctx(), "superadmin")
	if err != nil || superRole == nil {
		t.Fatalf("superadmin role: %v %v", superRole, err)
	}

	if first := makeUser(t, fx.branchID, "pertama", "rahasia-kuat-1"); first != 1 {
		t.Fatalf("asumsi uji: user pertama id=1, got %d", first)
	}
	rootID := makeUser(t, fx.branchID, "root", "rahasia-kuat-1")
	grantSuperadmin(t, rootID, superRole.ID)
	// Mengganti role pemegang lewat UpdateUser tidak mencabut superadmin.
	plain := makeRole(t, "POLOS", "Polos")
	if err := setUserRoles(t, fx.branchID, rootID, []int64{plain}); err != nil {
		t.Fatalf("update root: %v", err)
	}
	roleIDs, err := testMods.user.Service().GetRoleIDs(ctx(), rootID)
	if err != nil || len(roleIDs) != 1 || roleIDs[0] != superRole.ID {
		t.Fatalf("pemegang superadmin harus tetap superadmin: %v %v", roleIDs, err)
	}

	adminID := makeUser(t, fx.branchID, "admin", "rahasia-kuat-1")

	list, err := testMods.user.Service().ListUsers(ctx(), adminID, "", "", 1, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, u := range list.Users {
		if u.ID == rootID {
			t.Fatal("list non-superadmin memuat pemegang superadmin")
		}
	}
	if _, err := testMods.user.Service().GetUser(ctx(), adminID, rootID); mustAppErr(t, err).Code != apperror.CodeNotFound {
		t.Fatalf("get root: want 404, got %v", err)
	}
	if err := testMods.user.Service().SetStatus(ctx(), adminID, rootID, "inactive"); mustAppErr(t, err).Code != apperror.CodeNotFound {
		t.Fatalf("status root: want 404, got %v", err)
	}
	if err := testMods.user.Service().ChangePassword(ctx(), adminID, rootID, "baru-kuat-22"); mustAppErr(t, err).Code != apperror.CodeNotFound {
		t.Fatalf("password root: want 404, got %v", err)
	}
	roles, err := testMods.user.Service().ListRoles(ctx(), adminID)
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	for _, r := range roles {
		if r.Code == "superadmin" {
			t.Fatal("list roles non-superadmin memuat superadmin")
		}
	}
	// Penetapan superadmin via API ditolak.
	if _, err := testMods.user.Service().CreateUser(ctx(), adminID, userapp.CreateUserInput{
		Username: "jahat", Password: "rahasia-kuat-1", RoleIDs: []int64{superRole.ID},
		Branches: []usercontracts.BranchAccess{{BranchID: fx.branchID, IsDefault: true}},
	}); mustAppErr(t, err).Code != apperror.CodeValidation {
		t.Fatalf("assign superadmin: want 400, got %v", err)
	}
	// Sistem (actor 0) tetap melihat semuanya.
	all, err := testMods.user.Service().ListUsers(ctx(), 0, "", "", 1, 50)
	if err != nil {
		t.Fatalf("list system: %v", err)
	}
	found := false
	for _, u := range all.Users {
		if u.ID == rootID {
			found = true
		}
	}
	if !found {
		t.Fatal("actor sistem harus tetap melihat pemegang superadmin")
	}
}

// TestOwnerWithIDOneIsOrdinary: setelah migrasi data, users.id = 1 adalah
// pemilik usaha sungguhan (superadmin legacy tidak dimigrasi). Mengubahnya
// tidak boleh menjadikannya superadmin tersembunyi: role-nya tetap, ia tetap
// terlihat dan bisa dikelola owner lain.
func TestOwnerWithIDOneIsOrdinary(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	if err := bootstrap.EnsureAuthz(ctx(), testMods.user.Service()); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	svc := testMods.user.Service()
	owner, err := svc.GetRoleByCode(ctx(), "owner")
	if err != nil || owner == nil {
		t.Fatalf("owner: %v %v", owner, err)
	}
	admin, err := svc.GetRoleByCode(ctx(), "admin")
	if err != nil || admin == nil {
		t.Fatalf("admin: %v %v", admin, err)
	}
	ibu := makeUserWithRole(t, fx.branchID, "ibu-pemilik", owner.ID)
	if ibu != 1 {
		t.Fatalf("asumsi uji: pemilik hasil migrasi id=1, got %d", ibu)
	}
	rekan := makeUserWithRole(t, fx.branchID, "rekan-pemilik", owner.ID)
	branches := []usercontracts.BranchAccess{{BranchID: fx.branchID, IsDefault: true}}

	// Owner lain mengubah nomor HP user 1 dengan role yang sama.
	if _, err := svc.UpdateUser(ctx(), rekan, ibu, userapp.UpdateUserInput{
		Username: "ibu-pemilik", Phone: "081234567890", RoleIDs: []int64{owner.ID}, Branches: branches,
	}); err != nil {
		t.Fatalf("ubah HP user 1: %v", err)
	}
	if roleIDs, err := svc.GetRoleIDs(ctx(), ibu); err != nil || len(roleIDs) != 1 || roleIDs[0] != owner.ID {
		t.Fatalf("user 1 harus tetap owner: %v %v", roleIDs, err)
	}
	if got, err := svc.GetUser(ctx(), rekan, ibu); err != nil || got.Phone != "081234567890" {
		t.Fatalf("user 1 harus terlihat dengan HP baru: %+v %v", got, err)
	}
	list, err := svc.ListUsers(ctx(), rekan, "", "", 1, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, u := range list.Users {
		found = found || u.ID == ibu
	}
	if !found {
		t.Fatal("user 1 harus ada di daftar user owner lain")
	}
	// Tetap bisa dikelola: ganti sandi, nonaktif/aktifkan, ganti role.
	if err := svc.ChangePassword(ctx(), rekan, ibu, "sandi-baru-kuat-2"); err != nil {
		t.Fatalf("ganti sandi user 1: %v", err)
	}
	if err := svc.SetStatus(ctx(), rekan, ibu, "inactive"); err != nil {
		t.Fatalf("nonaktifkan user 1: %v", err)
	}
	if err := svc.SetStatus(ctx(), rekan, ibu, "active"); err != nil {
		t.Fatalf("aktifkan user 1: %v", err)
	}
	if _, err := svc.UpdateUser(ctx(), rekan, ibu, userapp.UpdateUserInput{
		Username: "ibu-pemilik", RoleIDs: []int64{admin.ID}, Branches: branches,
	}); err != nil {
		t.Fatalf("ganti role user 1: %v", err)
	}
	if roleIDs, err := svc.GetRoleIDs(ctx(), ibu); err != nil || len(roleIDs) != 1 || roleIDs[0] != admin.ID {
		t.Fatalf("user 1 harus menjadi admin: %v %v", roleIDs, err)
	}
	if super, err := testMods.user.Service().GetRoleByCode(ctx(), "superadmin"); err != nil || super == nil {
		t.Fatalf("superadmin: %v %v", super, err)
	} else if _, err := svc.UpdateUser(ctx(), rekan, ibu, userapp.UpdateUserInput{
		Username: "ibu-pemilik", RoleIDs: []int64{super.ID}, Branches: branches,
	}); mustAppErr(t, err).Code != apperror.CodeValidation {
		t.Fatalf("menetapkan superadmin ke user 1: want 400, got %v", err)
	}
}
