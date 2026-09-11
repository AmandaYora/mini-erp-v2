// Cerminan 1:1 dari view backend
// (apps/api/internal/modules/goodsreceipt/presentation/handler.go: receiptView
// + Create). Semua kunci JSON camelCase.
//
// Hasil verifikasi (JANGAN ditebak ulang):
// - receiptView = {id, branchId, purchaseOrderId, orderNumber, receivedAt,
//   notes, items[]} — TIDAK ada number / status / supplier.
// - Item = {id, productId, variantId, locationId, uom, uomFactor, qty, qtyBase}
//   — TIDAK ada productName / productCode (ditampilkan via join PO di detail).
// - List hanya mendukung page, limit, purchaseOrderId — TIDAK ada search.
// - RegisterRoutes hanya: GET list, POST create, GET :id. TIDAK ada endpoint
//   confirm/cancel — halaman detail bersifat read-only.
// - Over-receipt ditolak server (service.go: "Melebihi sisa PO") — klien
//   menjepit qty ke qty PO per baris sebagai perlindungan awal.

export interface GoodsReceiptItem {
  id: number;
  productId: number;
  variantId: number;
  locationId: number;
  uom: string;
  uomFactor: number;
  qty: number;
  qtyBase: number;
}

export interface GoodsReceipt {
  id: number;
  branchId: number;
  purchaseOrderId: number;
  orderNumber: string;
  receivedAt: string;
  notes: string;
  items: GoodsReceiptItem[];
}

// --- payload keluar (kunci persis struct Create di handler.go) ---

export interface GoodsReceiptLinePayload {
  productId: number;
  variantId: number;
  locationId: number;
  uom: string;
  qty: number;
}

export interface GoodsReceiptPayload {
  purchaseOrderId: number;
  receivedAt?: string;
  notes?: string;
  items: GoodsReceiptLinePayload[];
}

// --- lookup PO untuk modal Terima ---
// Subset orderView purchasing (purchasing/presentation/handler.go: orderView).
// PO items TIDAK membawa receivedQty — sisa sejati dihitung server dari
// jumlah penerimaan sebelumnya, jadi prefill klien = qty PO penuh.

export interface PurchaseOrderOptionItem {
  id: number;
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  uom: string;
  uomFactor: number;
  qty: number;
  qtyBase: number;
}

export interface PurchaseOrderOption {
  id: number;
  number: string;
  partyId: number;
  partyName: string;
  orderDate: string;
  status: string;
  items: PurchaseOrderOptionItem[];
}

// --- lookup lokasi (subset locationView stock) untuk locationId per baris ---

export interface StockLocationOption {
  id: number;
  code: string;
  name: string;
  status: string;
}
