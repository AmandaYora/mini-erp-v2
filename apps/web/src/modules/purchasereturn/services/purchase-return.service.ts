// Service retur beli — HANYA via helper http-client (apiGet/apiPage/
// apiPost). Bentuk respons terverifikasi dari backend:
// - GET /api/v1/purchase-returns?purchaseOrderId&status&page&limit →
//   paginasi returnView[] (purchasereturn/presentation/handler.go: List).
//   TIDAK ada param search — pencarian nomor difilter sisi klien.
// - GET /api/v1/purchase-returns/:id → returnView (Get).
// - POST /api/v1/purchase-returns {purchaseOrderId, returnDate?, notes?,
//   items[]} → 201 returnView (Create).
// - POST /api/v1/purchase-returns/:id/confirm → returnView, TANPA body.
// - POST /api/v1/purchase-returns/:id/cancel → returnView, TANPA body.
// - Kandidat sumber: GET /api/v1/purchase-orders?status=confirmed|completed
//   (hanya PO terkonfirmasi/selesai yang bisa diretur) + detail
//   GET /api/v1/purchase-orders/:id untuk baris item.
// - Lokasi: GET /api/v1/stock/locations → array langsung (bukan paginasi).
//
// Hasil verifikasi Tahap D (JANGAN ditebak ulang):
// - GET /api/v1/purchase-returns/context?purchaseOrderId= (perm
//   purchasereturn.view) → {purchaseOrderId, orderNumber, status, lines[]
//   sisa per produk, locations[]} (handler.go: ReturnContext).
// - POST /api/v1/purchase-returns/preview (perm purchasereturn.create) →
//   dry-run {subtotal, discountTotal, taxTotal, total, items[]} (tanpa
//   mode/pengganti — sisi beli TIDAK ada exchange).
// - POST /api/v1/purchase-returns/:id/settlements (perm
//   purchasereturn.create) → full returnView (handler.go: AddSettlement;
//   409 bila melebihi sisa).

import { apiGet, apiPage, apiPostFull } from "@/shared/services/http-client";
import type {
  LocationOption,
  PurchaseReturn,
  PurchaseReturnContext,
  PurchaseReturnPayload,
  PurchaseReturnPreview,
  PurchaseReturnPreviewPayload,
  PurchaseReturnSettlementPayload,
  SourcePurchaseOrder,
} from "@/modules/purchasereturn/types";

export interface PurchaseReturnListParams {
  status?: string;
  page: number;
  limit: number;
}

function cleanParams(p: PurchaseReturnListParams): Record<string, unknown> {
  return {
    status: p.status ? p.status : undefined,
    page: p.page,
    limit: p.limit,
  };
}

export const purchaseReturnService = {
  list: (p: PurchaseReturnListParams) =>
    apiPage<PurchaseReturn>("/api/v1/purchase-returns", cleanParams(p)),

  get: (id: number) => apiGet<PurchaseReturn>(`/api/v1/purchase-returns/${id}`),

  create: (body: PurchaseReturnPayload) =>
    apiPostFull<PurchaseReturn>("/api/v1/purchase-returns", body),

  // Tanpa body — backend hanya butuh :id (handler.go: Confirm/Cancel).
  confirm: (id: number) =>
    apiPostFull<PurchaseReturn>(`/api/v1/purchase-returns/${id}/confirm`),

  cancel: (id: number) =>
    apiPostFull<PurchaseReturn>(`/api/v1/purchase-returns/${id}/cancel`),

  /** Pratinjau perhitungan (dry-run, tanpa tulis, tanpa nomor dokumen). */
  preview: (body: PurchaseReturnPreviewPayload) =>
    apiPostFull<PurchaseReturnPreview>("/api/v1/purchase-returns/preview", body),

  /** Sisa bisa-diretur per produk + lokasi aktif untuk PO sumber. */
  context: (purchaseOrderId: number) =>
    apiGet<PurchaseReturnContext>("/api/v1/purchase-returns/context", {
      purchaseOrderId,
    }),

  /** Catat penyelesaian — mengembalikan full returnView. */
  addSettlement: (id: number, body: PurchaseReturnSettlementPayload) =>
    apiPostFull<PurchaseReturn>(`/api/v1/purchase-returns/${id}/settlements`, body),
};

export const purchaseReturnLookupService = {
  /** PO terkonfirmasi + selesai untuk dropdown sumber (maks 50 per status). */
  sourceOrders: async (): Promise<SourcePurchaseOrder[]> => {
    const [confirmed, completed] = await Promise.all([
      apiPage<SourcePurchaseOrder>("/api/v1/purchase-orders", {
        status: "confirmed",
        page: 1,
        limit: 50,
      }),
      apiPage<SourcePurchaseOrder>("/api/v1/purchase-orders", {
        status: "completed",
        page: 1,
        limit: 50,
      }),
    ]);
    return [...confirmed.items, ...completed.items];
  },

  /** Detail PO untuk inisialisasi baris retur (qty di-clamp ke qty order). */
  sourceOrderDetail: (id: number) =>
    apiGet<SourcePurchaseOrder>(`/api/v1/purchase-orders/${id}`),

  /** Lokasi cabang untuk dropdown lokasi per baris (array langsung). */
  locationOptions: async (): Promise<LocationOption[]> => {
    const rows = await apiGet<LocationOption[]>("/api/v1/stock/locations");
    const active = rows.filter((l) => l.status === "active");
    return active.length > 0 ? active : rows;
  },
};
