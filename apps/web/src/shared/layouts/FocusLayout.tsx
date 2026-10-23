import { Suspense, useCallback, useEffect, useState } from "react";
import { Link, Outlet, useNavigate } from "react-router-dom";
import { ArrowLeft, Maximize, Minimize, MoreHorizontal, LogOut, LayoutDashboard, RotateCw } from "lucide-react";
import { cn } from "@/shared/lib/cn";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { clearAllDrafts } from "@/shared/lib/draft-storage";
import {
  DetailSkeleton,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  ToastViewport,
} from "@/shared/components/ui";

/** Konteks yang diteruskan ke halaman di dalam layout lewat `useOutletContext`. */
export interface FocusLayoutContext {
  /** Elemen di topbar (antara judul dan jam) tempat halaman boleh me-render kontrolnya lewat portal. */
  topbarSlot: HTMLElement | null;
}

const CLOCK = new Intl.DateTimeFormat("id-ID", { hour: "2-digit", minute: "2-digit", hourCycle: "h23", timeZone: "Asia/Jakarta" });

function useClock(): string {
  const [now, setNow] = useState(() => CLOCK.format(new Date()));
  useEffect(() => {
    const id = window.setInterval(() => setNow(CLOCK.format(new Date())), 15_000);
    return () => window.clearInterval(id);
  }, []);
  return now.replace(".", ":");
}

function useFullscreen() {
  const supported = typeof document !== "undefined" && document.fullscreenEnabled === true;
  const [active, setActive] = useState(() => typeof document !== "undefined" && document.fullscreenElement != null);
  useEffect(() => {
    const onChange = () => setActive(document.fullscreenElement != null);
    document.addEventListener("fullscreenchange", onChange);
    return () => document.removeEventListener("fullscreenchange", onChange);
  }, []);
  const toggle = useCallback(() => {
    if (document.fullscreenElement) void document.exitFullscreen().catch(() => {});
    else void document.documentElement.requestFullscreen().catch(() => {});
  }, []);
  return { supported, active, toggle };
}

/** Tahan layar tetap menyala (Screen Wake Lock) selama layout terpasang; lepas saat keluar. */
function useWakeLock() {
  useEffect(() => {
    if (!("wakeLock" in navigator)) return;
    const wl = navigator.wakeLock;
    let sentinel: WakeLockSentinel | null = null;
    let alive = true;
    const acquire = async () => {
      try {
        const s = await wl.request("screen");
        if (!alive) {
          void s.release().catch(() => {});
          return;
        }
        sentinel = s;
      } catch {
        /* ditolak (mis. baterai hemat / tab tak terlihat): layar mengikuti pengaturan perangkat */
      }
    };
    // Wake lock dilepas otomatis saat tab disembunyikan; ambil ulang begitu terlihat lagi.
    const onVisible = () => {
      if (document.visibilityState === "visible" && (sentinel === null || sentinel.released)) void acquire();
    };
    void acquire();
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      alive = false;
      document.removeEventListener("visibilitychange", onVisible);
      void sentinel?.release().catch(() => {});
    };
  }, []);
}

/** Cegah tarik-untuk-muat-ulang, ubah ukuran tampilan mengikuti keyboard layar, dan izinkan isi sampai tepi
 * layar (viewport-fit=cover: env(safe-area-inset-*) bernilai di iPhone) — hanya selama layout aktif. */
function useTouchShell() {
  useEffect(() => {
    const html = document.documentElement;
    const body = document.body;
    const prev = { html: html.style.overscrollBehavior, body: body.style.overscrollBehavior };
    html.style.overscrollBehavior = "none";
    body.style.overscrollBehavior = "none";

    const meta = document.querySelector<HTMLMetaElement>('meta[name="viewport"]');
    const prevContent = meta?.getAttribute("content") ?? null;
    if (meta && prevContent) {
      const extra = [
        !prevContent.includes("interactive-widget") && "interactive-widget=resizes-content",
        !prevContent.includes("viewport-fit") && "viewport-fit=cover",
      ].filter(Boolean);
      if (extra.length > 0) meta.setAttribute("content", [prevContent, ...extra].join(", "));
    }
    return () => {
      html.style.overscrollBehavior = prev.html;
      body.style.overscrollBehavior = prev.body;
      if (meta && prevContent !== null) meta.setAttribute("content", prevContent);
    };
  }, []);
}

const TOPBAR_BUTTON =
  "flex size-12 flex-none touch-manipulation items-center justify-center rounded-lg text-sidebar-strong transition-[background-color,transform] duration-100 select-none active:scale-[0.98] active:bg-sidebar-active focus-visible:ring-2 focus-visible:ring-sidebar-ring focus-visible:outline-none";

/**
 * Layar penuh tanpa sidebar untuk alur kerja sentuh (Mode Kasir, PLAN §8 pos / T10).
 * Topbar berwarna bingkai navy (64px; 56px di bawah 1024): kembali, judul + cabang + pengguna, slot
 * halaman, jam, layar penuh, menu. Di bawah 1024 (HP, tablet tegak) jam disembunyikan, layar penuh
 * pindah ke menu, dan slot hanya tampil saat mendatar; halaman memasang tata letak ringkasnya sendiri (brief desain-pos-hp, temuan 5 —
 * ajakan "putar tablet" dihapus).
 */
export default function FocusLayout({ title }: { title: string }) {
  const navigate = useNavigate();
  const user = useAuthStore((s) => s.user);
  const branch = useAuthStore((s) => s.branch);
  const logout = useAuthStore((s) => s.logout);
  const clock = useClock();
  const fullscreen = useFullscreen();
  const [slot, setSlot] = useState<HTMLElement | null>(null);
  const [busy, setBusy] = useState(false);
  useWakeLock();
  useTouchShell();

  async function handleLogout() {
    setBusy(true);
    try {
      await logout();
      clearAllDrafts();
    } finally {
      setBusy(false);
      navigate(ROUTE_PATHS.login, { replace: true });
    }
  }

  const who = [branch?.code, user?.fullName || user?.username].filter(Boolean).join(" · ");

  return (
    <>
      <div className="fixed inset-0 flex touch-manipulation flex-col overflow-hidden overscroll-none bg-background">
        <header className="flex h-14 flex-none items-center gap-3 bg-sidebar px-3 text-sidebar-strong lg:h-16">
          <Link to={ROUTE_PATHS.dashboard} aria-label="Kembali ke dashboard" className={TOPBAR_BUTTON}>
            <ArrowLeft aria-hidden className="size-6" />
          </Link>
          <div className="min-w-0 flex-none max-lg:portrait:flex-1">
            <h1 className="font-display text-lg leading-tight font-semibold text-sidebar-strong">{title}</h1>
            {who && <p className="max-w-56 truncate text-xs text-sidebar-muted max-lg:portrait:max-w-none">{who}</p>}
          </div>
          {/* Di bawah 1024 slot hanya dipakai saat mendatar (HP diputar); tegak, halaman memasang isinya sendiri. */}
          <div ref={setSlot} className="flex min-w-0 flex-1 items-center gap-3 max-lg:portrait:hidden" />
          <p className="tabular flex-none px-1 font-display text-lg font-medium max-lg:hidden" aria-label={`Pukul ${clock}`}>
            {clock}
          </p>
          {fullscreen.supported && (
            <button
              type="button"
              onClick={fullscreen.toggle}
              aria-label={fullscreen.active ? "Keluar dari layar penuh" : "Layar penuh"}
              aria-pressed={fullscreen.active}
              className={cn(TOPBAR_BUTTON, "max-lg:hidden")}
            >
              {fullscreen.active ? <Minimize aria-hidden className="size-5" /> : <Maximize aria-hidden className="size-5" />}
            </button>
          )}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button type="button" aria-label="Menu lainnya" className={TOPBAR_BUTTON}>
                <MoreHorizontal aria-hidden className="size-6" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-60 p-1.5">
              <DropdownMenuItem className="h-12 gap-3 text-base" onSelect={() => navigate(ROUTE_PATHS.dashboard)}>
                <LayoutDashboard aria-hidden className="size-5" />
                Ke dashboard
              </DropdownMenuItem>
              {fullscreen.supported && (
                <DropdownMenuItem className="h-12 gap-3 text-base lg:hidden" onSelect={fullscreen.toggle}>
                  {fullscreen.active ? <Minimize aria-hidden className="size-5" /> : <Maximize aria-hidden className="size-5" />}
                  {fullscreen.active ? "Keluar dari layar penuh" : "Layar penuh"}
                </DropdownMenuItem>
              )}
              <DropdownMenuItem className="h-12 gap-3 text-base" onSelect={() => window.location.reload()}>
                <RotateCw aria-hidden className="size-5" />
                Muat ulang halaman
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                className="h-12 gap-3 text-base"
                disabled={busy}
                onSelect={() => void handleLogout()}
              >
                <LogOut aria-hidden className="size-5" />
                Keluar
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </header>

        <main className="min-h-0 flex-1">
          <Suspense fallback={<DetailSkeleton />}>
            <Outlet context={{ topbarSlot: slot } satisfies FocusLayoutContext} />
          </Suspense>
        </main>
      </div>
      <ToastViewport />
    </>
  );
}
