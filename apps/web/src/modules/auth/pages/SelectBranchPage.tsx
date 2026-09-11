import { useEffect, useState } from "react";
import { Navigate, useNavigate, useSearchParams } from "react-router-dom";
import { Store } from "lucide-react";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { useAccessibleBranches } from "@/modules/auth/hooks/use-accessible-branches";
import { toApiError } from "@/shared/services/http-client";
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
    <div className="flex min-h-screen items-center justify-center bg-surface-subtle p-4">
      <div className="w-full max-w-md rounded-[10px] border border-hairline bg-surface p-8 shadow-md">
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-md bg-brand text-base font-bold text-white">
            M
          </div>
          <span className="text-lg font-bold text-ink">mini-erp</span>
        </div>
        <h1 className="mt-4 font-semibold text-ink">Pilih Cabang</h1>
        <p className="mt-1 text-sm text-muted">
          {user
            ? `Halo ${user.fullName}, pilih cabang aktif untuk melanjutkan.`
            : "Pilih cabang aktif untuk melanjutkan."}
        </p>

        {error && (
          <div
            role="alert"
            className="mt-4 rounded-lg border border-bad/30 bg-bad-soft px-3 py-2 text-sm font-medium text-bad"
          >
            {error}
          </div>
        )}

        <div className="mt-6">
          {loadingList ? (
            <p className="text-sm text-muted">Memuat daftar cabang…</p>
          ) : options.length === 0 ? (
            <div className="rounded-lg border border-hairline bg-surface-subtle px-4 py-3 text-sm text-muted">
              Tidak ada cabang yang dapat diakses. Hubungi administrator.
            </div>
          ) : (
            <ul className="space-y-2">
              {options.map((b) => (
                <li key={b.id}>
                  <button
                    type="button"
                    disabled={selectingId !== null}
                    onClick={() => handleChoose(b.id)}
                    className="flex w-full items-center gap-3 rounded-lg border border-hairline px-4 py-3 text-left hover:border-brand hover:bg-surface-subtle disabled:opacity-60"
                  >
                    <Store size={18} className="shrink-0 text-muted" />
                    <span className="min-w-0 flex-1">
                      <span className="block text-sm font-bold text-ink">{b.code}</span>
                      <span className="block truncate text-sm text-muted">{b.name}</span>
                    </span>
                    {selectingId === b.id && (
                      <span className="text-sm text-muted">Memilih…</span>
                    )}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>

        <button
          type="button"
          onClick={handleLogout}
          className="mt-6 text-sm font-medium text-muted hover:text-ink hover:underline"
        >
          Keluar dan masuk dengan akun lain
        </button>
      </div>
    </div>
  );
}
