import { apiDeleteFull, apiGet, apiPostFull, apiPutFull } from "@/shared/services/http-client";
import type { PermissionItem, RoleItem } from "@/modules/users/types";

// Endpoint diverifikasi dari
// apps/api/internal/modules/user/presentation/routes.go dan handler.go.
// CATATAN: body SetRolePermissions memakai key `permissions` (bukan `codes`).
export const roleService = {
  list: () => apiGet<RoleItem[]>("/api/v1/roles"),
  create: (body: { code: string; name: string; description: string }) =>
    apiPostFull<RoleItem>("/api/v1/roles", body),
  update: (id: number, body: { name: string; description: string }) =>
    apiPutFull<RoleItem>(`/api/v1/roles/${id}`, body),
  remove: (id: number) => apiDeleteFull<unknown>(`/api/v1/roles/${id}`),
  permissionCatalog: () =>
    apiGet<PermissionItem[]>("/api/v1/permissions"),
  setPermissions: (id: number, permissions: string[]) =>
    apiPutFull<unknown>(`/api/v1/roles/${id}/permissions`, { permissions }),
};
