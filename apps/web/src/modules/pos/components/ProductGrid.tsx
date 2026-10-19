import { Barcode } from "lucide-react";
import { EmptyState } from "@/shared/components/feedback/empty-state";
import { Notice } from "@/shared/components/feedback/notice";
import { Skeleton } from "@/shared/components/ui";
import { formatIDR } from "@/shared/lib/format";
import { cn } from "@/shared/lib/cn";
import { PosCategoryDrilldown } from "@/modules/pos/components/PosCategoryDrilldown";
import type { PosCartApi } from "@/modules/pos/hooks/use-pos-cart";
import type { PosCatalogApi } from "@/modules/pos/hooks/use-pos-catalog";
import type { PosProductOption } from "@/modules/pos/types";

function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return `${parts[0][0]}${parts[1][0]}`.toUpperCase();
}

function Tile({
  product,
  inCart,
  short,
  busy,
  disabled,
  onAdd,
}: {
  product: PosProductOption;
  inCart: number;
  short: boolean;
  busy: boolean;
  disabled: boolean;
  onAdd: () => void;
}) {
  return (
    <button
      type="button"
      aria-label={`Tambah ${product.name}`}
      disabled={disabled || busy}
      onClick={onAdd}
      className={cn(
        "relative flex min-h-34 touch-manipulation flex-col items-stretch gap-1.5 rounded-xl border border-border bg-card p-3 text-left shadow-[var(--shadow-card)] transition-[transform,background-color] duration-100 select-none active:scale-[0.98] active:bg-accent disabled:opacity-50",
        inCart > 0 && "border-primary ring-1 ring-primary",
        busy && "opacity-70",
      )}
    >
      <span className="flex items-start justify-between gap-2">
        <span
          aria-hidden
          className="flex size-10 flex-none items-center justify-center rounded-lg bg-muted font-display text-sm font-semibold text-muted-foreground"
        >
          {initials(product.name)}
        </span>
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
}

/** Panel kiri Mode Kasir: kategori, input barcode (wedge), dan kisi tile produk. */
export function ProductGrid({ pos, catalog }: { pos: PosCartApi; catalog: PosCatalogApi }) {
  const { productOptions, searching } = catalog;
  const qtyByProduct = new Map<number, number>();
  const shortProducts = new Set<number>();
  for (const l of pos.cart) {
    qtyByProduct.set(l.productId, (qtyByProduct.get(l.productId) ?? 0) + l.qty);
    const st = pos.holds.lineState[`${l.productId}:${l.variantId}`];
    if (st && (st.shortfall > 0 || st.error)) shortProducts.add(l.productId);
  }

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
          <Barcode aria-hidden className="pointer-events-none absolute top-1/2 left-3 size-5 -translate-y-1/2 text-muted-foreground" />
          <input
            id="pos-barcode"
            aria-label="Barcode"
            value={pos.barcode}
            onChange={(e) => pos.setBarcode(e.target.value)}
            placeholder="Scanner / ketik kode"
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

      <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-4">
        {pos.scanNotice && (
          <div className="mb-3">
            <Notice tone="warning" title={pos.scanNotice} />
          </div>
        )}
        {searching && productOptions.length === 0 ? (
          <div role="status" aria-label="Memuat produk" className="grid grid-cols-[repeat(auto-fill,minmax(10rem,1fr))] gap-3">
            {Array.from({ length: 12 }, (_, i) => (
              <Skeleton key={i} className="h-34 rounded-xl" />
            ))}
          </div>
        ) : productOptions.length === 0 ? (
          <EmptyState title="Tidak ada hasil" description="Coba kata kunci atau kategori lain, atau pindai barcode produknya." />
        ) : (
          <div
            className={cn(
              "grid grid-cols-[repeat(auto-fill,minmax(10rem,1fr))] gap-3 transition-opacity",
              searching && "opacity-60",
            )}
          >
            {productOptions.map((p) => (
              <Tile
                key={p.id}
                product={p}
                inCart={qtyByProduct.get(p.id) ?? 0}
                short={shortProducts.has(p.id)}
                busy={pos.addingId === p.id}
                disabled={false}
                onAdd={() => void pos.addProductById(p.id)}
              />
            ))}
          </div>
        )}
      </div>
    </section>
  );
}
