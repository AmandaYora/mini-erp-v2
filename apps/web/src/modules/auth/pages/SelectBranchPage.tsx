import { useEffect, useState } from "react";
import { Navigate, useNavigate, useSearchParams } from "react-router-dom";
import { ChevronRight, Store } from "lucide-react";
import { Badge, Skeleton } from "@/shared/components/ui";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { useAccessibleBranches } from "@/modules/auth/hooks/use-accessible-branches";
import { toApiError } from "@/shared/services/http-client";
import { BRAND_MARK } from "@/modules/auth/lib/auth-styles";
import { Notice } from "@/shared/components/feedback/notice";
import type { AuthBranch } from "@/modules/auth/types";

export default function SelectBranchPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const next = searchParams.get("next") ?? ROUTE_PATHS.dashboard;

  const ready = useAuthStore((s) => s.ready);
  const user = useAuthStore((s) => s.user);
  const branch = useAuthStore((s) => s.branch);
  const pendingBranches = useAuthStore((s) => s.pendingBranches);
  const loadMe = useAuthStore((s) => s.loadMe);
  const chooseBranch = useAuthStore((s) => s.chooseBranch);
  const logout = useAuthStore((s) => s.logout);

  const [selectingId, setSelectingId] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Alur login membawa pendingBranches; alur ganti-cabang (sudah ada cabang
  // aktif) memuat ulang daftar cabang yang bisa diakses.
  const { branches: fetchedBranches, loading: loadingBranches } = useAccessibleBranches(
    ready && pendingBranches.length === 0,
  );

  // Refresh halaman menghilangkan pendingBranches dari memori: sinkronkan
  // sesi via /me dulu sebelum menampilkan notice kosong.
  useEffect(() => {
    if (ready && !branch && pendingBranches.length === 0) {
      loadMe().catch(() => undefined);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ready]);

  const options: AuthBranch[] =
    pendingBranches.length > 0 ? pendingBranches : fetchedBranches;
  const loadingList = !ready || loadingBranches;

  // Sudah ada cabang aktif dan tidak ada alternatif untuk dipilih
  // → kembali ke dashboard. (Jika ada >1 cabang, tampilkan daftar
  // sebagai mode ganti-cabang.)
  if (ready && branch && options.length === 0 && !loadingBranches) {
    return <Navigate to={ROUTE_PATHS.dashboard} replace />;
  }

  async function handleChoose(id: number) {
    setSelectingId(id);
    setError(null);
    try {
      await chooseBranch(id);
      navigate(next, { replace: true });
    } catch (err) {
      setError(toApiError(err).message);
    } finally {
      setSelectingId(null);
    }
  }

  async function handleLogout() {
    await logout();
    navigate(ROUTE_PATHS.login, { replace: true });
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-6">
      <div className="w-full max-w-xl">
        <div className="mb-8 text-center">
          <span className={`${BRAND_MARK} mx-auto mb-4 bg-sidebar`}>M</span>
          <h1 className="font-display text-3xl font-semibold tracking-[-0.02em] text-foreground">Pilih cabang</h1>
          <p className="mx-auto mt-1 max-w-md text-base text-muted-foreground">
            {user
              ? `Halo ${user.fullName}, pilih cabang aktif untuk melanjutkan.`
              : "Pilih cabang aktif untuk melanjutkan."}
          </p>
        </div>

        {error && (
          <div className="mb-4">
            <Notice tone="danger" title="Gagal memilih cabang">
              {error}
            </Notice>
          </div>
        )}

        {loadingList ? (
          <div role="status" aria-busy="true" aria-label="Memuat daftar cabang" className="flex flex-col gap-2">
            <Skeleton className="h-16 w-full" />
            <Skeleton className="h-16 w-full" />
          </div>
        ) : options.length === 0 ? (
          <div className="rounded-lg border border-border bg-card px-4 py-3 text-sm text-muted-foreground">
            Tidak ada cabang yang dapat diakses. Hubungi administrator.
          </div>
        ) : (
          <ul className="flex flex-col gap-2">
            {options.map((b) => (
              <li key={b.id}>
                <button
                  type="button"
                  disabled={selectingId !== null}
                  onClick={() => handleChoose(b.id)}
                  className="flex w-full items-center gap-3 rounded-xl border border-border bg-card p-4 text-left transition-colors hover:border-primary hover:bg-accent disabled:opacity-60"
                >
                  <span className="flex size-10 flex-none items-center justify-center rounded-lg bg-muted text-muted-foreground">
                    <Store size={18} aria-hidden />
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-base font-semibold text-foreground">{b.name}</span>
                    <span className="block font-mono text-xs text-muted-foreground">{b.code}</span>
                  </span>
                  {b.isDefault && <Badge tone="info">Utama</Badge>}
                  {selectingId === b.id ? (
                    <span className="text-sm text-muted-foreground">Memilih…</span>
                  ) : (
                    <ChevronRight size={16} aria-hidden className="text-muted-foreground" />
                  )}
                </button>
              </li>
            ))}
          </ul>
        )}

        <div className="mt-8 text-center">
          <button
            type="button"
            onClick={handleLogout}
            className="text-sm font-medium text-muted-foreground hover:text-foreground hover:underline"
          >
            Keluar dan masuk dengan akun lain
          </button>
        </div>
      </div>
    </div>
  );
}
