// Stok tersedia di POS (temuan 5 lanjutan: cek harga & stok di HP tanpa keranjang). Satu aturan
// untuk tile, pemilih varian, dan lembar info — sama dengan katalog server (GET /products
// `stockAvailable`): Σ (on_hand − reserved) di lokasi aktif cabang kecuali lokasi rusak, dalam
// satuan jual (dasar ÷ faktor jual).
import type { PosLocation, PosProductListItem, ScanBalance } from "@/modules/pos/types";

export type TileStock =
  | { kind: "none" } // tidak dilacak stok, atau tanpa izin lihat stok
  | { kind: "out"; uom: string }
  | { kind: "low"; qty: number; uom: string }
  | { kind: "ok"; qty: number; uom: string };

/** Status stok tile katalog dari field server (`stockAvailable` sudah dalam satuan jual). */
export function tileStock(p: Pick<PosProductListItem, "tracked" | "stockAvailable" | "stockUom" | "belowMinStock">): TileStock {
  if (p.tracked === false || p.stockAvailable === undefined || p.stockAvailable === null) return { kind: "none" };
  const uom = p.stockUom ?? "";
  if (p.stockAvailable <= 0) return { kind: "out", uom };
  if (p.belowMinStock) return { kind: "low", qty: p.stockAvailable, uom };
  return { kind: "ok", qty: p.stockAvailable, uom };
}

/** Lokasi tempat stok bisa dijual: aktif dan bukan lokasi rusak. Daftar kosong/tak terbaca = null
 * (tidak diketahui → semua posisi dihitung, bukan nol). */
export function sellableLocationIds(locations: readonly PosLocation[] | null | undefined): Set<number> | null {
  if (!locations || locations.length === 0) return null;
  return new Set(locations.filter((l) => l.status === "active" && l.kind !== "damaged").map((l) => l.id));
}

function toSalesUnit(base: number, salesFactor: number | undefined): number {
  const f = salesFactor && salesFactor > 0 ? salesFactor : 1;
  return Math.round((base / f) * 10_000) / 10_000;
}

/** Tersedia per varian (satuan jual) di lokasi jual; lokasi tak dikenal (sellable null) = semua. */
export function availableByVariant(
  balances: readonly ScanBalance[],
  sellable: ReadonlySet<number> | null,
  salesFactor?: number,
): Map<number, number> {
  const base = new Map<number, number>();
  for (const b of balances) {
    if (sellable && !sellable.has(b.locationId)) continue;
    base.set(b.variantId, (base.get(b.variantId) ?? 0) + b.available);
  }
  const out = new Map<number, number>();
  for (const [v, q] of base) out.set(v, toSalesUnit(q, salesFactor));
  return out;
}

/** Tersedia per lokasi jual (satuan jual), hanya yang tidak nol, urut terbanyak. */
export function availableByLocation(
  balances: readonly ScanBalance[],
  sellable: ReadonlySet<number> | null,
  salesFactor?: number,
): { locationId: number; qty: number }[] {
  const base = new Map<number, number>();
  for (const b of balances) {
    if (sellable && !sellable.has(b.locationId)) continue;
    base.set(b.locationId, (base.get(b.locationId) ?? 0) + b.available);
  }
  return [...base]
    .map(([locationId, q]) => ({ locationId, qty: toSalesUnit(q, salesFactor) }))
    .filter((r) => r.qty !== 0)
    .sort((a, b) => b.qty - a.qty);
}
