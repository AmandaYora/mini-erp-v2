import type { StockBalance } from "@/modules/stock/types";

export interface BalanceFilter {
  locationId: number | null;
  negativeOnly: boolean;
}

/** Saring saldo satu produk (mode per-produk tak punya filter server). */
export function filterBalances(
  rows: StockBalance[],
  f: BalanceFilter,
): StockBalance[] {
  return rows.filter(
    (b) =>
      (f.locationId === null || b.locationId === f.locationId) &&
      (!f.negativeOnly || b.onHand < 0 || b.available < 0),
  );
}

/** Id produk unik berurutan — untuk memuat nama item halaman saldo. */
export function distinctProductIds(rows: StockBalance[]): number[] {
  return [...new Set(rows.map((b) => b.productId))];
}
