// Qty keranjang POS (temuan 7): desimal > 0 diizinkan untuk semua produk —
// paritas legacy (quantityStep 0,0001; kabel per meter, besi, pasir). Presisi
// 4 desimal; masukan menerima koma (id-ID) maupun titik.

/** Jumlah desimal maksimum qty (legacy: langkah 0,0001). */
export const QTY_DECIMALS = 4;
const SCALE = 10 ** QTY_DECIMALS;

/** Bulatkan ke 4 desimal (membuang galat floating point, mis. 0,8 + 1). Null bila bukan angka > 0. */
export function normalizeQty(n: number): number | null {
  if (!Number.isFinite(n)) return null;
  const q = Math.round(n * SCALE) / SCALE;
  return q > 0 ? q : null;
}

/** "0,8" / "0.8" / "12" → angka; null bila tidak valid, ≤ 0, atau lebih dari 4 desimal. */
export function parseQtyInput(raw: string): number | null {
  const s = raw.trim().replace(",", ".");
  if (!/^\d+(\.\d{1,4})?$/.test(s) && !/^\d+\.$/.test(s)) return null;
  return normalizeQty(Number(s));
}

/** Draf Numpad (desimal selalu "."): tolak digit desimal ke-5. */
export function acceptQtyDraft(next: string): boolean {
  const dot = next.indexOf(".");
  return dot < 0 || next.length - dot - 1 <= QTY_DECIMALS;
}

const qtyFormat = new Intl.NumberFormat("id-ID", { maximumFractionDigits: QTY_DECIMALS });

/** Qty tampil, mis. 0,8 / 1,25 / 12. */
export function formatQty(n: number): string {
  return qtyFormat.format(n);
}

/** Tombol kurang (langkah 1) hanya aktif bila hasilnya tetap > 0. */
export function canDecrease(qty: number): boolean {
  return normalizeQty(qty - 1) !== null;
}
