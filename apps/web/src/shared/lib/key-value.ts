// Editor "Info Tambahan" (peta kunci → nilai bebas). Domain-agnostik: batas
// dilewatkan pemanggil (produk 30 kunci, party 20 kunci; kunci 1-60, nilai
// 1-255 — cerminan cleanAttrs/cleanAttributes backend).

export interface KeyValueRow {
  key: string;
  value: string;
}

export interface KeyValueLimits {
  maxRows: number;
  maxKey: number;
  maxValue: number;
}

export const PARTY_ATTR_LIMITS: KeyValueLimits = { maxRows: 20, maxKey: 60, maxValue: 255 };
export const PRODUCT_ATTR_LIMITS: KeyValueLimits = { maxRows: 30, maxKey: 60, maxValue: 255 };

/** Peta dari server → baris editor (urut kunci supaya tampilan stabil). */
export function attrsToRows(attrs: Record<string, string> | null | undefined): KeyValueRow[] {
  return Object.entries(attrs ?? {})
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([key, value]) => ({ key, value }));
}

/** Baris editor → peta kirim (kunci/nilai di-trim; baris kosong total dibuang). */
export function rowsToAttrs(rows: readonly KeyValueRow[]): Record<string, string> {
  const out: Record<string, string> = {};
  for (const r of rows) {
    const k = r.key.trim();
    const v = r.value.trim();
    if (k === "" && v === "") continue;
    out[k] = v;
  }
  return out;
}

/**
 * Validasi baris editor. Mengembalikan satu pesan per baris (indeks → pesan)
 * dan pesan umum (mis. terlalu banyak baris). Kosong = valid.
 */
export function validateRows(
  rows: readonly KeyValueRow[],
  limits: KeyValueLimits,
): { general: string | null; byRow: Record<number, string> } {
  const byRow: Record<number, string> = {};
  const used = rows.filter((r) => r.key.trim() !== "" || r.value.trim() !== "");
  let general: string | null = null;
  if (used.length > limits.maxRows) {
    general = `Maksimal ${limits.maxRows} info tambahan`;
  }
  const seen = new Map<string, number>();
  rows.forEach((r, i) => {
    const k = r.key.trim();
    const v = r.value.trim();
    if (k === "" && v === "") return;
    if (k === "" || [...k].length > limits.maxKey) {
      byRow[i] = `Nama wajib 1-${limits.maxKey} karakter`;
      return;
    }
    if (v === "" || [...v].length > limits.maxValue) {
      byRow[i] = `Isi wajib 1-${limits.maxValue} karakter`;
      return;
    }
    const prev = seen.get(k);
    if (prev !== undefined) {
      byRow[i] = `Nama '${k}' ganda`;
      return;
    }
    seen.set(k, i);
  });
  return { general, byRow };
}
