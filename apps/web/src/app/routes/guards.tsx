import type { ReactNode } from "react";
import { Navigate, useLocation } from "react-router-dom";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";

/** Belum login → /login (ingat tujuan via ?next di halaman login). */
export function RequireAuth({ children }: { children: ReactNode }) {
  const ready = useAuthStore((s) => s.ready);
  const user = useAuthStore((s) => s.user);
  const bootError = useAuthStore((s) => s.bootError);
  const location = useLocation();

  if (!ready) {
    return (
      <div className="flex min-h-screen items-center justify-center text-muted-foreground">
        Memuat…
      </div>
    );
  }
  if (!user && bootError === "network") {
    return (
      <div className="flex min-h-screen items-center justify-center p-4">
        <div role="alert" className="w-full max-w-sm rounded-lg border border-border bg-card p-8 text-center">
          <h1 className="font-semibold text-foreground">Tidak dapat terhubung ke server</h1>
          <p className="text-muted-foreground mt-1 text-sm">
            Sesi Anda masih tersimpan. Periksa koneksi lalu coba lagi — data yang belum disimpan tidak hilang.
          </p>
          <button
            type="button"
            onClick={() => void useAuthStore.getState().loadMe()}
            className="mt-6 cursor-pointer rounded-lg bg-primary px-4 py-2 font-semibold text-white hover:bg-primary-hover"
          >
            Coba lagi
          </button>
        </div>
      </div>
    );
  }
  if (!user) {
    // Pertahankan path + query agar pengguna kembali ke halaman yang sama setelah masuk.
    const next = encodeURIComponent(location.pathname + location.search);
    return <Navigate to={`${ROUTE_PATHS.login}?next=${next}`} replace />;
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
