import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Clock, FileText } from "lucide-react";
import { QUICK_ACTIONS, type MenuItem } from "@/app/routes/registry";
import type { DocumentHit } from "@/app/routes/document-search";
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
const DOC_SEARCH_DEBOUNCE_MS = 250;

/** Buka command palette dari mana saja (tombol Cari di header). */
export function openCommandPalette(): void {
  window.dispatchEvent(new Event(OPEN_EVENT));
}

interface PaletteGroup {
  title: string;
  items: MenuItem[];
}

/**
 * Command palette (Ctrl/Cmd+K): lompat ke menu, jalankan aksi cepat, buka dokumen terakhir, dan
 * lompat ke dokumen dengan mengetik nomornya (U6-2). Hanya item yang diizinkan peran; tanpa
 * endpoint baru (PLAN U11) — pencarian dokumen memakai endpoint daftar per modul.
 */
export function CommandPalette({ groups }: { groups: PaletteGroup[] }) {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const [open, setOpen] = useState(false);
  const [recent, setRecent] = useState<RecentDoc[]>([]);
  const [query, setQuery] = useState("");
  const [docs, setDocs] = useState<{ query: string; hits: DocumentHit[] }>({ query: "", hits: [] });
  const [searching, setSearching] = useState(false);

  // Cari dokumen bernomor setelah jeda ketik; modul pencarian dimuat malas agar enam service
  // dokumen tidak masuk chunk awal (anggaran bundle U7-4).
  useEffect(() => {
    const q = query.trim();
    if (!open || q.length < 3) return;
    const ctrl = new AbortController();
    const timer = window.setTimeout(() => {
      setSearching(true);
      import("@/app/routes/document-search")
        .then(({ looksLikeDocumentNumber, searchDocuments }) =>
          looksLikeDocumentNumber(q) ? searchDocuments(q, can, ctrl.signal) : [],
        )
        .then((hits) => {
          if (!ctrl.signal.aborted) setDocs({ query: q, hits });
        })
        .catch(() => {
          if (!ctrl.signal.aborted) setDocs({ query: q, hits: [] });
        })
        .finally(() => {
          if (!ctrl.signal.aborted) setSearching(false);
        });
    }, DOC_SEARCH_DEBOUNCE_MS);
    return () => {
      window.clearTimeout(timer);
      ctrl.abort();
    };
  }, [query, open, can]);

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
    else {
      setQuery("");
      setDocs({ query: "", hits: [] });
      setSearching(false);
    }
    setOpen(next);
  }

  function go(path: string) {
    setOpen(false);
    navigate(path);
  }

  const actions = QUICK_ACTIONS.filter((a) => can(a.perm));
  const docHits = docs.query === query.trim() ? docs.hits : [];
  // Pencarian yang dibatalkan (ketikan berlanjut) tidak mereset `searching`; tampilkan hanya
  // selama kueri masih layak dicari.
  const showSearching = searching && query.trim().length >= 3;

  return (
    <CommandDialog
      open={open}
      onOpenChange={handleOpenChange}
      title="Cari menu dan aksi"
      description="Ketik untuk melompat ke halaman atau menjalankan aksi cepat."
      showCloseButton={false}
      className="sm:max-w-xl"
    >
      <CommandInput placeholder="Cari menu, aksi, atau nomor dokumen…" value={query} onValueChange={setQuery} />
      <CommandList className="max-h-[60vh]">
        {showSearching ? (
          <p role="status" className="px-4 py-3 text-sm text-muted-foreground">
            Mencari dokumen…
          </p>
        ) : (
          <CommandEmpty>Tidak ada yang cocok. Untuk dokumen, ketik nomornya, mis. SJ-HQ/2026/00012.</CommandEmpty>
        )}
        {docHits.length > 0 && (
          <>
            <CommandGroup heading="Dokumen">
              {docHits.map((d) => (
                <CommandItem
                  key={d.key}
                  value={`dokumen ${d.key}`}
                  keywords={[d.number, query]}
                  onSelect={() => go(d.path)}
                >
                  <FileText aria-hidden />
                  <span className="flex min-w-0 flex-col">
                    <span className="truncate font-medium">{d.number}</span>
                    <span className="truncate text-xs text-muted-foreground">{`${d.kind} · ${d.detail}`}</span>
                  </span>
                </CommandItem>
              ))}
            </CommandGroup>
            <CommandSeparator />
          </>
        )}
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
