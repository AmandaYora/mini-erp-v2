import { describe, expect, it } from "vitest";
import { quickCashOptions, roundUpCash } from "@/modules/pos/lib/quick-cash";

const amounts = (total: number) => quickCashOptions(total).map((o) => o.amount);

describe("roundUpCash", () => {
  it("membulatkan ke atas ke kelipatan", () => {
    expect(roundUpCash(7_000, 10_000)).toBe(10_000);
    expect(roundUpCash(10_000, 10_000)).toBe(10_000);
    expect(roundUpCash(65_000, 50_000)).toBe(100_000);
  });
  it("step tak valid / total nol tidak meledak", () => {
    expect(roundUpCash(0, 10_000)).toBe(0);
    expect(roundUpCash(5, 0)).toBe(5);
  });
});

describe("quickCashOptions", () => {
  it("total kecil: pas, 10.000, 50.000, 100.000", () => {
    expect(amounts(7_000)).toEqual([7_000, 10_000, 50_000, 100_000]);
    expect(quickCashOptions(7_000)[0].kind).toBe("exact");
  });
  it("total di atas pecahan: pecahan kecil disembunyikan", () => {
    expect(amounts(120_000)).toEqual([120_000, 150_000, 200_000]);
  });
  it("total pas pecahan tidak diduplikasi", () => {
    expect(amounts(50_000)).toEqual([50_000, 100_000]);
  });
  it("total 0 / tak valid: kosong", () => {
    expect(quickCashOptions(0)).toEqual([]);
    expect(quickCashOptions(Number.NaN)).toEqual([]);
  });
  it("tidak pernah menawarkan nominal di bawah total", () => {
    for (const t of [1, 999, 33_500, 98_000, 1_400_000]) {
      expect(quickCashOptions(t).every((o) => o.amount >= t)).toBe(true);
    }
  });
});
