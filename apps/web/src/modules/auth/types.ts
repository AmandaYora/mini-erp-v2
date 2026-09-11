export interface AuthUser {
  id: number;
  username: string;
  email?: string | null;
  fullName: string;
  roleIds: number[];
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
}

export interface MeResponse {
  user: AuthUser;
  branch: AuthBranch | null;
  permissions: string[];
  company: AuthCompany | null;
}
