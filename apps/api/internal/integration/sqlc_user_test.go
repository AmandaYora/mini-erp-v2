package integration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"mini-erp/internal/modules/user/contracts"
	userinfra "mini-erp/internal/modules/user/infrastructure"
)

// Characterization of the user Repository (F8-18): users, roles, permissions,
// RBAC links, manager guard, seed bookkeeping.

func sqlcUserRepo() *userinfra.Repository { return userinfra.NewRepository(testMods.db) }

func sqlcStrPtr(s string) *string { return &s }

func sqlcMkUser(t *testing.T, repo *userinfra.Repository, username string, email *string, full string, roles []int64, br []contracts.BranchAccess) int64 {
	t.Helper()
	id, err := repo.CreateUser(ctx(), &contracts.User{Username: username, Email: email, FullName: full}, "hash-"+username, roles, br, 1)
	if err != nil {
		t.Fatalf("create %s: %v", username, err)
	}
	return id
}

func TestSqlcUserAccounts(t *testing.T) {
	freshDB(t)
	repo := sqlcUserRepo()
	// Role links are real foreign keys: create the roles first and use live
	// ids (hardcoded ids can never exist in a fresh DB).
	mkRole := func(code string) int64 {
		t.Helper()
		id, err := repo.CreateRole(ctx(), &contracts.Role{Code: code, Name: code})
		if err != nil {
			t.Fatalf("create role %s: %v", code, err)
		}
		return id
	}
	roleA, roleB, roleC := mkRole("ra"), mkRole("rb"), mkRole("rc")

	if u, err := repo.GetByID(ctx(), 1); u != nil || err != nil {
		t.Fatalf("missing = %v %v", u, err)
	}
	if u, err := repo.GetByUsername(ctx(), "x"); u != nil || err != nil {
		t.Fatalf("missing by username = %v %v", u, err)
	}
	if u, err := repo.GetByLogin(ctx(), "  "); u != nil || err != nil {
		t.Fatalf("blank login = %v %v", u, err)
	}
	if h, err := repo.GetPasswordHash(ctx(), 1); h != "" || err != nil {
		t.Fatalf("missing hash = %q %v", h, err)
	}

	// Hook runs inside the tx and sees the new id; an error rolls everything back.
	var hookID int64
	var hookInTx bool
	id1, err := repo.CreateUser(ctx(), &contracts.User{
		Username: "alice", Email: sqlcStrPtr("alice@x.id"), FullName: "Alice A", Phone: "0811", DisplayTitle: "Kasir",
	}, "hash-alice", []int64{roleC, roleA}, []contracts.BranchAccess{{BranchID: 9, IsDefault: true}, {BranchID: 2}}, 7,
		func(c context.Context, tx *sql.Tx, id int64) error {
			hookID = id
			var n int
			hookInTx = tx.QueryRowContext(c, `SELECT COUNT(*) FROM users WHERE id = ?`, id).Scan(&n) == nil && n == 1
			return nil
		}, nil)
	if err != nil || id1 == 0 || hookID != id1 || !hookInTx {
		t.Fatalf("create = %d %v hook=%d inTx=%v", id1, err, hookID, hookInTx)
	}
	wantErr := errors.New("hook boom")
	if _, err := repo.CreateUser(ctx(), &contracts.User{Username: "ghost", FullName: "G"}, "h", []int64{roleA}, nil, 1,
		func(context.Context, *sql.Tx, int64) error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("hook error = %v", err)
	}
	if u, _ := repo.GetByUsername(ctx(), "ghost"); u != nil {
		t.Fatal("hook failure must roll the user back")
	}
	if n := count(t, `SELECT COUNT(*) FROM user_roles`); n != 2 {
		t.Fatalf("stray user_roles = %d", n)
	}
	// Duplicate username fails.
	if _, err := repo.CreateUser(ctx(), &contracts.User{Username: "alice", FullName: "dup"}, "h", nil, nil, 1); err == nil {
		t.Fatal("duplicate username must fail")
	}

	u, err := repo.GetByID(ctx(), id1)
	if err != nil || u == nil || u.Username != "alice" || u.Email == nil || *u.Email != "alice@x.id" || u.FullName != "Alice A" ||
		u.Status != "active" || u.Phone != "0811" || u.DisplayTitle != "Kasir" {
		t.Fatalf("user = %+v %v", u, err)
	}
	var createdBy, updatedBy int64
	if err := testMods.db.QueryRow(`SELECT created_by, updated_by FROM users WHERE id = ?`, id1).Scan(&createdBy, &updatedBy); err != nil || createdBy != 7 || updatedBy != 7 {
		t.Fatalf("audit cols = %d %d %v", createdBy, updatedBy, err)
	}
	if h, _ := repo.GetPasswordHash(ctx(), id1); h != "hash-alice" {
		t.Fatalf("hash = %q", h)
	}
	roles, _ := repo.GetRoleIDs(ctx(), id1)
	if !reflect.DeepEqual(roles, []int64{roleA, roleC}) {
		t.Fatalf("roles = %v", roles)
	}
	br, _ := repo.GetBranchAccess(ctx(), id1)
	if !reflect.DeepEqual(br, []contracts.BranchAccess{{BranchID: 2}, {BranchID: 9, IsDefault: true}}) {
		t.Fatalf("branches = %+v", br)
	}
	if r, _ := repo.GetRoleIDs(ctx(), 999); len(r) != 0 {
		t.Fatalf("roles of nobody = %v", r)
	}

	// NULL columns: nil/empty email, empty phone/title.
	id2 := sqlcMkUser(t, repo, "bob", nil, "Bob", nil, nil)
	id3 := sqlcMkUser(t, repo, "carol", sqlcStrPtr(""), "  ", nil, nil)
	var nulls int
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM users WHERE id IN (?, ?) AND email IS NULL AND phone IS NULL AND display_title IS NULL`, id2, id3).Scan(&nulls); err != nil || nulls != 2 {
		t.Fatalf("NULL columns = %d %v", nulls, err)
	}
	if u, _ := repo.GetByID(ctx(), id2); u.Email != nil || u.Phone != "" || u.DisplayTitle != "" {
		t.Fatalf("bob = %+v", u)
	}

	// Lookups.
	if u, _ := repo.GetByUsername(ctx(), "bob"); u == nil || u.ID != id2 {
		t.Fatalf("by username = %+v", u)
	}
	if u, _ := repo.GetByLogin(ctx(), " alice@x.id "); u == nil || u.ID != id1 {
		t.Fatalf("by email = %+v", u)
	}
	if u, _ := repo.GetByLogin(ctx(), "bob"); u == nil || u.ID != id2 {
		t.Fatalf("by username login = %+v", u)
	}
	// A's username = B's email: the username match wins.
	idX := sqlcMkUser(t, repo, "dave@x.id", nil, "Dave", nil, nil)
	idY := sqlcMkUser(t, repo, "erin", sqlcStrPtr("dave@x.id"), "Erin", nil, nil)
	if u, _ := repo.GetByLogin(ctx(), "dave@x.id"); u == nil || u.ID != idX {
		t.Fatalf("username must win: %+v (x=%d y=%d)", u, idX, idY)
	}
	if u, _ := repo.GetByLogin(ctx(), "nobody"); u != nil {
		t.Fatalf("unknown login = %+v", u)
	}

	// Exists*.
	for _, tc := range []struct {
		name  string
		got   func() (bool, error)
		wantV bool
	}{
		{"username taken", func() (bool, error) { return repo.ExistsByUsername(ctx(), "alice", 0) }, true},
		{"username own id", func() (bool, error) { return repo.ExistsByUsername(ctx(), "alice", id1) }, false},
		{"username free", func() (bool, error) { return repo.ExistsByUsername(ctx(), "zed", 0) }, false},
		{"email taken", func() (bool, error) { return repo.ExistsByEmail(ctx(), "alice@x.id", 0) }, true},
		{"email own id", func() (bool, error) { return repo.ExistsByEmail(ctx(), "alice@x.id", id1) }, false},
		{"email free", func() (bool, error) { return repo.ExistsByEmail(ctx(), "zed@x.id", 0) }, false},
	} {
		if got, err := tc.got(); err != nil || got != tc.wantV {
			t.Fatalf("%s = %v %v", tc.name, got, err)
		}
	}

	// NamesByIDs: full_name else username; dedupe, non-positive ids, unknown.
	names, err := repo.NamesByIDs(ctx(), []int64{id1, id1, id3, -4, 0, 9999})
	if err != nil || len(names) != 2 || names[id1] != "Alice A" || names[id3] != "carol" {
		t.Fatalf("names = %v %v", names, err)
	}
	if n, err := repo.NamesByIDs(ctx(), nil); err != nil || n == nil || len(n) != 0 {
		t.Fatalf("empty names = %v %v", n, err)
	}
	// More than one chunk (1000).
	for i := 0; i < 5; i++ {
		sqlcMkUser(t, repo, fmt.Sprintf("bulk%d", i), nil, fmt.Sprintf("Bulk %d", i), nil, nil)
	}
	big := make([]int64, 0, 2500)
	for i := int64(1); i <= 2400; i++ {
		big = append(big, 100000+i)
	}
	big = append(big, id1, id2)
	if n, err := repo.NamesByIDs(ctx(), big); err != nil || len(n) != 2 {
		t.Fatalf("big names = %d %v", len(n), err)
	}
	all := []int64{}
	for i := int64(1); i <= 12; i++ {
		all = append(all, i)
	}
	// Only 10 users exist (alice, bob, carol, dave, erin + 5 bulk); 11 and
	// 12 are absent.
	if n, _ := repo.NamesByIDs(ctx(), append(big, all...)); len(n) != 10 {
		t.Fatalf("big+all names = %d", len(n))
	}

	// TouchLogin.
	if err := repo.TouchLogin(ctx(), id1); err != nil {
		t.Fatal(err)
	}
	var drift float64
	if err := testMods.db.QueryRow(`SELECT ABS(TIMESTAMPDIFF(SECOND, last_login_at, UTC_TIMESTAMP(6))) FROM users WHERE id = ?`, id1).Scan(&drift); err != nil || drift > 5 {
		t.Fatalf("last_login drift = %v %v", drift, err)
	}

	// Passwords: set stamps updated_by; upgrade is compare-and-swap.
	if err := repo.SetPasswordHash(ctx(), id1, "hash-2", 11); err != nil {
		t.Fatal(err)
	}
	if h, _ := repo.GetPasswordHash(ctx(), id1); h != "hash-2" {
		t.Fatalf("hash = %q", h)
	}
	if err := testMods.db.QueryRow(`SELECT updated_by FROM users WHERE id = ?`, id1).Scan(&updatedBy); err != nil || updatedBy != 11 {
		t.Fatalf("updated_by = %d %v", updatedBy, err)
	}
	if err := repo.UpgradePasswordHash(ctx(), id1, "wrong", "hash-3"); err != nil {
		t.Fatal(err)
	}
	if h, _ := repo.GetPasswordHash(ctx(), id1); h != "hash-2" {
		t.Fatalf("CAS must not apply on mismatch: %q", h)
	}
	if err := repo.UpgradePasswordHash(ctx(), id1, "hash-2", "hash-3"); err != nil {
		t.Fatal(err)
	}
	if h, _ := repo.GetPasswordHash(ctx(), id1); h != "hash-3" {
		t.Fatalf("CAS = %q", h)
	}
	if err := testMods.db.QueryRow(`SELECT updated_by FROM users WHERE id = ?`, id1).Scan(&updatedBy); err != nil || updatedBy != 11 {
		t.Fatalf("upgrade must not touch updated_by: %d", updatedBy)
	}
	// Hooks of SetPasswordHash run in the tx and can veto.
	if err := repo.SetPasswordHash(ctx(), id1, "hash-4", 1, func(context.Context, *sql.Tx) error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("hook err = %v", err)
	}
	if h, _ := repo.GetPasswordHash(ctx(), id1); h != "hash-3" {
		t.Fatalf("vetoed password stuck: %q", h)
	}

	// UpdateUser without guard: profile + links replaced, hooks run.
	called := 0
	err = repo.UpdateUser(ctx(), &contracts.User{ID: id1, Username: "alice2", Email: nil, FullName: "Alice Two", Phone: "", DisplayTitle: "Boss"},
		[]int64{roleB}, []contracts.BranchAccess{{BranchID: 4, IsDefault: true}}, 12, false,
		func(context.Context, *sql.Tx) error { called++; return nil }, nil)
	if err != nil || called != 1 {
		t.Fatalf("update = %v called=%d", err, called)
	}
	u, _ = repo.GetByID(ctx(), id1)
	if u.Username != "alice2" || u.Email != nil || u.FullName != "Alice Two" || u.Phone != "" || u.DisplayTitle != "Boss" {
		t.Fatalf("after update = %+v", u)
	}
	if r, _ := repo.GetRoleIDs(ctx(), id1); !reflect.DeepEqual(r, []int64{roleB}) {
		t.Fatalf("roles = %v", r)
	}
	if b, _ := repo.GetBranchAccess(ctx(), id1); !reflect.DeepEqual(b, []contracts.BranchAccess{{BranchID: 4, IsDefault: true}}) {
		t.Fatalf("branches = %+v", b)
	}
	if err := testMods.db.QueryRow(`SELECT updated_by FROM users WHERE id = ?`, id1).Scan(&updatedBy); err != nil || updatedBy != 12 {
		t.Fatalf("updated_by = %d", updatedBy)
	}
	// A failing hook rolls the update back.
	if err := repo.UpdateUser(ctx(), &contracts.User{ID: id1, Username: "rolled", FullName: "R"}, nil, nil, 1, false,
		func(context.Context, *sql.Tx) error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v", err)
	}
	if u, _ := repo.GetByID(ctx(), id1); u.Username != "alice2" {
		t.Fatalf("rollback failed: %+v", u)
	}
	if r, _ := repo.GetRoleIDs(ctx(), id1); !reflect.DeepEqual(r, []int64{roleB}) {
		t.Fatalf("roles after rollback = %v", r)
	}

	// SetStatus.
	if err := repo.SetStatus(ctx(), id1, "inactive", 13, false); err != nil {
		t.Fatal(err)
	}
	if u, _ := repo.GetByID(ctx(), id1); u.Status != "inactive" {
		t.Fatalf("status = %q", u.Status)
	}
	if err := repo.SetStatus(ctx(), id1, "active", 13, true, func(context.Context, *sql.Tx) error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v", err)
	}
	if u, _ := repo.GetByID(ctx(), id1); u.Status != "inactive" {
		t.Fatalf("status must stay: %q", u.Status)
	}
}

func TestSqlcUserList(t *testing.T) {
	freshDB(t)
	repo := sqlcUserRepo()
	reserved, err := repo.CreateRole(ctx(), &contracts.Role{Code: "superadmin", Name: "Super", IsReserved: true})
	if err != nil {
		t.Fatal(err)
	}
	plain, _ := repo.CreateRole(ctx(), &contracts.Role{Code: "kasir", Name: "Kasir"})
	mk := func(username, email, full string, roles ...int64) int64 {
		var e *string
		if email != "" {
			e = &email
		}
		return sqlcMkUser(t, repo, username, e, full, roles, nil)
	}
	root := mk("root", "root@x.id", "Root Admin", reserved, plain)
	a := mk("andi", "andi@kantor.id", "Andi Wijaya", plain)
	b := mk("budi", "", "Budi Santoso", plain)
	c := mk("citra", "citra@kantor.id", "Citra Wijaya")
	d := mk("dodi", "dodi@x.id", "Dodi Wijaya Santoso")
	if err := repo.SetStatus(ctx(), d, "inactive", 1, false); err != nil {
		t.Fatal(err)
	}
	_ = root
	names := func(us []*contracts.User) []string {
		out := []string{}
		for _, u := range us {
			out = append(out, u.Username)
		}
		return out
	}
	list := func(search, status string, limit, offset int, hide bool) []string {
		t.Helper()
		us, err := repo.ListUsers(ctx(), search, status, limit, offset, hide)
		if err != nil {
			t.Fatal(err)
		}
		return names(us)
	}
	eq := func(name string, got, want []string) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: got %v want %v", name, got, want)
		}
	}
	// ORDER BY full_name: Andi, Budi, Citra, Dodi, Root.
	eq("all", list("", "", 100, 0, false), []string{"andi", "budi", "citra", "dodi", "root"})
	eq("hide reserved", list("", "", 100, 0, true), []string{"andi", "budi", "citra", "dodi"})
	eq("status", list("", "inactive", 100, 0, false), []string{"dodi"})
	eq("search username", list("citra", "", 100, 0, false), []string{"citra"})
	eq("search name", list("wijaya", "", 100, 0, false), []string{"andi", "citra", "dodi"})
	eq("search email", list("kantor", "", 100, 0, false), []string{"andi", "citra"})
	eq("words AND across columns", list("wijaya kantor", "", 100, 0, false), []string{"andi", "citra"})
	eq("words AND", list(" wijaya  santoso ", "", 100, 0, false), []string{"dodi"})
	eq("search+status+hide", list("wijaya", "active", 100, 0, true), []string{"andi", "citra"})
	eq("no match", list("zzz", "", 100, 0, false), []string{})
	eq("page 1", list("", "", 2, 0, false), []string{"andi", "budi"})
	eq("page 2", list("", "", 2, 2, false), []string{"citra", "dodi"})
	eq("page 3", list("", "", 2, 4, false), []string{"root"})
	eq("beyond", list("", "", 2, 40, false), []string{})
	for _, tc := range []struct {
		search, status string
		hide           bool
	}{
		{"", "", false}, {"", "", true}, {"", "inactive", false}, {"wijaya", "", false},
		{"wijaya kantor", "", false}, {"wijaya", "active", true}, {"zzz", "", false}, {"root", "", true},
	} {
		n, err := repo.CountUsers(ctx(), tc.search, tc.status, tc.hide)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(list(tc.search, tc.status, 1000, 0, tc.hide)); int64(got) != n {
			t.Fatalf("count %+v = %d, list = %d", tc, n, got)
		}
	}
	// Reserved + hide: holders of ANY reserved role vanish even with other roles.
	if has, _ := repo.UserHasReservedRole(ctx(), root); !has {
		t.Fatal("root must hold a reserved role")
	}
	if has, _ := repo.UserHasReservedRole(ctx(), a); has {
		t.Fatal("andi must not")
	}
	_, _ = b, c
}

func TestSqlcUserRolesPermissions(t *testing.T) {
	freshDB(t)
	repo := sqlcUserRepo()

	if r, err := repo.RoleByID(ctx(), 1); r != nil || err != nil {
		t.Fatalf("missing role = %v %v", r, err)
	}
	if r, err := repo.RoleByCode(ctx(), "x"); r != nil || err != nil {
		t.Fatalf("missing role code = %v %v", r, err)
	}
	if l, err := repo.ListRoles(ctx()); err != nil || len(l) != 0 {
		t.Fatalf("empty roles = %v %v", l, err)
	}
	var hookID int64
	rid, err := repo.CreateRole(ctx(), &contracts.Role{Code: "zeta", Name: "Zeta", Description: "desc"},
		func(_ context.Context, _ *sql.Tx, id int64) error { hookID = id; return nil })
	if err != nil || rid == 0 || hookID != rid {
		t.Fatalf("create role = %d %v hook=%d", rid, err, hookID)
	}
	res, _ := repo.CreateRole(ctx(), &contracts.Role{Code: "alpha", Name: "Alpha", IsReserved: true}) // reserved implies system
	sys, _ := repo.CreateRole(ctx(), &contracts.Role{Code: "mid", Name: "Mid", IsSystem: true})
	if _, err := repo.CreateRole(ctx(), &contracts.Role{Code: "zeta", Name: "dup"}); err == nil {
		t.Fatal("duplicate role code must fail")
	}
	if _, err := repo.CreateRole(ctx(), &contracts.Role{Code: "veto", Name: "V"},
		func(context.Context, *sql.Tx, int64) error { return errors.New("no") }); err == nil {
		t.Fatal("hook veto must fail")
	}
	if r, _ := repo.RoleByCode(ctx(), "veto"); r != nil {
		t.Fatal("vetoed role must roll back")
	}
	got, _ := repo.RoleByID(ctx(), rid)
	if got == nil || *got != (contracts.Role{ID: rid, Code: "zeta", Name: "Zeta", Description: "desc"}) {
		t.Fatalf("role = %+v", got)
	}
	r2, _ := repo.RoleByCode(ctx(), "alpha")
	if r2 == nil || r2.ID != res || !r2.IsReserved || !r2.IsSystem {
		t.Fatalf("reserved role = %+v", r2)
	}
	if r3, _ := repo.RoleByID(ctx(), sys); !r3.IsSystem || r3.IsReserved {
		t.Fatalf("system role = %+v", r3)
	}
	l, _ := repo.ListRoles(ctx())
	if len(l) != 3 || l[0].Code != "alpha" || l[1].Code != "mid" || l[2].Code != "zeta" {
		t.Fatalf("list = %+v", l)
	}
	if err := repo.UpdateRole(ctx(), &contracts.Role{ID: rid, Name: "Zeta 2", Description: ""}); err != nil {
		t.Fatal(err)
	}
	if g, _ := repo.RoleByID(ctx(), rid); g.Name != "Zeta 2" || g.Description != "" || g.Code != "zeta" {
		t.Fatalf("updated role = %+v", g)
	}
	vetoErr := errors.New("veto")
	if err := repo.UpdateRole(ctx(), &contracts.Role{ID: rid, Name: "Nope"}, func(context.Context, *sql.Tx) error { return vetoErr }); !errors.Is(err, vetoErr) {
		t.Fatalf("err = %v", err)
	}
	if g, _ := repo.RoleByID(ctx(), rid); g.Name != "Zeta 2" {
		t.Fatal("vetoed update stuck")
	}
	if err := repo.SetRoleSystem(ctx(), "zeta"); err != nil {
		t.Fatal(err)
	}
	if g, _ := repo.RoleByID(ctx(), rid); !g.IsSystem || g.IsReserved {
		t.Fatalf("SetRoleSystem = %+v", g)
	}
	if err := repo.SetRoleReserved(ctx(), "mid"); err != nil {
		t.Fatal(err)
	}
	if g, _ := repo.RoleByCode(ctx(), "mid"); !g.IsSystem || !g.IsReserved {
		t.Fatalf("SetRoleReserved = %+v", g)
	}
	if err := repo.SetRoleReserved(ctx(), "nope"); err != nil {
		t.Fatal(err)
	}

	// Permissions.
	if ids, ok, err := repo.PermissionIDByCode(ctx(), "a.b"); ids != 0 || ok || err != nil {
		t.Fatalf("missing perm = %d %v %v", ids, ok, err)
	}
	for _, p := range [][2]string{{"users.update", "Ubah"}, {"roles.manage", "Kelola"}, {"a.view", "A"}, {"users.update", "Ubah Baru"}} {
		if err := repo.EnsurePermission(ctx(), p[0], p[1]); err != nil {
			t.Fatal(err)
		}
	}
	codes, err := repo.PermissionCodes(ctx())
	if err != nil || !reflect.DeepEqual(codes, []string{"a.view", "roles.manage", "users.update"}) {
		t.Fatalf("codes = %v %v", codes, err)
	}
	pl, err := repo.ListPermissions(ctx())
	if err != nil || len(pl) != 3 || pl[2] != (contracts.PermissionSeed{Code: "users.update", Name: "Ubah Baru"}) {
		t.Fatalf("perms = %+v %v", pl, err)
	}
	pA, _, _ := repo.PermissionIDByCode(ctx(), "a.view")
	pU, okU, _ := repo.PermissionIDByCode(ctx(), "users.update")
	pR, _, _ := repo.PermissionIDByCode(ctx(), "roles.manage")
	if pA == 0 || !okU || pR == 0 {
		t.Fatal("permission ids")
	}

	// Empty catalog.
	if gc, err := repo.GetPermissionCodes(ctx(), nil); err != nil || gc != nil {
		t.Fatalf("no roles = %v %v", gc, err)
	}
	if err := repo.SetRolePermissions(ctx(), rid, []int64{pA, pU}, false); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetRolePermissions(ctx(), sys, []int64{pU, pR}, false); err != nil {
		t.Fatal(err)
	}
	gc, err := repo.GetPermissionCodes(ctx(), []int64{rid, sys, 999})
	if err != nil || !reflect.DeepEqual(gc, []string{"a.view", "roles.manage", "users.update"}) {
		t.Fatalf("distinct sorted codes = %v %v", gc, err)
	}
	if gc, _ := repo.GetPermissionCodes(ctx(), []int64{rid}); !reflect.DeepEqual(gc, []string{"a.view", "users.update"}) {
		t.Fatalf("codes of one = %v", gc)
	}
	if gc, _ := repo.GetPermissionCodes(ctx(), []int64{999}); len(gc) != 0 {
		t.Fatalf("unknown role = %v", gc)
	}
	bigRoles := make([]int64, 0, 3000)
	for i := int64(0); i < 3000; i++ {
		bigRoles = append(bigRoles, 50000+i)
	}
	if gc, err := repo.GetPermissionCodes(ctx(), append(bigRoles, rid)); err != nil || len(gc) != 2 {
		t.Fatalf("big role list = %v %v", gc, err)
	}
	// Replace and veto.
	if err := repo.SetRolePermissions(ctx(), rid, []int64{pR}, false, func(context.Context, *sql.Tx) error { return vetoErr }); !errors.Is(err, vetoErr) {
		t.Fatalf("err = %v", err)
	}
	if gc, _ := repo.GetPermissionCodes(ctx(), []int64{rid}); len(gc) != 2 {
		t.Fatalf("vetoed replace stuck: %v", gc)
	}
	if err := repo.SetRolePermissions(ctx(), rid, nil, false); err != nil {
		t.Fatal(err)
	}
	if gc, _ := repo.GetPermissionCodes(ctx(), []int64{rid}); len(gc) != 0 {
		t.Fatalf("cleared = %v", gc)
	}
	// AddRolePermissions is additive and idempotent.
	if err := repo.AddRolePermissions(ctx(), rid, []int64{pA, pU}); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddRolePermissions(ctx(), rid, []int64{pU, pR}); err != nil {
		t.Fatal(err)
	}
	if gc, _ := repo.GetPermissionCodes(ctx(), []int64{rid}); len(gc) != 3 {
		t.Fatalf("add = %v", gc)
	}
	if err := repo.AddRolePermissions(ctx(), rid, nil); err != nil {
		t.Fatal(err)
	}

	// Seed bookkeeping.
	if g, err := repo.SeedGranted(ctx(), "zeta"); err != nil || g == nil || len(g) != 0 {
		t.Fatalf("empty seed = %v %v", g, err)
	}
	for i := 0; i < 2; i++ {
		if err := repo.MarkSeedGranted(ctx(), "zeta", "a.view"); err != nil {
			t.Fatal(err)
		}
	}
	_ = repo.MarkSeedGranted(ctx(), "zeta", "users.update")
	_ = repo.MarkSeedGranted(ctx(), "other", "x")
	if g, _ := repo.SeedGranted(ctx(), "zeta"); !reflect.DeepEqual(g, map[string]bool{"a.view": true, "users.update": true}) {
		t.Fatalf("seed = %v", g)
	}

	// RoleInUse / UserHasRoleCode / DeleteRole.
	uid := sqlcMkUser(t, repo, "holder", nil, "Holder", []int64{rid}, nil)
	if in, err := repo.RoleInUse(ctx(), rid); err != nil || !in {
		t.Fatalf("in use = %v %v", in, err)
	}
	if in, _ := repo.RoleInUse(ctx(), sys); in {
		t.Fatal("sys not in use")
	}
	if has, err := repo.UserHasRoleCode(ctx(), uid, "zeta"); err != nil || !has {
		t.Fatalf("has role = %v %v", has, err)
	}
	if has, _ := repo.UserHasRoleCode(ctx(), uid, "mid"); has {
		t.Fatal("must not hold mid")
	}
	if has, _ := repo.UserHasRoleCode(ctx(), 999, "zeta"); has {
		t.Fatal("unknown user")
	}
	if has, _ := repo.UserHasReservedRole(ctx(), uid); has {
		t.Fatal("zeta is not reserved")
	}
	if err := repo.DeleteRole(ctx(), sys, func(context.Context, *sql.Tx) error { return vetoErr }); !errors.Is(err, vetoErr) {
		t.Fatalf("err = %v", err)
	}
	if r, _ := repo.RoleByID(ctx(), sys); r == nil {
		t.Fatal("vetoed delete stuck")
	}
	if gc, _ := repo.GetPermissionCodes(ctx(), []int64{sys}); len(gc) != 2 {
		t.Fatalf("vetoed delete removed permissions: %v", gc)
	}
	if err := repo.DeleteRole(ctx(), sys); err != nil {
		t.Fatal(err)
	}
	if r, _ := repo.RoleByID(ctx(), sys); r != nil {
		t.Fatal("role not deleted")
	}
	if gc, _ := repo.GetPermissionCodes(ctx(), []int64{sys}); len(gc) != 0 {
		t.Fatalf("role permissions must go: %v", gc)
	}
	if n := count(t, `SELECT COUNT(*) FROM role_permissions WHERE role_id = `+fmt.Sprint(sys)); n != 0 {
		t.Fatalf("role_permissions left = %d", n)
	}
	if err := repo.DeleteRole(ctx(), 9999); err != nil {
		t.Fatalf("deleting a missing role = %v", err)
	}
	_ = strings.TrimSpace
}

func TestSqlcUserManagerGuard(t *testing.T) {
	freshDB(t)
	repo := sqlcUserRepo()
	for _, p := range [][2]string{{"users.update", "U"}, {"roles.manage", "R"}} {
		if err := repo.EnsurePermission(ctx(), p[0], p[1]); err != nil {
			t.Fatal(err)
		}
	}
	pU, _, _ := repo.PermissionIDByCode(ctx(), "users.update")
	pR, _, _ := repo.PermissionIDByCode(ctx(), "roles.manage")
	both, _ := repo.CreateRole(ctx(), &contracts.Role{Code: "both", Name: "Both"})
	onlyU, _ := repo.CreateRole(ctx(), &contracts.Role{Code: "onlyu", Name: "OnlyU"})
	onlyR, _ := repo.CreateRole(ctx(), &contracts.Role{Code: "onlyr", Name: "OnlyR"})
	_ = repo.SetRolePermissions(ctx(), both, []int64{pU, pR}, false)
	_ = repo.SetRolePermissions(ctx(), onlyU, []int64{pU}, false)
	_ = repo.SetRolePermissions(ctx(), onlyR, []int64{pR}, false)

	if n, err := repo.CountManagers(ctx()); err != nil || n != 0 {
		t.Fatalf("no managers = %d %v", n, err)
	}
	m1 := sqlcMkUser(t, repo, "m1", nil, "M1", []int64{both}, nil)
	// m2 holds the two codes through two different roles: still a manager.
	m2 := sqlcMkUser(t, repo, "m2", nil, "M2", []int64{onlyU, onlyR}, nil)
	_ = sqlcMkUser(t, repo, "half", nil, "Half", []int64{onlyU}, nil)
	if n, _ := repo.CountManagers(ctx()); n != 2 {
		t.Fatalf("managers = %d, want 2", n)
	}
	if err := repo.SetStatus(ctx(), m2, "inactive", 1, false); err != nil {
		t.Fatal(err)
	}
	if n, _ := repo.CountManagers(ctx()); n != 1 {
		t.Fatalf("inactive manager counted: %d", n)
	}
	// Last manager: every guarded write that would remove it is refused.
	if err := repo.SetStatus(ctx(), m1, "inactive", 1, true); !errors.Is(err, userinfra.ErrNoManagerLeft) {
		t.Fatalf("SetStatus guard = %v", err)
	}
	if err := repo.UpdateUser(ctx(), &contracts.User{ID: m1, Username: "m1", FullName: "M1"}, []int64{onlyU}, nil, 1, true); !errors.Is(err, userinfra.ErrNoManagerLeft) {
		t.Fatalf("UpdateUser guard = %v", err)
	}
	if err := repo.SetRolePermissions(ctx(), both, []int64{pU}, true); !errors.Is(err, userinfra.ErrNoManagerLeft) {
		t.Fatalf("SetRolePermissions guard = %v", err)
	}
	if n, _ := repo.CountManagers(ctx()); n != 1 {
		t.Fatalf("guard must roll back, managers = %d", n)
	}
	if r, _ := repo.GetRoleIDs(ctx(), m1); !reflect.DeepEqual(r, []int64{both}) {
		t.Fatalf("guarded update stuck: %v", r)
	}
	// Harmless guarded writes pass.
	if err := repo.SetStatus(ctx(), m2, "active", 1, true); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetStatus(ctx(), m1, "inactive", 1, true); err != nil {
		t.Fatalf("with another manager the write is fine: %v", err)
	}
	// Zero managers already: guarded writes are allowed (repair).
	if err := repo.SetStatus(ctx(), m2, "inactive", 1, false); err != nil {
		t.Fatal(err)
	}
	if n, _ := repo.CountManagers(ctx()); n != 0 {
		t.Fatalf("managers = %d", n)
	}
	if err := repo.SetStatus(ctx(), m1, "active", 1, true); err != nil {
		t.Fatalf("repair write refused: %v", err)
	}
}
