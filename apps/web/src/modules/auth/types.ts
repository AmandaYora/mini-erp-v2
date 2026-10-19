export interface AuthUser {
  id: number;
  username: string;
  email?: string | null;
  fullName: string;
  roleIds: number[];
}

/** Role yang dimiliki pengguna (F3b-1). */
export interface AuthRole {
  id: number;
  code: string;
  name: string;
}

export interface AuthBranch {
  id: number;
  code: string;
  name: string;
  status: string;
  /** Penanda default dari GET /branches/my-access (J6, legacy F-05.3). */
  isDefault?: boolean;
}

export interface AuthCompany {
  name: string;
  city: string;
  phone: string;
}

export interface LoginResponse {
  accessToken: string;
  refreshToken: string;
  user: AuthUser;
  branch: AuthBranch | null;
  requiresBranchSelection: boolean;
  branches?: AuthBranch[];
  activeRole?: AuthRole | null;
  availableRoles?: AuthRole[];
}

export interface MeResponse {
  user: AuthUser;
  branch: AuthBranch | null;
  permissions: string[];
  company: AuthCompany | null;
  /** Izin sesi hanya berasal dari role aktif (F3b-1). */
  activeRole?: AuthRole | null;
  availableRoles?: AuthRole[];
}
