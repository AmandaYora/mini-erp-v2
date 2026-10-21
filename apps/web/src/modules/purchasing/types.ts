// Cerminan 1:1 dari view backend (purchasing/presentation/handler.go):
// orderView + itemView. Semua kunci JSON camelCase.
//
// Catatan hasil verifikasi (JANGAN ditebak ulang):
// - Supplier = partyId + partyName (TIDAK ada supplierId / objek supplier).
// - Total = grandTotal (TIDAK ada field "total").
// - TIDAK ada receivedQty di item — progres penerimaan dibaca dari status
//   (completed via penerimaan), bukan dari baris order.
// - Status: draft → confirmed → completed / cancelled (contracts/client.go).
// - Confirm: POST /:id/confirm tanpa body. Cancel: POST /:id/cancel dengan
//   body {reason} (wajib).
//
// Hasil verifikasi Tahap C (JANGAN ditebak ulang):
// - orderView + orderPayload membawa supplierInvoiceNumber,
//   supplierInvoiceDate (handler.go: orderView/orderPayload).

export interface PurchaseOrderItem {
  id: number;
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  uom: string;
  uomFactor: number;
  qty: number;
  qtyBase: number;
  unitPrice: number;
  discountPct: number;
  discountNominal: number;
  lineTotal: number;
  // Snapshot baris (orderView items).
  stockTracked: boolean;
  itemType: string;
  variantName: string;
  variantCode: string;
  baseUom: string;
  discountReason: string;
  note: string;
  taxable: boolean;
  /** Tarif pajak yang dipakai baris (PO-03): tarif barisnya, atau tarif order; 0 bila bebas. */
  taxRate: number;
  taxBase: number;
  taxAmount: number;
}

export type PurchasePaymentTerms = "net" | "cod" | "prepaid";

export function purchaseTermsLabel(terms: string | null | undefined): string {
  if (terms === "net") return "Net (tempo)";
  if (terms === "cod") return "COD (tunai)";
  if (terms === "prepaid") return "Prabayar (bayar dulu)";
  return terms ?? "-";
}

export interface PurchaseOrder {
  id: number;
  number: string;
  branchId: number;
  partyId: number;
  partyName: string;
  orderDate: string;
  dueDate: string;
  paymentTerms: string;
  taxType: string;
  taxRate: number;
  subtotal: number;
  discountTotal: number;
  taxTotal: number;
  grandTotal: number;
  status: string;
  notes: string;
  supplierInvoiceNumber: string;
  supplierInvoiceDate: string;
  /** Masa tempo (hari, 0..3650); dengan tanggal faktur supplier menentukan jatuh tempo. */
  paymentTermDays: number;
  /** Faktur pajak supplier (PO-03). */
  taxInvoiceNumber: string;
  taxInvoiceDate: string;
  /** Penanggung jawab order (0 = tidak ada) dan pembuat dokumen (PO-01/PO-03). */
  assignedUserId: number;
  assignedUserName: string;
  createdBy: number;
  createdByName: string;
  /** Status retur terkonfirmasi: none | partial | full — hanya di daftar/detail. */
  returnSummary?: "none" | "partial" | "full";
  // Ringkasan pembayaran — hanya ada di list/get (withPayment), tidak di
  // respons create/update/confirm/cancel.
  /** Uang yang benar-benar dibayar ke pemasok (offset retur tidak termasuk). */
  paidAmount?: number;
  /** Bagian tagihan yang dilunasi retur tanpa uang berpindah. */
  returnOffset?: number;
  outstandingAmount?: number;
  paymentStatus?: string;
  effectiveDueDate?: string;
  isOverdue?: boolean;
  /** Alasan, waktu (WIB), dan pembatal order (kosong bila tidak dibatalkan). */
  cancelReason?: string;
  cancelledAt?: string;
  cancelledByName?: string;
  /** "Tutup sisa order": alasan, waktu (WIB), dan penutup. */
  shortCloseReason?: string;
  shortClosedAt?: string;
  shortClosedByName?: string;
  /** Hanya detail order terkonfirmasi: sudah ada penerimaan/pembayaran/retur. */
  hasDependents?: boolean;
  /** Hanya detail: riwayat koreksi (koreksi harga, tutup sisa order). */
  corrections?: PurchaseOrderCorrection[];
  items: PurchaseOrderItem[];
}

export type PurchaseCorrectionKind = "price" | "short_close";

export interface PurchaseOrderCorrectionLine {
  itemId: number;
  productId: number;
  variantId: number;
  productName: string;
  uom: string;
  stockTracked: boolean;
  receivedQty: number;
  oldQty: number;
  newQty: number;
  oldUnitPrice: number;
  newUnitPrice: number;
  oldDiscountPct: number;
  newDiscountPct: number;
  oldDiscountNominal: number;
  newDiscountNominal: number;
  oldLineTotal: number;
  newLineTotal: number;
  valueDelta: number;
  taxDelta: number;
}

export interface PurchaseOrderCorrection {
  id: number;
  kind: PurchaseCorrectionKind;
  date: string;
  reason: string;
  oldGrandTotal: number;
  newGrandTotal: number;
  valueDelta: number;
  taxDelta: number;
  createdAt: string;
  createdByName: string;
  lines: PurchaseOrderCorrectionLine[];
}

/** Pratinjau koreksi (handler: CorrectionPreview). */
export interface PurchaseCorrectionPreviewLine {
  itemId: number;
  productId: number;
  variantId: number;
  productName: string;
  uom: string;
  stockTracked: boolean;
  qty: number;
  newQty: number;
  receivedQty: number;
  oldUnitPrice: number;
  newUnitPrice: number;
  oldDiscountPct: number;
  newDiscountPct: number;
  oldDiscountNominal: number;
  newDiscountNominal: number;
  oldLineTotal: number;
  newLineTotal: number;
  valueDelta: number;
  taxDelta: number;
}

export interface PurchaseCorrectionPreview {
  kind: PurchaseCorrectionKind;
  lines: PurchaseCorrectionPreviewLine[];
  oldGrandTotal: number;
  newGrandTotal: number;
  paid: number;
  /** Total baru − dibayar; negatif = pemasok kelebihan dibayar. */
  outstanding: number;
  valueDelta: number;
  taxDelta: number;
  payableDelta: number;
  /** Bagian barang yang masih di gudang (Persediaan). */
  stockDelta: number;
  /** Bagian barang yang sudah keluar/terjual (HPP). */
  cogsDelta: number;
  /** Baris jasa (beban). */
  expenseDelta: number;
  correctionId?: number;
}

export interface PurchaseHeaderPayload {
  dueDate?: string;
  paymentTermDays?: number;
  notes?: string;
  supplierInvoiceNumber?: string;
  supplierInvoiceDate?: string;
  taxInvoiceNumber?: string;
  taxInvoiceDate?: string;
  /** 0 = tanpa penanggung jawab. */
  assignedUserId: number;
  reason?: string;
}

export interface PriceCorrectionLinePayload {
  lineId: number;
  unitPrice: number;
  discountPct?: number;
  discountNominal?: number;
}

export interface PriceCorrectionPayload {
  lines: PriceCorrectionLinePayload[];
  reason?: string;
}

// --- payload keluar (kunci persis orderPayload/linePayload di handler.go) ---

export interface PurchaseOrderLinePayload {
  productId: number;
  variantId: number;
  uom: string;
  qty: number;
  unitPrice: number;
  discountPct: number;
  discountNominal: number;
  discountReason?: string;
  note?: string;
  /** false = baris tidak kena pajak order. Dihilangkan = kena pajak. */
  taxable?: boolean;
  /** Tarif pajak baris (0..100) yang menimpa tarif order; dihilangkan = tarif order. */
  taxRate?: number;
}

export interface PurchaseOrderPayload {
  partyId: number;
  orderDate?: string;
  dueDate?: string;
  paymentTerms?: string;
  taxType?: string;
  taxRate?: number;
  notes?: string;
  supplierInvoiceNumber?: string;
  supplierInvoiceDate?: string;
  taxInvoiceNumber?: string;
  taxInvoiceDate?: string;
  /** Penanggung jawab (id user aktif di cabang); 0/dihilangkan = tidak ada. */
  assignedUserId?: number;
  /** 0..3650; net tanpa dueDate butuh paymentTermDays > 0 + supplierInvoiceDate. */
  paymentTermDays?: number;
  /** Alasan mengubah order terkonfirmasi (opsional, diaudit). */
  reason?: string;
  items: PurchaseOrderLinePayload[];
}

// --- lookup untuk line editor ---

// GET /api/v1/products/search-options?q= mengembalikan array langsung
// (response.OK, BUKAN paginasi): [{id, code, name, sellingPrice}].
// TIDAK ada variants / UOM / purchasePrice di sini — dilengkapi via
// GET /api/v1/products/:id (productView).
export interface ProductSearchOption {
  id: number;
  code: string;
  name: string;
  sellingPrice: number;
}

// Subset productView yang dibutuhkan line editor.
export interface PurchasingProductDetail {
  id: number;
  code: string;
  name: string;
  baseUom: string;
  purchaseUom: string;
  salesUom: string;
  purchasePrice: number;
  variants: PurchasingProductVariant[];
}

export interface PurchasingProductVariant {
  id: number;
  code: string;
  name: string;
  isDefault: boolean;
  status: string;
}

// Subset partyView untuk opsi supplier.
export interface SupplierLookup {
  id: number;
  code: string;
  name: string;
}

export const PURCHASE_ORDER_STATUSES = [
  "draft",
  "confirmed",
  "completed",
  "cancelled",
] as const;

export type PurchaseOrderStatus = (typeof PURCHASE_ORDER_STATUSES)[number];

export function isPoStatus(s: string): s is PurchaseOrderStatus {
  return (PURCHASE_ORDER_STATUSES as readonly string[]).includes(s);
}
