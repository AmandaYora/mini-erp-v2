import type { DashboardTrendPoint, TrendRangeKey } from "@/modules/dashboard/types";

// Tren penjualan dasbor: data harian dari /reporting/sales-trend (deret padat, satu titik per
// hari) dikelompokkan di sini menjadi harian, mingguan (Senin–Minggu), atau bulanan. Omzet
// adalah rupiah bulat sehingga penjumlahan per kelompok tetap tepat.

export type TrendInterval = "day" | "week" | "month";
export type TrendChartType = "bar" | "line" | "candle";

export const TREND_INTERVALS: { value: TrendInterval; label: string }[] = [
  { value: "day", label: "Harian" },
  { value: "week", label: "Mingguan" },
  { value: "month", label: "Bulanan" },
];

export const TREND_CHART_TYPES: { value: TrendChartType; label: string }[] = [
  { value: "bar", label: "Batang" },
  { value: "line", label: "Garis" },
  { value: "candle", label: "Candlestick" },
];

/** Periode yang masuk akal per rentang: minimal dua kelompok, bulanan baru dari 90 hari. */
const INTERVALS_BY_RANGE: Record<TrendRangeKey, TrendInterval[]> = {
  "7d": ["day"],
  "30d": ["day", "week"],
  "90d": ["day", "week", "month"],
  "12m": ["day", "week", "month"],
};

/** Jumlah minggu / bulan per rentang (jendela diratakan ke awal minggu / bulan). */
const WEEKS: Record<TrendRangeKey, number> = { "7d": 1, "30d": 5, "90d": 13, "12m": 52 };
const MONTHS: Record<TrendRangeKey, number> = { "7d": 1, "30d": 1, "90d": 3, "12m": 12 };
const DAYS: Record<TrendRangeKey, number> = { "7d": 7, "30d": 30, "90d": 90, "12m": 365 };

export function intervalsFor(range: TrendRangeKey): TrendInterval[] {
  return INTERVALS_BY_RANGE[range];
}

export function isIntervalAllowed(range: TrendRangeKey, interval: TrendInterval): boolean {
  return INTERVALS_BY_RANGE[range].includes(interval);
}

/** Candlestick butuh beberapa hari per kelompok: hanya mingguan dan bulanan. */
export function isChartAllowed(interval: TrendInterval, chart: TrendChartType): boolean {
  return chart !== "candle" || interval !== "day";
}

/** Pilihan yang sah terdekat: periode jatuh ke harian, candlestick jatuh ke batang. */
export function normalizeTrendView(
  range: TrendRangeKey,
  interval: TrendInterval,
  chart: TrendChartType,
): { interval: TrendInterval; chart: TrendChartType } {
  const i = isIntervalAllowed(range, interval) ? interval : "day";
  return { interval: i, chart: isChartAllowed(i, chart) ? chart : "bar" };
}

function toDate(ymd: string): Date {
  return new Date(`${ymd}T00:00:00Z`);
}

function ymd(d: Date): string {
  return d.toISOString().slice(0, 10);
}

function plusDays(day: string, delta: number): string {
  const d = toDate(day);
  d.setUTCDate(d.getUTCDate() + delta);
  return ymd(d);
}

/** Senin pada minggu tanggal itu (minggu Senin–Minggu, kebiasaan Indonesia). */
export function weekStart(day: string): string {
  const dow = toDate(day).getUTCDay(); // 0 = Minggu
  return plusDays(day, -((dow + 6) % 7));
}

export function monthStart(day: string): string {
  return `${day.slice(0, 7)}-01`;
}

function monthsBack(firstOfMonth: string, n: number): string {
  const d = toDate(firstOfMonth);
  d.setUTCMonth(d.getUTCMonth() - n);
  return ymd(d);
}

function daysInMonth(firstOfMonth: string): number {
  const d = toDate(firstOfMonth);
  return new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + 1, 0)).getUTCDate();
}

/**
 * Jendela yang diminta ke server. Mingguan/bulanan diratakan ke awal minggu/bulan agar kelompok
 * pertama utuh; panjangnya tetap ≤ 366 hari (batas /reporting/sales-trend).
 */
export function trendWindow(range: TrendRangeKey, interval: TrendInterval, today: string): { from: string; to: string } {
  switch (interval) {
    case "week":
      return { from: plusDays(weekStart(today), -7 * (WEEKS[range] - 1)), to: today };
    case "month":
      return { from: monthsBack(monthStart(today), MONTHS[range] - 1), to: today };
    default:
      return { from: plusDays(today, -(DAYS[range] - 1)), to: today };
  }
}

export interface TrendBucket {
  /** Hari pertama kelompok (kunci sumbu X). */
  start: string;
  /** Hari terakhir yang ada datanya. */
  end: string;
  /** Omzet kelompok (jumlah harian). */
  revenue: number;
  /** OHLC omzet harian dalam kelompok, untuk candlestick. */
  open: number;
  high: number;
  low: number;
  close: number;
  /** Rentang [terendah, tertinggi] untuk batang rentang recharts. */
  range: [number, number];
  days: number;
  /** Kelompok belum lengkap (minggu/bulan berjalan). */
  partial: boolean;
}

/** Kelompokkan deret harian (urut tanggal) menjadi harian, mingguan, atau bulanan. */
export function bucketTrend(points: DashboardTrendPoint[], interval: TrendInterval): TrendBucket[] {
  const out: TrendBucket[] = [];
  let cur: TrendBucket | null = null;
  for (const p of points) {
    const key = interval === "week" ? weekStart(p.date) : interval === "month" ? monthStart(p.date) : p.date;
    if (!cur || cur.start !== key) {
      cur = { start: key, end: p.date, revenue: 0, open: p.revenue, high: p.revenue, low: p.revenue, close: p.revenue, range: [p.revenue, p.revenue], days: 0, partial: false };
      out.push(cur);
    }
    cur.end = p.date;
    cur.revenue += p.revenue;
    cur.high = Math.max(cur.high, p.revenue);
    cur.low = Math.min(cur.low, p.revenue);
    cur.close = p.revenue;
    cur.range = [cur.low, cur.high];
    cur.days += 1;
  }
  for (const b of out) {
    const full = interval === "week" ? 7 : interval === "month" ? daysInMonth(b.start) : 1;
    b.partial = b.days < full;
  }
  return out;
}

const MONTH_SHORT = new Intl.DateTimeFormat("id-ID", { month: "short", timeZone: "UTC" });
const MONTH_LONG = new Intl.DateTimeFormat("id-ID", { month: "long", year: "numeric", timeZone: "UTC" });
const DAY_MONTH = new Intl.DateTimeFormat("id-ID", { day: "numeric", month: "short", timeZone: "UTC" });
const FULL_DATE = new Intl.DateTimeFormat("id-ID", { day: "numeric", month: "short", year: "numeric", timeZone: "UTC" });

/** Label sumbu X yang ringkas. */
export function bucketTick(start: string, interval: TrendInterval, long: boolean): string {
  if (interval === "month") return `${MONTH_SHORT.format(toDate(start))} ${start.slice(2, 4)}`;
  if (long && interval === "day") return `${start.slice(5, 7)}/${start.slice(2, 4)}`;
  return `${start.slice(8, 10)}/${start.slice(5, 7)}`;
}

/** Judul tooltip: tanggal, rentang minggu, atau nama bulan. */
export function bucketTitle(b: TrendBucket, interval: TrendInterval): string {
  if (interval === "month") return MONTH_LONG.format(toDate(b.start));
  if (interval === "week") return `Minggu ${DAY_MONTH.format(toDate(b.start))} – ${FULL_DATE.format(toDate(plusDays(b.start, 6)))}`;
  return FULL_DATE.format(toDate(b.start));
}

/** Kunci dasar localStorage (dibatasi per pengguna lewat userKey). */
export const TREND_PREFS_BASE = "mini-erp.dashboard.trend";

export interface TrendPrefs {
  range: TrendRangeKey;
  interval: TrendInterval;
  chart: TrendChartType;
}

export const DEFAULT_TREND_PREFS: TrendPrefs = { range: "30d", interval: "day", chart: "bar" };

const RANGES: TrendRangeKey[] = ["7d", "30d", "90d", "12m"];

/** Baca preferensi tampilan tren (per pengguna); nilai rusak/asing jatuh ke bawaan. */
export function readTrendPrefs(key: string): TrendPrefs {
  try {
    const raw = JSON.parse(localStorage.getItem(key) ?? "null") as Partial<TrendPrefs> | null;
    const range = RANGES.includes(raw?.range as TrendRangeKey) ? (raw?.range as TrendRangeKey) : DEFAULT_TREND_PREFS.range;
    const interval = TREND_INTERVALS.some((i) => i.value === raw?.interval) ? (raw?.interval as TrendInterval) : "day";
    const chart = TREND_CHART_TYPES.some((c) => c.value === raw?.chart) ? (raw?.chart as TrendChartType) : "bar";
    return { range, ...normalizeTrendView(range, interval, chart) };
  } catch {
    return DEFAULT_TREND_PREFS;
  }
}

export function writeTrendPrefs(key: string, prefs: TrendPrefs): void {
  try {
    localStorage.setItem(key, JSON.stringify(prefs));
  } catch {
    /* penyimpanan privat: preferensi sesi ini saja */
  }
}
