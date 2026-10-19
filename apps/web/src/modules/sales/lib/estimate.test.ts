import { describe, expect, it } from "vitest";
import { estimateLine, estimateTotal } from "@/modules/sales/lib/estimate";

describe("estimateLine", () => {
  it("computes net without discount", () => {
    expect(estimateLine({ qty: 2, catalogPrice: 15000, discountPct: 0, discountNominal: 0 })).toBe(30000);
  });
  it("applies percent then nominal discount", () => {
    expect(estimateLine({ qty: 2, catalogPrice: 15000, discountPct: 10, discountNominal: 500 })).toBe(26500);
  });
  it("floors at zero when discounts exceed gross", () => {
    expect(estimateLine({ qty: 1, catalogPrice: 1000, discountPct: 0, discountNominal: 5000 })).toBe(0);
  });
});

describe("estimateLine manual price", () => {
  it("uses manual price over catalog", () => {
    expect(
      estimateLine({ qty: 2, catalogPrice: 15000, manualPrice: 12000, discountPct: 0, discountNominal: 0 }),
    ).toBe(24000);
  });
  it("treats null manual price as catalog", () => {
    expect(
      estimateLine({ qty: 2, catalogPrice: 15000, manualPrice: null, discountPct: 0, discountNominal: 0 }),
    ).toBe(30000);
  });
  it("allows a manual price of zero", () => {
    expect(
      estimateLine({ qty: 2, catalogPrice: 15000, manualPrice: 0, discountPct: 0, discountNominal: 0 }),
    ).toBe(0);
  });
});

describe("estimateTotal", () => {
  it("sums lines", () => {
    expect(
      estimateTotal([
        { qty: 2, catalogPrice: 15000, discountPct: 0, discountNominal: 0 },
        { qty: 1, catalogPrice: 5000, discountPct: 0, discountNominal: 0 },
      ]),
    ).toBe(35000);
  });
  it("is zero for empty cart", () => {
    expect(estimateTotal([])).toBe(0);
  });
});
