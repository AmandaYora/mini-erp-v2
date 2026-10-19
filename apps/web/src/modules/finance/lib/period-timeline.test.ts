import { describe, expect, it } from "vitest";
import { buildYearTimeline, closedCount, timelineYears } from "@/modules/finance/lib/period-timeline";

describe("timeline periode", () => {
  const periods = [
    { year: 2026, month: 8, status: "closed" },
    { year: 2026, month: 9, status: "open" },
    { year: 2025, month: 12, status: "closed" },
  ];

  it("selalu dua belas bulan; bulan tanpa baris dianggap terbuka dan belum tersentuh", () => {
    const tl = buildYearTimeline(2026, periods);
    expect(tl).toHaveLength(12);
    expect(tl[7]).toEqual({ year: 2026, month: 8, state: "closed", touched: true });
    expect(tl[8]).toEqual({ year: 2026, month: 9, state: "open", touched: true });
    expect(tl[0]).toEqual({ year: 2026, month: 1, state: "open", touched: false });
    expect(closedCount(tl)).toBe(1);
  });

  it("periode null (Go) tidak membuat crash", () => {
    expect(buildYearTimeline(2026, null).every((m) => m.state === "open")).toBe(true);
  });

  it("daftar tahun memuat tahun berjalan dan tahun berperiode, terbaru dulu", () => {
    expect(timelineYears(periods, 2026)).toEqual([2026, 2025]);
    expect(timelineYears([], 2026)).toEqual([2026]);
  });
});
