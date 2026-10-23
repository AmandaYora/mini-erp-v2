import { memo, useEffect, useRef, useState } from "react";
import { ScanQrCode } from "lucide-react";
import { EmptyState } from "@/shared/components/feedback/empty-state";
import { Notice } from "@/shared/components/feedback/notice";
import { Button, Skeleton } from "@/shared/components/ui";
import { formatIDR } from "@/shared/lib/format";
import { cn } from "@/shared/lib/cn";
import { PosCategoryDrilldown } from "@/modules/pos/components/PosCategoryDrilldown";
import type { PosCartApi } from "@/modules/pos/hooks/use-pos-cart";
import type { PosCatalogApi } from "@/modules/pos/hooks/use-pos-catalog";
import type { PosProductListItem } from "@/modules/pos/types";

// Mulai memuat halaman berikutnya ±600 px sebelum dasar kisi terlihat.
const PRELOAD_MARGIN = "0px 0px 600px 0px";
const GRID = "grid grid-cols-[repeat(auto-fill,minmax(10rem,1fr))] gap-3";

function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return `${parts[0][0]}${parts[1][0]}`.toUpperCase();
}

/**
 * Foto produk di tile (paritas legacy "foto square lazy", temuan 4) dengan cadangan inisial:
 * tanpa foto, atau gambar gagal dimuat (mis. URL bertanda tangan kedaluwarsa pada katalog yang
 * lama terbuka). Ukuran kotak sama untuk keduanya agar kisi tidak bergeser.
 */
export function TileThumb({ name, src }: { name: string; src?: string | null }) {
  const [failed, setFailed] = useState(false);
  const box = "flex size-14 flex-none items-center justify-center overflow-hidden rounded-lg bg-muted";
  if (src && !failed) {
    return (
      <span aria-hidden className={box}>
        <img src={src} alt="" loading="lazy" decoding="async" className="size-full object-cover" onError={() => setFailed(true)} />
      </span>
    );
  }
  return (
    <span aria-hidden className={cn(box, "font-display text-sm font-semibold text-muted-foreground")}>
      {initials(name)}
    </span>
  );
}

// memo + onAdd stabil (bukan closure per tile): ubah keranjang hanya me-render tile yang berubah,
// bukan ratusan tile hasil infinite scroll.
const Tile = memo(function Tile({
  product,
  inCart,
  short,
  busy,
  onAdd,
}: {
  product: PosProductListItem;
  inCart: number;
  short: boolean;
  busy: boolean;
  onAdd: (productId: number) => unknown;
}) {
  return (
    <button
      type="button"
      aria-label={`Tambah ${product.name}`}
      disabled={busy}
      onClick={() => void onAdd(product.id)}
      className={cn(
        "relative flex min-h-34 touch-manipulation flex-col items-stretch gap-1.5 rounded-xl border border-border bg-card p-3 text-left shadow-[var(--shadow-card)] transition-[transform,background-color] duration-100 select-none active:scale-[0.98] active:bg-accent disabled:opacity-50",
        inCart > 0 && "border-primary ring-1 ring-primary",
        busy && "opacity-70",
      )}
    >
      <span className="flex items-start justify-between gap-2">
        <TileThumb key={product.primaryImageUrl ?? ""} name={product.name} src={product.primaryImageUrl} />
        {inCart > 0 && (
          <span className="tabular rounded-full bg-primary px-2 py-0.5 text-xs font-semibold text-primary-foreground">
            ×{inCart}
          </span>
        )}
      </span>
      <span className="line-clamp-2 text-sm leading-snug font-medium text-foreground">{product.name}</span>
      <span className="mt-auto flex items-end justify-between gap-2">
        <span className="tabular font-display text-lg font-semibold text-foreground">{formatIDR(product.sellingPrice)}</span>
      </span>
      <span className="flex items-center justify-between gap-2 text-2xs text-muted-foreground">
        <span className="truncate font-mono">{product.code}</span>
        {short && <span className="rounded-full bg-warning-soft px-1.5 py-0.5 font-medium text-warning">Stok kurang</span>}
      </span>
    </button>
  );
});

function statusText(catalog: PosCatalogApi): string {
  if (catalog.loadingMore) return "Memuat produk berikutnya…";
  if (catalog.page <= 1) return "";
  if (!catalog.hasMore) return "Semua produk ditampilkan";
  return `${catalog.lastPageCount} produk lagi dimuat`;
}

/** Panel kiri Mode Kasir: kategori, input pindai (QR label / barcode pabrik, wedge), dan kisi tile produk. */
export function ProductGrid({ pos, catalog }: { pos: PosCartApi; catalog: PosCatalogApi }) {
  const { items, loadingFirst, loadingMore, hasMore, error, loadMore, generation } = catalog;
  const qtyByProduct = new Map<number, number>();
  const shortProducts = new Set<number>();
  for (const l of pos.cart) {
    qtyByProduct.set(l.productId, (qtyByProduct.get(l.productId) ?? 0) + l.qty);
    const st = pos.holds.lineState[`${l.productId}:${l.variantId}`];
    if (st && (st.shortfall > 0 || st.error)) shortProducts.add(l.productId);
  }

  const scrollRef = useRef<HTMLDivElement | null>(null);
  const sentinelRef = useRef<HTMLDivElement | null>(null);

  // Kata kunci/kategori baru (halaman 1 diterapkan): kembali ke atas kisi.
  useEffect(() => {
    if (scrollRef.current) scrollRef.current.scrollTop = 0;
  }, [generation]);

  // Sentinel di bawah kisi mendekati layar -> muat halaman berikutnya. Observer dibuat ulang tiap
  // jumlah item berubah: callback awalnya memeriksa ulang posisi sentinel, sehingga layar besar
  // dengan halaman pendek terus memuat sampai penuh atau katalog habis.
  useEffect(() => {
    const root = scrollRef.current;
    const sentinel = sentinelRef.current;
    if (!root || !sentinel || !hasMore || error !== null || typeof IntersectionObserver === "undefined") return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) loadMore();
      },
      { root, rootMargin: PRELOAD_MARGIN },
    );
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [items.length, hasMore, error, generation, loadMore]);

  return (
    <section aria-label="Produk" className="flex min-h-0 min-w-0 flex-1 flex-col">
      <div className="flex flex-none items-center gap-3 border-b border-border bg-background px-4 py-3">
        <form
          className="relative w-48 flex-none xl:w-56"
          onSubmit={(e) => {
            e.preventDefault();
            void pos.submitBarcode();
          }}
        >
          <ScanQrCode aria-hidden className="pointer-events-none absolute top-1/2 left-3 size-5 -translate-y-1/2 text-muted-foreground" />
          <input
            id="pos-barcode"
            aria-label="Pindai QR / kode"
            value={pos.barcode}
            onChange={(e) => pos.setBarcode(e.target.value)}
            placeholder="Pindai QR / ketik kode"
            inputMode="none"
            enterKeyHint="done"
            autoComplete="off"
            className="h-11 w-full rounded-lg border border-input bg-card pr-3 pl-10 text-base text-foreground placeholder:text-muted-foreground/70 focus:border-ring focus:ring-[3px] focus:ring-ring/25 focus:outline-none"
          />
        </form>
        <PosCategoryDrilldown
          categories={catalog.categories}
          value={catalog.activeCategoryId}
          onChange={catalog.setActiveCategoryId}
        />
      </div>

      <div ref={scrollRef} className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-4">
        {pos.scanNotice && (
          <div className="mb-3">
            <Notice tone="warning" title={pos.scanNotice} />
          </div>
        )}
        {loadingFirst && items.length === 0 ? (
          <div role="status" aria-label="Memuat produk" className={GRID}>
            {Array.from({ length: 12 }, (_, i) => (
              <Skeleton key={i} className="h-34 rounded-xl" />
            ))}
          </div>
        ) : items.length === 0 ? (
          <EmptyState title="Tidak ada hasil" description="Coba kata kunci atau kategori lain, atau pindai QR produknya." />
        ) : (
          <>
            <div
              aria-busy={loadingFirst || loadingMore}
              className={cn(GRID, "transition-opacity", loadingFirst && "opacity-60")}
            >
              {items.map((p) => (
                <Tile
                  key={p.id}
                  product={p}
                  inCart={qtyByProduct.get(p.id) ?? 0}
                  short={shortProducts.has(p.id)}
                  busy={pos.addingId === p.id}
                  onAdd={pos.addProductById}
                />
              ))}
              {loadingMore &&
                Array.from({ length: 4 }, (_, i) => <Skeleton key={`more-${i}`} aria-hidden className="h-34 rounded-xl" />)}
            </div>
            <div ref={sentinelRef} aria-hidden className="h-px" />
            <div className="flex justify-center pt-4 pb-2">
              {error !== null ? (
                <Notice tone="warning" title="Produk berikutnya gagal dimuat" className="w-full max-w-md">
                  <p>{error}</p>
                  <Button variant="secondary" className="mt-2 h-11 px-5" onClick={catalog.retry}>
                    Coba lagi
                  </Button>
                </Notice>
              ) : hasMore ? (
                !loadingMore && (
                  <Button variant="secondary" className="h-11 px-5" disabled={loadingFirst} onClick={loadMore}>
                    Muat lagi
                  </Button>
                )
              ) : (
                !loadingFirst && (
                  <p className="text-sm text-muted-foreground">Semua produk ditampilkan ({items.length})</p>
                )
              )}
            </div>
          </>
        )}
        <p role="status" className="sr-only">
          {statusText(catalog)}
        </p>
      </div>
    </section>
  );
}
