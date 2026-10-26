import { memo, useEffect, useRef, useState, type PointerEvent } from "react";
import { CircleSlash, Image as ImageIcon, Info, LayoutGrid, ScanQrCode, TriangleAlert } from "lucide-react";
import { EmptyState } from "@/shared/components/feedback/empty-state";
import { Notice } from "@/shared/components/feedback/notice";
import { Button, Skeleton, ToggleGroup, ToggleGroupItem } from "@/shared/components/ui";
import { formatIDR } from "@/shared/lib/format";
import { cn } from "@/shared/lib/cn";
import { PosCategoryDrilldown } from "@/modules/pos/components/PosCategoryDrilldown";
import type { PosCartApi } from "@/modules/pos/hooks/use-pos-cart";
import type { PosCatalogApi } from "@/modules/pos/hooks/use-pos-catalog";
import { formatQty } from "@/modules/pos/lib/qty";
import { tileStock } from "@/modules/pos/lib/stock";
import { useCatalogView, type CatalogView } from "@/modules/pos/lib/catalog-view";
import { sellableVariants } from "@/modules/products/lib/variants";
import type { PosProductListItem } from "@/modules/pos/types";

// Mulai memuat halaman berikutnya ±600 px sebelum dasar kisi terlihat.
const PRELOAD_MARGIN = "0px 0px 600px 0px";
// Foto besar: HP 2 kolom ±170 px; tablet (lg) 3–4 kolom ±200 px — foto 4:3 ±200×150 px (analysis/003).
// Ringkas: kolom ±160 px, foto kecil, lebih banyak barang per layar.
const GRID: Record<CatalogView, string> = {
  photo: "grid grid-cols-[repeat(auto-fill,minmax(10rem,1fr))] gap-3 lg:grid-cols-[repeat(auto-fill,minmax(11rem,1fr))]",
  compact: "grid grid-cols-[repeat(auto-fill,minmax(10rem,1fr))] gap-3",
};

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

/**
 * Foto utama kartu katalog (analysis/003): kotak 4:3 selebar kartu, `object-cover`. Ruang dipesan lebih
 * dulu sehingga kisi tidak bergeser saat foto dimuat atau gagal; tanpa foto / gagal dimuat = inisial besar di
 * kotak yang sama. Stok habis meredupkan foto (chip "Stok habis" tetap ada di baris info — bukan warna saja);
 * inisial tidak diredupkan agar kontras teksnya tetap memenuhi WCAG AA (V01).
 */
export function TileImage({ name, src, dimmed = false }: { name: string; src?: string | null; dimmed?: boolean }) {
  const [failed, setFailed] = useState(false);
  const box = "flex aspect-[4/3] w-full flex-none items-center justify-center overflow-hidden border-b border-border bg-muted";
  if (src && !failed) {
    return (
      <span aria-hidden className={box}>
        <img
          src={src}
          alt=""
          loading="lazy"
          decoding="async"
          className={cn("size-full object-cover", dimmed && "opacity-45 grayscale")}
          onError={() => setFailed(true)}
        />
      </span>
    );
  }
  return (
    <span
      aria-hidden
      className={cn(box, "font-display text-3xl font-semibold tracking-tight text-muted-foreground")}
    >
      {initials(name)}
    </span>
  );
}

/** Kerangka kartu saat memuat: kotak foto 4:3 + baris teks, setinggi kartu asli. */
function TileSkeleton({ view }: { view: CatalogView }) {
  if (view === "compact") return <Skeleton aria-hidden className="h-34 rounded-xl" />;
  return (
    <div aria-hidden className="overflow-hidden rounded-xl border border-border bg-card">
      <Skeleton className="aspect-[4/3] w-full rounded-none" />
      <div className="space-y-2 p-3">
        <Skeleton className="h-4 w-4/5" />
        <Skeleton className="h-4 w-1/2" />
        <Skeleton className="h-5 w-2/5" />
      </div>
    </div>
  );
}

/** Stok tersedia di tile: angka + satuan; menipis/habis juga bertanda ikon dan kata (bukan warna saja). */
function StockChip({ product, short }: { product: PosProductListItem; short: boolean }) {
  // Stok di keranjang ini tidak cukup (tahanan gagal): lebih mendesak dari angka katalog.
  if (short) return <span className="flex-none rounded-full bg-warning-soft px-1.5 py-0.5 font-medium text-warning">Stok kurang</span>;
  const st = tileStock(product);
  const unit = (uom: string) => (uom ? ` ${uom}` : "");
  switch (st.kind) {
    case "none":
      return null;
    case "out":
      return (
        <span className="flex flex-none items-center gap-1 rounded-full bg-destructive-soft px-1.5 py-0.5 font-medium text-destructive">
          <CircleSlash aria-hidden className="size-3" />
          Stok habis
        </span>
      );
    case "low":
      // Ringkas agar kode produk tetap terbaca di tile 160 px; ikon + kata tersembunyi = bukan warna saja.
      return (
        <span
          title={`Stok menipis: ${formatQty(st.qty)}${unit(st.uom)}`}
          className="tabular flex flex-none items-center gap-1 rounded-full bg-warning-soft px-1.5 py-0.5 font-medium text-warning"
        >
          <TriangleAlert aria-hidden className="size-3" />
          <span className="sr-only">Stok menipis: </span>
          {formatQty(st.qty)}
          {unit(st.uom)}
        </span>
      );
    case "ok":
      return (
        <span className="tabular flex-none font-medium text-foreground">
          Stok {formatQty(st.qty)}
          {unit(st.uom)}
        </span>
      );
  }
}

/** Tekan-lama (≥ 500 ms) atau klik kanan membuka info produk; ketuk biasa tetap menambah ke keranjang. */
const LONG_PRESS_MS = 500;

// memo + onAdd/onInfo stabil (bukan closure per tile): ubah keranjang hanya me-render tile yang
// berubah, bukan ratusan tile hasil infinite scroll.
const Tile = memo(function Tile({
  product,
  inCart,
  short,
  busy,
  compact,
  onAdd,
  onInfo,
}: {
  product: PosProductListItem;
  inCart: number;
  short: boolean;
  busy: boolean;
  /** Tampilan ringkas: foto kecil di pojok, kartu lebih pendek. */
  compact: boolean;
  onAdd: (productId: number) => unknown;
  onInfo?: (productId: number) => void;
}) {
  const timer = useRef<number | undefined>(undefined);
  const origin = useRef<{ x: number; y: number } | null>(null);
  // Tekan-lama sudah membuka info: klik yang menyusul tidak menambah ke keranjang.
  const consumed = useRef(false);
  const cancel = () => {
    window.clearTimeout(timer.current);
    timer.current = undefined;
  };
  const openInfo = () => {
    cancel();
    consumed.current = true;
    onInfo?.(product.id);
  };
  const variantCount = product.variants ? sellableVariants(product.variants).length : 0;
  const soldOut = tileStock(product).kind === "out";
  const pressHandlers = onInfo
    ? {
        onPointerDown: (e: PointerEvent) => {
          consumed.current = false;
          if (e.pointerType === "mouse" && e.button !== 0) return;
          origin.current = { x: e.clientX, y: e.clientY };
          cancel();
          timer.current = window.setTimeout(openInfo, LONG_PRESS_MS);
        },
        onPointerMove: (e: PointerEvent) => {
          const o = origin.current;
          if (o && Math.hypot(e.clientX - o.x, e.clientY - o.y) > 10) cancel(); // menggulir, bukan menekan
        },
        onPointerUp: cancel,
        onPointerLeave: cancel,
        onPointerCancel: cancel,
        onContextMenu: (e: { preventDefault: () => void }) => {
          e.preventDefault();
          if (!consumed.current) openInfo();
        },
      }
    : {};
  return (
    <div className="relative">
      <button
        type="button"
        aria-label={`Tambah ${product.name}`}
        disabled={busy}
        onClick={() => {
          if (consumed.current) {
            consumed.current = false;
            return;
          }
          void onAdd(product.id);
        }}
        {...pressHandlers}
        style={{ WebkitTouchCallout: "none" }}
        className={cn(
          "relative flex h-full w-full touch-manipulation flex-col items-stretch overflow-hidden rounded-xl border border-border bg-card text-left shadow-[var(--shadow-card)] transition-[transform,background-color] duration-100 select-none active:scale-[0.98] active:bg-accent disabled:opacity-50",
          compact && "min-h-34",
          inCart > 0 && "border-primary ring-1 ring-primary",
          busy && "opacity-70",
        )}
      >
        {compact ? (
          <span className="flex items-start justify-between gap-2 px-3 pt-3">
            <TileThumb key={product.primaryImageUrl ?? ""} name={product.name} src={product.primaryImageUrl} />
            {inCart > 0 && (
              <span className="tabular rounded-full bg-primary px-2 py-0.5 text-xs font-semibold text-primary-foreground max-lg:mr-10">
                ×{formatQty(inCart)}
              </span>
            )}
          </span>
        ) : (
          <>
            <TileImage key={product.primaryImageUrl ?? ""} name={product.name} src={product.primaryImageUrl} dimmed={soldOut} />
            {inCart > 0 && (
              <span className="tabular absolute top-2 left-2 rounded-full bg-primary px-2.5 py-0.5 text-sm font-semibold text-primary-foreground shadow-[var(--shadow-card)]">
                ×{formatQty(inCart)}
              </span>
            )}
          </>
        )}
        <span className={cn("flex flex-1 flex-col gap-1.5 p-3", compact && "pt-1.5")}>
          {/* Selalu dua baris (2 × text-sm leading-snug ≈ 40 px): harga di satu baris kisi tetap sejajar. */}
          <span className="line-clamp-2 min-h-10 text-sm leading-snug font-medium text-foreground">{product.name}</span>
          <span className="mt-auto flex items-end justify-between gap-2">
            <span className="tabular font-display text-lg font-semibold text-foreground">{formatIDR(product.sellingPrice)}</span>
            {variantCount > 1 && <span className="pb-0.5 text-xs text-muted-foreground">{variantCount} varian</span>}
          </span>
          <span className="flex items-center justify-between gap-2 text-2xs text-muted-foreground">
            <span className="min-w-0 truncate font-mono">{product.code}</span>
            <StockChip product={product} short={short} />
          </span>
        </span>
      </button>
      {onInfo && (
        // HP: tombol info terlihat (tekan-lama sulit ditemukan); tablet memakai tekan-lama/klik kanan.
        <button
          type="button"
          aria-label={`Info ${product.name}`}
          onClick={() => onInfo(product.id)}
          className="absolute top-1 right-1 flex size-11 touch-manipulation items-center justify-center select-none lg:hidden"
        >
          {/* Lingkaran berlatar kartu agar terbaca di atas foto; area sentuh tetap 44 px. */}
          <span className="flex size-8 items-center justify-center rounded-full bg-card/90 text-foreground shadow-[var(--shadow-card)] active:bg-accent">
            <Info aria-hidden className="size-4.5" />
          </span>
        </button>
      )}
    </div>
  );
});

function statusText(catalog: PosCatalogApi): string {
  if (catalog.loadingMore) return "Memuat produk berikutnya…";
  if (catalog.page <= 1) return "";
  if (!catalog.hasMore) return "Semua produk ditampilkan";
  return `${catalog.lastPageCount} produk lagi dimuat`;
}

/** Panel kiri Mode Kasir: kategori, input pindai (QR label / barcode pabrik, wedge), dan kisi tile produk. */
export function ProductGrid({
  pos,
  catalog,
  onInfo,
}: {
  pos: PosCartApi;
  catalog: PosCatalogApi;
  /** Buka info produk (harga + stok) tanpa menambah ke keranjang. */
  onInfo?: (productId: number) => void;
}) {
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
  const [view, setView] = useCatalogView();

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
          className="relative w-48 flex-none max-lg:hidden xl:w-56"
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
        <ToggleGroup
          type="single"
          variant="outline"
          aria-label="Tampilan katalog"
          value={view}
          onValueChange={(v) => {
            if (v === "photo" || v === "compact") setView(v);
          }}
          className="flex-none"
        >
          <ToggleGroupItem value="photo" aria-label="Foto besar" title="Foto besar" className="h-11 min-w-11">
            <ImageIcon aria-hidden className="size-5" />
          </ToggleGroupItem>
          <ToggleGroupItem value="compact" aria-label="Ringkas" title="Ringkas" className="h-11 min-w-11">
            <LayoutGrid aria-hidden className="size-5" />
          </ToggleGroupItem>
        </ToggleGroup>
      </div>

      <div ref={scrollRef} className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-4 max-lg:p-3">
        {pos.scanNotice && (
          <div className="mb-3">
            <Notice tone="warning" title={pos.scanNotice} />
          </div>
        )}
        {loadingFirst && items.length === 0 ? (
          <div role="status" aria-label="Memuat produk" className={GRID[view]}>
            {Array.from({ length: 12 }, (_, i) => (
              <TileSkeleton key={i} view={view} />
            ))}
          </div>
        ) : items.length === 0 ? (
          <EmptyState title="Tidak ada hasil" description="Coba kata kunci atau kategori lain, atau pindai QR produknya." />
        ) : (
          <>
            <div
              aria-busy={loadingFirst || loadingMore}
              className={cn(GRID[view], "transition-opacity", loadingFirst && "opacity-60")}
            >
              {items.map((p) => (
                <Tile
                  key={p.id}
                  product={p}
                  inCart={qtyByProduct.get(p.id) ?? 0}
                  short={shortProducts.has(p.id)}
                  busy={pos.addingId === p.id}
                  compact={view === "compact"}
                  onAdd={pos.addProductById}
                  onInfo={onInfo}
                />
              ))}
              {loadingMore &&
                Array.from({ length: 4 }, (_, i) => <TileSkeleton key={`more-${i}`} view={view} />)}
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
