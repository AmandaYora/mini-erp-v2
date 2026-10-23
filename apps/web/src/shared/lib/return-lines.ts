// Util murni untuk form retur (jual & beli) — domain-agnostik. Dipakai
// ReturnLinesTable + kedua modal buat retur (F3a "komponen retur parametrik").
// Server tetap final; ini hanya cermin agar pesan muncul cepat.

export const RETURN_REASON_MAX = 255;

/** Baris minimal yang dikenali util ini (qty berupa string input). */
export interface ReturnLineLike {
  qty: string;
  remaining: number;
}

/** Batasi qty input ke [0, max]; kosong/tak-numerik dikembalikan apa adanya. */
export function clampQty(raw: string, max: number): string {
  if (raw.trim() === "") return "";
  const n = Number(raw);
  if (Number.isNaN(n)) return raw;
  if (n < 0) return "0";
  if (n > max) return String(max);
  return raw;
}

/** Qty numerik > 0 atau 0 bila kosong/tak valid. */
export function parseQty(raw: string): number {
  const n = Number(raw);
  return Number.isFinite(n) && n > 0 ? n : 0;
}

/** Baris yang ikut dikirim (qty > 0). */
export function activeLines<T extends ReturnLineLike>(lines: T[]): T[] {
  return lines.filter((l) => parseQty(l.qty) > 0);
}

/** Alasan wajib non-kosong (setelah trim) dan <= RETURN_REASON_MAX. Null = valid. */
export function validateReason(reason: string, max: number = RETURN_REASON_MAX): string | null {
  const t = reason.trim();
  if (t === "") return "Alasan retur wajib diisi.";
  if (t.length > max) return `Alasan maksimal ${max} karakter.`;
  return null;
}

/** Tanggal retur kosong (= hari ini di server) atau YYYY-MM-DD yang tidak di masa depan. */
export function validateReturnDate(date: string, today: string, earliest?: string): string | null {
  const t = date.trim();
  if (t === "") return null;
  if (!/^\d{4}-\d{2}-\d{2}$/.test(t)) return "Format tanggal harus YYYY-MM-DD.";
  if (t > today) return "Tanggal retur tidak boleh di masa depan.";
  // RT-04: tidak boleh sebelum barang diterima.
  const first = (earliest ?? "").slice(0, 10);
  if (first !== "" && t < first) return `Tanggal retur tidak boleh lebih awal dari tanggal penerimaan (${first}).`;
  return null;
}

/** Satuan yang boleh dipakai baris retur, dengan faktornya ke satuan dasar. */
export interface ReturnUom {
  uom: string;
  factor: number;
}

/** Baris retur yang bisa berganti satuan (sisa sejati disimpan dalam satuan dasar). */
export interface UomSwitchable {
  qty: string;
  uom: string;
  uomFactor?: number;
  remaining: number;
  remainingBase?: number;
  uoms?: ReturnUom[];
}

/** Sisa dalam satuan terpilih: sisa dasar / faktor (dibulatkan 6 desimal). Tanpa data dasar: sisa baris. */
export function remainingInUnit(row: Pick<UomSwitchable, "remaining" | "remainingBase" | "uomFactor">): number {
  if (row.remainingBase === undefined || !row.uomFactor || row.uomFactor <= 0) return row.remaining;
  return Math.round((row.remainingBase / row.uomFactor) * 1e6) / 1e6;
}

/** Ganti satuan baris: qty dikonversi lewat satuan dasar, lalu dibatasi sisa dalam satuan baru. */
export function switchUom<T extends UomSwitchable>(row: T, uom: string): Partial<T> {
  const next = row.uoms?.find((u) => u.uom === uom);
  if (!next || next.factor <= 0) return {};
  const patch = { uom, uomFactor: next.factor } as Partial<T>;
  const q = parseQty(row.qty);
  if (q > 0 && row.uomFactor && row.uomFactor > 0) {
    const converted = Math.round(((q * row.uomFactor) / next.factor) * 1e6) / 1e6;
    patch.qty = clampQty(String(converted), remainingInUnit({ ...row, uomFactor: next.factor }));
  }
  return patch;
}
