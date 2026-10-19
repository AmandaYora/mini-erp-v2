package bootstrap

// Role seeding run by the server before it listens and by `seed init`
// (F1-6). Idempotent: safe to run on every start (~75 permission upserts +
// a handful of role writes, milliseconds).
//
//   - The aggregated catalog is upserted (names refresh, codes never change
//     meaning).
//   - System roles are created once (missing) or flipped to is_system (old
//     databases, e.g. owner from the pre-bootstrap seed).
//   - owner/superadmin re-attach every known code each run, so new modules'
//     codes land on them immediately. Their grants are not tracked in
//     authz_seed_applied: they are locked to "all" by definition (F1-7
//     additionally forbids editing them through the API).
//   - admin/staff/kasir defaults are granted ONCE per code and recorded in
//     authz_seed_applied: codes an admin later removes are never re-added,
//     while new catalog codes entering the defaults are granted once.

import (
	"context"
	"fmt"

	userapp "mini-erp/internal/modules/user/application"
)

// EnsureAuthz seeds the permission catalog and the system/default roles.
// Actor 0 = system (bootstrap), trailed in audit like any other mutation.
func EnsureAuthz(ctx context.Context, users *userapp.Service) error {
	if err := users.EnsurePermissions(ctx, AllPermissionSeeds()); err != nil {
		return fmt.Errorf("seed katalog izin: %w", err)
	}
	all := AllPermissionCodes()
	for _, def := range SystemRoles() {
		if def.System {
			r, err := users.EnsureSystemRole(ctx, def.Code, def.Name, def.Description)
			if err != nil {
				return fmt.Errorf("seed role %s: %w", def.Code, err)
			}
			if def.SyncAll {
				if err := users.SyncRolePermissions(ctx, 0, r.ID, all); err != nil {
					return fmt.Errorf("sinkron role %s: %w", def.Code, err)
				}
			}
			continue
		}
		// Non-system defaults (kasir): create once, never flip is_system.
		existing, err := users.GetRoleByCode(ctx, def.Code)
		if err != nil {
			return fmt.Errorf("baca role %s: %w", def.Code, err)
		}
		if existing == nil {
			if _, err := users.CreateRole(ctx, 0, userapp.CreateRoleInput{
				Code: def.Code, Name: def.Name, Description: def.Description,
			}); err != nil {
				return fmt.Errorf("seed role %s: %w", def.Code, err)
			}
		}
	}
	grants := DefaultGrants()
	for _, def := range SystemRoles() {
		if def.SyncAll {
			continue
		}
		want, ok := grants[def.Code]
		if !ok {
			continue
		}
		role, err := users.GetRoleByCode(ctx, def.Code)
		if err != nil {
			return fmt.Errorf("baca role %s: %w", def.Code, err)
		}
		if role == nil {
			return fmt.Errorf("role %s hilang setelah ensure", def.Code)
		}
		done, err := users.SeedGranted(ctx, def.Code)
		if err != nil {
			return fmt.Errorf("baca jejak seed %s: %w", def.Code, err)
		}
		var fresh []string
		for _, code := range want {
			if !done[code] {
				fresh = append(fresh, code)
			}
		}
		if len(fresh) == 0 {
			continue
		}
		if err := users.AddRolePermissions(ctx, 0, role.ID, fresh); err != nil {
			return fmt.Errorf("grant default %s: %w", def.Code, err)
		}
		for _, code := range fresh {
			if err := users.MarkSeedGranted(ctx, def.Code, code); err != nil {
				return fmt.Errorf("catat seed %s/%s: %w", def.Code, code, err)
			}
		}
	}
	return nil
}
