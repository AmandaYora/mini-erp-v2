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

import { apiGet, apiPage, apiPostFull } from "@/shared/services/http-client";
import type {
  GoodsReceipt,
  GoodsReceiptPayload,
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
  list: (p: GoodsReceiptListParams) =>
    apiPage<GoodsReceipt>("/api/v1/goods-receipts", cleanParams(p)),

  get: (id: number) => apiGet<GoodsReceipt>(`/api/v1/goods-receipts/${id}`),

  create: (body: GoodsReceiptPayload) =>
    apiPostFull<GoodsReceipt>("/api/v1/goods-receipts", body),

  /** PO terkonfirmasi untuk SearchSelect modal Terima (maks 50). */
  purchaseOrderOptions: () =>
    apiPage<PurchaseOrderOption>("/api/v1/purchase-orders", {
      status: "confirmed",
      page: 1,
      limit: 50,
    }),

  /** Satu PO beserta barisnya (untuk membangun baris terima + nama produk). */
  purchaseOrderDetail: (id: number) =>
    apiGet<PurchaseOrderOption>(`/api/v1/purchase-orders/${id}`),

  /** Lokasi cabang untuk locationId per baris (array langsung). */
  locationOptions: () =>
    apiGet<StockLocationOption[]>("/api/v1/stock/locations"),
};
