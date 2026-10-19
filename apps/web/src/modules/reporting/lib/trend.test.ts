import { describe, expect, it } from "vitest";
import { compactIDR, diffDays, paginate, sumTrend, trendRangeError } from "@/modules/reporting/lib/trend";

describe("lib tren laporan", () => {
  it("diffDays inklusif", () => {
    expect(diffDays("2026-10-01", "2026-10-01")).toBe(1);
    expect(diffDays("2026-10-01", "2026-10-31")).toBe(31);
  });

  it("trendRangeError memeriksa kosong, terbalik, dan > 366 hari", () => {
    expect(trendRangeError("", "2026-10-01")).toMatch(/Pilih tanggal/);
    expect(trendRangeError("2026-10-02", "2026-10-01")).toMatch(/sebelum/);
    expect(trendRangeError("2025-01-01", "2026-10-01")).toMatch(/366/);
    expect(trendRangeError("2026-10-01", "2026-10-16")).toBeNull();
  });

  it("sumTrend menjumlah omzet, beban, laba", () => {
    expect(
      sumTrend([
        { date: "a", revenue: 10, expense: 4, profit: 6 },
        { date: "b", revenue: 5, expense: 1, profit: 4 },
      ]),
    ).toEqual({ revenue: 15, expense: 5, profit: 10 });
    expect(sumTrend([])).toEqual({ revenue: 0, expense: 0, profit: 0 });
  });

  it("compactIDR meringkas angka sumbu", () => {
    expect(compactIDR(250_000)).toBe("250 rb");
    expect(compactIDR(1_500_000)).toBe("2 jt");
  });

  it("paginate memotong dan menjepit halaman", () => {
    const rows = Array.from({ length: 7 }, (_, i) => i + 1);
    expect(paginate(rows, 1, 3)).toEqual([1, 2, 3]);
    expect(paginate(rows, 3, 3)).toEqual([7]);
    expect(paginate(rows, 9, 3)).toEqual([7]);
    expect(paginate(rows, 0, 3)).toEqual([1, 2, 3]);
    expect(paginate([], 1, 3)).toEqual([]);
  });
});
