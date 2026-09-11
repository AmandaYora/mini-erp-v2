import { describe, expect, it } from "vitest";
import { calcLineEstimate } from "@/modules/purchasing/lib/estimate";

describe("calcLineEstimate", () => {
  it("computes gross/net without tax or discount", () => {
    expect(calcLineEstimate(2, 10000, 0, 0, "none", 0)).toEqual({
      gross: 20000, disc: 0, net: 20000, tax: 0, total: 20000,
    });
  });
  it("applies percent then nominal discount", () => {
    // gross 20000, disc 10% = 2000, nominal 500 → net 17500
    const r = calcLineEstimate(2, 10000, 10, 500, "none", 0);
    expect(r.disc).toBe(2000);
    expect(r.net).toBe(17500);
    expect(r.total).toBe(17500);
  });
  it("adds tax on top for exclude", () => {
    const r = calcLineEstimate(1, 10000, 0, 0, "exclude", 11);
    expect(r.tax).toBe(1100);
    expect(r.total).toBe(11100);
  });
  it("extracts tax from within for include", () => {
    // net 11100 @11% → base 10000, tax 1100, total stays 11100
    const r = calcLineEstimate(1, 11100, 0, 0, "include", 11);
    expect(r.total).toBe(11100);
    expect(r.tax).toBe(1100);
  });
  it("rounds fractional rupiah", () => {
    // 3 × 3333 = 9999; 10% = 999.9 → 1000
    const r = calcLineEstimate(3, 3333, 10, 0, "none", 0);
    expect(r.gross).toBe(9999);
    expect(r.disc).toBe(1000);
    expect(r.net).toBe(8999);
  });
});
