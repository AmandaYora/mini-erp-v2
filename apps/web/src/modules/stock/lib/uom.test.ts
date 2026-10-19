import { describe, expect, it } from "vitest";
import { dualUomLabel } from "@/modules/stock/lib/uom";

const info = { baseUom: "pcs", units: [{ uom: "dus", factor: 12 }, { uom: "pcs", factor: 1 }] };

describe("dualUomLabel", () => {
  it("memecah ke satuan besar dan sisa", () => {
    expect(dualUomLabel(17, info)).toBe("1 dus 5 pcs");
    expect(dualUomLabel(24, info)).toBe("2 dus");
  });
  it("null bila kurang dari satu satuan besar atau tanpa satuan besar", () => {
    expect(dualUomLabel(5, info)).toBeNull();
    expect(dualUomLabel(30, { baseUom: "pcs", units: [{ uom: "pcs", factor: 1 }] })).toBeNull();
    expect(dualUomLabel(30, undefined)).toBeNull();
  });
  it("memilih faktor terbesar dan menjaga tanda negatif", () => {
    const two = { baseUom: "pcs", units: [{ uom: "pak", factor: 6 }, { uom: "dus", factor: 24 }] };
    expect(dualUomLabel(50, two)).toBe("2 dus 2 pcs");
    expect(dualUomLabel(-17, info)).toBe("-1 dus 5 pcs");
  });
  it("sisa desimal dirapikan", () => {
    expect(dualUomLabel(12.5, info)).toBe("1 dus 0.5 pcs");
  });
});
