// Cerminan 1:1 dari view backend (salesreturn/presentation/handler.go:
// returnView + Create). Semua kunci JSON camelCase.
//
// Hasil verifikasi (JANGAN ditebak ulang):
// - View: {id, number, branchId, salesOrderId, orderNumber, returnDate,
//   subtotal, discountTotal, taxTotal, taxType, taxRate, total, status, notes,
//   items[]:{id, productId, variantId, locationId, uom, uomFactor, qty,
//   qtyBase, unitPrice, discountPct, discountNominal, taxBase, taxAmount,
//   lineTotal}} (rincian D1 sejak Tahap B).
// - Create body: {salesOrderId!, returnDate?, notes?, items[]:{productId!,
//   variantId!, locationId!, uom!, qty!}} — TIDAK ada deliveryId/items[]
//   tanpa harga (harga = snapshot SO di server).
// - Sumber valid: SO confirmed/completed (application/service.go
//   Create) — kandidat: GET /sales-orders?status=confirmed(+completed).
// - Batas qty server: terkirim−diretur per (product, variant); TIDAK ada
//   key sisa di view mana pun → clamp klien = qty order, server final.
// - Confirm/Cancel: POST /:id/confirm & POST /:id/cancel TANPA body.
// - Status: draft → confirmed / cancelled (contracts/client.go).
// - Uang integer rupiah, qty float, tanggal YYYY-MM-DD.
//
// Hasil verifikasi Tahap D (JANGAN ditebak ulang):
// - View membawa returnMode ("return_only"|"exchange",
//   contracts/client.go: ReturnModeReturnOnly/Exchange; kosong = return_only),
//   replacementDeliveryStatus ("not_required"|"pending"|"dispatched"|
//   "confirmed"), replacementItems[] {id, productId, variantId, locationId,
//   uom, uomFactor, qty, qtyBase, unitPrice, lineTotal} (TANPA diskon/pajak —
//   harga katalog saat buat), settlements[] {id, type, date, amount,
//   paymentMethod, referenceNumber, notes} + settledTotal + unsettledTotal
//   (handler.go: returnView; settled = jumlah amount di server).
// - Create menerima returnMode? + replacementItems?[] (wajib diisi iff mode
//   exchange, dilarang iff return_only — service.go: normalizeMode).
// - Preview: POST /preview {salesOrderId!, returnMode?, items[]!,
//   replacementItems?[]} → {mode, subtotal, discountTotal, taxTotal, total,
//   items[], replacementItems[], replacementTotal} (dry-run, tanpa tulis).
// - Context: GET /context?salesOrderId= → {salesOrderId, orderNumber,
//   status, lines[{productId, variantId, productCode, productName, uom,
//   uomFactor, orderedQty, delivered, returned, remaining}], locations[{
//   id, code, name}]} (service.go: ReturnContext; remaining =
//   terkirim−diretur, lokasi aktif saja).
// - Dispatch pengganti: POST /:id/replacement-deliveries (semua field
//   opsional; bare POST valid) → {id, number, status}. Syarat: mode
//   exchange + retur confirmed + status pending (service.go:
//   CreateReplacementDelivery). Stok dicek saat dispatch (saldo cukup +
//   harga modal katalog > 0) tapi stok baru keluar saat confirm.
// - Confirm pengganti: POST /:id/replacement-deliveries/{deliveryId}/confirm
//   (body opsional, bare POST valid) → {id, number, status}. ID surat jalan
//   pengganti TIDAK ada di view retur — resolve via
//   GET /api/v1/deliveries?salesOrderId= (noteView: documentKind +
//   salesReturnId).
// - Settlement: POST /:id/settlements {type!, date?, amount!, paymentMethod?,
//   referenceNumber?, notes?} → full returnView. Syarat: retur confirmed.
//   K-04c: jenis sah menurut ARAH selisih — retur murni / tukar lebih murah:
//   refund | customer_credit (nominal <= externalRemaining = nilai terutang −
//   pengurang piutang otomatis − yang sudah diselesaikan); tukar lebih mahal:
//   collect_payment saja (nominal <= chargeRemaining). reduce_receivable =
//   offset otomatis saat konfirmasi, tidak bisa dicatat manual. customer_credit
//   butuh order ber-customer dan dijurnal (Piutang → Uang Muka Pelanggan, K-04a).
// - K-04c/d: view membawa externalRemaining, chargeRemaining,
//   allowedSettlementTypes; preview membawa difference, owedToCustomer,
//   chargeToCustomer, orderOutstanding, receivableReduction, externalRemaining,
//   allowedSettlementTypes, hasParty (angka sama dengan hasil konfirmasi).
// - K-04e: confirm pengganti menerima settlement? {mode: "pay_now", amount?,
//   method?, paymentDate?, reference?, amountTendered?} untuk menagih selisih
//   tukar yang lebih mahal saat serah terima (butuh izin payment.create).

export interface SalesReturnItem {
  id: number;
  productId: number;
  variantId: number;
  locationId: number;
  uom: string;
  uomFactor: number;
  qty: number;
  qtyBase: number;
  unitPrice: number;
  discountPct: number;
  discountNominal: number;
  taxBase: number;
  taxAmount: number;
  lineTotal: number;
  /** normal | damaged | not_returned. */
  condition: string;
}

/** Satu baris barang pengganti (sejak F2a membawa dasar pajak + PPN). */
export interface SalesReturnReplacementItem {
  id: number;
  productId: number;
  variantId: number;
  locationId: number;
  uom: string;
  uomFactor: number;
  qty: number;
  qtyBase: number;
  unitPrice: number;
  taxBase: number;
  taxAmount: number;
  lineTotal: number;
}

/** Satu baris memo penyelesaian (uang riil tetap lewat modul payment). */
export interface ReturnSettlement {
  id: number;
  type: string;
  date: string;
  amount: number;
  paymentMethod: string;
  referenceNumber: string;
  notes: string;
}

export interface SalesReturn {
  id: number;
  number: string;
  branchId: number;
  salesOrderId: number;
  orderNumber: string;
  returnDate: string;
  subtotal: number;
  discountTotal: number;
  taxTotal: number;
  taxType: string;
  taxRate: number;
  total: number;
  status: string;
  notes: string;
  reason: string;
  replacementSubtotal: number;
  replacementTaxTotal: number;
  replacementTotal: number;
  /** replacementTotal - total (bertanda): >0 pelanggan bayar, <0 pelanggan dikreditkan. */
  differenceAmount: number;
  /** Bagian nilai retur yang otomatis mengurangi piutang. */
  receivableReduction: number;
  returnMode: string;
  replacementDeliveryStatus: string;
  items: SalesReturnItem[];
  replacementItems: SalesReturnReplacementItem[];
  settlements: ReturnSettlement[];
  settledTotal: number;
  unsettledTotal: number;
  /** Nilai yang masih boleh direfund / dijadikan saldo pelanggan (K-04c). */
  externalRemaining: number;
  /** Selisih tukar (lebih mahal) yang belum ditagih. */
  chargeRemaining: number;
  /** Jenis penyelesaian manual yang masih sah ([] bila tak ada). */
  allowedSettlementTypes: string[];
  /** Jejak pembatalan (alasan wajib sejak koreksi salah input); kosong bila tidak/belum dibatalkan. */
  cancelReason?: string;
  /** Waktu batal, "YYYY-MM-DD HH:mm:ss" WIB ("" bila tidak dibatalkan). */
  cancelledAt?: string;
  cancelledBy?: number;
}

/** Satu baris body POST — kunci persis struct Create di handler.go. */
export interface SalesReturnLinePayload {
  productId: number;
  variantId: number;
  /** Normal: wajib hanya untuk produk berstok; damaged/not_returned: dikosongkan. */
  locationId?: number;
  uom: string;
  qty: number;
  condition: ReturnCondition;
}

/** Baris pengganti: harga opsional (kosong = harga katalog). */
export interface SalesReturnReplacementPayload {
  productId: number;
  variantId: number;
  locationId?: number;
  uom: string;
  qty: number;
  unitPrice?: number;
}

export interface SalesReturnPayload {
  salesOrderId: number;
  returnDate?: string;
  notes?: string;
  returnMode?: string;
  /** Wajib non-kosong. */
  reason: string;
  items: SalesReturnLinePayload[];
  replacementItems?: SalesReturnReplacementPayload[];
}

/** Subset orderView SO untuk modal buat (sales/presentation/handler.go). */
export interface SourceSalesOrderItem {
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  uom: string;
  uomFactor: number;
  qty: number;
  qtyBase: number;
  unitPrice: number;
}

export interface SourceSalesOrder {
  id: number;
  number: string;
  partyName: string;
  status: string;
  items: SourceSalesOrderItem[];
}

/** Opsi lokasi dari GET /api/v1/stock/locations (array langsung). */
export interface LocationOption {
  id: number;
  code: string;
  name: string;
  status: string;
}

/** Satu baris konteks retur: sisa = terkirim − sudah diretur. */
export interface SalesReturnContextLine {
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  uom: string;
  uomFactor: number;
  orderedQty: number;
  delivered: number;
  returned: number;
  remaining: number;
}

export interface SalesReturnContextLocation {
  id: number;
  code: string;
  name: string;
}

/** GET /api/v1/sales-returns/context?salesOrderId= (perm salesreturn.view). */
export interface SalesReturnContext {
  salesOrderId: number;
  orderNumber: string;
  status: string;
  lines: SalesReturnContextLine[];
  locations: SalesReturnContextLocation[];
}

/** Satu baris hasil pratinjau (harga snapshot server, tanpa id). */
export interface SalesReturnPreviewItem {
  productId: number;
  variantId: number;
  locationId: number;
  uom: string;
  uomFactor: number;
  qty: number;
  qtyBase: number;
  unitPrice: number;
  discountPct: number;
  discountNominal: number;
  taxBase: number;
  taxAmount: number;
  lineTotal: number;
}

export interface SalesReturnPreviewReplacement {
  productId: number;
  variantId: number;
  locationId: number;
  uom: string;
  uomFactor: number;
  qty: number;
  qtyBase: number;
  unitPrice: number;
  lineTotal: number;
}

/** POST /api/v1/sales-returns/preview (dry-run, tanpa tulis). */
export interface SalesReturnPreview {
  mode: string;
  subtotal: number;
  discountTotal: number;
  taxTotal: number;
  total: number;
  items: SalesReturnPreviewItem[];
  replacementItems: SalesReturnPreviewReplacement[];
  replacementTotal: number;
  /** replacementTotal − total untuk tukar; 0 untuk retur murni. */
  difference: number;
  /** Nilai terutang ke pelanggan (retur murni / tukar lebih murah). */
  owedToCustomer: number;
  /** Selisih yang harus dibayar pelanggan (tukar lebih mahal). */
  chargeToCustomer: number;
  /** Sisa piutang order sebelum retur ini. */
  orderOutstanding: number;
  /** Bagian retur yang otomatis mengurangi piutang saat dikonfirmasi. */
  receivableReduction: number;
  /** Sisa yang bisa direfund / dijadikan saldo pelanggan. */
  externalRemaining: number;
  allowedSettlementTypes: string[];
  hasParty: boolean;
}

export interface SalesReturnPreviewPayload {
  salesOrderId: number;
  returnMode?: string;
  items: SalesReturnLinePayload[];
  replacementItems?: SalesReturnReplacementPayload[];
}

/** Body POST /:id/replacement-deliveries — semua opsional. */
export interface ReplacementDispatchPayload {
  deliveryDate?: string;
  notes?: string;
  driverName?: string;
  vehiclePlate?: string;
  warehouseStaffName?: string;
  dropLocationNote?: string;
}

/** Hasil dispatch/confirm pengganti: {id, number, status}. */
export interface ReplacementDeliveryResult {
  id: number;
  number: string;
  status: string;
}

/** Body POST .../{deliveryId}/confirm — semua opsional. */
export interface ReplacementConfirmPayload {
  recipientName?: string;
  recipientSignatureStatus?: "signed" | "missing";
  recipientSignatureMissingReason?: string;
  /** Tagih selisih tukar saat serah (K-04e); atomik dengan konfirmasi. */
  settlement?: ReplacementPayNow;
}

/** Pembayaran selisih tukar yang lebih mahal, dicatat saat SJ pengganti dikonfirmasi. */
export interface ReplacementPayNow {
  mode: "pay_now";
  /** Kosong = seluruh selisih yang masih terbuka. */
  amount?: number;
  method?: "cash" | "transfer" | "check";
  paymentDate?: string;
  reference?: string;
  amountTendered?: number;
}

/** Body POST /:id/settlements. */
export interface ReturnSettlementPayload {
  type: string;
  date?: string;
  amount: number;
  paymentMethod?: string;
  referenceNumber?: string;
  notes?: string;
}

/** Opsi produk untuk editor barang pengganti (search-options: id/code/name). */
export interface ReplacementProductOption {
  id: number;
  code: string;
  name: string;
  sellingPrice: number;
}

/** Subset productView untuk varian/satuan editor pengganti. */
export interface ReplacementProductDetail {
  id: number;
  code: string;
  name: string;
  baseUom: string;
  purchaseUom: string;
  salesUom: string;
  variants: { id: number; code: string; name: string }[];
}

/** Subset noteView untuk resolve ID surat jalan pengganti. */
export interface ReplacementDeliveryNote {
  id: number;
  number: string;
  salesOrderId: number;
  status: string;
  documentKind: string;
  salesReturnId: number;
}

export const SALES_RETURN_STATUSES = ["draft", "confirmed", "cancelled"] as const;

export type SalesReturnStatus = (typeof SALES_RETURN_STATUSES)[number];

export const RETURN_CONDITIONS = ["normal", "damaged", "not_returned"] as const;

export type ReturnCondition = (typeof RETURN_CONDITIONS)[number];

/** Label Indonesia untuk kondisi barang retur. */
export function returnConditionLabel(c: string | null | undefined): string {
  switch (c) {
    case "normal":
      return "Normal (masuk stok)";
    case "damaged":
      return "Rusak (ke lokasi rusak)";
    case "not_returned":
      return "Tidak dikembalikan (tanpa stok)";
    default:
      return c ?? "-";
  }
}

export type ReturnDifferenceKind = "refund" | "charge" | "none";

/**
 * Arah selisih retur tukar dari tanda differenceAmount (replacementTotal -
 * total; service.go projection): <0 pelanggan dikreditkan (refund),
 * >0 pelanggan membayar lebih (charge). View tidak membawa Kind.
 */
export function returnDifferenceKind(difference: number): ReturnDifferenceKind {
  if (difference < 0) return "refund";
  if (difference > 0) return "charge";
  return "none";
}

export function returnDifferenceLabel(difference: number): string {
  const kind = returnDifferenceKind(difference);
  if (kind === "refund") return "Pelanggan dikreditkan (refund)";
  if (kind === "charge") return "Pelanggan membayar selisih (tagihan)";
  return "Impas";
}

export const SALES_RETURN_MODES = ["return_only", "exchange"] as const;

export type SalesReturnMode = (typeof SALES_RETURN_MODES)[number];

export const RETURN_SETTLEMENT_TYPES = [
  "collect_payment",
  "reduce_receivable",
  "refund",
  "customer_credit",
] as const;

export type ReturnSettlementType = (typeof RETURN_SETTLEMENT_TYPES)[number];

/** Label Indonesia untuk mode retur. */
export function returnModeLabel(mode: string | null | undefined): string {
  if (mode === "exchange") return "Tukar barang";
  if (mode === "return_only") return "Retur saja";
  return mode ?? "-";
}

/** Label Indonesia untuk status kirim barang pengganti. */
export function replacementStatusLabel(status: string | null | undefined): string {
  switch (status) {
    case "not_required":
      return "Tidak perlu";
    case "pending":
      return "Menunggu kirim";
    case "dispatched":
      return "Dikirim";
    case "confirmed":
      return "Diterima / diserahkan";
    default:
      return status ?? "-";
  }
}

/** Label Indonesia untuk jenis penyelesaian. */
export function settlementTypeLabel(type: string | null | undefined): string {
  switch (type) {
    case "collect_payment":
      return "Tagih pembayaran";
    case "reduce_receivable":
      return "Kurangi piutang";
    case "refund":
      return "Refund";
    case "customer_credit":
      return "Saldo pelanggan (uang muka)";
    default:
      return type ?? "-";
  }
}

/**
 * Baris-baris "apa yang akan terjadi pada nilai retur ini" untuk langkah Tinjau
 * (K-04d). Angka datang dari server (preview memakai aturan & helper offset
 * yang sama dengan konfirmasi) — fungsi ini hanya memformat.
 */
export function salesSettlementOutlook(
  p: Pick<
    SalesReturnPreview,
    | "chargeToCustomer"
    | "owedToCustomer"
    | "orderOutstanding"
    | "receivableReduction"
    | "externalRemaining"
    | "allowedSettlementTypes"
  >,
  formatMoney: (n: number) => string,
): string[] {
  if (p.chargeToCustomer > 0) {
    return [
      `Pelanggan membayar selisih ${formatMoney(p.chargeToCustomer)} — tagih saat barang pengganti diserahkan (atau catat sebagai tagih pembayaran)`,
    ];
  }
  if (p.owedToCustomer <= 0) return ["Impas: tidak ada selisih yang perlu diselesaikan"];
  const out: string[] = [
    `Nilai terutang ke pelanggan: ${formatMoney(p.owedToCustomer)}`,
    `Sisa piutang order sebelum retur: ${formatMoney(p.orderOutstanding)}`,
  ];
  if (p.receivableReduction > 0) {
    out.push(`Mengurangi piutang otomatis saat dikonfirmasi: ${formatMoney(p.receivableReduction)}`);
  }
  if (p.externalRemaining > 0) {
    const labels = p.allowedSettlementTypes.map((t) => settlementTypeLabel(t).toLowerCase()).join(" atau ");
    out.push(`Sisa ${formatMoney(p.externalRemaining)} dapat diselesaikan sebagai ${labels}`);
  } else {
    out.push("Seluruh nilai retur mengurangi piutang: tidak ada yang perlu direfund atau dijadikan saldo pelanggan");
  }
  return out;
}
