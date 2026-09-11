// Estimasi klien per baris SO (net tanpa pajak — pajak dihitung server).
// Dipindah dari SalesOrderFormPage agar bisa diuji (logika identik).
export interface SalesLineEstimate {
  qty: number;
  catalogPrice: number;
  discountPct: number;
  discountNominal: number;
}

export function estimateLine(row: SalesLineEstimate): number {
  const gross = row.qty * row.catalogPrice;
  return Math.max(0, Math.round(gross - (gross * row.discountPct) / 100 - row.discountNominal));
}

export function estimateTotal(rows: SalesLineEstimate[]): number {
  return rows.reduce((sum, r) => sum + estimateLine(r), 0);
}
