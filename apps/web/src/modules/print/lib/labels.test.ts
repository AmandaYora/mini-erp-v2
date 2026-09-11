import { describe, expect, it } from "vitest";
import { expandLabelTargets } from "@/modules/print/lib/labels";
import type { Product } from "@/modules/products/types";

function product(over: Partial<Product>): Product {
  return {
    id: 1,
    code: "P",
    name: "Induk",
    categoryId: null,
    categoryName: "",
    type: "barang",
    tracked: true,
    baseUom: "pcs",
    purchaseUom: "pcs",
    salesUom: "pcs",
    purchaseFactor: 1,
    salesFactor: 1,
    purchasePrice: 0,
    sellingPrice: 0,
    minSellingPrice: 0,
    minStock: 0,
    status: "active",
    variants: [],
    ...over,
  };
}

describe("expandLabelTargets", () => {
  it("produk bervarian kipas per varian aktif tanpa induk", () => {
    const targets = expandLabelTargets(
      product({
        variants: [
          { id: 11, code: "V1", name: "Merah", barcode: "B11", isDefault: true, status: "active" },
          { id: 12, code: "V2", name: "", barcode: "", isDefault: false, status: "active" },
          { id: 13, code: "V3", name: "Lama", barcode: "", isDefault: false, status: "archived" },
        ],
      }),
    );
    expect(targets).toHaveLength(2);
    expect(targets[0]).toMatchObject({
      variantId: 11,
      code: "V1",
      name: "Induk - Merah",
      payload: "B11",
    });
    expect(targets[1]).toMatchObject({
      variantId: 12,
      name: "Induk - V2",
      payload: "V2",
    });
  });

  it("produk tanpa varian jadi satu label kode produk", () => {
    const targets = expandLabelTargets(product({}));
    expect(targets).toEqual([
      { productId: 1, variantId: null, code: "P", name: "Induk", payload: "P" },
    ]);
  });
});
