// Cerminan 1:1 dari view backend
// (apps/api/internal/modules/goodsreceipt/presentation/handler.go: receiptView
// + Create). Semua kunci JSON camelCase.
//
// Hasil verifikasi (JANGAN ditebak ulang):
// - receiptView = {id, branchId, purchaseOrderId, number, status,
//   supplierDeliveryNumber, orderNumber, receivedAt, notes, items[]}.
//   status: pending (tersimpan, stok belum masuk) | posted | void.
// - Item = {id, productId, variantId, locationId, uom, uomFactor, qty, qtyBase}
//   — TIDAK ada productName / productCode (ditampilkan via join PO di detail).
// - List hanya mendukung page, limit, purchaseOrderId — TIDAK ada search.
// - Rute: GET list, POST create, GET :id, POST :id/complete (posting ulang
//   receipt pending, idempoten), POST :id/discard (batalkan pending, stok
//   yang sudah bergerak dibalik). Izin: goodsreceipt.create untuk keduanya.
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

export type GoodsReceiptStatus = "pending" | "posted" | "void";

export interface GoodsReceipt {
  id: number;
  branchId: number;
  purchaseOrderId: number;
  number: string;
  status: GoodsReceiptStatus | string;
  supplierDeliveryNumber: string;
  orderNumber: string;
  receivedAt: string;
  notes: string;
  /** Jumlah baris, penerima (user pencatat) — diisi list/detail (GR-01). */
  lineCount: number;
  receivedBy: number;
  receivedByName: string;
  /** Pembayaran pay_now yang dibuat panggilan ini (GR-02); tidak ada pada baca ulang. */
  payment?: ReceiptPayment;
  items: GoodsReceiptItem[];
  /** Jejak pembatalan (alasan wajib sejak koreksi salah input); kosong bila tidak/belum dibatalkan. */
  cancelReason?: string;
  /** Waktu batal, "YYYY-MM-DD HH:mm:ss" WIB ("" bila tidak dibatalkan). */
  cancelledAt?: string;
  cancelledBy?: number;
}

/** Pembayaran hasil pelunasan saat terima: dasar melampirkan bukti transfer. */
export interface ReceiptPayment {
  id: number;
  number: string;
}

// --- payload keluar (kunci persis struct Create di handler.go) ---

export interface GoodsReceiptLinePayload {
  productId: number;
  variantId: number;
  /** Dihilangkan untuk barang non-stok. */
  locationId?: number;
  uom: string;
  qty: number;
}

/** Pelunasan PO COD yang dituntaskan penerimaan ini (handler settlementPayload). */
export interface ReceiptSettlementPayload {
  mode: "pay_now" | "switch_to_net";
  /** cash | transfer | check — kosong = cash (hanya pay_now). */
  method?: "cash" | "transfer" | "check";
  paymentDate?: string;
  reference?: string;
  /** Wajib untuk switch_to_net (YYYY-MM-DD). */
  dueDate?: string;
  /** Catatan pembayaran (pay_now, maks 255). */
  notes?: string;
}

export interface GoodsReceiptPayload {
  purchaseOrderId: number;
  receivedAt?: string;
  notes?: string;
  /** Nomor surat jalan supplier (maks 60). */
  supplierDeliveryNumber?: string;
  /** UUID per pembukaan modal; sama = melanjutkan penerimaan yang sama. */
  clientRef?: string;
  settlement?: ReceiptSettlementPayload;
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
  /** false = barang/jasa non-stok: lokasi tidak diperlukan. */
  stockTracked?: boolean;
}

export interface PurchaseOrderOption {
  id: number;
  number: string;
  partyId: number;
  partyName: string;
  orderDate: string;
  status: string;
  paymentTerms?: string;
  grandTotal?: number;
  items: PurchaseOrderOptionItem[];
}

// --- lookup lokasi (subset locationView stock) untuk locationId per baris ---

export interface StockLocationOption {
  id: number;
  code: string;
  name: string;
  status: string;
}
