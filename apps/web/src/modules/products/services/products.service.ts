import { apiDeleteFull, apiGet, apiPage, apiPostFull, apiPutFull, apiUploadFull, httpClient } from "@/shared/services/http-client";
import { compressImageForUpload } from "@/shared/lib/image-compression";
import type {
  ImportPreview,
  ImportResult,
  Product,
  ProductCategory,
  ProductMedia,
  ProductOptionRow,
  ProductPayload,
  ProductScanLookup,
  ProductStockBalance,
} from "@/modules/products/types";

export interface ProductListParams {
  search?: string;
  categoryId?: number | null;
  status?: string;
  /** barang | jasa | paket | non_stok */
  type?: string;
  /** "true" | "false" | "" (semua). */
  tracked?: string;
  page?: number;
  limit?: number;
}

export interface ProductSearchParams {
  q?: string;
  /** Backend membatasi 1-30 (default 20). */
  limit?: number;
  status?: string;
  type?: string;
  tracked?: boolean;
  categoryId?: number | null;
  /** Memulihkan SATU opsi terpilih tanpa mencari ulang (juga produk yang sudah nonaktif/arsip). */
  productId?: number | null;
  /** Produk dengan stok tersedia di cabang aktif naik ke atas (relevansi tetap menang). */
  preferStock?: boolean;
}

/** Jepit limit search-options ke rentang yang diterima server (1-30). */
export function clampSearchLimit(limit: number | undefined): number {
  if (limit === undefined || !Number.isFinite(limit)) return 20;
  return Math.min(30, Math.max(1, Math.trunc(limit)));
}

function cleanParams(p: ProductListParams): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  if (p.search && p.search.trim() !== "") out.search = p.search.trim();
  if (p.categoryId) out.categoryId = p.categoryId;
  // "all" = Semua status (tanpa filter); kosong juga tanpa filter.
  if (p.status && p.status !== "" && p.status !== "all") out.status = p.status;
  if (p.type && p.type !== "") out.type = p.type;
  if (p.tracked === "true" || p.tracked === "false") out.tracked = p.tracked;
  out.page = p.page ?? 1;
  out.limit = p.limit ?? 20;
  return out;
}

function toForm(file: File): FormData {
  const form = new FormData();
  form.append("file", file);
  return form;
}

export const productsService = {
  list: (params: ProductListParams, signal?: AbortSignal) =>
    apiPage<Product>("/api/v1/products", cleanParams(params), signal),

  /** Baris ringkas kaya (satuan, faktor, harga, tipe, varian) untuk pemilih. */
  searchOptions: (params: ProductSearchParams, signal?: AbortSignal) =>
    apiGet<ProductOptionRow[]>(
      "/api/v1/products/search-options",
      {
        q: params.q?.trim() ? params.q.trim() : undefined,
        limit: clampSearchLimit(params.limit),
        status: params.status || undefined,
        type: params.type || undefined,
        tracked: params.tracked === undefined ? undefined : String(params.tracked),
        categoryId: params.categoryId || undefined,
        productId: params.productId || undefined,
        preferStock: params.preferStock ? true : undefined,
      },
      signal,
    ),

  detail: (id: number) => apiGet<Product>(`/api/v1/products/${id}`),

  /** Arti satu kode pindai (barcode/kode varian/kode produk). 404 = tak dikenal atau ambigu; butuh products.view saja. */
  scanLookup: (code: string) =>
    apiGet<ProductScanLookup>("/api/v1/products/scan", { code: code.trim() }),

  create: (body: ProductPayload) => apiPostFull<Product>("/api/v1/products", body),

  update: (id: number, body: ProductPayload) =>
    apiPutFull<Product>(`/api/v1/products/${id}`, body),

  archive: (id: number) => apiPostFull<unknown>(`/api/v1/products/${id}/archive`),

  /** Saldo stok produk per varian/lokasi (perlu izin stock.view); tanpa productId server membalas []. */
  stockBalances: (productId: number) =>
    apiGet<ProductStockBalance[]>("/api/v1/stock/balances", { productId }),

  listCategories: () => apiGet<ProductCategory[]>("/api/v1/product-categories"),

  createCategory: (body: { code: string; name: string; parentId: number | null; sortOrder?: number }) =>
    apiPostFull<ProductCategory>("/api/v1/product-categories", body),

  updateCategory: (id: number, body: { name: string; parentId: number | null; sortOrder?: number }) =>
    apiPutFull<ProductCategory>(`/api/v1/product-categories/${id}`, body),

  archiveCategory: (id: number) =>
    apiPostFull<unknown>(`/api/v1/product-categories/${id}/archive`),

  listMedia: (productId: number) =>
    apiGet<ProductMedia[]>(`/api/v1/products/${productId}/media`),

  // OP-03: foto produk dikompres di klien dulu (gagal → berkas asli).
  uploadMedia: async (productId: number, file: File, opts?: { isPrimary?: boolean; sortOrder?: number }) => {
    const form = toForm(await compressImageForUpload(file));
    if (opts?.isPrimary !== undefined) form.append("is_primary", String(opts.isPrimary));
    if (opts?.sortOrder !== undefined) form.append("sort_order", String(opts.sortOrder));
    return apiUploadFull<ProductMedia>(`/api/v1/products/${productId}/media`, form);
  },

  setPrimaryMedia: (productId: number, mediaId: number) =>
    apiPostFull<unknown>(`/api/v1/products/${productId}/media/${mediaId}/primary`),

  deleteMedia: (productId: number, mediaId: number) =>
    apiDeleteFull<unknown>(`/api/v1/products/${productId}/media/${mediaId}`),

  previewImport: (file: File) =>
    apiUploadFull<ImportPreview>("/api/v1/product-imports/preview", toForm(file)),

  commitImport: (file: File) =>
    apiUploadFull<ImportResult>("/api/v1/product-imports/commit", toForm(file)),

  /** Unduh template Excel — blob, bukan envelope JSON. */
  downloadTemplate: async (): Promise<void> => {
    const res = await httpClient.get("/api/v1/product-imports/template", {
      responseType: "blob",
    });
    const blob = new Blob([res.data], {
      type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "template-produk.xlsx";
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  },

  /** Ambil PNG QR satu label sebagai blob URL (endpoint ber-permission,
   *  jadi <img> langsung tidak bisa — tanpa header Authorization). */
  qrPngUrl: async (productId: number, variantId?: number): Promise<string> => {
    const res = await httpClient.get(`/api/v1/products/${productId}/qr`, {
      params: variantId ? { variantId, size: 480 } : { size: 480 },
      responseType: "blob",
    });
    return URL.createObjectURL(
      new Blob([res.data], { type: "image/png" }),
    );
  },

  /** Unduh satu PNG QR label. */
  downloadQr: async (
    productId: number,
    fileName: string,
    variantId?: number,
  ): Promise<void> => {
    const url = await productsService.qrPngUrl(productId, variantId);
    const a = document.createElement("a");
    a.href = url;
    a.download = fileName;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  },

  /** Unduh ZIP seluruh label QR katalog (J2). */
  downloadQrZip: async (): Promise<void> => {
    const res = await httpClient.get("/api/v1/products/qr-codes/export", {
      responseType: "blob",
    });
    const blob = new Blob([res.data], { type: "application/zip" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "Produk-QR-Codes.zip";
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  },
};
