// Service pengiriman — HANYA via helper http-client
// (apiGet/apiPage/apiPost/apiUpload). Bentuk respons terverifikasi dari
// backend (delivery/presentation/handler.go):
// - GET /api/v1/deliveries?search&status(CSV)&salesOrderId&salesReturnId&
//   documentKind(order|replacement)&from&to&page&limit → paginasi
//   noteView[] (List). Confirm: POST /:id/confirm tanpa body. Cancel:
//   POST /:id/cancel {reason} — alasan wajib (hanya draft).
// - POST /api/v1/deliveries {salesOrderId, deliveryDate, driverName,
//   vehiclePlate, warehouseStaffName, dropLocationNote, notes,
//   items[{productId, variantId, locationId, uom, qty}]} (Create, draf).
// - GET /api/v1/deliveries/:id/proofs → DeliveryProof[] (Proofs).
// - POST /api/v1/deliveries/:id/proof multipart "file" + "purpose"
//   (delivery_signed_document | delivery_location_photo) → {url}.
// - GET /api/v1/sales-orders?status=confirmed → paginasi orderView[]
//   (sales/presentation/handler.go: List) untuk SearchSelect SO.
// - GET /api/v1/stock/locations → array langsung untuk dropdown lokasi.
// - SJ Langsung (ADR-0022): POST /api/v1/deliveries/direct (CreateDirect,
//   izin delivery.create_direct) dan GET /api/v1/deliveries/open-direct
//   ?partyId&limit (OpenDirect). Daftar menerima origin=order|direct dan
//   unordered=1. Bentuk lengkap di types.ts.

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
  DirectDeliveryPayload,
  OpenDirectResult,
  SalesOrderOption,
  StockLocationOption,
  WorkQueue,
} from "@/modules/delivery/types";
import type { ProductOptionRow } from "@/modules/products/types";

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
  /** Asal SJ: "order" | "direct" (SJ Langsung); kosong = semua. */
  origin?: string;
  /** true = hanya SJ Langsung yang belum diorder (dikirim sebagai unordered=1). */
  unordered?: boolean;
  /** Status tanda tangan penerima: "none" (SJ bersopir tanpa status, alert A11) | "signed" | "missing". */
  signature?: string;
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
    origin: p.origin,
    unordered: p.unordered ? "1" : undefined,
    signature: p.signature,
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

  /** SJ Langsung: surat jalan untuk pelanggan tanpa order (ADR-0022), draf. */
  createDirect: (body: DirectDeliveryPayload) =>
    apiPostFull<DeliveryNote>("/api/v1/deliveries/direct", body),

  /**
   * SJ Langsung terkonfirmasi yang belum diorder (tertua dulu). limit 0 =
   * hanya count/oldestDate (items kosong).
   */
  openDirect: (p?: { partyId?: number; limit?: number }, signal?: AbortSignal) =>
    apiGet<OpenDirectResult>(
      "/api/v1/deliveries/open-direct",
      { partyId: positiveId(p?.partyId), limit: p?.limit },
      signal,
    ),

  // Body opsional (bukti serah terima) — body kosong tetap valid
  // (handler.go: Confirm). Cancel {reason} wajib, hanya draf.
  confirm: (id: number, body?: DeliveryConfirmPayload) =>
    apiPostFull<DeliveryNote>(`/api/v1/deliveries/${id}/confirm`, body),

  cancel: (id: number, reason: string) =>
    apiPostFull<DeliveryNote>(`/api/v1/deliveries/${id}/cancel`, { reason }),

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

  /**
   * Satu produk (nama, kode, varian) dari GET /products/search-options?productId=
   * — untuk melabeli baris SJ Langsung yang belum punya order (noteView tidak
   * membawa nama produk). Produk arsip pun ikut terbaca.
   */
  productOption: (productId: number) =>
    apiGet<ProductOptionRow[]>("/api/v1/products/search-options", { productId, limit: 1 }).then(
      (rows) => rows[0] ?? null,
    ),

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
