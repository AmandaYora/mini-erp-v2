import { useGlobalLoading } from "@/shared/hooks/use-global-loading";

/**
 * GlobalLoader — bilah progres tipis di puncak viewport selama ada request
 * http-client yang berjalan.
 *
 * Diport dari aplikasi lama dengan penyederhanaan sengaja: versi lama memakai
 * state machine growing→completing→fading + timer (2 effect + setState di
 * effect). Di sini bilah indeterminate murni CSS (tanpa effect, tanpa timer,
 * tanpa peringatan lint): tampil saat `useGlobalLoading()` true, animasi
 * geser via keyframes `loader-slide` di theme.css.
 *
 * Dipasang sekali di AppLayout di atas header.
 */
export function GlobalLoader() {
  const active = useGlobalLoading();
  if (!active) return null;
  return (
    <div
      role="progressbar"
      aria-label="Memuat data"
      className="pointer-events-none fixed inset-x-0 top-0 z-[9999] h-[3px] overflow-hidden bg-brand/10"
    >
      <div aria-hidden="true" className="animate-loader-slide h-full w-1/3 rounded-r-[2px] bg-brand shadow-[0_0_10px_rgba(30,58,95,0.5)]" />
    </div>
  );
}
