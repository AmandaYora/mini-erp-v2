// Service pengiriman — HANYA via helper http-client
// (apiGet/apiPage/apiPost/apiUpload). Bentuk respons terverifikasi dari
// backend (delivery/presentation/handler.go):
// - GET /api/v1/deliveries?search&status(CSV)&salesOrderId&salesReturnId&
//   documentKind(order|replacement)&from&to&page&limit → paginasi
//   noteView[] (List). Confirm: POST /:id/confirm tanpa body. Cancel:
//   POST /:id/cancel tanpa body (hanya draft).
// - POST /api/v1/deliveries {salesOrderId, deliveryDate, driverName,
//   vehiclePlate, warehouseStaffName, dropLocationNote, notes,
//   items[{productId, variantId, locationId, uom, qty}]} (Create, draf).
// - GET /api/v1/deliveries/:id/proofs → DeliveryProof[] (Proofs).
// - POST /api/v1/deliveries/:id/proof multipart "file" + "purpose"
//   (delivery_signed_document | delivery_location_photo) → {url}.
// - GET /api/v1/sales-orders?status=confirmed → paginasi orderView[]
//   (sales/presentation/handler.go: List) untuk SearchSelect SO.
// - GET /api/v1/stock/locations → array langsung untuk dropdown lokasi.

import {
  apiGet,
  apiPage,
  apiPostFull,
  apiUploadFull,
} from "@/shared/services/http-client";
import { compressImageForUpload } from "@/shared/lib/image-compression";
import { compactParams, positiveId } from "@/shared/lib/list-query";
import type {
  DeliveryConfirmPayload,
  DeliveryNote,
  DeliveryPayload,
  DeliveryProof,
  DeliveryProofPurpose,
  SalesOrderOption,
  StockLocationOption,
  WorkQueue,
} from "@/modules/delivery/types";

export interface DeliveryFilterParams {
  /** Nomor SJ, penerima, sopir, atau plat kendaraan. */
  search?: string;
  /** CSV: "draft,confirmed". */
  status?: string;
  salesOrderId?: number;
  salesReturnId?: number;
  documentKind?: string;
  /** YYYY-MM-DD (WIB), inklusif. */
  from?: string;
  to?: string;
}

export interface DeliveryListParams extends DeliveryFilterParams {
  page: number;
  limit: number;
}

export function buildDeliveryListQuery(p: DeliveryListParams): Record<string, unknown> {
  return compactParams({
    search: p.search,
    status: p.status,
    salesOrderId: positiveId(p.salesOrderId),
    salesReturnId: positiveId(p.salesReturnId),
    documentKind: p.documentKind,
    from: p.from,
    to: p.to,
    page: p.page,
    limit: p.limit,
  });
}

export const deliveryService = {
  list: (p: DeliveryListParams, signal?: AbortSignal) =>
    apiPage<DeliveryNote>("/api/v1/deliveries", buildDeliveryListQuery(p), signal),

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

  /** Unggah bukti kirim — multipart "file" + "purpose" (DL-01; default foto lokasi). */
  uploadProof: async (id: number, file: File, purpose: DeliveryProofPurpose = "delivery_location_photo") => {
    const form = new FormData();
    form.append("purpose", purpose);
    form.append("file", await compressImageForUpload(file)); // OP-03
    return apiUploadFull<{ url: string }>(
      `/api/v1/deliveries/${id}/proof`,
      form,
    );
  },

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
