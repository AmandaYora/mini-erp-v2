import { describe, expect, it } from "vitest";
import {
  cartEstimate,
  cashChange,
  clampLineDiscount,
  lineDiscount,
  lineTotal,
  maxLineDiscount,
} from "@/modules/pos/lib/cart";

describe("lineTotal / cartEstimate", () => {
  it("multiplies qty by catalog price", () => {
    expect(lineTotal({ qty: 3, unitPrice: 15500 })).toBe(46500);
  });
  it("sums the cart", () => {
    expect(
      cartEstimate([
        { qty: 2, unitPrice: 15000 },
        { qty: 1, unitPrice: 5000 },
      ]),
    ).toBe(35000);
  });
  it("is zero for empty cart", () => {
    expect(cartEstimate([])).toBe(0);
  });
});

describe("diskon per baris", () => {
  it("persen dihitung dari bruto baris", () => {
    expect(lineTotal({ qty: 2, unitPrice: 10000, discountPct: 10 })).toBe(18000);
  });
  it("nominal adalah potongan per baris, bukan per satuan", () => {
    expect(lineTotal({ qty: 3, unitPrice: 10000, discountNominal: 2500 })).toBe(27500);
  });
  it("persen lalu nominal bertumpuk (sama dengan LineAmounts server)", () => {
    const l = { qty: 2, unitPrice: 10000, discountPct: 10, discountNominal: 500 };
    expect(lineDiscount(l)).toBe(2500);
    expect(lineTotal(l)).toBe(17500);
  });
  it("estimasi keranjang memakai netto", () => {
    expect(
      cartEstimate([
        { qty: 1, unitPrice: 10000, discountPct: 50 },
        { qty: 1, unitPrice: 5000 },
      ]),
    ).toBe(10000);
  });
});

describe("klem harga minimum", () => {
  const line = { qty: 4, unitPrice: 10000 };

  it("potongan maksimum = bruto − min × qty", () => {
    expect(maxLineDiscount(line, 8000)).toBe(8000);
  });
  it("tanpa minimum, batas atas hanya bruto", () => {
    expect(maxLineDiscount(line, 0)).toBe(40000);
  });
  it("tidak melewati batas: diskon dalam batas dibiarkan", () => {
    const r = clampLineDiscount(line, { discountPct: 10 }, 8000);
    expect(r).toEqual({ discountPct: 10, discountNominal: 0, clamped: false });
  });
  it("persen yang melampaui batas diturunkan", () => {
    const r = clampLineDiscount(line, { discountPct: 50 }, 8000);
    expect(r.clamped).toBe(true);
    expect(lineTotal({ ...line, ...r })).toBeGreaterThanOrEqual(8000 * 4);
    expect(r.discountPct).toBe(20);
  });
  it("nominal yang melampaui batas dipotong", () => {
    const r = clampLineDiscount(line, { discountNominal: 20000 }, 8000);
    expect(r).toEqual({ discountPct: 0, discountNominal: 8000, clamped: true });
  });
  it("persen + nominal: nominal menyesuaikan sisa batas", () => {
    const r = clampLineDiscount(line, { discountPct: 10, discountNominal: 9000 }, 8000);
    expect(r.discountNominal).toBe(4000);
    expect(lineTotal({ ...line, ...r })).toBe(32000);
  });
  it("menormalkan input ngawur (negatif, >100, NaN)", () => {
    expect(clampLineDiscount(line, { discountPct: -5, discountNominal: -1 }, 0)).toEqual({
      discountPct: 0,
      discountNominal: 0,
      clamped: false,
    });
    expect(clampLineDiscount(line, { discountPct: 250 }, 0).discountPct).toBe(100);
    expect(clampLineDiscount(line, { discountPct: Number.NaN }, 0).discountPct).toBe(0);
  });
  it("harga satuan efektif tidak pernah di bawah minimum", () => {
    for (const pct of [0, 7.5, 33.33, 99, 100]) {
      const r = clampLineDiscount({ qty: 3, unitPrice: 12345 }, { discountPct: pct }, 9000);
      expect(lineTotal({ qty: 3, unitPrice: 12345, ...r })).toBeGreaterThanOrEqual(27000);
    }
  });
});

describe("cashChange", () => {
  it("computes positive change", () => {
    expect(cashChange(50000, 35000)).toBe(15000);
  });
  it("goes negative on short payment", () => {
    expect(cashChange(30000, 35000)).toBe(-5000);
  });
  it("is zero on exact payment", () => {
    expect(cashChange(35000, 35000)).toBe(0);
  });
});
