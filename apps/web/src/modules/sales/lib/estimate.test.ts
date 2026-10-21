import { describe, expect, it } from "vitest";
import { estimateLine, estimateTotal, priceForUnit } from "@/modules/sales/lib/estimate";

describe("estimateLine", () => {
  it("membulatkan seperti server: bruto dulu, lalu potongan persen", () => {
    // 1,5 x 1.001 = 1.501,5 -> 1.502; 10% = 150,2 -> 150; net 1.352 (bukan 1.351).
    expect(estimateLine({ qty: 1.5, catalogPrice: 1001, discountPct: 10, discountNominal: 0 })).toBe(1352);
  });
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

describe("priceForUnit (cermin money.PriceForUnit server)", () => {
  it("satuan lain dihargai menurut rasio qty dasar", () => {
    expect(priceForUnit(25000, 36, 1)).toBe(900000); // 1 DUS isi 36 BH
    expect(priceForUnit(25000, 1, 1)).toBe(25000);
    expect(priceForUnit(120000, 1, 12)).toBe(10000); // satuan dasar dari pak isi 12
    expect(priceForUnit(25000, 36, 0)).toBe(25000);
  });
});
