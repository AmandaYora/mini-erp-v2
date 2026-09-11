// Target label QR cetak (J2). Cerminan aturan server
// (product/application/qr.go: assembleQRTargets) untuk layout cetak:
// produk bervarian → satu label per varian aktif, else satu label produk.
// Isi = barcode bila ada, else kode mentah.

import type { Product } from "@/modules/products/types";

export interface LabelTarget {
  productId: number;
  variantId: number | null;
  code: string;
  name: string;
  payload: string;
}

export function expandLabelTargets(p: Product): LabelTarget[] {
  const variants = (p.variants ?? []).filter((v) => v.status === "active");
  if (variants.length > 0) {
    return variants.map((v) => ({
      productId: p.id,
      variantId: v.id,
      code: v.code,
      name: `${p.name} - ${v.name || v.code}`,
      payload: v.barcode?.trim() ? v.barcode.trim() : v.code,
    }));
  }
  return [
    {
      productId: p.id,
      variantId: null,
      code: p.code,
      name: p.name,
      payload: p.code,
    },
  ];
}
