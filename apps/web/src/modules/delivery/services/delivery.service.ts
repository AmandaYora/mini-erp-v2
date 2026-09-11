// Service pengiriman — HANYA via helper http-client
// (apiGet/apiPage/apiPost/apiUpload). Bentuk respons terverifikasi dari
// backend (delivery/presentation/handler.go):
// - GET /api/v1/deliveries?salesOrderId=&status=&page=&limit= → paginasi
//   noteView[] (List). Confirm: POST /:id/confirm tanpa body. Cancel:
//   POST /:id/cancel tanpa body (hanya draft).
// - POST /api/v1/deliveries {salesOrderId, deliveryDate, driverName,
//   vehiclePlate, warehouseStaffName, dropLocationNote, notes,
//   items[{productId, variantId, locationId, uom, qty}]} (Create, draf).
// - GET /api/v1/deliveries/:id/proofs → DeliveryProof[] (Proofs).
// - POST /api/v1/deliveries/:id/proof multipart field "file" → {url}.
// - GET /api/v1/sales-orders?status=confirmed → paginasi orderView[]
//   (sales/presentation/handler.go: List) untuk SearchSelect SO.
// - GET /api/v1/stock/locations → array langsung untuk dropdown lokasi.

import {
  apiGet,
  apiPage,
  apiPostFull,
  apiUploadFull,
} from "@/shared/services/http-client";
import type {
  DeliveryConfirmPayload,
  DeliveryNote,
  DeliveryPayload,
  DeliveryProof,
  SalesOrderOption,
  StockLocationOption,
  WorkQueue,
} from "@/modules/delivery/types";

export interface DeliveryListParams {
  salesOrderId?: number;
  status?: string;
  page: number;
  limit: number;
}

function cleanParams(p: DeliveryListParams): Record<string, unknown> {
  const out: Record<string, unknown> = { page: p.page, limit: p.limit };
  if (p.salesOrderId && p.salesOrderId > 0) out.salesOrderId = p.salesOrderId;
  if (p.status && p.status !== "") out.status = p.status;
  return out;
}

export const deliveryService = {
  list: (p: DeliveryListParams) =>
    apiPage<DeliveryNote>("/api/v1/deliveries", cleanParams(p)),

  get: (id: number) => apiGet<DeliveryNote>(`/api/v1/deliveries/${id}`),

  create: (body: DeliveryPayload) =>
    apiPostFull<DeliveryNote>("/api/v1/deliveries", body),

  // Body opsional (bukti serah terima) — body kosong tetap valid
  // (handler.go: Confirm). Cancel tanpa body, hanya draf.
  confirm: (id: number, body?: DeliveryConfirmPayload) =>
    apiPostFull<DeliveryNote>(`/api/v1/deliveries/${id}/confirm`, body),

  cancel: (id: number) =>
    apiPostFull<DeliveryNote>(`/api/v1/deliveries/${id}/cancel`),

  proofs: (id: number) =>
    apiGet<DeliveryProof[]>(`/api/v1/deliveries/${id}/proofs`),

  /** Unggah foto bukti kirim — multipart field "file" (httpx.MultipartFile). */
  uploadProof: (id: number, file: File) => {
    const form = new FormData();
    form.append("file", file);
    return apiUploadFull<{ url: string }>(
      `/api/v1/deliveries/${id}/proof`,
      form,
    );
  },

  /** SO terkonfirmasi untuk SearchSelect modal Buat (maks 50). */
  salesOrderOptions: () =>
    apiPage<SalesOrderOption>("/api/v1/sales-orders", {
      status: "confirmed",
      page: 1,
      limit: 50,
    }),

  /** Satu SO beserta barisnya (untuk membangun baris kirim + nama produk). */
  salesOrderDetail: (id: number) =>
    apiGet<SalesOrderOption>(`/api/v1/sales-orders/${id}`),

  /** Lokasi cabang untuk locationId per baris (array langsung). */
  locationOptions: () =>
    apiGet<StockLocationOption[]>("/api/v1/stock/locations"),

  /** Antrian kerja gudang (J3): SO siap kirim + SJ menunggu kembali. */
  workQueue: (p?: {
    search?: string;
    from?: string;
    to?: string;
    age?: string;
    limit?: number;
  }) =>
    apiGet<WorkQueue>("/api/v1/deliveries/work-queue", {
      search: p?.search?.trim() ? p.search.trim() : undefined,
      from: p?.from || undefined,
      to: p?.to || undefined,
      age: p?.age || undefined,
      limit: p?.limit,
    }),
};
