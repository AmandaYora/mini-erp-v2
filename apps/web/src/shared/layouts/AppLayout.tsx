import { Suspense, useState } from "react";
import { Link, NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import {
  Building2,
  ChevronDown,
  LogOut,
  Menu,
  PanelLeftClose,
  PanelLeftOpen,
  Store,
  X,
} from "lucide-react";
import { MENU_GROUPS, getPageTitle } from "@/app/routes/registry";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { useAccessibleBranches } from "@/modules/auth/hooks/use-accessible-branches";
import { ToastViewport } from "@/shared/components/ui/toast-viewport";
import { GlobalLoader } from "@/shared/components/ui/global-loader";

const SIDEBAR_KEY = "mini-erp.sidebar.collapsed";

const LINK_BASE =
  "rounded-lg px-3 py-2 text-[0.9rem] font-medium text-sidebar-fg hover:bg-white/5 hover:text-sidebar-strong flex items-center gap-3";
const LINK_ACTIVE = "bg-white/10 font-semibold text-white";

function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return `${parts[0][0]}${parts[parts.length - 1][0]}`.toUpperCase();
}

function AvatarMenu({ canSwitch }: { canSwitch: boolean }) {
  const navigate = useNavigate();
  const user = useAuthStore((s) => s.user);
  const branch = useAuthStore((s) => s.branch);
  const logout = useAuthStore((s) => s.logout);
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);

  if (!user) return null;

  async function handleLogout() {
    setBusy(true);
    try {
      await logout();
    } finally {
      setBusy(false);
      setOpen(false);
      navigate(ROUTE_PATHS.login, { replace: true });
    }
  }

  return (
    <div className="relative">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label="Menu akun"
        className="flex items-center gap-1.5 rounded-full p-1 hover:bg-surface-sunken"
      >
        <span className="flex h-8 w-8 items-center justify-center rounded-full bg-brand text-sm font-bold text-white">
          {initials(user.fullName || user.username)}
        </span>
        <ChevronDown size={14} className="text-muted" />
      </button>
      {open && (
        <>
          <button
            type="button"
            aria-label="Tutup menu akun"
            onClick={() => setOpen(false)}
            className="fixed inset-0 z-10 cursor-default"
          />
          <div
            role="menu"
            className="absolute right-0 top-full z-20 mt-2 w-[260px] rounded-[10px] border border-hairline bg-surface p-2 shadow-lg"
          >
            <div className="px-3 py-2">
              <p className="truncate text-sm font-semibold text-ink">{user.fullName}</p>
              <p className="truncate text-xs text-muted">@{user.username}</p>
              {branch && (
                <p className="mt-1 truncate text-xs text-muted">
                  {branch.code} — {branch.name}
                </p>
              )}
            </div>
            <div className="my-1 border-t border-hairline" />
            {canSwitch && (
              <Link
                to={ROUTE_PATHS.selectBranch}
                onClick={() => setOpen(false)}
                className="block rounded-lg px-3 py-2 text-sm font-medium text-ink hover:bg-surface-subtle"
              >
                Ganti Cabang
              </Link>
            )}
            <button
              type="button"
              onClick={handleLogout}
              disabled={busy}
              className="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm font-medium text-bad hover:bg-bad-soft disabled:opacity-60"
            >
              <LogOut size={14} />
              {busy ? "Keluar…" : "Keluar"}
            </button>
          </div>
        </>
      )}
    </div>
  );
}

export default function AppLayout() {
  const { pathname } = useLocation();
  const permissions = useAuthStore((s) => s.permissions);
  const branch = useAuthStore((s) => s.branch);
  const company = useAuthStore((s) => s.company);

  const [collapsed, setCollapsed] = useState<boolean>(() => {
    try {
      return localStorage.getItem(SIDEBAR_KEY) === "1";
    } catch {
      return false;
    }
  });
  const [mobileOpen, setMobileOpen] = useState(false);

  // Ganti halaman (mobile) → tutup drawer. Render-phase adjustment (bukan
  // effect): perilaku sama, tanpa render beruntun.
  const [prevPath, setPrevPath] = useState(pathname);
  if (pathname !== prevPath) {
    setPrevPath(pathname);
    if (mobileOpen) setMobileOpen(false);
  }

  function toggleCollapsed() {
    setCollapsed((prev) => {
      const next = !prev;
      try {
        localStorage.setItem(SIDEBAR_KEY, next ? "1" : "0");
      } catch {
        // Penyimpanan privat: preferensi sesi ini saja.
      }
      return next;
    });
  }

  const { branches: accessibleBranches } = useAccessibleBranches(Boolean(branch));
  const canSwitch = accessibleBranches.length > 1;
  const title = getPageTitle(pathname);

  const visibleGroups = MENU_GROUPS.map((group) => ({
    ...group,
    items: group.items.filter((item) => permissions.includes(item.perm)),
  })).filter((group) => group.items.length > 0);

  const sidebarWidth = collapsed ? "w-[76px]" : "w-64";

  return (
    <div className="flex min-h-screen bg-surface-subtle">
      <GlobalLoader />
      {mobileOpen && (
        <div
          onClick={() => setMobileOpen(false)}
          className="fixed inset-0 z-[70] bg-ink/50 min-[901px]:hidden"
        />
      )}

      <aside
        className={`sticky top-0 z-40 flex h-screen shrink-0 flex-col bg-sidebar text-sidebar-fg transition-all duration-200 ${sidebarWidth} max-[900px]:fixed max-[900px]:left-0 max-[900px]:top-0 max-[900px]:z-[80] max-[900px]:w-64 ${mobileOpen ? "max-[900px]:translate-x-0" : "max-[900px]:-translate-x-full"}`}
      >
        <div className={`flex items-center gap-3 px-4 py-4 ${collapsed ? "flex-col justify-center px-2" : ""}`}>
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-brand text-base font-bold text-white">
            M
          </div>
          {!collapsed && (
            <span className="min-w-0 flex-1 truncate text-lg font-bold text-sidebar-strong">
              mini-erp
            </span>
          )}
          <button
            type="button"
            onClick={toggleCollapsed}
            aria-label={collapsed ? "Bentangkan sidebar" : "Ciutkan sidebar"}
            className="hidden shrink-0 rounded-md p-1.5 text-sidebar-fg hover:bg-white/5 hover:text-sidebar-strong min-[901px]:block"
          >
            {collapsed ? <PanelLeftOpen size={18} /> : <PanelLeftClose size={18} />}
          </button>
          <button
            type="button"
            onClick={() => setMobileOpen(false)}
            aria-label="Tutup menu"
            className="shrink-0 rounded-md p-1.5 text-sidebar-fg hover:bg-white/5 hover:text-sidebar-strong min-[901px]:hidden"
          >
            <X size={18} />
          </button>
        </div>

        <nav className="flex-1 overflow-y-auto px-3 pb-4">
          {visibleGroups.map((group) => (
            <div key={group.title}>
              {collapsed ? (
                <div className="mx-3 mt-4 border-t border-white/10" />
              ) : (
                <p className="px-3 pt-4 text-[0.75rem] font-semibold uppercase text-muted">
                  {group.title}
                </p>
              )}
              <ul className="mt-1 space-y-1">
                {group.items.map((item) => {
                  const Icon = item.icon;
                  return (
                    <li key={item.path}>
                      <NavLink
                        to={item.path}
                        title={collapsed ? item.label : undefined}
                        className={({ isActive }) =>
                          `${LINK_BASE} ${collapsed ? "justify-center px-0" : ""} ${isActive ? LINK_ACTIVE : ""}`
                        }
                      >
                        <Icon size={18} className="shrink-0" aria-hidden />
                        {!collapsed && <span className="truncate">{item.label}</span>}
                      </NavLink>
                    </li>
                  );
                })}
              </ul>
            </div>
          ))}
        </nav>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-30 min-h-[64px] border-b border-hairline bg-surface px-8 shadow-md max-[900px]:px-4">
          <div className="flex min-h-[64px] items-center justify-between gap-4">
            <div className="flex min-w-0 items-center gap-3">
              <button
                type="button"
                onClick={() => setMobileOpen(true)}
                aria-label="Buka menu"
                className="shrink-0 rounded-md p-1.5 text-ink hover:bg-surface-subtle min-[901px]:hidden"
              >
                <Menu size={20} />
              </button>
              <div className="min-w-0">
                <p className="text-xs text-muted">Cabang Aktif</p>
                <h1 className="truncate font-semibold text-ink">{title}</h1>
              </div>
            </div>
            <div className="flex shrink-0 items-center gap-3">
              {branch && (
                <div className="flex items-center gap-2 rounded-full border border-hairline bg-surface-subtle px-3 py-1.5">
                  <Store size={14} className="shrink-0 text-muted" />
                  <span className="text-sm font-bold text-ink">{branch.code}</span>
                  <span className="hidden text-sm text-muted min-[1100px]:inline">
                    {branch.name}
                  </span>
                  {canSwitch && (
                    <Link
                      to={ROUTE_PATHS.selectBranch}
                      className="text-sm font-medium text-brand hover:underline"
                    >
                      Ganti
                    </Link>
                  )}
                </div>
              )}
              {company && (
                <div className="hidden items-center gap-2 rounded-full border border-hairline bg-surface-subtle px-3 py-1.5 min-[1100px]:flex">
                  <Building2 size={14} className="shrink-0 text-muted" />
                  <span className="max-w-[180px] truncate text-sm text-muted">
                    {company.name}
                  </span>
                </div>
              )}
              <AvatarMenu canSwitch={canSwitch} />
            </div>
          </div>
        </header>

        <main className="mx-auto w-full max-w-[1400px] flex-1 p-8 max-[900px]:p-4">
          <Suspense
            fallback={<div className="text-sm text-muted">Memuat halaman…</div>}
          >
            <Outlet />
          </Suspense>
        </main>
      </div>

      <ToastViewport />
    </div>
  );
}
