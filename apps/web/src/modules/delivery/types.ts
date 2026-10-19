// Cerminan 1:1 dari view backend
// (apps/api/internal/modules/delivery/presentation/handler.go: noteView +
// Create/Confirm/Cancel/ProofUpload/Proofs). Semua kunci JSON camelCase.
//
// Hasil verifikasi (JANGAN ditebak ulang):
// - noteView = {id, number, branchId, salesOrderId, orderNumber, deliveryDate,
//   status, notes, items[]} — TIDAK ada customer (partyName).
// - Item = {id, productId, variantId, locationId, uom, uomFactor, qty, qtyBase}
//   — TIDAK ada productName.
// - List hanya mendukung page, limit, salesOrderId, status — TIDAK ada search.
// - Status: draft → confirmed (stok keluar saat confirm); cancel hanya untuk
//   draft ("Surat jalan terkonfirmasi tidak dapat dibatalkan (gunakan retur)").
// - Proofs: GET /:id/proofs → [{id, originalName, mime, sizeBytes, url}]
//   (service.go: Proofs). Upload: POST /:id/proof multipart field "file"
//   → {url} (hanya image/, maks 6MB — httpx.MultipartFile).
// - Over-SO ditolak server ("Melebihi sisa SO") — klien menjepit qty ke qty
//   SO per baris sebagai perlindungan awal.
//
// Hasil verifikasi Tahap C (JANGAN ditebak ulang):
// - noteView membawa dokumen & serah terima: driverName, vehiclePlate,
//   warehouseStaffName, recipientName, recipientSignatureStatus,
//   recipientSignatureMissingReason, dropLocationNote, dispatchedAt,
//   dispatchedBy, confirmedAt, confirmedBy (handler.go: noteView).
// - Create menerima driverName (maks 100), vehiclePlate (maks 20),
//   warehouseStaffName (maks 100), dropLocationNote — semua opsional
//   (service.go: validateDoc).
// - Confirm menerima body opsional {recipientName,
//   recipientSignatureStatus: "signed"|"missing",
//   recipientSignatureMissingReason} — body kosong tetap valid; bila status
//   diisi maka nama wajib, bila missing maka alasan wajib (service.go:
//   validateRecipient).

export interface DeliveryItem {
  id: number;
  productId: number;
  variantId: number;
  locationId: number;
  uom: string;
  uomFactor: number;
  qty: number;
  qtyBase: number;
}

export interface DeliveryNote {
  /** true hanya pada respons replay create (HTTP 200, clientRef sama). */
  replayed?: boolean;
  id: number;
  number: string;
  branchId: number;
  salesOrderId: number;
  orderNumber: string;
  deliveryDate: string;
  status: string;
  notes: string;
  driverName: string;
  vehiclePlate: string;
  warehouseStaffName: string;
  recipientName: string;
  recipientSignatureStatus: string;
  recipientSignatureMissingReason: string;
  dropLocationNote: string;
  dispatchedAt: string | null;
  dispatchedBy: number | null;
  confirmedAt: string | null;
  confirmedBy: number | null;
  /** "order" (SJ order) | "replacement" (barang pengganti retur). */
  documentKind?: string;
  salesReturnId?: number;
  /** Peringatan lunak hanya pada respons create (DL-04): [0] = stok belum ditahan, sisanya per posisi. */
  warnings?: string[];
  items: DeliveryItem[];
}

// --- payload keluar (kunci persis struct Create di handler.go) ---

export interface DeliveryLinePayload {
  productId: number;
  variantId: number;
  /** Dihilangkan/0 untuk barang stok = server mengalokasikan lintas lokasi
   * (satu baris bisa menjadi beberapa item di respons). */
  locationId?: number;
  uom: string;
  qty: number;
  /** Tahanan stok (keranjang POS) yang dikonsumsi saat konfirmasi. */
  reservationKey?: string;
}

export interface DeliveryPayload {
  salesOrderId: number;
  deliveryDate?: string;
  driverName?: string;
  vehiclePlate?: string;
  warehouseStaffName?: string;
  dropLocationNote?: string;
  notes?: string;
  /** UUID per percobaan simpan: ulang dengan clientRef sama = replay. */
  clientRef?: string;
  items: DeliveryLinePayload[];
}

export type SettlementMode = "pay_now" | "switch_to_net";

/** Pelunasan order COD yang dituntaskan surat jalan ini (handler.Confirm). */
export interface DeliverySettlementPayload {
  mode: SettlementMode;
  /** cash | transfer | check — kosong = cash (hanya pay_now). */
  method?: "cash" | "transfer" | "check";
  paymentDate?: string;
  reference?: string;
  amountTendered?: number;
  /** Wajib untuk switch_to_net (YYYY-MM-DD). */
  dueDate?: string;
}

/** Body POST /:id/confirm — semua opsional, body kosong tetap valid. */
export interface DeliveryConfirmPayload {
  recipientName?: string;
  recipientSignatureStatus?: "signed" | "missing" | "";
  recipientSignatureMissingReason?: string;
  settlement?: DeliverySettlementPayload;
}

// --- bukti kirim (service.go: Proofs) ---

/** Tujuan bukti kirim (DL-01): scan SJ fisik bertanda tangan (wajib) vs foto lokasi (opsional). */
export type DeliveryProofPurpose = "delivery_signed_document" | "delivery_location_photo";

export interface DeliveryProof {
  id: number;
  originalName: string;
  mime: string;
  sizeBytes: number;
  url: string;
  purpose: DeliveryProofPurpose;
}

// --- lookup SO untuk modal Buat ---
// Subset orderView sales (sales/presentation/handler.go: orderView).
// SO items TIDAK membawa deliveredQty — sisa sejati dihitung server dari
// pengiriman terkonfirmasi, jadi prefill klien = qty SO penuh.

export interface SalesOrderOptionItem {
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

export interface SalesOrderOption {
  id: number;
  number: string;
  partyId: number;
  partyName: string;
  channel: string;
  orderDate: string;
  status: string;
  paymentTerms?: string;
  grandTotal?: number;
  items: SalesOrderOptionItem[];
}

// --- lookup lokasi (subset locationView stock) untuk locationId per baris ---

export interface StockLocationOption {
  id: number;
  code: string;
  name: string;
  status: string;
}

export const DELIVERY_STATUSES = ["draft", "confirmed", "cancelled"] as const;

export type DeliveryStatus = (typeof DELIVERY_STATUSES)[number];

// --- antrean kerja gudang (backend delivery/presentation/handler.go:
// WorkQueue → {createSj[], waitingReturn[], limit}). Semua kunci camelCase.

export interface QueueLine {
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  uom: string;
  ordered: number;
  delivered: number;
  remaining: number;
}

export interface QueueOrder {
  orderId: number;
  number: string;
  partyName: string;
  orderDate: string;
  dueDate: string;
  paymentTerms: string;
  grandTotal: number;
  lines: QueueLine[];
  totalOrdered: number;
  totalDelivered: number;
  pendingDrafts: number;
  /** Jumlah baris produk yang masih harus dikirim (DL-02). */
  itemCount: number;
}

export interface WaitingNote {
  id: number;
  number: string;
  salesOrderId: number;
  orderNumber: string;
  partyName: string;
  deliveryDate: string;
  ageDays: number;
  driverName: string;
  vehiclePlate: string;
  /** Scan/foto SJ bertanda tangan sudah diunggah (DL-01). */
  hasSignedDocument: boolean;
}

export interface WorkQueue {
  createSj: QueueOrder[];
  waitingReturn: WaitingNote[];
  limit: number;
}

export const QUEUE_AGES = [
  { value: "", label: "Semua umur" },
  { value: "today", label: "Hari ini" },
  { value: "gt_2", label: "> 2 hari" },
  { value: "gt_7", label: "> 7 hari" },
] as const;
