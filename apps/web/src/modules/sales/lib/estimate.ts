// Estimasi klien per baris SO (net tanpa pajak — pajak dihitung server).
// Dipindah dari SalesOrderFormPage agar bisa diuji (logika identik).
export interface SalesLineEstimate {
  qty: number;
  catalogPrice: number;
  /** Harga manual (rupiah/satuan). null/undefined = pakai catalogPrice. */
  manualPrice?: number | null;
  discountPct: number;
  discountNominal: number;
}

/** Harga satuan efektif untuk estimasi: manual bila diisi, selain itu katalog. */
export function effectiveUnitPrice(row: SalesLineEstimate): number {
  return row.manualPrice != null ? row.manualPrice : row.catalogPrice;
}

export function estimateLine(row: SalesLineEstimate): number {
  const gross = row.qty * effectiveUnitPrice(row);
  return Math.max(0, Math.round(gross - (gross * row.discountPct) / 100 - row.discountNominal));
}

export function estimateTotal(rows: SalesLineEstimate[]): number {
  return rows.reduce((sum, r) => sum + estimateLine(r), 0);
}

export interface MinPriceCheck extends SalesLineEstimate {
  /** Harga jual minimum produk per satuan; 0/undefined = tanpa minimum. */
  minPrice?: number;
  /** Harga member menang dan tidak dijaga minimum (aturan server). */
  memberPrice?: boolean;
}

/**
 * Cermin klem harga minimum server (sales/application priceLines): net baris
 * setelah diskon dibandingkan dengan minimum x qty. Hanya BANTUAN tampilan —
 * server tetap menolak dan pesannya (per baris) ditampilkan apa adanya.
 */
export function belowMinPrice(row: MinPriceCheck): boolean {
  const min = row.minPrice ?? 0;
  if (min <= 0 || row.memberPrice === true) return false;
  const gross = Math.round(row.qty * effectiveUnitPrice(row));
  const net = gross - Math.round((gross * row.discountPct) / 100) - row.discountNominal;
  return net < Math.round(min * row.qty);
}
