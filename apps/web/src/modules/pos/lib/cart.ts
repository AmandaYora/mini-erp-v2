// Matematika keranjang POS sisi klien (estimasi; total resmi dari server).
// Diekstrak dari PosPage agar bisa diuji (logika identik).
export interface CartLineEstimate {
  qty: number;
  unitPrice: number;
}

/** Estimasi total = Σ qty × harga katalog. */
export function cartEstimate(lines: CartLineEstimate[]): number {
  return lines.reduce((sum, l) => sum + lineTotal(l), 0);
}

/** Estimasi satu baris. */
export function lineTotal(line: CartLineEstimate): number {
  return line.qty * line.unitPrice;
}

/** Kembalian = tunai − total (negatif = kurang bayar). */
export function cashChange(tendered: number, total: number): number {
  return tendered - total;
}
