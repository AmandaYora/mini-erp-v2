// Service order beli — HANYA via helper http-client (apiGet/apiPage/
// apiPost/apiPut). Bentuk respons terverifikasi dari backend:
// - GET /api/v1/purchase-orders?search&status&page&limit → paginasi
//   orderView[] (handler.go: List).
// - GET /api/v1/suppliers?search&status&page&limit → paginasi partyView[]
//   (party/presentation/handler.go: list) — dipakai untuk opsi supplier.
// - GET /api/v1/products/search-options?q= → array langsung
//   [{id, code, name, sellingPrice}] (product/presentation/handler.go:
//   SearchOptions, response.OK — BUKAN paginasi).
// - GET /api/v1/products/:id → productView lengkap (variants, baseUom,
//   purchaseUom, salesUom, purchasePrice) untuk dropdown varian & satuan.

import { apiGet, apiPage, apiPostFull, apiPutFull, httpClient } from "@/shared/services/http-client";
import type {
  ProductSearchOption,
  PurchaseOrder,
  PurchaseOrderPayload,
  PurchasingProductDetail,
  SupplierLookup,
} from "@/modules/purchasing/types";

export interface PurchaseOrderListParams {
  search?: string;
  status?: string;
  page: number;
  limit: number;
}

function cleanParams(p: PurchaseOrderListParams): Record<string, unknown> {
  return {
    search: p.search?.trim() ? p.search.trim() : undefined,
    status: p.status ? p.status : undefined,
    page: p.page,
    limit: p.limit,
  };
}

export const purchaseOrderService = {
  list: (p: PurchaseOrderListParams) =>
    apiPage<PurchaseOrder>("/api/v1/purchase-orders", cleanParams(p)),

  get: (id: number) => apiGet<PurchaseOrder>(`/api/v1/purchase-orders/${id}`),

  create: (body: PurchaseOrderPayload) =>
    apiPostFull<PurchaseOrder>("/api/v1/purchase-orders", body),

  update: (id: number, body: PurchaseOrderPayload) =>
    apiPutFull<PurchaseOrder>(`/api/v1/purchase-orders/${id}`, body),

  // Tanpa body — backend hanya butuh :id (handler.go: Confirm/Cancel).
  confirm: (id: number) =>
    apiPostFull<PurchaseOrder>(`/api/v1/purchase-orders/${id}/confirm`),

  cancel: (id: number) =>
    apiPostFull<PurchaseOrder>(`/api/v1/purchase-orders/${id}/cancel`),

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
  /** Supplier aktif untuk SearchSelect (maks 50, filter lanjutan di klien). */
  supplierOptions: () =>
    apiPage<SupplierLookup>("/api/v1/suppliers", {
      status: "active",
      page: 1,
      limit: 50,
    }),

  /** Satu supplier (untuk prefill edit bila tak ada di daftar opsi). */
  supplierById: (id: number) => apiGet<SupplierLookup>(`/api/v1/suppliers/${id}`),

  /** Opsi produk — array langsung, BUKAN paginasi. */
  productOptions: (q?: string) =>
    apiGet<ProductSearchOption[]>(
      "/api/v1/products/search-options",
      q?.trim() ? { q: q.trim() } : undefined,
    ),

  /** Detail produk untuk varian, satuan, dan harga beli acuan. */
  productDetail: (id: number) =>
    apiGet<PurchasingProductDetail>(`/api/v1/products/${id}`),
};
