// Service order beli — HANYA via helper http-client (apiGet/apiPage/
// apiPost/apiPut). Bentuk respons terverifikasi dari backend:
// - GET /api/v1/purchase-orders?search&status&page&limit → paginasi
//   orderView[] (handler.go: List).
// - GET /api/v1/suppliers?search&status&page&limit → paginasi partyView[]
//   (party/presentation/handler.go: list) — dipakai untuk opsi supplier.
// - Opsi produk baris: modules/products/lib/product-picker (GET /products
//   berhalaman, dimuat lagi saat daftar digulir).
// - GET /api/v1/products/:id → productView lengkap (variants, baseUom,
//   purchaseUom, salesUom, purchasePrice) untuk dropdown varian & satuan.

import { apiGet, apiPage, apiPost, apiPostFull, apiPutFull, httpClient } from "@/shared/services/http-client";
import { compactParams, positiveId, sortParams } from "@/shared/lib/list-query";
import type { SortDir } from "@/shared/lib/list-query";
import type {
  PriceCorrectionPayload,
  PurchaseCorrectionPreview,
  PurchaseHeaderPayload,
  PurchaseOrder,
  PurchaseOrderPayload,
  PurchasingProductDetail,
} from "@/modules/purchasing/types";

export interface PurchaseOrderListParams {
  search?: string;
  /** CSV: "confirmed,completed". */
  status?: string;
  paymentTerms?: string;
  partyId?: number;
  /** YYYY-MM-DD (WIB), inklusif. */
  from?: string;
  to?: string;
  sort?: string;
  dir?: SortDir | "";
  page: number;
  limit: number;
}

export type PurchaseOrderFilterParams = Omit<PurchaseOrderListParams, "page" | "limit">;

/** SATU sumber query untuk GET /purchase-orders dan /purchase-orders/export. */
export function buildPurchaseFilterQuery(p: PurchaseOrderFilterParams): Record<string, unknown> {
  return compactParams({
    search: p.search,
    status: p.status,
    paymentTerms: p.paymentTerms,
    partyId: positiveId(p.partyId),
    from: p.from,
    to: p.to,
    ...sortParams(p.sort ?? "", p.dir ?? ""),
  });
}

export function buildPurchaseListQuery(p: PurchaseOrderListParams): Record<string, unknown> {
  const { page, limit, ...filter } = p;
  return { ...buildPurchaseFilterQuery(filter), page, limit };
}

export const purchaseOrderService = {
  list: (p: PurchaseOrderListParams, signal?: AbortSignal) =>
    apiPage<PurchaseOrder>("/api/v1/purchase-orders", buildPurchaseListQuery(p), signal),

  get: (id: number) => apiGet<PurchaseOrder>(`/api/v1/purchase-orders/${id}`),

  create: (body: PurchaseOrderPayload) =>
    apiPostFull<PurchaseOrder>("/api/v1/purchase-orders", body),

  update: (id: number, body: PurchaseOrderPayload) =>
    apiPutFull<PurchaseOrder>(`/api/v1/purchase-orders/${id}`, body),

  // Tanpa body — backend hanya butuh :id (handler.go: Confirm).
  confirm: (id: number) =>
    apiPostFull<PurchaseOrder>(`/api/v1/purchase-orders/${id}/confirm`),

  /** Batal dengan alasan (wajib). */
  cancel: (id: number, reason: string) =>
    apiPostFull<PurchaseOrder>(`/api/v1/purchase-orders/${id}/cancel`, { reason }),

  /** Ubah info dokumen saja (jatuh tempo, faktur, catatan, penanggung jawab). */
  updateHeader: (id: number, body: PurchaseHeaderPayload) =>
    apiPutFull<PurchaseOrder>(`/api/v1/purchase-orders/${id}/header`, body),

  /** Pratinjau "Tutup sisa order". */
  previewShortClose: (id: number) =>
    apiGet<PurchaseCorrectionPreview>(`/api/v1/purchase-orders/${id}/close-short/preview`),

  shortClose: (id: number, reason: string) =>
    apiPostFull<PurchaseOrder>(`/api/v1/purchase-orders/${id}/close-short`, { reason }),

  /** Pratinjau koreksi harga beli (pembagian Persediaan/HPP dihitung server). */
  previewPriceCorrection: (id: number, body: PriceCorrectionPayload) =>
    apiPost<PurchaseCorrectionPreview>(`/api/v1/purchase-orders/${id}/price-corrections/preview`, body),

  correctPrices: (id: number, body: PriceCorrectionPayload) =>
    apiPostFull<PurchaseOrder>(`/api/v1/purchase-orders/${id}/price-corrections`, body),

  /** Unduh daftar order (J1) — blob xlsx/pdf dengan filter yang sama seperti list. */
  downloadExport: async (
    format: "xlsx" | "pdf",
    p: { search?: string; status?: string },
  ): Promise<void> => {
    const params: Record<string, unknown> = { format };
    if (p.search?.trim()) params.search = p.search.trim();
    if (p.status) params.status = p.status;
    const res = await httpClient.get("/api/v1/purchase-orders/export", {
      params,
      responseType: "blob",
    });
    const blob = new Blob([res.data], {
      type:
        format === "pdf"
          ? "application/pdf"
          : "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = format === "pdf" ? "order-beli.pdf" : "order-beli.xlsx";
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  },
};

export const purchasingLookupService = {
  /** Detail produk untuk varian, satuan, dan harga beli acuan. */
  productDetail: (id: number) =>
    apiGet<PurchasingProductDetail>(`/api/v1/products/${id}`),
};
