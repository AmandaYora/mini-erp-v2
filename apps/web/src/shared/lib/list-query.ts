// Utilitas murni untuk query daftar (domain-agnostik): membersihkan param,
// CSV status, validasi rentang tanggal. Dipakai semua halaman daftar agar
// kontrak query (search, status CSV, from/to, partyId, sort/dir) seragam.

/** Buang nilai kosong (undefined/null/""/spasi/NaN/0 untuk id) dan trim string. */
export function compactParams(p: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(p)) {
    if (v === undefined || v === null) continue;
    if (typeof v === "string") {
      const t = v.trim();
      if (t !== "") out[k] = t;
      continue;
    }
    if (typeof v === "number" && !Number.isFinite(v)) continue;
    out[k] = v;
  }
  return out;
}

/** Id positif atau undefined (0/negatif/NaN berarti "tidak difilter"). */
export function positiveId(v: number | null | undefined): number | undefined {
  return typeof v === "number" && Number.isFinite(v) && v > 0 ? v : undefined;
}

/** ["a","b"] -> "a,b"; kosong -> undefined. Duplikat & item kosong dibuang. */
export function toCsv(values: readonly string[] | undefined | null): string | undefined {
  if (!values) return undefined;
  const seen = new Set<string>();
  for (const v of values) {
    const t = v.trim();
    if (t !== "") seen.add(t);
  }
  return seen.size > 0 ? [...seen].join(",") : undefined;
}

/** "a,b,,a" -> ["a","b"]. */
export function fromCsv(csv: string | undefined | null): string[] {
  if (!csv) return [];
  return [...new Set(csv.split(",").map((s) => s.trim()).filter(Boolean))];
}

/** Pesan galat bila rentang tanggal terbalik (server menolak dengan 400). */
export function dateRangeError(from: string, to: string): string | null {
  if (from && to && from > to) return "Tanggal awal tidak boleh setelah tanggal akhir";
  return null;
}

/** Aktifkan/matikan satu nilai pada set CSV (untuk filter status multi). */
export function toggleCsv(csv: string, value: string): string {
  const set = fromCsv(csv);
  const next = set.includes(value) ? set.filter((v) => v !== value) : [...set, value];
  return next.join(",");
}

export type SortDir = "asc" | "desc";

/** Param urut: sort+dir hanya dikirim bila ada pilihan eksplisit. */
export function sortParams(sort: string, dir: SortDir | ""): Record<string, unknown> {
  if (!sort) return {};
  return { sort, dir: dir || undefined };
}
