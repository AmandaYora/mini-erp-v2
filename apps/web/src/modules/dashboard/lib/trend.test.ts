import { afterEach, describe, expect, it } from "vitest";
import {
  bucketTick,
  bucketTitle,
  bucketTrend,
  isChartAllowed,
  normalizeTrendView,
  readTrendPrefs,
  trendWindow,
  weekStart,
  writeTrendPrefs,
} from "@/modules/dashboard/lib/trend";
import type { DashboardTrendPoint } from "@/modules/dashboard/types";

const day = (date: string, revenue: number): DashboardTrendPoint => ({ date, revenue, expense: 0, profit: 0 });

function days(from: string, revenues: number[]): DashboardTrendPoint[] {
  return revenues.map((r, i) => {
    const d = new Date(`${from}T00:00:00Z`);
    d.setUTCDate(d.getUTCDate() + i);
    return day(d.toISOString().slice(0, 10), r);
  });
}

describe("trendWindow", () => {
  it("harian: N hari terakhir", () => {
    expect(trendWindow("30d", "day", "2026-10-22")).toEqual({ from: "2026-09-23", to: "2026-10-22" });
  });
  it("mingguan: mulai Senin, 52 minggu untuk 12 bulan, tetap ≤ 366 hari", () => {
    const w = trendWindow("12m", "week", "2026-10-22"); // Kamis
    expect(weekStart("2026-10-22")).toBe("2026-10-19");
    expect(w.from).toBe("2025-10-27");
    expect(new Date(`${w.to}T00:00:00Z`).getTime() - new Date(`${w.from}T00:00:00Z`).getTime()).toBeLessThan(366 * 864e5);
  });
  it("bulanan: mulai tanggal 1, 12 bulan termasuk bulan berjalan", () => {
    expect(trendWindow("12m", "month", "2026-10-22")).toEqual({ from: "2025-11-01", to: "2026-10-22" });
    expect(trendWindow("90d", "month", "2026-03-31")).toEqual({ from: "2026-01-01", to: "2026-03-31" });
  });
});

describe("bucketTrend", () => {
  it("harian: satu kelompok per hari, OHLC = nilai hari itu", () => {
    const b = bucketTrend(days("2026-10-20", [5, 7]), "day");
    expect(b).toHaveLength(2);
    expect(b[1]).toMatchObject({ start: "2026-10-21", revenue: 7, open: 7, close: 7, high: 7, low: 7, partial: false });
  });
  it("mingguan: jumlah dan OHLC per minggu Senin–Minggu; minggu berjalan parsial", () => {
    // Senin 12 Okt .. Rabu 21 Okt: minggu 12–18 utuh, 19–21 berjalan.
    const b = bucketTrend(days("2026-10-12", [10, 40, 5, 20, 30, 0, 25, 8, 12, 9]), "week");
    expect(b.map((x) => x.start)).toEqual(["2026-10-12", "2026-10-19"]);
    expect(b[0]).toMatchObject({ revenue: 130, open: 10, close: 25, high: 40, low: 0, range: [0, 40], days: 7, partial: false });
    expect(b[1]).toMatchObject({ revenue: 29, open: 8, close: 9, high: 12, low: 8, days: 3, partial: true });
  });
  it("bulanan: bulan penuh tidak parsial, total tepat", () => {
    const sep = days("2026-09-01", Array.from({ length: 30 }, () => 1_000_000));
    const b = bucketTrend([...sep, day("2026-10-01", 3)], "month");
    expect(b[0]).toMatchObject({ start: "2026-09-01", revenue: 30_000_000, days: 30, partial: false });
    expect(b[1]).toMatchObject({ start: "2026-10-01", revenue: 3, partial: true });
  });
});

describe("pilihan tampilan", () => {
  it("candlestick hanya untuk mingguan/bulanan", () => {
    expect(isChartAllowed("day", "candle")).toBe(false);
    expect(isChartAllowed("week", "candle")).toBe(true);
  });
  it("kombinasi tidak sah dinormalkan", () => {
    expect(normalizeTrendView("7d", "month", "candle")).toEqual({ interval: "day", chart: "bar" });
    expect(normalizeTrendView("12m", "month", "candle")).toEqual({ interval: "month", chart: "candle" });
  });
  it("label sumbu dan judul tooltip", () => {
    expect(bucketTick("2026-10-01", "month", true)).toMatch(/^Okt 26$/);
    expect(bucketTitle({ start: "2026-10-19" } as never, "week")).toMatch(/^Minggu 19 Okt – 25 Okt 2026$/);
  });
});

describe("preferensi", () => {
  afterEach(() => localStorage.clear());
  it("menyimpan dan membaca; nilai rusak jatuh ke bawaan yang sah", () => {
    writeTrendPrefs("k", { range: "12m", interval: "month", chart: "candle" });
    expect(readTrendPrefs("k")).toEqual({ range: "12m", interval: "month", chart: "candle" });
    localStorage.setItem("k", JSON.stringify({ range: "7d", interval: "month", chart: "candle" }));
    expect(readTrendPrefs("k")).toEqual({ range: "7d", interval: "day", chart: "bar" });
    localStorage.setItem("k", "{rusak");
    expect(readTrendPrefs("k")).toEqual({ range: "30d", interval: "day", chart: "bar" });
  });
});
