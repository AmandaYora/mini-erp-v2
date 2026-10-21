// Hasil POST /stock/scan-product -> tindakan UI (logika murni, diuji di scan.test.ts).
//
// Backend (stock/application/service.go Scan): status "ok" (varian pasti),
// "variant_required" (kode produk memuat >1 varian: pilih dari variants[]),
// "inactive" (dikenal tapi tak bisa dijual). Kode tak dikenal ATAU kode
// varian yang sama dimiliki beberapa produk (ambigu, server tak menebak)
// dijawab 404.

import type { ScanResult, ScanVariantChoice } from "@/modules/pos/types";

export type ScanAction =
  | { kind: "add"; productId: number; variantId: number }
  | {
      kind: "choose";
      productId: number;
      productCode: string;
      productName: string;
      variants: ScanVariantChoice[];
    }
  | { kind: "message"; tone: "warning" | "danger"; text: string };

export function scanAction(res: ScanResult, code: string): ScanAction {
  switch (res.status) {
    case "inactive":
      return {
        kind: "message",
        tone: "warning",
        text: `Produk ${res.product.code} — ${res.product.name} tidak aktif dan tidak bisa dijual.`,
      };
    case "variant_required": {
      const variants = res.variants ?? [];
      if (variants.length === 0) {
        return {
          kind: "message",
          tone: "warning",
          text: `Kode "${code}" memerlukan pilihan varian, tetapi produk ${res.product.code} tidak punya varian aktif.`,
        };
      }
      return {
        kind: "choose",
        productId: res.product.id,
        productCode: res.product.code,
        productName: res.product.name,
        variants,
      };
    }
    case "ok":
    default:
      if (!res.variant) {
        return {
          kind: "message",
          tone: "warning",
          text: `Kode "${code}" dikenali tetapi tidak menunjuk varian tertentu.`,
        };
      }
      return { kind: "add", productId: res.product.id, variantId: res.variant.id };
  }
}

/** Galat pindai: 404 = tak dikenal atau ambigu (dipakai >1 produk); lainnya pesan server. */
export function scanErrorMessage(status: number, serverMessage: string, code: string): string {
  if (status === 404) {
    return `Kode "${code}" tidak dikenal di katalog, atau dipakai oleh lebih dari satu produk (ambigu) — pindai ulang QR produknya atau cari produknya manual.`;
  }
  return serverMessage;
}

/** Label baris pemilih varian. */
export function variantChoiceLabel(v: ScanVariantChoice): string {
  const name = v.name && v.name !== v.code ? `${v.code} — ${v.name}` : v.code;
  return v.barcode ? `${name} (${v.barcode})` : name;
}
