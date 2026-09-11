import { describe, expect, it } from "vitest";
import { cartEstimate, cashChange, lineTotal } from "@/modules/pos/lib/cart";

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
