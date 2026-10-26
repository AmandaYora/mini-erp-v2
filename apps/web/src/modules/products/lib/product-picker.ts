import type { AsyncSearchLoadResult } from "@/shared/components/ui";
import { clampSearchLimit, productsService } from "@/modules/products/services/products.service";
import type { ProductSearchParams } from "@/modules/products/services/products.service";
import type { ProductOptionRow } from "@/modules/products/types";

/** Barang per halaman pemilih barang order (halaman berikutnya dimuat saat daftar digulir). */
export const PRODUCT_PICKER_PAGE_SIZE = 30;

/**
 * Satu halaman barang aktif untuk AsyncSearchSelect `paged` (Order Jual / Order Beli).
 * GET /products: pencarian server-side ber-ranking (kode persis, awalan kode, awalan nama)
 * dengan meta halaman. Pemilih yang butuh baris kaya (satuan, varian) memakai searchOptionPage.
 */
export async function loadActiveProductPage(
  search: string,
  signal?: AbortSignal,
  page = 1,
): Promise<AsyncSearchLoadResult> {
  const res = await productsService.list(
    { search, status: "active", page, limit: PRODUCT_PICKER_PAGE_SIZE },
    signal,
  );
  return {
    options: res.items.map((p) => ({ value: p.id, label: `${p.code} — ${p.name}` })),
    hasMore: res.meta.page < res.meta.totalPages,
  };
}

/**
 * Satu halaman baris search-options (kaya: satuan, harga, tipe, varian) untuk pemilih
 * berhalaman. Setiap produk yang cocok terjangkau dengan menggulir — search-options tanpa
 * `page` hanya memberi baris pertama (maks. 30). `hasMore` = halaman penuh; halaman
 * terakhir yang kebetulan penuh berakhir dengan satu halaman kosong.
 */
export async function searchOptionPage(
  search: string,
  page: number,
  params: Omit<ProductSearchParams, "q" | "page">,
  signal?: AbortSignal,
): Promise<{ rows: ProductOptionRow[]; hasMore: boolean }> {
  const limit = clampSearchLimit(params.limit ?? PRODUCT_PICKER_PAGE_SIZE);
  const rows = await productsService.searchOptions({ ...params, q: search || undefined, page, limit }, signal);
  return { rows, hasMore: rows.length >= limit };
}
