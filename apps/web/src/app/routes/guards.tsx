import type { ReactNode } from "react";
import { Navigate, useLocation } from "react-router-dom";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";

/** Belum login → /login (ingat tujuan via ?next di halaman login). */
export function RequireAuth({ children }: { children: ReactNode }) {
  const ready = useAuthStore((s) => s.ready);
  const user = useAuthStore((s) => s.user);
  const location = useLocation();

  if (!ready) {
    return (
      <div className="flex min-h-screen items-center justify-center text-muted">
        Memuat…
      </div>
    );
  }
  if (!user) {
    return <Navigate to={`${ROUTE_PATHS.login}?next=${encodeURIComponent(location.pathname)}`} replace />;
  }
  return <>{children}</>;
}

/**
 * Wajib cabang aktif, kecuali rute yang mengizinkan tanpa cabang
 * (halaman pilih cabang).
 */
export function RequireBranch({
  children,
  allowBranchless = false,
}: {
  children: ReactNode;
  allowBranchless?: boolean;
}) {
  const user = useAuthStore((s) => s.user);
  const branch = useAuthStore((s) => s.branch);

  if (user && !branch && !allowBranchless) {
    return <Navigate to={ROUTE_PATHS.selectBranch} replace />;
  }
  return <>{children}</>;
}

/** Tanpa permission → /403. */
export function RequirePerm({ children, perm }: { children: ReactNode; perm: string }) {
  const permissions = useAuthStore((s) => s.permissions);

  if (!permissions.includes(perm)) {
    return <Navigate to={ROUTE_PATHS.forbidden} replace />;
  }
  return <>{children}</>;
}
