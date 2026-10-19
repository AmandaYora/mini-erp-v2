import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Clock } from "lucide-react";
import { QUICK_ACTIONS, type MenuItem } from "@/app/routes/registry";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { readRecent, type RecentDoc } from "@/shared/stores/ui-prefs.store";
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "@/shared/components/primitives/command";

const OPEN_EVENT = "mini-erp:open-palette";

/** Buka command palette dari mana saja (tombol Cari di header). */
export function openCommandPalette(): void {
  window.dispatchEvent(new Event(OPEN_EVENT));
}

interface PaletteGroup {
  title: string;
  items: MenuItem[];
}

/**
 * Command palette (Ctrl/Cmd+K): lompat ke menu, jalankan aksi cepat, buka dokumen terakhir.
 * Hanya item yang diizinkan peran; tanpa endpoint baru (PLAN U11).
 */
export function CommandPalette({ groups }: { groups: PaletteGroup[] }) {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const [open, setOpen] = useState(false);
  const [recent, setRecent] = useState<RecentDoc[]>([]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key.toLowerCase() === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        setOpen((v) => !v);
      }
    };
    const onOpen = () => setOpen(true);
    document.addEventListener("keydown", onKey);
    window.addEventListener(OPEN_EVENT, onOpen);
    return () => {
      document.removeEventListener("keydown", onKey);
      window.removeEventListener(OPEN_EVENT, onOpen);
    };
  }, []);

  function handleOpenChange(next: boolean) {
    if (next) setRecent(readRecent());
    setOpen(next);
  }

  function go(path: string) {
    setOpen(false);
    navigate(path);
  }

  const actions = QUICK_ACTIONS.filter((a) => can(a.perm));

  return (
    <CommandDialog
      open={open}
      onOpenChange={handleOpenChange}
      title="Cari menu dan aksi"
      description="Ketik untuk melompat ke halaman atau menjalankan aksi cepat."
      showCloseButton={false}
      className="sm:max-w-xl"
    >
      <CommandInput placeholder="Cari menu, aksi, atau dokumen…" />
      <CommandList className="max-h-[60vh]">
        <CommandEmpty>Tidak ada yang cocok.</CommandEmpty>
        {recent.length > 0 && (
          <>
            <CommandGroup heading="Terakhir dibuka">
              {recent.map((r) => (
                <CommandItem key={r.path} value={`terakhir ${r.label}`} onSelect={() => go(r.path)}>
                  <Clock aria-hidden /> {r.label}
                </CommandItem>
              ))}
            </CommandGroup>
            <CommandSeparator />
          </>
        )}
        {actions.length > 0 && (
          <>
            <CommandGroup heading="Aksi">
              {actions.map((a) => {
                const Icon = a.icon;
                return (
                  <CommandItem key={a.label} value={`aksi ${a.label}`} onSelect={() => go(a.path)}>
                    <Icon aria-hidden /> {a.label}
                  </CommandItem>
                );
              })}
            </CommandGroup>
            <CommandSeparator />
          </>
        )}
        {groups.map((g) => (
          <CommandGroup key={g.title} heading={g.title}>
            {g.items.map((i) => {
              const Icon = i.icon;
              return (
                <CommandItem key={i.path} value={`${g.title} ${i.label}`} onSelect={() => go(i.path)}>
                  <Icon aria-hidden /> {i.label}
                </CommandItem>
              );
            })}
          </CommandGroup>
        ))}
      </CommandList>
    </CommandDialog>
  );
}
