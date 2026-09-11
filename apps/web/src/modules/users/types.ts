// Tipe respons backend modul user/role — diverifikasi dari
// apps/api/internal/modules/user/presentation/handler.go (userView,
// ListRoles, ListPermissions). Semua key camelCase.

export interface UserItem {
  id: number;
  username: string;
  email: string | null;
  fullName: string;
  status: string; // active | inactive
}

/** Detail GET /users/{id} — termasuk penugasan (untuk prefill form edit). */
export interface UserDetail extends UserItem {
  roleIds: number[];
  branches: UserBranchInput[];
}

export interface RoleItem {
  id: number;
  code: string;
  name: string;
  description: string;
  isSystem: boolean;
}

export interface PermissionItem {
  code: string;
  name: string;
}

export interface BranchOption {
  id: number;
  code: string;
  name: string;
  status: string; // active | inactive
}

export interface UserBranchInput {
  branchId: number;
  isDefault: boolean;
}

export interface CreateUserInput {
  username: string;
  email?: string;
  fullName: string;
  password: string;
  roleIds: number[];
  branches: UserBranchInput[];
}

export interface UpdateUserInput {
  username: string;
  email?: string;
  fullName: string;
  roleIds: number[];
  branches: UserBranchInput[];
}
