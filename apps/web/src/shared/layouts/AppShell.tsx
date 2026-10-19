import { Suspense, useEffect, useMemo, useState } from "react";
import { Link, NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { Check, ChevronRight, ChevronsUpDown, LogOut, Plus, Search } from "lucide-react";
import { MENU_GROUPS, QUICK_ACTIONS, getBreadcrumb, groupSections, type MenuItem } from "@/app/routes/registry";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { isPathPermitted } from "@/app/routes/route-access";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { useAccessibleBranches } from "@/modules/auth/hooks/use-accessible-branches";
import { clearAllDrafts } from "@/shared/lib/draft-storage";
import { usePageMetaStore } from "@/shared/stores/page-meta.store";
import { pushRecent, useUiPrefs, type Density } from "@/shared/stores/ui-prefs.store";
import { cn } from "@/shared/lib/cn";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
  SidebarProvider,
  SidebarRail,
  SidebarTrigger,
  useSidebar,
} from "@/shared/components/primitives/sidebar";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
  Button,
  DetailSkeleton,
  GlobalLoader,
  Kbd,
  Popover,
  PopoverContent,
  PopoverTrigger,
  ToastViewport,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/shared/components/ui";
import { CommandPalette, openCommandPalette } from "./command-palette";

const SIDEBAR_KEY = "mini-erp.sidebar.collapsed";

function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return `${parts[0][0]}${parts[parts.length - 1][0]}`.toUpperCase();
}

const DENSITIES: { value: Density; label: string }[] = [
  { value: "compact", label: "Ringkas" },
  { value: "default", label: "Standar" },
  { value: "comfortable", label: "Lega" },
];

/** Panel akun di kaki sidebar: identitas, ganti role, kepadatan tampilan, keluar. */
function NavUser() {
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const { state, isMobile } = useSidebar();
  const user = useAuthStore((s) => s.user);
  const branch = useAuthStore((s) => s.branch);
  const logout = useAuthStore((s) => s.logout);
  const activeRole = useAuthStore((s) => s.activeRole);
  const availableRoles = useAuthStore((s) => s.availableRoles);
  const switchRole = useAuthStore((s) => s.switchRole);
  const density = useUiPrefs((s) => s.density);
  const setDensity = useUiPrefs((s) => s.setDensity);
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);

  if (!user) return null;
  const displayName = user.fullName || user.username;
  const multiRole = availableRoles.length > 1;

  async function handleLogout() {
    setBusy(true);
    try {
      await logout();
      // Keluar atas kehendak sendiri: draf formulir ikut dibersihkan
      // (sesi habis TIDAK memanggil ini, agar draf bisa dipulihkan).
      clearAllDrafts();
    } finally {
      setBusy(false);
      setOpen(false);
      navigate(ROUTE_PATHS.login, { replace: true });
    }
  }

  async function handleSwitchRole(roleId: number) {
    if (roleId === activeRole?.id) return;
    setBusy(true);
    try {
      await switchRole(roleId);
      setOpen(false);
      // Izin berubah mengikuti role baru: bila rute aktif tak lagi diizinkan, ke dashboard.
      if (!isPathPermitted(pathname, useAuthStore.getState().can)) {
        navigate(ROUTE_PATHS.dashboard, { replace: true });
      }
    } catch (err) {
      toast.danger("Gagal mengganti role", toApiError(err).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <SidebarMenuButton
          size="lg"
          aria-label="Menu akun"
          aria-haspopup="menu"
          className="data-[state=open]:bg-sidebar-accent"
        >
          <span className="flex size-8 flex-none items-center justify-center rounded-md bg-sidebar-active text-xs font-semibold text-sidebar-strong">
            {initials(displayName)}
          </span>
          <span className="grid min-w-0 flex-1 text-left leading-tight">
            <span className="truncate text-sm font-medium text-sidebar-strong">{displayName}</span>
            {activeRole && <span className="truncate text-xs text-sidebar-muted">{activeRole.name}</span>}
          </span>
          <ChevronsUpDown className="ml-auto size-4 text-sidebar-muted" aria-hidden />
        </SidebarMenuButton>
      </PopoverTrigger>
      <PopoverContent
        side={isMobile || state === "collapsed" ? "right" : "top"}
        align="start"
        sideOffset={8}
        className="w-72 p-0"
      >
        <div className="border-b border-border p-4">
          <p className="truncate text-base font-semibold text-foreground">{displayName}</p>
          <div className="mt-1.5 flex flex-wrap gap-1.5">
            {activeRole && (
              <span className="rounded-full bg-accent px-2 py-0.5 text-xs font-medium text-accent-foreground">
                {activeRole.name}
              </span>
            )}
            {branch && (
              <span className="rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
                {branch.code}
              </span>
            )}
          </div>
        </div>
        {multiRole && (
          <div role="group" aria-label="Ganti role" className="border-b border-border p-2">
            <p className="px-2 pb-1 text-xs font-medium text-muted-foreground">Ganti role</p>
            {availableRoles.map((role) => (
              <button
                key={role.id}
                type="button"
                disabled={busy}
                onClick={() => void handleSwitchRole(role.id)}
                className="flex w-full items-center justify-between rounded-md px-2 py-1.5 text-left text-sm text-foreground hover:bg-accent disabled:opacity-60"
              >
                {role.name}
                {role.id === activeRole?.id && <Check size={14} aria-hidden className="text-primary" />}
              </button>
            ))}
          </div>
        )}
        <div role="group" aria-label="Kepadatan tampilan" className="border-b border-border p-2">
          <p className="px-2 pb-1 text-xs font-medium text-muted-foreground">Kepadatan tampilan</p>
          <div className="flex gap-1 px-1">
            {DENSITIES.map((d) => (
              <button
                key={d.value}
                type="button"
                aria-pressed={density === d.value}
                onClick={() => setDensity(d.value)}
                className={cn(
                  "flex-1 rounded-md px-2 py-1 text-xs font-medium",
                  density === d.value ? "bg-accent text-accent-foreground" : "text-muted-foreground hover:bg-muted",
                )}
              >
                {d.label}
              </button>
            ))}
          </div>
        </div>
        <div className="p-2">
          <button
            type="button"
            onClick={handleLogout}
            disabled={busy}
            className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm text-destructive hover:bg-destructive-soft disabled:opacity-60"
          >
            <LogOut size={14} aria-hidden />
            {busy ? "Keluar…" : "Keluar"}
          </button>
        </div>
      </PopoverContent>
    </Popover>
  );
}

/** Kepala sidebar: logo + perusahaan; cabang aktif (klik = ganti cabang bila ada lebih dari satu). */
function BrandHeader() {
  const navigate = useNavigate();
  const branch = useAuthStore((s) => s.branch);
  const company = useAuthStore((s) => s.company);
  const { branches } = useAccessibleBranches(Boolean(branch));
  const canSwitch = branches.length > 1;
  const inner = (
    <>
      <span className="flex size-8 flex-none items-center justify-center rounded-md bg-sidebar-active font-display text-base font-bold text-sidebar-strong">
        M
      </span>
      <span className="grid min-w-0 flex-1 text-left leading-tight">
        <span className="truncate font-display text-base font-semibold text-sidebar-strong">
          {company?.name ?? "Mini ERP"}
        </span>
        <span className="truncate text-xs text-sidebar-muted">
          {branch ? `${branch.code} · ${branch.name}` : "Sistem operasional"}
        </span>
      </span>
      {canSwitch && <ChevronsUpDown className="ml-auto size-4 text-sidebar-muted" aria-hidden />}
    </>
  );
  return (
    <SidebarMenu>
      <SidebarMenuItem>
        {canSwitch ? (
          <SidebarMenuButton
            size="lg"
            tooltip="Ganti cabang"
            aria-label="Ganti cabang"
            onClick={() => navigate(ROUTE_PATHS.selectBranch)}
          >
            {inner}
          </SidebarMenuButton>
        ) : (
          <SidebarMenuButton size="lg" asChild tooltip={branch?.name}>
            <div>{inner}</div>
          </SidebarMenuButton>
        )}
      </SidebarMenuItem>
    </SidebarMenu>
  );
}

type VisibleGroup = { title: string; items: MenuItem[]; sections: { label?: string; items: MenuItem[] }[] };

function NavItems({ groups, activePath }: { groups: VisibleGroup[]; activePath: string | null }) {
  const { state } = useSidebar();
  const compact = state === "collapsed";
  // Seksi collapsible: "Keuangan:Harian" terbuka awal; seksi yang memuat halaman aktif selalu terbuka.
  const [openSections, setOpenSections] = useState<Record<string, boolean>>({ "Keuangan:Harian": true });

  const renderItem = (item: MenuItem, sub = false) => {
    const Icon = item.icon;
    const active = activePath === item.path;
    const link = (
      <NavLink to={item.path} aria-current={active ? "page" : undefined} className="text-current hover:no-underline">
        {!sub && <Icon aria-hidden />}
        <span>{item.label}</span>
      </NavLink>
    );
    return sub ? (
      <SidebarMenuSubItem key={item.path}>
        <SidebarMenuSubButton asChild isActive={active}>
          {link}
        </SidebarMenuSubButton>
      </SidebarMenuSubItem>
    ) : (
      <SidebarMenuItem key={item.path}>
        <SidebarMenuButton asChild isActive={active} tooltip={item.label}>
          {link}
        </SidebarMenuButton>
      </SidebarMenuItem>
    );
  };

  return (
    <>
      {groups.map((group) => (
        <SidebarGroup key={group.title}>
          <SidebarGroupLabel className="text-2xs font-semibold tracking-[0.08em] text-sidebar-muted uppercase">
            {group.title}
          </SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {group.sections.map((section) => {
                if (!section.label || compact) {
                  return section.items.map((i) => renderItem(i));
                }
                const key = `${group.title}:${section.label}`;
                const hasActive = section.items.some((i) => i.path === activePath);
                const expanded = hasActive || (openSections[key] ?? false);
                return (
                  <Collapsible
                    key={key}
                    open={expanded}
                    onOpenChange={(v) => setOpenSections((cur) => ({ ...cur, [key]: v }))}
                    asChild
                  >
                    <SidebarMenuItem>
                      <CollapsibleTrigger asChild>
                        <SidebarMenuButton
                          aria-expanded={expanded}
                          className={cn("text-sidebar-foreground", hasActive && "text-sidebar-strong")}
                        >
                          <span className="font-medium">{section.label}</span>
                          <ChevronRight
                            aria-hidden
                            className={cn("ml-auto size-4 transition-transform", expanded && "rotate-90")}
                          />
                        </SidebarMenuButton>
                      </CollapsibleTrigger>
                      <CollapsibleContent>
                        <SidebarMenuSub>{section.items.map((i) => renderItem(i, true))}</SidebarMenuSub>
                      </CollapsibleContent>
                    </SidebarMenuItem>
                  </Collapsible>
                );
              })}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      ))}
    </>
  );
}

function QuickCreate() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const [open, setOpen] = useState(false);
  const actions = QUICK_ACTIONS.filter((a) => can(a.perm));
  if (actions.length === 0) return null;
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button size="sm">
          <Plus aria-hidden /> Buat
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-56 p-1">
        {actions.map((a) => {
          const Icon = a.icon;
          return (
            <button
              key={a.path + a.label}
              type="button"
              onClick={() => {
                setOpen(false);
                navigate(a.path);
              }}
              className="flex w-full items-center gap-2 rounded-md px-2 py-2 text-left text-sm text-foreground hover:bg-accent"
            >
              <Icon size={16} aria-hidden className="text-muted-foreground" />
              {a.label}
            </button>
          );
        })}
      </PopoverContent>
    </Popover>
  );
}

function AppHeader({ pathname }: { pathname: string }) {
  const crumb = usePageMetaStore((s) => s.crumb);
  const bc = getBreadcrumb(pathname);
  return (
    <header className="sticky top-0 z-30 flex h-12 items-center gap-2 rounded-t-xl border-b border-border bg-card px-4">
      <SidebarTrigger className="-ml-1" aria-label="Buka/tutup menu" />
      <Breadcrumb className="min-w-0 flex-1">
        <BreadcrumbList className="flex-nowrap text-sm">
          {bc.group && (
            <>
              <BreadcrumbItem className="max-md:hidden">{bc.group}</BreadcrumbItem>
              <BreadcrumbSeparator className="max-md:hidden" />
            </>
          )}
          <BreadcrumbItem className="min-w-0">
            {crumb && bc.path ? (
              <BreadcrumbLink asChild>
                <Link to={bc.path}>{bc.title}</Link>
              </BreadcrumbLink>
            ) : (
              <BreadcrumbPage className="truncate">{bc.title}</BreadcrumbPage>
            )}
          </BreadcrumbItem>
          {crumb && (
            <>
              <BreadcrumbSeparator />
              <BreadcrumbItem className="min-w-0">
                <BreadcrumbPage className="truncate font-mono text-xs">{crumb}</BreadcrumbPage>
              </BreadcrumbItem>
            </>
          )}
        </BreadcrumbList>
      </Breadcrumb>
      <Button variant="secondary" size="sm" onClick={openCommandPalette} aria-label="Cari (Ctrl K)" className="text-muted-foreground">
        <Search aria-hidden />
        <span className="max-sm:hidden">Cari…</span>
        <Kbd className="max-sm:hidden">Ctrl K</Kbd>
      </Button>
      <QuickCreate />
    </header>
  );
}

/**
 * Kerangka aplikasi "Bingkai Navy" (PLAN §6): sidebar navy gelap varian inset, konten di
 * kanvas terang yang mengambang. Menggantikan AppLayout lama (511 baris, logika lebar/drawer manual).
 */
export default function AppShell() {
  const { pathname } = useLocation();
  const permissions = useAuthStore((s) => s.permissions);
  const [defaultOpen] = useState<boolean>(() => {
    try {
      return localStorage.getItem(SIDEBAR_KEY) !== "1";
    } catch {
      return true;
    }
  });

  const visibleGroups = useMemo<VisibleGroup[]>(
    () =>
      MENU_GROUPS.map((group) => {
        const items = group.items.filter((item) => permissions.includes(item.perm));
        return { title: group.title, items, sections: groupSections(group.title, items) };
      }).filter((group) => group.items.length > 0),
    [permissions],
  );

  // Item aktif = path menu terpanjang yang cocok (rute detail mewarisi menu induknya),
  // sehingga "/stock/balances" tidak menyalakan "Stok" juga.
  const activePath = useMemo(() => {
    const all = visibleGroups.flatMap((g) => g.items);
    return (
      all
        .filter((i) => pathname === i.path || pathname.startsWith(`${i.path}/`))
        .sort((a, b) => b.path.length - a.path.length)[0]?.path ?? null
    );
  }, [visibleGroups, pathname]);

  // Catat halaman detail yang dibuka untuk "Terakhir dibuka" di command palette.
  useEffect(() => {
    if (activePath && pathname !== activePath) {
      pushRecent({ path: pathname, label: `${getBreadcrumb(pathname).title} · ${pathname.split("/").pop()}` });
    }
  }, [pathname, activePath]);

  return (
    <SidebarProvider
      defaultOpen={defaultOpen}
      onOpenChange={(open) => {
        try {
          localStorage.setItem(SIDEBAR_KEY, open ? "0" : "1");
        } catch {
          /* preferensi sesi ini saja */
        }
      }}
    >
      <GlobalLoader />
      <Sidebar variant="inset" collapsible="icon">
        <SidebarHeader>
          <BrandHeader />
        </SidebarHeader>
        <SidebarContent>
          <nav aria-label="Menu utama" className="contents">
            <NavItems groups={visibleGroups} activePath={activePath} />
          </nav>
        </SidebarContent>
        <SidebarFooter>
          <SidebarMenu>
            <SidebarMenuItem>
              <NavUser />
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarFooter>
        <SidebarRail />
      </Sidebar>
      <SidebarInset className="min-w-0 bg-background">
        <AppHeader pathname={pathname} />
        <div className="flex-1 p-6 max-md:p-4">
          <div className="mx-auto w-full max-w-[1440px]">
            <Suspense fallback={<DetailSkeleton />}>
              <Outlet />
            </Suspense>
          </div>
        </div>
      </SidebarInset>
      <CommandPalette groups={visibleGroups} />
      <ToastViewport />
    </SidebarProvider>
  );
}
