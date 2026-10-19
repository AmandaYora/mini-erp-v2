/** Potong daftar untuk halaman `page` (1-based) — pagination sisi klien untuk endpoint yang belum berpaginasi. */
export function paginate<T>(rows: readonly T[], page: number, limit: number): T[] {
  const p = clampPage(page, rows.length, limit);
  return rows.slice((p - 1) * limit, p * limit);
}

/** Jaga halaman tetap valid saat jumlah baris menyusut (mis. setelah filter). */
export function clampPage(page: number, total: number, limit: number): number {
  const last = Math.max(1, Math.ceil(total / Math.max(1, limit)));
  return Math.min(Math.max(1, Math.floor(page) || 1), last);
}
