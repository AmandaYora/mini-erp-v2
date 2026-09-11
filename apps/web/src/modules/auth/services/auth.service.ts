import { apiGet, apiPost } from "@/shared/services/http-client";
import type { LoginResponse, MeResponse } from "@/modules/auth/types";

export const authService = {
  login: (username: string, password: string) =>
    apiPost<LoginResponse>("/api/v1/auth/login", { username, password }),
  logout: () => apiPost<null>("/api/v1/auth/logout"),
  me: () => apiGet<MeResponse>("/api/v1/auth/me"),
  switchBranch: (branchId: number) =>
    apiPost<{ accessToken: string }>("/api/v1/auth/switch-branch", { branchId }),
};
