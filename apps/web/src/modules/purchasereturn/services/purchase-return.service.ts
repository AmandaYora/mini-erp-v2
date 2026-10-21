// Service retur beli — HANYA via helper http-client (apiGet/apiPage/
// apiPost). Bentuk respons terverifikasi dari backend:
// - GET /api/v1/purchase-returns?purchaseOrderId&status&page&limit →
//   paginasi returnView[] (purchasereturn/presentation/handler.go: List).
//   Filter di server: search (nomor/catatan), status CSV, purchaseOrderId, from, to.
// - GET /api/v1/purchase-returns/:id → returnView (Get).
// - POST /api/v1/purchase-returns {purchaseOrderId, returnDate?, notes?,
//   items[]} → 201 returnView (Create).
// - POST /api/v1/purchase-returns/:id/confirm → returnView, TANPA body.
// - POST /api/v1/purchase-returns/:id/cancel {reason} → returnView (alasan wajib).
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
import { compactParams, positiveId } from "@/shared/lib/list-query";
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

export interface PurchaseReturnFilterParams {
  /** Nomor retur atau catatan. */
  search?: string;
  /** CSV: "draft,confirmed". */
  status?: string;
  /** Filter dokumen sumber (purchaseOrderId backend). */
  purchaseOrderId?: number;
  /** YYYY-MM-DD (WIB), inklusif. */
  from?: string;
  to?: string;
}

export interface PurchaseReturnListParams extends PurchaseReturnFilterParams {
  page: number;
  limit: number;
}

export function buildPurchaseReturnListQuery(p: PurchaseReturnListParams): Record<string, unknown> {
  return compactParams({
    search: p.search,
    status: p.status,
    purchaseOrderId: positiveId(p.purchaseOrderId),
    from: p.from,
    to: p.to,
    page: p.page,
    limit: p.limit,
  });
}

export const purchaseReturnService = {
  list: (p: PurchaseReturnListParams, signal?: AbortSignal) =>
    apiPage<PurchaseReturn>("/api/v1/purchase-returns", buildPurchaseReturnListQuery(p), signal),

  get: (id: number) => apiGet<PurchaseReturn>(`/api/v1/purchase-returns/${id}`),

  create: (body: PurchaseReturnPayload) =>
    apiPostFull<PurchaseReturn>("/api/v1/purchase-returns", body),

  // Confirm tanpa body; Cancel {reason} wajib (handler.go: Confirm/Cancel).
  confirm: (id: number) =>
    apiPostFull<PurchaseReturn>(`/api/v1/purchase-returns/${id}/confirm`),

  cancel: (id: number, reason: string) =>
    apiPostFull<PurchaseReturn>(`/api/v1/purchase-returns/${id}/cancel`, { reason }),

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
