import type { FiscalPeriod } from "@/modules/finance/types";

export type MonthState = "open" | "closed";

export interface TimelineMonth {
  year: number;
  month: number;
  /** "closed" bila ada baris periode berstatus closed; selain itu terbuka (termasuk bulan yang belum pernah disentuh). */
  state: MonthState;
  /** Bulan pernah tercatat di server (ada baris periode). */
  touched: boolean;
}

/** Dua belas bulan satu tahun sebagai timeline terbuka/terkunci dari daftar periode server. */
export function buildYearTimeline(year: number, periods: FiscalPeriod[] | null | undefined): TimelineMonth[] {
  const byMonth = new Map<number, FiscalPeriod>();
  for (const p of periods ?? []) if (p.year === year) byMonth.set(p.month, p);
  return Array.from({ length: 12 }, (_, i) => {
    const month = i + 1;
    const p = byMonth.get(month);
    return { year, month, state: p?.status === "closed" ? "closed" : "open", touched: p !== undefined };
  });
}

/** Tahun-tahun yang pernah punya periode, ditambah tahun berjalan; terbaru dulu. */
export function timelineYears(periods: FiscalPeriod[] | null | undefined, currentYear: number): number[] {
  const set = new Set<number>([currentYear]);
  for (const p of periods ?? []) set.add(p.year);
  return [...set].sort((a, b) => b - a);
}

export function closedCount(months: TimelineMonth[]): number {
  return months.filter((m) => m.state === "closed").length;
}
