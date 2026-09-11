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
//   referenceNumber?, notes?} → full returnView. Syarat: retur confirmed;
//   total settlement TIDAK boleh melebihi total retur (server 409).
//   type ∈ collect_payment | reduce_receivable | refund | customer_credit
//   (contracts/client.go).

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
}

/** Satu baris barang pengganti — TANPA rincian diskon/pajak (harga katalog). */
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
  returnMode: string;
  replacementDeliveryStatus: string;
  items: SalesReturnItem[];
  replacementItems: SalesReturnReplacementItem[];
  settlements: ReturnSettlement[];
  settledTotal: number;
  unsettledTotal: number;
}

/** Satu baris body POST — kunci persis struct Create di handler.go. */
export interface SalesReturnLinePayload {
  productId: number;
  variantId: number;
  locationId: number;
  uom: string;
  qty: number;
}

export interface SalesReturnPayload {
  salesOrderId: number;
  returnDate?: string;
  notes?: string;
  returnMode?: string;
  items: SalesReturnLinePayload[];
  replacementItems?: SalesReturnLinePayload[];
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
}

export interface SalesReturnPreviewPayload {
  salesOrderId: number;
  returnMode?: string;
  items: SalesReturnLinePayload[];
  replacementItems?: SalesReturnLinePayload[];
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
      return "Diterima";
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
      return "Kredit pelanggan";
    default:
      return type ?? "-";
  }
}
