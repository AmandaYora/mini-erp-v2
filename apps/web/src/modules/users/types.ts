// Tipe respons backend modul user/role — diverifikasi dari
// apps/api/internal/modules/user/presentation/handler.go (userView,
// ListRoles, ListPermissions). Semua key camelCase.

export interface UserItem {
  id: number;
  username: string;
  email: string | null;
  fullName: string;
  status: string; // active | inactive
  /** Telepon opsional (maks 30 karakter). */
  phone?: string;
  /** Jabatan opsional (maks 100 karakter). */
  displayTitle?: string;
}

/** Label peran/cabang pada baris daftar pengguna (GET /users). */
export interface UserRoleLabel {
  id: number;
  name: string;
}
export interface UserBranchLabel {
  id: number;
  name: string;
  isDefault: boolean;
}

/** Baris GET /users: UserItem + dekorasi daftar (peran, cabang, login terakhir). */
export interface UserListItem extends UserItem {
  roles: UserRoleLabel[];
  branches: UserBranchLabel[];
  /** "YYYY-MM-DD HH:mm:ss" waktu server; null = belum pernah login. */
  lastLoginAt: string | null;
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

export interface RoleDetail extends RoleItem {
  /** owner/superadmin: izin dikelola bootstrap, hanya-baca di UI. */
  locked: boolean;
  permissions: string[];
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
  phone?: string;
  displayTitle?: string;
  password: string;
  roleIds: number[];
  branches: UserBranchInput[];
}

export interface UpdateUserInput {
  username: string;
  email?: string;
  fullName: string;
  phone?: string;
  displayTitle?: string;
  roleIds: number[];
  branches: UserBranchInput[];
}
