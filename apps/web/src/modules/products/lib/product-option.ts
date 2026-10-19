import type { Product, ProductOptionRow, ProductVariant } from "@/modules/products/types";

/** Varian yang punya label sendiri: aktif dan bukan varian bawaan tersembunyi. */
export function labelVariants(variants: readonly ProductVariant[] | undefined): ProductVariant[] {
  return (variants ?? []).filter((v) => !v.isHidden && v.status === "active");
}

/** Detail produk → bentuk baris search-options (untuk memulihkan pilihan tersimpan). */
export function productToOptionRow(p: Product): ProductOptionRow {
  return {
    id: p.id,
    code: p.code,
    name: p.name,
    sellingPrice: p.sellingPrice,
    baseUom: p.baseUom,
    purchaseUom: p.purchaseUom,
    salesUom: p.salesUom,
    purchaseFactor: p.purchaseFactor,
    salesFactor: p.salesFactor,
    purchasePrice: p.purchasePrice,
    minSellingPrice: p.minSellingPrice,
    type: p.type,
    tracked: p.tracked,
    status: p.status,
    variants: p.variants ?? [],
  };
}
