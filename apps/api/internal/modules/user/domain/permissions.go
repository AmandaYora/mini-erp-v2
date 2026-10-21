package domain

import (
	usercontracts "mini-erp/internal/modules/user/contracts"
)

// Permission is one row of the permission catalog. Codes are namespaced
// `resource.action` and double as the fail-closed route guard value.
type Permission struct {
	Code string
	Name string
}

// Catalog is the single source of permission codes (L1 set; extended as
// modules are implemented — each new module appends its codes here via its
// own seed, never by editing handlers). The data lives in
// user/contracts (F1-6: the seed aggregates it without importing internals);
// this stays as the domain-level view.
func Catalog() []Permission {
	seeds := usercontracts.PermissionCatalog()
	out := make([]Permission, 0, len(seeds))
	for _, s := range seeds {
		out = append(out, Permission{Code: s.Code, Name: s.Name})
	}
	return out
}

// MinPasswordLength guards credential strength at create and change.
const MinPasswordLength = 8

// MaxPasswordBytes is bcrypt's hard input limit. It counts bytes, not
// characters, so a long multibyte password can exceed it. Anything longer is
// rejected up front instead of being silently truncated (legacy bcryptjs did
// that: two long passwords sharing a 72-byte prefix were interchangeable).
const MaxPasswordBytes = 72

// BcryptCost matches the legacy system (bcryptjs cost 12) so hashes migrated
// from it are already at the target cost and never trigger a rehash.
const BcryptCost = 12

// Reserved role identity (F1-7, legacy reserved-role-policy parity). The
// superadmin role can never be granted through the API, and its holders (the
// hidden developer account) are invisible to non-superadmin sessions. The
// account is recognised by the role it holds, never by its id: after the
// data migration users.id = 1 is an ordinary business account.
const ReservedSuperadminRole = "superadmin"

// LockedRoleCodes cannot have their permission set changed through the API
// (owner/superadmin always hold every code; the bootstrap re-syncs them).
// Admin/staff defaults stay editable (legacy parity).
func LockedRoleCodes() []string { return []string{"owner", ReservedSuperadminRole} }

// ManagerPermissionA/B define a "pengelola" (last-admin guard, F1-7): an
// active user holding BOTH codes through any role. Operations that would
// leave zero managers are rejected with 409.
const ManagerPermissionA = "roles.manage"
const ManagerPermissionB = "users.update"
