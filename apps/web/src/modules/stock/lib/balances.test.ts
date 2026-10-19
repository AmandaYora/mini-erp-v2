import { describe, expect, it } from "vitest";
import { distinctProductIds, filterBalances } from "@/modules/stock/lib/balances";
import type { StockBalance } from "@/modules/stock/types";

const row = (p: number, loc: number, onHand: number, reserved = 0): StockBalance => ({
  branchId: 1,
  productId: p,
  variantId: 1,
  locationId: loc,
  onHand,
  reserved,
  available: onHand - reserved,
});

describe("balances", () => {
  const rows = [row(1, 10, 5), row(1, 11, -2), row(2, 10, 3, 5)];
  it("menyaring lokasi", () => {
    expect(filterBalances(rows, { locationId: 10, negativeOnly: false })).toHaveLength(2);
  });
  it("menyaring negatif (onHand atau available)", () => {
    expect(filterBalances(rows, { locationId: null, negativeOnly: true })).toHaveLength(2);
  });
  it("id produk unik", () => {
    expect(distinctProductIds(rows)).toEqual([1, 2]);
  });
});
