package domain

// Permission is one row of the permission catalog. Codes are namespaced
// `resource.action` and double as the fail-closed route guard value.
type Permission struct {
	Code string
	Name string
}

// Catalog is the single source of permission codes (L1 set; extended as
// modules are implemented — each new module appends its codes here via its
// own seed, never by editing handlers).
func Catalog() []Permission {
	return []Permission{
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

// MinPasswordLength guards credential strength at create and change.
const MinPasswordLength = 8
