import { apiGet } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import type { AuthBranch } from "@/modules/auth/types";

/**
 * Daftar cabang yang bisa diakses user aktif (untuk affordance "Ganti"
 * cabang di topbar/avatar dan mode ganti-cabang di SelectBranchPage).
 *
 * Memakai GET /branches/my-access (J6, login saja — tanpa branches.view)
 * sehingga kasir tanpa izin itu tetap bisa ganti cabang. Bentuk respons
 * backend ditangani defensif: array langsung atau { branches: [...] }.
 */
export function extractBranches(payload: unknown): AuthBranch[] {
  if (Array.isArray(payload)) return payload as AuthBranch[];
  if (payload && typeof payload === "object") {
    const rec = payload as Record<string, unknown>;
    const candidates = [rec.branches, rec.items, rec.data];
    for (const candidate of candidates) {
      if (Array.isArray(candidate)) return candidate as AuthBranch[];
    }
  }
  return [];
}

export function useAccessibleBranches(enabled: boolean) {
  const { data, loading } = useAsyncData(
    () =>
      enabled
        ? apiGet<unknown>("/api/v1/branches/my-access").then(
            (payload) => extractBranches(payload),
            () => [] as AuthBranch[],
          )
        : Promise.resolve([] as AuthBranch[]),
    [enabled],
  );

  return { branches: data ?? [], loading };
}
