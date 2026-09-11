import { apiDeleteFull, apiGet, apiPage, apiPostFull, apiPutFull, apiUploadFull, httpClient } from "@/shared/services/http-client";
import type {
  ImportPreview,
  ImportResult,
  Product,
  ProductCategory,
  ProductMedia,
  ProductPayload,
} from "@/modules/products/types";

export interface ProductListParams {
  search?: string;
  categoryId?: number | null;
  status?: string;
  page?: number;
  limit?: number;
}

function cleanParams(p: ProductListParams): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  if (p.search && p.search.trim() !== "") out.search = p.search.trim();
  if (p.categoryId) out.categoryId = p.categoryId;
  if (p.status && p.status !== "") out.status = p.status;
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
  list: (params: ProductListParams) =>
    apiPage<Product>("/api/v1/products", cleanParams(params)),

  detail: (id: number) => apiGet<Product>(`/api/v1/products/${id}`),

  create: (body: ProductPayload) => apiPostFull<Product>("/api/v1/products", body),

  update: (id: number, body: ProductPayload) =>
    apiPutFull<Product>(`/api/v1/products/${id}`, body),

  archive: (id: number) => apiPostFull<unknown>(`/api/v1/products/${id}/archive`),

  listCategories: () => apiGet<ProductCategory[]>("/api/v1/product-categories"),

  createCategory: (body: { code: string; name: string; parentId: number | null }) =>
    apiPostFull<ProductCategory>("/api/v1/product-categories", body),

  updateCategory: (id: number, body: { name: string; parentId: number | null }) =>
    apiPutFull<ProductCategory>(`/api/v1/product-categories/${id}`, body),

  archiveCategory: (id: number) =>
    apiPostFull<unknown>(`/api/v1/product-categories/${id}/archive`),

  listMedia: (productId: number) =>
    apiGet<ProductMedia[]>(`/api/v1/products/${productId}/media`),

  uploadMedia: (productId: number, file: File) =>
    apiUploadFull<ProductMedia>(`/api/v1/products/${productId}/media`, toForm(file)),

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
