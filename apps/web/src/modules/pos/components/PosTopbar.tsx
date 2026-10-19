import { createPortal } from "react-dom";
import { ScanLine, Search, X } from "lucide-react";

/**
 * Isi topbar Mode Kasir (antara judul dan jam): pencarian produk + tombol PINDAI.
 * Di-render lewat portal ke slot FocusLayout; tanpa slot (mis. di test) tampil inline.
 * Input TIDAK auto-focus — keyboard layar hanya muncul saat kasir mengetuknya.
 */
export function PosTopbar({
  slot,
  query,
  onQueryChange,
  onScan,
}: {
  /** undefined = tanpa layout (tampil inline); null = slot belum terpasang. */
  slot: HTMLElement | null | undefined;
  query: string;
  onQueryChange: (q: string) => void;
  onScan: () => void;
}) {
  const content = (
    <>
      <div className="relative min-w-0 flex-1">
        <Search aria-hidden className="pointer-events-none absolute top-1/2 left-3.5 size-5 -translate-y-1/2 text-sidebar-muted" />
        <input
          id="pos-search"
          type="search"
          aria-label="Cari Produk"
          value={query}
          onChange={(e) => onQueryChange(e.target.value)}
          placeholder="Cari produk (kode / nama)…"
          inputMode="search"
          enterKeyHint="search"
          autoComplete="off"
          className="h-12 w-full rounded-lg border border-sidebar-border bg-sidebar-accent pr-12 pl-11 text-base text-sidebar-strong placeholder:text-sidebar-muted focus:border-sidebar-ring focus:ring-2 focus:ring-sidebar-ring/40 focus:outline-none"
        />
        {query !== "" && (
          <button
            type="button"
            aria-label="Hapus pencarian"
            onClick={() => onQueryChange("")}
            className="absolute top-0 right-0 flex size-12 touch-manipulation items-center justify-center rounded-lg text-sidebar-muted select-none active:bg-sidebar-active"
          >
            <X aria-hidden className="size-5" />
          </button>
        )}
      </div>
      <button
        type="button"
        onClick={onScan}
        aria-label="Pindai dengan kamera"
        className="flex h-12 flex-none touch-manipulation items-center gap-2 rounded-lg bg-sidebar-primary px-5 text-base font-semibold text-sidebar-primary-foreground transition-transform duration-100 select-none active:scale-[0.98]"
      >
        <ScanLine aria-hidden className="size-5" />
        Pindai
      </button>
    </>
  );
  if (slot === null) return null;
  return slot ? createPortal(content, slot) : <div className="flex items-center gap-3 bg-sidebar p-3">{content}</div>;
}
