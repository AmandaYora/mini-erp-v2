import { create } from "zustand";
import { setAuthExpiredHandler, tokenStore } from "@/shared/services/http-client";
import { authService } from "@/modules/auth/services/auth.service";
import type { AuthBranch, AuthCompany, AuthUser } from "@/modules/auth/types";

interface AuthState {
  ready: boolean;
  user: AuthUser | null;
  branch: AuthBranch | null;
  permissions: string[];
  company: AuthCompany | null;
  pendingBranches: AuthBranch[];
  login: (username: string, password: string) => Promise<{ requiresBranchSelection: boolean }>;
  chooseBranch: (branchId: number) => Promise<void>;
  loadMe: () => Promise<void>;
  switchBranch: (branchId: number) => Promise<void>;
  logout: () => Promise<void>;
  can: (perm: string) => boolean;
}

let expiredWired = false;

export const useAuthStore = create<AuthState>((set, get) => ({
  ready: false,
  user: null,
  branch: null,
  permissions: [],
  company: null,
  pendingBranches: [],

  login: async (username, password) => {
    const res = await authService.login(username, password);
    tokenStore.set(res.accessToken, res.refreshToken);
    if (res.requiresBranchSelection || !res.branch) {
      // Tanpa cabang: tidak ada rute ber-permission yang bisa dituju, jadi
      // state parsial aman — LoginPage mengarah ke /select-branch.
      set({ user: res.user, branch: null, permissions: [], company: null, pendingBranches: res.branches ?? [], ready: true });
      return { requiresBranchSelection: true };
    }
    // JANGAN set user/branch dulu dengan permission kosong: Navigate
    // deklaratif di LoginPage bereaksi ke state parsial dan memantul ke /403
    // sebelum loadMe selesai. loadMe mengisi semuanya atomik.
    await get().loadMe();
    return { requiresBranchSelection: false };
  },

  chooseBranch: async (branchId) => {
    const { accessToken } = await authService.switchBranch(branchId);
    if (tokenStore.refresh) tokenStore.set(accessToken, tokenStore.refresh);
    await get().loadMe();
  },

  loadMe: async () => {
    try {
      const me = await authService.me();
      set({ user: me.user, branch: me.branch, permissions: me.permissions ?? [], company: me.company, pendingBranches: [], ready: true });
    } catch {
      tokenStore.clear();
      set({ user: null, branch: null, permissions: [], company: null, pendingBranches: [], ready: true });
    }
  },

  switchBranch: async (branchId) => {
    const { accessToken } = await authService.switchBranch(branchId);
    if (tokenStore.refresh) tokenStore.set(accessToken, tokenStore.refresh);
    await get().loadMe();
  },

  logout: async () => {
    try {
      await authService.logout();
    } catch {
      // Token mati pun sesi lokal tetap dibersihkan.
    }
    tokenStore.clear();
    set({ user: null, branch: null, permissions: [], company: null, pendingBranches: [], ready: true });
  },

  can: (perm) => get().permissions.includes(perm),
}));

/** Sekali saat boot: refresh gagal total = tendang ke login. */
export function wireAuthExpiry(onExpired: () => void) {
  if (expiredWired) return;
  expiredWired = true;
  setAuthExpiredHandler(() => {
    useAuthStore.getState().logout().finally(onExpired);
  });
}
