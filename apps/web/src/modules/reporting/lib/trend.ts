// Fungsi murni laporan tren: rentang tanggal, ringkasan, dan pagination sisi klien.

import type { TrendPoint } from "@/modules/reporting/types";

/** Jumlah hari inklusif antara dua tanggal YYYY-MM-DD. */
export function diffDays(from: string, to: string): number {
  const a = new Date(`${from}T00:00:00Z`).getTime();
  const b = new Date(`${to}T00:00:00Z`).getTime();
  return Math.round((b - a) / 86400000) + 1;
}

/** Pesan galat rentang tren (maksimal 366 hari, ditegakkan backend); null bila sah. */
export function trendRangeError(from: string, to: string): string | null {
  if (!from || !to) return "Pilih tanggal mulai dan tanggal akhir.";
  if (from > to) return "Tanggal akhir sebelum tanggal awal.";
  if (diffDays(from, to) > 366) return "Rentang maksimal 366 hari.";
  return null;
}

export function sumTrend(points: TrendPoint[]): { revenue: number; expense: number; profit: number } {
  return points.reduce(
    (acc, p) => ({ revenue: acc.revenue + p.revenue, expense: acc.expense + p.expense, profit: acc.profit + p.profit }),
    { revenue: 0, expense: 0, profit: 0 },
  );
}

/** Sumbu Y padat: 1500000 → "2 jt", 250000 → "250 rb". */
export function compactIDR(v: number): string {
  const abs = Math.abs(v);
  if (abs >= 1_000_000_000) return `${Math.round((v / 1_000_000_000) * 10) / 10} M`;
  if (abs >= 1_000_000) return `${Math.round(v / 1_000_000)} jt`;
  if (abs >= 1_000) return `${Math.round(v / 1_000)} rb`;
  return String(Math.round(v));
}

/** Potong daftar untuk halaman `page` (mulai 1); halaman di luar jangkauan dijepit. */
export function paginate<T>(rows: readonly T[], page: number, limit: number): T[] {
  const last = Math.max(1, Math.ceil(rows.length / limit));
  const p = Math.min(Math.max(1, page), last);
  return rows.slice((p - 1) * limit, p * limit);
}
