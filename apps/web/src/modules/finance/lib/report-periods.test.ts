import { describe, expect, it } from "vitest";
import { defaultYearMonth, monthYearLabel, yearOptions } from "@/modules/finance/lib/report-periods";

describe("report-periods", () => {
  it("memecah tanggal WIB menjadi tahun dan bulan", () => {
    expect(defaultYearMonth("2026-10-16")).toEqual({ year: 2026, month: 10 });
    expect(defaultYearMonth("2027-01-01")).toEqual({ year: 2027, month: 1 });
  });

  it("menyusun lima tahun terakhir, terbaru dulu", () => {
    expect(yearOptions("2026-10-16")).toEqual([2026, 2025, 2024, 2023, 2022]);
  });

  it("memberi nama bulan penuh", () => {
    expect(monthYearLabel(2026, 10)).toBe("Oktober 2026");
    expect(monthYearLabel(2026, 13)).toBe("13 2026");
  });
});
