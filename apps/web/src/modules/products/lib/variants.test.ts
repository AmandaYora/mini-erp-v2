import { describe, expect, it } from "vitest";
import { sellableVariants, soleSellableVariant } from "@/modules/products/lib/variants";

const v = (id: number, code: string, over: Partial<{ status: string; isDefault: boolean; isHidden: boolean; sortOrder: number }> = {}) => ({
  id, code, status: "active", isDefault: false, isHidden: false, sortOrder: 0, ...over,
});

describe("sellableVariants", () => {
  it("menyembunyikan 'Standar' tersembunyi dan varian nonaktif, urut sort_order", () => {
    const list = sellableVariants([
      v(1, "CAT", { isDefault: true, isHidden: true }),
      v(2, "CAT-002", { sortOrder: 2 }),
      v(3, "CAT-001", { sortOrder: 1 }),
      v(4, "CAT-003", { status: "inactive" }),
    ]);
    expect(list.map((x) => x.id)).toEqual([3, 2]);
    expect(soleSellableVariant(list)).toBeNull();
  });

  it("produk tanpa varian nyata memakai varian bawaannya", () => {
    const only = [v(1, "SMN", { isDefault: true, isHidden: true })];
    expect(soleSellableVariant(only)?.id).toBe(1);
    expect(soleSellableVariant([v(1, "SMN", { isDefault: true }), v(9, "SMN-X", { status: "archived" })])?.id).toBe(1);
  });
});
