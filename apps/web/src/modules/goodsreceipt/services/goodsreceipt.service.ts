// Service penerimaan barang — HANYA via helper http-client
// (apiGet/apiPage/apiPost). Bentuk respons terverifikasi dari backend:
// - GET /api/v1/goods-receipts?purchaseOrderId=&page=&limit= → paginasi
//   receiptView[] (goodsreceipt/presentation/handler.go: List).
// - POST /api/v1/goods-receipts {purchaseOrderId, receivedAt, notes,
//   items[{productId, variantId, locationId, uom, qty}]} (handler.go: Create).
// - GET /api/v1/purchase-orders?status=confirmed → paginasi orderView[]
//   (purchasing/presentation/handler.go: List) untuk SearchSelect PO.
// - GET /api/v1/stock/locations → array langsung StockLocation[]
//   (stock.service.ts: listLocations) untuk dropdown lokasi per baris.
// - POST /api/v1/goods-receipts/:id/complete {settlement?} → posting ulang
//   receipt pending (idempoten); POST :id/discard {reason} → batalkan receipt pending
//   (handler.go: Complete/Discard, izin goodsreceipt.create).

import { apiGet, apiPage, apiPostFull } from "@/shared/services/http-client";
import type {
  GoodsReceipt,
  GoodsReceiptPayload,
  ReceiptSettlementPayload,
  PurchaseOrderOption,
  StockLocationOption,
} from "@/modules/goodsreceipt/types";

export interface GoodsReceiptListParams {
  purchaseOrderId?: number;
  page: number;
  limit: number;
}

function cleanParams(p: GoodsReceiptListParams): Record<string, unknown> {
  const out: Record<string, unknown> = { page: p.page, limit: p.limit };
  if (p.purchaseOrderId && p.purchaseOrderId > 0) {
    out.purchaseOrderId = p.purchaseOrderId;
  }
  return out;
}

export const goodsreceiptService = {
  list: (p: GoodsReceiptListParams, signal?: AbortSignal) =>
    apiPage<GoodsReceipt>("/api/v1/goods-receipts", cleanParams(p), signal),

  get: (id: number) => apiGet<GoodsReceipt>(`/api/v1/goods-receipts/${id}`),

  create: (body: GoodsReceiptPayload) =>
    apiPostFull<GoodsReceipt>("/api/v1/goods-receipts", body),

  /** Posting ulang receipt pending; settlement hanya untuk PO COD yang tuntas. */
  complete: (id: number, settlement?: ReceiptSettlementPayload) =>
    apiPostFull<GoodsReceipt>(
      `/api/v1/goods-receipts/${id}/complete`,
      settlement ? { settlement } : undefined,
    ),

  /** Batalkan receipt pending (stok yang sudah bergerak dibalik server). */
  discard: (id: number, reason: string) =>
    apiPostFull<GoodsReceipt>(`/api/v1/goods-receipts/${id}/discard`, { reason }),

  /** Satu PO beserta barisnya (untuk membangun baris terima + nama produk). */
  purchaseOrderDetail: (id: number) =>
    apiGet<PurchaseOrderOption>(`/api/v1/purchase-orders/${id}`),

  /** Lokasi cabang untuk locationId per baris (array langsung). */
  locationOptions: () =>
    apiGet<StockLocationOption[]>("/api/v1/stock/locations"),
};
