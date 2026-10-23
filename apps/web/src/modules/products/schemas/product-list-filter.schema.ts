import { z } from "zod";
import { enumOrEmpty, idFilter, pageFilter, searchFilter } from "@/shared/lib/list-schema";

// Filter daftar produk (GET /products). State hidup di URL (`use-url-filters`).
// `view` hanya preferensi tampilan (tabel/galeri), bukan param API.
export const productListFilterSchema = z.object({
  search: searchFilter,
  categoryId: idFilter,
  // Default "active": arsip tidak bercampur. "all" = pilihan eksplisit "Semua status"
  // (nilai terpisah agar tahan di URL walau default-nya "active").
  status: z.enum(["active", "inactive", "archived", "all"]),
  type: enumOrEmpty(["barang", "jasa", "paket", "non_stok"] as const),
  tracked: enumOrEmpty(["true", "false"] as const),
  // Filter dari alert kesehatan data (ADR-0025): tampil sebagai chip yang bisa dihapus.
  priceIssue: enumOrEmpty(["min_above_selling"] as const),
  duplicateName: enumOrEmpty(["1"] as const),
  view: z.enum(["table", "gallery"]),
  page: pageFilter,
});

export type ProductListFilter = z.infer<typeof productListFilterSchema>;

export const PRODUCT_LIST_DEFAULTS: ProductListFilter = {
  search: "",
  categoryId: 0,
  status: "active",
  type: "",
  tracked: "",
  priceIssue: "",
  duplicateName: "",
  view: "table",
  page: 1,
};

/** Nilai param API: "all" berarti tanpa filter status. */
export function productStatusParam(status: string): string {
  return status === "all" ? "" : status;
}
