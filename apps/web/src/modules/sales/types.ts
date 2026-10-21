// Cerminan 1:1 dari view backend (sales/presentation/handler.go: orderView +
// orderPayload/linePayload). Semua kunci JSON camelCase.
// Uang integer rupiah, qty float (mendukung pecahan), tanggal YYYY-MM-DD.
// Harga: aturan member menang, lalu harga manual (unitPrice opsional di
// payload), lalu katalog. Klem harga minimum berlaku untuk manual/katalog.
//
// Hasil verifikasi Tahap C (JANGAN ditebak ulang):
// - orderView membawa shipToAddressId (0 = tanpa alamat kirim), shipToLabel,
//   shipToRecipient, shipToPhone, shipToAddress, taxInvoiceNumber,
//   taxInvoiceDate (handler.go: orderView).
// - orderPayload menerima shipToAddressId, taxInvoiceNumber, taxInvoiceDate.

export interface SalesOrderItem {
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
  // Snapshot baris (F2a-7/10: orderView items).
  stockTracked: boolean;
  itemType: string;
  variantName: string;
  variantCode: string;
  baseUom: string;
  pricingSource: SalesPricingSource | string;
  basePrice: number;
  discountReason: string;
  note: string;
  taxable: boolean;
  taxBase: number;
  taxAmount: number;
}

export type SalesPricingSource = "catalog" | "member_rule" | "manual";

export function pricingSourceLabel(src: string | null | undefined): string {
  if (src === "member_rule") return "Harga member";
  if (src === "manual") return "Harga manual";
  if (src === "catalog") return "Harga katalog";
  return "-";
}

export interface SalesOrder {
  id: number;
  number: string;
  branchId: number;
  partyId: number;
  partyName: string;
  /** Kontak pelanggan sendiri — hanya di detail (GET /sales-orders/{id}); cadangan cetak tanpa alamat kirim. */
  partyPhone?: string;
  partyAddress?: string;
  channel: string;
  memberCode: string;
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
  shipToAddressId: number;
  shipToLabel: string;
  shipToRecipient: string;
  shipToPhone: string;
  shipToAddress: string;
  taxInvoiceNumber: string;
  taxInvoiceDate: string;
  /** UUID klien (idempotensi/POS); kosong bila order dibuat tanpa clientRef. */
  clientRef?: string;
  /** true hanya pada respons replay create (HTTP 200). */
  replayed?: boolean;
  // Ringkasan pembayaran — hanya ada di list/get (payment_view.go), tidak di
  // respons create/update/confirm/cancel/approve-credit: muat ulang detailnya.
  paidAmount?: number;
  outstandingAmount?: number;
  /** unpaid | partial | paid */
  paymentStatus?: string;
  effectiveDueDate?: string;
  isOverdue?: boolean;
  // Ringkasan setelah retur (SL-03): total bersih = total order + penyesuaian retur.
  originalTotal?: number;
  returnAdjustment?: number;
  netTotal?: number;
  returnOffset?: number;
  returnRefunded?: number;
  returnCredit?: number;
  /** Kelebihan bayar yang harus dikembalikan ke pelanggan. */
  refundDue?: number;
  /** Selisih tukar lebih mahal yang belum dipungut. */
  exchangeChargeDue?: number;
  /** settled | overdue | partial | open */
  financialStatus?: string;
  // Siapa di balik order (SL-09, SL-10); nama hanya bila pengguna dikenal.
  createdAt?: string;
  createdBy?: number;
  createdByName?: string;
  assignedUserId?: number;
  assignedUserName?: string;
  creditApprovedBy?: number;
  creditApprovedByName?: string;
  creditApprovedAt?: string;
  /** Hanya di detail: pencatat pembayaran (kasir struk). */
  cashierName?: string;
  // Koreksi (hanya di detail, GET /sales-orders/{id}): mode ubah yang
  // diizinkan server, alasan kunci, dan apakah sisa order boleh ditutup.
  editMode?: SalesEditMode;
  editLockReason?: string;
  canCloseShort?: boolean;
  /** Jejak batal / tutup sisa order (kosong bila tidak pernah). */
  cancelReason?: string;
  cancelledAt?: string;
  cancelledByName?: string;
  shortCloseReason?: string;
  shortClosedAt?: string;
  shortClosedByName?: string;
  items: SalesOrderItem[];
}

/** full = form lengkap; header = hanya kolom ringan (sudah ada dokumen turunan); none = hanya baca. */
export type SalesEditMode = "full" | "header" | "none";

/** GET /sales-orders/{id}/close-short/preview (sales/application/corrections.go ShortClosePlan). */
export interface ShortCloseLine {
  itemId: number;
  productCode: string;
  productName: string;
  variantName: string;
  uom: string;
  orderedQty: number;
  deliveredQty: number;
  oldLineTotal: number;
  newLineTotal: number;
  /** Tidak ada yang terkirim: baris keluar dari order. */
  removed: boolean;
}

export interface ShortClosePlan {
  orderId: number;
  number: string;
  lines: ShortCloseLine[];
  oldGrandTotal: number;
  newSubtotal: number;
  newDiscountTotal: number;
  newTaxTotal: number;
  newGrandTotal: number;
  paidAmount: number;
  outstandingAfter: number;
  /** > 0: pelanggan sudah membayar lebih dari total baru. */
  refundDueAfter: number;
}

/** Satu baris kiriman: tanpa harga (server yang memberi harga). */
export interface SalesOrderLinePayload {
  productId: number;
  variantId: number;
  uom: string;
  qty: number;
  discountPct?: number;
  discountNominal?: number;
  /** Harga manual per satuan (rupiah bulat). Dihilangkan = server memberi harga. */
  unitPrice?: number;
  discountReason?: string;
  note?: string;
  /** false = baris tidak kena pajak order. Dihilangkan = kena pajak. */
  taxable?: boolean;
}

/** Body POST/PUT — kunci persis orderPayload backend. */
export interface SalesOrderPayload {
  partyId: number;
  channel: string;
  orderDate?: string;
  dueDate?: string;
  paymentTerms: string;
  taxType?: string;
  taxRate?: number;
  notes?: string;
  /** 0 = tanpa alamat kirim. */
  shipToAddressId?: number;
  /** Alamat kirim sekali pakai (SL-01), dipakai bila shipToAddressId 0: snapshot ke order saja. */
  shipToLabel?: string;
  shipToRecipient?: string;
  shipToPhone?: string;
  shipToAddress?: string;
  /** Penanggung jawab order (SL-09); 0/dihilangkan = tidak ada. */
  assignedUserId?: number;
  taxInvoiceNumber?: string;
  taxInvoiceDate?: string;
  /** UUID per percobaan simpan; create diulang dengan clientRef sama = replay. */
  clientRef?: string;
  /** Alasan koreksi (opsional, maks 255) — hanya masuk jejak audit. */
  reason?: string;
  /** true = order terkonfirmasi: hanya kolom ringan, tanpa hitung ulang harga. */
  headerOnly?: boolean;
  items: SalesOrderLinePayload[];
}

/** Baris GET /api/v1/products/search-options (product/presentation/handler.go:
 * SearchOptions): {id, code, name, sellingPrice}. */
export interface ProductOption {
  id: number;
  code: string;
  name: string;
  sellingPrice: number;
}

export const PAYMENT_TERMS = ["cod", "net", "prepaid"] as const;

export type PaymentTerms = (typeof PAYMENT_TERMS)[number];

export const SALES_CHANNELS = ["regular", "pos"] as const;

export const SALES_STATUSES = ["draft", "confirmed", "completed", "cancelled"] as const;

export function channelLabel(channel: string | null | undefined): string {
  if (channel === "pos") return "POS";
  if (channel === "regular") return "Reguler";
  return channel ?? "-";
}

export function paymentTermsLabel(terms: string | null | undefined): string {
  if (terms === "net") return "Net / Tempo";
  if (terms === "cod") return "COD / Tunai";
  if (terms === "prepaid") return "Prabayar (bayar dulu)";
  return terms ?? "-";
}

export type SalesNextStep =
  | "confirm"
  | "create_delivery"
  | "confirm_delivery"
  | "pay"
  | "done"
  | "cancelled";

/** GET /api/v1/sales-orders/by-client-ref/{ref}/state (handler.State). */
export interface SalesClientRefState {
  order: SalesOrder;
  deliveries: { id: number; number: string; status: string }[];
  paidAmount: number;
  outstanding: number;
  nextStep: SalesNextStep;
}
