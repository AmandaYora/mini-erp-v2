import type { Product } from "@/modules/products/types";

// Logika murni kolom stok daftar produk (PR-05), diuji di product-list-view.test.ts.

/** Stok tidak dipantau → tak ada angka; selebihnya angka dari server (bisa belum dimuat). */
export function stockCell(p: Product): { text: string; below: boolean; tracked: boolean } {
  if (!p.tracked || p.type !== "barang") return { text: "", below: false, tracked: false };
  if (p.stockAvailable === undefined) return { text: "", below: false, tracked: true };
  const qty = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 }).format(p.stockAvailable);
  return { text: `${qty} ${p.stockUom ?? ""}`.trim(), below: p.belowMinStock === true, tracked: true };
}

/** Teks "Min X" dalam satuan jual (minimum disimpan dalam satuan stok); kosong bila tak ada minimum. */
export function minStockText(p: Product): string {
  if (!p.tracked || !(p.minStock > 0)) return "";
  const factor = p.salesFactor > 0 ? p.salesFactor : 1;
  const qty = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 }).format(p.minStock / factor);
  return `Min ${qty}`;
}
