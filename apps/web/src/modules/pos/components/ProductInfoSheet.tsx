import { useEffect, useState } from "react";
import { TriangleAlert } from "lucide-react";
import { Button, FormSheet, Skeleton } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { cn } from "@/shared/lib/cn";
import { formatIDR } from "@/shared/lib/format";
import { TileThumb } from "@/modules/pos/components/ProductGrid";
import { formatQty } from "@/modules/pos/lib/qty";
import { availableByLocation, availableByVariant, sellableLocationIds } from "@/modules/pos/lib/stock";
import { readApiError } from "@/modules/pos/lib/api-error";
import { posService } from "@/modules/pos/services/pos.service";
import { sellableVariants } from "@/modules/products/lib/variants";
import type { PosLocation, PosProductDetail, PosProductListItem, ScanBalance } from "@/modules/pos/types";

interface InfoData {
  detail: PosProductDetail;
  /** null = stok tidak bisa dibaca (izin/galat): harga tetap tampil. */
  balances: ScanBalance[] | null;
  locations: PosLocation[] | null;
}

/**
 * Info produk tanpa keranjang (cek harga & stok, terutama di HP): harga, harga minimum, stok
 * tersedia per lokasi dan per varian, kode, barcode. Hanya membaca — tidak menahan stok dan tidak
 * menyentuh keranjang; "Tambah ke keranjang" memakai jalur ketuk tile biasa.
 */
export function ProductInfoSheet({
  product,
  onClose,
  onAdd,
}: {
  /** Tile yang dibuka (nama/foto tampil seketika); null = tertutup. */
  product: PosProductListItem | null;
  onClose: () => void;
  onAdd: (productId: number) => void;
}) {
  return (
    <FormSheet
      open={product !== null}
      title="Info produk"
      onClose={onClose}
      actions={
        product && (
          <>
            <Button variant="secondary" className="h-12 flex-1 text-base" onClick={onClose}>
              Tutup
            </Button>
            <Button className="h-12 flex-1 text-base" onClick={() => onAdd(product.id)}>
              Tambah ke keranjang
            </Button>
          </>
        )
      }
    >
      {product && <InfoBody key={product.id} product={product} />}
    </FormSheet>
  );
}

function InfoBody({ product }: { product: PosProductListItem }) {
  const [data, setData] = useState<InfoData | null>(null);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    let live = true;
    Promise.all([
      posService.productDetail(product.id),
      posService.productBalances(product.id).catch(() => null),
      posService.locations().catch(() => null),
    ]).then(
      ([detail, balances, locations]) => live && setData({ detail, balances, locations }),
      (err) => live && setError(readApiError(err).message),
    );
    return () => {
      live = false;
    };
  }, [product.id]);

  if (error) return <Notice tone="danger" title="Info produk gagal dimuat">{error}</Notice>;
  return (
    <div className="space-y-5">
      <div className="flex items-start gap-3">
        <TileThumb name={product.name} src={product.primaryImageUrl} />
        <div className="min-w-0">
          <p className="text-base leading-snug font-semibold text-foreground">{product.name}</p>
          <p className="font-mono text-xs text-muted-foreground">{product.code}</p>
        </div>
      </div>
      {data ? <InfoFacts data={data} low={product.belowMinStock === true} /> : <InfoSkeleton />}
    </div>
  );
}

function InfoSkeleton() {
  return (
    <div role="status" aria-label="Memuat info produk" className="space-y-3">
      <Skeleton className="h-16 rounded-lg" />
      <Skeleton className="h-24 rounded-lg" />
    </div>
  );
}

function Row({ label, value, strong = false }: { label: string; value: string; strong?: boolean }) {
  return (
    <div className="flex items-baseline justify-between gap-3 py-2">
      <dt className="text-sm text-muted-foreground">{label}</dt>
      <dd className={cn("tabular text-right", strong ? "text-base font-semibold text-foreground" : "text-sm text-foreground")}>{value}</dd>
    </div>
  );
}

function InfoFacts({ data, low }: { data: InfoData; /** Status tile (≤ stok minimum). */ low: boolean }) {
  const { detail, balances, locations } = data;
  const factor = detail.salesFactor && detail.salesFactor > 0 ? detail.salesFactor : 1;
  const uom = detail.salesUom || detail.baseUom;
  const sellable = sellableLocationIds(locations);
  const names = new Map((locations ?? []).map((l) => [l.id, l.name]));
  const variants = sellableVariants(detail.variants);
  const tracked = detail.tracked !== false;
  const perVariant = balances ? availableByVariant(balances, sellable, factor) : null;
  const total = perVariant ? [...perVariant.values()].reduce((a, b) => a + b, 0) : null;
  const perLocation = balances ? availableByLocation(balances, sellable, factor) : [];
  const barcodes = variants.map((v) => v.barcode).filter((b) => b && b.trim() !== "");
  const qty = (n: number) => `${formatQty(Math.max(0, n))} ${uom}`;

  return (
    <>
      <dl className="divide-y divide-border rounded-lg border border-border px-4">
        <Row label={`Harga per ${uom}`} value={formatIDR(detail.sellingPrice)} strong />
        {detail.minSellingPrice > 0 && (
          <Row label="Harga minimum" value={`${formatIDR(Math.round(detail.minSellingPrice * factor))} / ${uom}`} />
        )}
        {barcodes.length === 1 && <Row label="Barcode" value={barcodes[0]} />}
      </dl>

      <section aria-label="Stok tersedia" className="space-y-2">
        <div className="flex items-baseline justify-between gap-3">
          <h3 className="text-sm font-semibold text-foreground">Stok tersedia</h3>
          {tracked && total !== null && (
            <p className="flex items-center gap-2">
              {total > 0 && low && (
                <span className="flex items-center gap-1 rounded-full bg-warning-soft px-2 py-0.5 text-xs font-medium text-warning">
                  <TriangleAlert aria-hidden className="size-3.5" />
                  Menipis
                </span>
              )}
              <span className={cn("tabular font-display text-2xl font-semibold", total <= 0 ? "text-destructive" : "text-foreground")}>
                {total <= 0 ? "Habis" : qty(total)}
              </span>
            </p>
          )}
        </div>
        {!tracked ? (
          <p className="text-sm text-muted-foreground">Produk ini tidak dilacak stoknya (jasa/non-stok).</p>
        ) : total === null ? (
          <p className="text-sm text-muted-foreground">Stok tidak dapat dibaca untuk akun ini.</p>
        ) : (
          <>
            <p className="text-xs text-muted-foreground">Di luar stok yang sedang ditahan keranjang lain dan lokasi rusak.</p>
            {perLocation.length > 0 && (
              <dl className="divide-y divide-border rounded-lg border border-border px-4">
                {perLocation.map((r) => (
                  <Row key={r.locationId} label={names.get(r.locationId) ?? `Lokasi #${r.locationId}`} value={qty(r.qty)} />
                ))}
              </dl>
            )}
          </>
        )}
      </section>

      {variants.length > 1 && (
        <section aria-label="Varian" className="space-y-2">
          <h3 className="text-sm font-semibold text-foreground">{variants.length} varian</h3>
          <ul className="divide-y divide-border rounded-lg border border-border">
            {variants.map((v) => {
              const avail = perVariant?.get(v.id) ?? 0;
              return (
                <li key={v.id} className="flex items-center justify-between gap-3 px-4 py-2">
                  <span className="min-w-0">
                    <span className="block truncate text-sm text-foreground">{v.name || v.code}</span>
                    <span className="block truncate font-mono text-2xs text-muted-foreground">
                      {v.code}
                      {v.barcode && v.barcode !== v.code ? ` · ${v.barcode}` : ""}
                    </span>
                  </span>
                  {tracked && perVariant && (
                    <span className={cn("tabular flex-none text-sm", avail <= 0 ? "text-destructive" : "text-foreground")}>
                      {avail <= 0 ? "Habis" : qty(avail)}
                    </span>
                  )}
                </li>
              );
            })}
          </ul>
        </section>
      )}
    </>
  );
}
