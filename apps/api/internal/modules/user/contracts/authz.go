package contracts

// PermissionCatalog is the L1 permission set owned by the user module
// (users/roles/branches/company guards). Other modules describe their own
// codes in their contracts; internal/bootstrap aggregates them all. Codes
// never change meaning once issued.
func PermissionCatalog() []PermissionSeed {
	return []PermissionSeed{
		{Code: "users.view", Name: "Pengguna: Lihat"},
		{Code: "users.create", Name: "Pengguna: Tambah"},
		{Code: "users.update", Name: "Pengguna: Ubah"},
		{Code: "users.archive", Name: "Pengguna: Nonaktifkan"},
		{Code: "roles.view", Name: "Role: Lihat"},
		{Code: "roles.manage", Name: "Role: Kelola"},
		{Code: "branches.view", Name: "Cabang: Lihat"},
		{Code: "branches.manage", Name: "Cabang: Kelola"},
		{Code: "company.view", Name: "Perusahaan: Lihat"},
		{Code: "company.manage", Name: "Perusahaan: Kelola"},
	}
}

// SystemRoleDef describes one role the bootstrap ensures on every start
// (F1-6). System roles cannot be deleted or renamed through the API (F1-7);
// owner/superadmin additionally hold every catalog code, re-synced each run.
type SystemRoleDef struct {
	Code        string
	Name        string
	Description string
	// System marks is_system roles (owner, superadmin, admin, staff).
	System bool
	// SyncAll re-attaches every known code on each run (owner, superadmin).
	SyncAll bool
}
