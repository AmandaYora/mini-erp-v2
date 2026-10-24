import type { AsyncSearchLoadResult } from "@/shared/components/ui";
import { productsService } from "@/modules/products/services/products.service";

/** Barang per halaman pemilih barang order (halaman berikutnya dimuat saat daftar digulir). */
export const PRODUCT_PICKER_PAGE_SIZE = 30;

/**
 * Satu halaman barang aktif untuk AsyncSearchSelect `paged` (Order Jual / Order Beli).
 * GET /products: pencarian server-side ber-ranking (kode persis, awalan kode, awalan nama)
 * dengan meta halaman — search-options tidak berhalaman (maks. 30 baris, selalu offset 0).
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
