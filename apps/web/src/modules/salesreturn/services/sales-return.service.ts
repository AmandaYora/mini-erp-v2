// Service retur jual — HANYA via helper http-client (apiGet/apiPage/
// apiPost). Bentuk respons terverifikasi dari backend:
// - GET /api/v1/sales-returns?salesOrderId&status&page&limit → paginasi
//   returnView[] (salesreturn/presentation/handler.go: List). TIDAK ada
//   param search — pencarian nomor difilter sisi klien.
// - GET /api/v1/sales-returns/:id → returnView (Get).
// - POST /api/v1/sales-returns {salesOrderId, returnDate?, notes?,
//   returnMode?, items[], replacementItems?[]} → 201 returnView (Create).
// - POST /api/v1/sales-returns/:id/confirm → returnView, TANPA body.
// - POST /api/v1/sales-returns/:id/cancel → returnView, TANPA body.
// - Kandidat sumber: GET /api/v1/sales-orders?status=confirmed|completed
//   (hanya SO terkonfirmasi/selesai yang bisa diretur) + detail
//   GET /api/v1/sales-orders/:id untuk baris item.
// - Lokasi: GET /api/v1/stock/locations → array langsung (bukan paginasi).
//
// Hasil verifikasi Tahap D (JANGAN ditebak ulang):
// - GET /api/v1/sales-returns/context?salesOrderId= (perm salesreturn.view)
//   → {salesOrderId, orderNumber, status, lines[] sisa per produk, locations[]}
//   (handler.go: ReturnContext).
// - POST /api/v1/sales-returns/preview (perm salesreturn.create) → dry-run
//   {mode, subtotal, discountTotal, taxTotal, total, items[],
//   replacementItems[], replacementTotal} (handler.go: Preview).
// - POST /api/v1/sales-returns/:id/replacement-deliveries (perm
//   salesreturn.create, body opsional penuh) → {id, number, status}
//   (handler.go: DispatchReplacement).
// - POST /:id/replacement-deliveries/{deliveryId}/confirm (perm
//   salesreturn.create, body opsional) → {id, number, status}. deliveryId
//   TIDAK ada di view retur — resolve via GET /api/v1/deliveries?
//   salesOrderId= lalu cocokkan documentKind==="replacement" &&
//   salesReturnId===id (delivery/presentation/handler.go: noteView).
// - POST /api/v1/sales-returns/:id/settlements (perm salesreturn.create)
//   → full returnView (handler.go: AddSettlement; 409 bila melebihi sisa).
// - Opsi produk pengganti: GET /api/v1/products/search-options?q= (array
//   {id, code, name}) + detail GET /api/v1/products/:id untuk varian/satuan
//   (perm products.view) — diambil lokal di service ini agar modul tetap
//   mandiri (tanpa impor service modul lain).

import { apiGet, apiPage, apiPostFull } from "@/shared/services/http-client";
import type {
  LocationOption,
  ReplacementConfirmPayload,
  ReplacementDeliveryNote,
  ReplacementDeliveryResult,
  ReplacementDispatchPayload,
  ReplacementProductDetail,
  ReplacementProductOption,
  ReturnSettlementPayload,
  SalesReturn,
  SalesReturnContext,
  SalesReturnPayload,
  SalesReturnPreview,
  SalesReturnPreviewPayload,
  SourceSalesOrder,
} from "@/modules/salesreturn/types";

export interface SalesReturnListParams {
  status?: string;
  page: number;
  limit: number;
}

function cleanParams(p: SalesReturnListParams): Record<string, unknown> {
  return {
    status: p.status ? p.status : undefined,
    page: p.page,
    limit: p.limit,
  };
}

export const salesReturnService = {
  list: (p: SalesReturnListParams) =>
    apiPage<SalesReturn>("/api/v1/sales-returns", cleanParams(p)),

  get: (id: number) => apiGet<SalesReturn>(`/api/v1/sales-returns/${id}`),

  create: (body: SalesReturnPayload) =>
    apiPostFull<SalesReturn>("/api/v1/sales-returns", body),

  // Tanpa body — backend hanya butuh :id (handler.go: Confirm/Cancel).
  confirm: (id: number) =>
    apiPostFull<SalesReturn>(`/api/v1/sales-returns/${id}/confirm`),

  cancel: (id: number) =>
    apiPostFull<SalesReturn>(`/api/v1/sales-returns/${id}/cancel`),

  /** Pratinjau perhitungan (dry-run, tanpa tulis, tanpa nomor dokumen). */
  preview: (body: SalesReturnPreviewPayload) =>
    apiPostFull<SalesReturnPreview>("/api/v1/sales-returns/preview", body),

  /** Sisa bisa-diretur per produk + lokasi aktif untuk SO sumber. */
  context: (salesOrderId: number) =>
    apiGet<SalesReturnContext>("/api/v1/sales-returns/context", {
      salesOrderId,
    }),

  /** Kirim barang pengganti — semua field body opsional (bare POST valid). */
  dispatchReplacement: (id: number, body?: ReplacementDispatchPayload) =>
    apiPostFull<ReplacementDeliveryResult>(
      `/api/v1/sales-returns/${id}/replacement-deliveries`,
      body ?? {},
    ),

  /** Konfirmasi terima pengganti — body opsional (bare POST valid). */
  confirmReplacement: (
    id: number,
    deliveryId: number,
    body?: ReplacementConfirmPayload,
  ) =>
    apiPostFull<ReplacementDeliveryResult>(
      `/api/v1/sales-returns/${id}/replacement-deliveries/${deliveryId}/confirm`,
      body ?? {},
    ),

  /** Catat penyelesaian — mengembalikan full returnView. */
  addSettlement: (id: number, body: ReturnSettlementPayload) =>
    apiPostFull<SalesReturn>(`/api/v1/sales-returns/${id}/settlements`, body),

  /**
   * Cari surat jalan pengganti milik retur ini. Backend TIDAK menyimpan ID-nya
   * di view retur, tapi surat jalan pengganti membawa salesOrderId SO yang
   * sama + documentKind "replacement" + salesReturnId (delivery noteView).
   */
  findReplacementDelivery: async (
    salesOrderId: number,
    salesReturnId: number,
  ): Promise<ReplacementDeliveryNote | null> => {
    const res = await apiPage<ReplacementDeliveryNote>("/api/v1/deliveries", {
      salesOrderId,
      page: 1,
      limit: 50,
    });
    return (
      res.items.find(
        (n) =>
          n.documentKind === "replacement" && n.salesReturnId === salesReturnId,
      ) ?? null
    );
  },
};

export const salesReturnLookupService = {
  /** SO terkonfirmasi + selesai untuk dropdown sumber (maks 50 per status). */
  sourceOrders: async (): Promise<SourceSalesOrder[]> => {
    const [confirmed, completed] = await Promise.all([
      apiPage<SourceSalesOrder>("/api/v1/sales-orders", {
        status: "confirmed",
        page: 1,
        limit: 50,
      }),
      apiPage<SourceSalesOrder>("/api/v1/sales-orders", {
        status: "completed",
        page: 1,
        limit: 50,
      }),
    ]);
    return [...confirmed.items, ...completed.items];
  },

  /** Detail SO untuk inisialisasi baris retur (qty di-clamp ke qty order). */
  sourceOrderDetail: (id: number) =>
    apiGet<SourceSalesOrder>(`/api/v1/sales-orders/${id}`),

  /** Lokasi cabang untuk dropdown lokasi per baris (array langsung). */
  locationOptions: async (): Promise<LocationOption[]> => {
    const rows = await apiGet<LocationOption[]>("/api/v1/stock/locations");
    const active = rows.filter((l) => l.status === "active");
    return active.length > 0 ? active : rows;
  },

  /** Opsi produk untuk editor barang pengganti (array langsung). */
  productOptions: (q?: string) =>
    apiGet<ReplacementProductOption[]>("/api/v1/products/search-options", {
      q: q?.trim() ? q.trim() : undefined,
    }),

  /** Detail produk untuk varian/satuan baris pengganti. */
  productDetail: (id: number) =>
    apiGet<ReplacementProductDetail>(`/api/v1/products/${id}`),
};
