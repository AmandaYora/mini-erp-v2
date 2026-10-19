import { todayWIB } from "@/shared/lib/format";

export const MONTH_NAMES = [
  "Januari",
  "Februari",
  "Maret",
  "April",
  "Mei",
  "Juni",
  "Juli",
  "Agustus",
  "September",
  "Oktober",
  "November",
  "Desember",
] as const;

/** Tahun + bulan berjalan menurut WIB (Asia/Jakarta terkunci, ADR-0006). */
export function defaultYearMonth(today: string = todayWIB()): { year: number; month: number } {
  return { year: Number(today.slice(0, 4)), month: Number(today.slice(5, 7)) };
}

/** Lima tahun terakhir, terbaru dulu — pilihan tahun pada laporan bulanan. */
export function yearOptions(today: string = todayWIB()): number[] {
  const y = defaultYearMonth(today).year;
  return [y, y - 1, y - 2, y - 3, y - 4];
}

/** "Oktober 2026" — nama bulan penuh untuk judul kartu/laporan. */
export function monthYearLabel(year: number, month: number): string {
  const name = MONTH_NAMES[month - 1] ?? String(month);
  return `${name} ${year}`;
}
