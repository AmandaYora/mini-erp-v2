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
//
// Hasil verifikasi SJ Langsung / ADR-0022 (JANGAN ditebak ulang):
// - noteView juga membawa origin ("order" | "direct"), partyId, partyName
//   (terisi untuk SJ Langsung; "" untuk SJ order), shipToLabel,
//   shipToRecipient, shipToPhone, shipToAddress, linkedAt ("" sampai SJ
//   Langsung mendapat order), linkedBy (handler.go: noteView).
// - SJ Langsung yang belum diorder: salesOrderId = 0 dan orderNumber = "".
//   Setelah tertaut keduanya terisi dan SJ berperilaku seperti SJ order.
// - Filter daftar tambahan: origin=order|direct dan unordered=1 (SJ Langsung
//   tanpa order, semua status kecuali cancelled).
// - POST /deliveries/direct (izin delivery.create_direct) {partyId wajib,
//   deliveryDate?, shipToAddressId?, shipToLabel?, shipToRecipient?,
//   shipToPhone?, shipToAddress?, driverName?, vehiclePlate?,
//   warehouseStaffName?, dropLocationNote?, notes?, clientRef?,
//   items[{productId, variantId, locationId?, uom, qty}]} → 201 noteView +
//   warnings?[] + replayed?. Aturan alamat kirim sama dengan form order:
//   shipToAddressId memilih alamat tersimpan, else isian ad-hoc (alamat wajib
//   bila ada detail), else tanpa alamat. Hanya barang berstok yang diterima
//   (server menolak jasa/non-stok dengan pesan). Tanpa harga.
// - GET /deliveries/open-direct?partyId=&limit= (izin delivery.view) →
//   {items: noteView[] (SJ Langsung terkonfirmasi tanpa order, tertua dulu),
//   count, oldestDate "YYYY-MM-DD"|""}; limit=0 → hanya count/oldestDate.
// - Confirm SJ Langsung tanpa order: body sama, tetapi `settlement` TIDAK
//   boleh dikirim (400). Cancel {reason}: SJ Langsung tanpa order boleh
//   dibatalkan saat draft DAN confirmed (stok dibalik); setelah punya order
//   berlaku aturan lama (confirmed → via retur).
// - work-queue waitingReturn[] membawa origin; SJ "direct" tanpa order punya
//   salesOrderId 0, orderNumber "" tetapi partyName terisi.

export type DeliveryOrigin = "order" | "direct";

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
  /** "order" (dibuat dari order jual) | "direct" (SJ Langsung, ADR-0022). */
  origin?: DeliveryOrigin;
  /** Pelanggan SJ Langsung (snapshot); 0/"" untuk SJ order. */
  partyId?: number;
  partyName?: string;
  shipToLabel?: string;
  shipToRecipient?: string;
  shipToPhone?: string;
  shipToAddress?: string;
  /** Waktu SJ Langsung mendapat order ("" selama belum diorder). */
  linkedAt?: string;
  linkedBy?: number;
  /** Peringatan lunak hanya pada respons create (DL-04): [0] = stok belum ditahan, sisanya per posisi. */
  warnings?: string[];
  items: DeliveryItem[];
  /** Jejak pembatalan (alasan wajib sejak koreksi salah input); kosong bila tidak/belum dibatalkan. */
  cancelReason?: string;
  /** Waktu batal, "YYYY-MM-DD HH:mm:ss" WIB ("" bila tidak dibatalkan). */
  cancelledAt?: string;
  cancelledBy?: number;
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

/** Body POST /deliveries/direct (handler.go: CreateDirect) — tanpa harga. */
export interface DirectDeliveryPayload {
  partyId: number;
  deliveryDate?: string;
  /** Alamat tersimpan pelanggan; dihilangkan = pakai isian ad-hoc (bila ada). */
  shipToAddressId?: number;
  shipToLabel?: string;
  shipToRecipient?: string;
  shipToPhone?: string;
  shipToAddress?: string;
  driverName?: string;
  vehiclePlate?: string;
  warehouseStaffName?: string;
  dropLocationNote?: string;
  notes?: string;
  /** UUID per form dibuka: ulang dengan clientRef sama = replay. */
  clientRef?: string;
  items: DeliveryLinePayload[];
}

/** GET /deliveries/open-direct — SJ Langsung terkonfirmasi yang belum diorder. */
export interface OpenDirectResult {
  items: DeliveryNote[];
  count: number;
  /** "YYYY-MM-DD" SJ tertua, "" bila tidak ada. */
  oldestDate: string;
}

/** SJ Langsung yang belum mendapat order (salesOrderId masih 0). */
export function isUnorderedDirect(n: Pick<DeliveryNote, "origin" | "salesOrderId">): boolean {
  return n.origin === "direct" && n.salesOrderId === 0;
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
  /** Rekening perusahaan transfer/cek (ADR-0026); kosong = rekening default. */
  cashAccountId?: number;
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
  /** "direct" = SJ Langsung (tanpa order: salesOrderId 0, orderNumber ""). */
  origin?: DeliveryOrigin;
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
