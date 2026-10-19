// Cerminan 1:1 dari view backend (purchasereturn/presentation/handler.go:
// returnView + Create). Semua kunci JSON camelCase.
//
// Hasil verifikasi (JANGAN ditebak ulang):
// - View: {id, number, branchId, purchaseOrderId, orderNumber, returnDate,
//   subtotal, discountTotal, taxTotal, taxType, taxRate, total, status, notes,
//   items[]:{id, productId, variantId, locationId, uom, uomFactor, qty,
//   qtyBase, unitPrice, discountPct, discountNominal, taxBase, taxAmount,
//   lineTotal}} (rincian D1 sejak Tahap B).
// - Create body: {purchaseOrderId!, returnDate?, notes?, items[]:
//   {productId!, variantId!, locationId!, uom!, qty!}} — TIDAK ada
//   goodsReceiptId (sumber selalu PO, bukan BPB).
// - Sumber valid: PO confirmed/completed (application/service.go
//   Create) — kandidat: GET /purchase-orders?status=confirmed(+completed).
// - Batas qty server: diterima−diretur per (product, variant); TIDAK ada
//   key sisa di view mana pun → clamp klien = qty PO, server final.
// - Confirm/Cancel: POST /:id/confirm & POST /:id/cancel TANPA body.
// - Status: draft → confirmed / cancelled.
// - Uang integer rupiah, qty float, tanggal YYYY-MM-DD.
//
// Hasil verifikasi Tahap D (JANGAN ditebak ulang):
// - View membawa settlements[] {id, type, date, amount, paymentMethod,
//   referenceNumber, notes} + settledTotal + unsettledTotal (handler.go:
//   returnView; settled = jumlah amount di server). TIDAK ada field exchange.
// - Preview: POST /preview {purchaseOrderId!, items[]!} → {subtotal,
//   discountTotal, taxTotal, total, items[]} (dry-run, tanpa mode/pengganti).
// - Context: GET /context?purchaseOrderId= → {purchaseOrderId, orderNumber,
//   status, lines[{productId, variantId, productCode, productName, uom,
//   uomFactor, orderedQty, received, returned, remaining}], locations[{
//   id, code, name}]} (service.go: ReturnContext; remaining =
//   diterima−diretur).
// - Settlement: POST /:id/settlements {type!, date?, amount!, paymentMethod?,
//   referenceNumber?, notes?} → full returnView. Syarat: retur confirmed.
//   K-04b: kamus PEMBELIAN — type ∈ refund | supplier_credit (manual);
//   reduce_payable = offset otomatis saat konfirmasi, tidak bisa dicatat
//   manual. Nominal <= externalRemaining (nilai retur − pengurang utang −
//   yang sudah diselesaikan); server 409 bila lebih / tak ada kelebihan.
// - K-04c/d: view membawa externalRemaining + allowedSettlementTypes (jenis yang
//   masih sah); preview membawa orderOutstanding, payableReduction,
//   externalRemaining, allowedSettlementTypes (angka sama dengan hasil konfirmasi).

export interface PurchaseReturnItem {
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

/** Satu baris memo penyelesaian (uang riil tetap lewat modul payment). */
export interface PurchaseReturnSettlement {
  id: number;
  type: string;
  date: string;
  amount: number;
  paymentMethod: string;
  referenceNumber: string;
  notes: string;
}

export interface PurchaseReturn {
  id: number;
  number: string;
  branchId: number;
  purchaseOrderId: number;
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
  /** Alasan retur (wajib saat buat, <= 255). */
  reason: string;
  /** Bagian nilai retur yang otomatis mengurangi utang (offset). */
  payableReduction: number;
  /** Kelebihan yang masih bisa direfund / dijadikan saldo supplier (0 bila tak ada). */
  externalRemaining: number;
  /** Jenis penyelesaian manual yang masih sah ([] bila tak ada kelebihan). */
  allowedSettlementTypes: string[];
  items: PurchaseReturnItem[];
  settlements: PurchaseReturnSettlement[];
  settledTotal: number;
  unsettledTotal: number;
}

/** Satu baris body POST — kunci persis struct Create di handler.go. */
export interface PurchaseReturnLinePayload {
  productId: number;
  variantId: number;
  locationId: number;
  uom: string;
  qty: number;
}

export interface PurchaseReturnPayload {
  purchaseOrderId: number;
  returnDate?: string;
  notes?: string;
  /** Wajib non-kosong (server menolak alasan kosong). */
  reason: string;
  items: PurchaseReturnLinePayload[];
}

/** Subset orderView PO untuk modal buat (purchasing/presentation/handler.go). */
export interface SourcePurchaseOrderItem {
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

export interface SourcePurchaseOrder {
  id: number;
  number: string;
  partyName: string;
  status: string;
  items: SourcePurchaseOrderItem[];
}

/** Opsi lokasi dari GET /api/v1/stock/locations (array langsung). */
export interface LocationOption {
  id: number;
  code: string;
  name: string;
  status: string;
}

/** Satu baris konteks retur: sisa = diterima − sudah diretur. */
export interface PurchaseReturnContextLine {
  productId: number;
  variantId: number;
  productCode: string;
  productName: string;
  uom: string;
  uomFactor: number;
  orderedQty: number;
  received: number;
  returned: number;
  remaining: number;
  /** Lokasi penerimaan terbaru barang ini — lokasi asal bawaan retur (RT-04). */
  defaultLocationId?: number;
}

export interface PurchaseReturnContextLocation {
  id: number;
  code: string;
  name: string;
}

/** GET /api/v1/purchase-returns/context?purchaseOrderId= (perm purchasereturn.view). */
export interface PurchaseReturnContext {
  purchaseOrderId: number;
  orderNumber: string;
  status: string;
  /** Tanggal penerimaan terposting paling awal ("YYYY-MM-DD HH:MM:SS"), kosong bila belum ada. */
  firstReceivedAt?: string;
  lines: PurchaseReturnContextLine[];
  locations: PurchaseReturnContextLocation[];
}

/** Satu baris hasil pratinjau (harga snapshot server, tanpa id). */
export interface PurchaseReturnPreviewItem {
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

/** POST /api/v1/purchase-returns/preview (dry-run, tanpa tulis). */
export interface PurchaseReturnPreview {
  subtotal: number;
  discountTotal: number;
  taxTotal: number;
  total: number;
  items: PurchaseReturnPreviewItem[];
  /** Sisa utang order sebelum retur ini. */
  orderOutstanding: number;
  /** Bagian retur yang otomatis mengurangi utang saat dikonfirmasi. */
  payableReduction: number;
  /** Kelebihan di atas sisa utang: bisa direfund / dijadikan saldo supplier. */
  externalRemaining: number;
  allowedSettlementTypes: string[];
}

export interface PurchaseReturnPreviewPayload {
  purchaseOrderId: number;
  items: PurchaseReturnLinePayload[];
}

/** Body POST /:id/settlements. */
export interface PurchaseReturnSettlementPayload {
  type: string;
  date?: string;
  amount: number;
  paymentMethod?: string;
  referenceNumber?: string;
  notes?: string;
}

export const PURCHASE_RETURN_STATUSES = ["draft", "confirmed", "cancelled"] as const;

export type PurchaseReturnStatus = (typeof PURCHASE_RETURN_STATUSES)[number];

/** Kamus penyelesaian retur PEMBELIAN (K-04b); bukan kamus penjualan. */
export const PURCHASE_RETURN_SETTLEMENT_TYPES = [
  "reduce_payable",
  "refund",
  "supplier_credit",
] as const;

export type PurchaseReturnSettlementType =
  (typeof PURCHASE_RETURN_SETTLEMENT_TYPES)[number];

/** Label Indonesia untuk jenis penyelesaian (sisi supplier). */
export function purchaseSettlementTypeLabel(type: string | null | undefined): string {
  switch (type) {
    case "reduce_payable":
      return "Kurangi hutang";
    case "refund":
      return "Refund dari supplier";
    case "supplier_credit":
      return "Kredit supplier";
    default:
      return type ?? "-";
  }
}

/**
 * Baris-baris "apa yang akan terjadi pada nilai retur ini" untuk langkah Tinjau
 * (K-04d). Angka datang dari server (preview memakai aturan & helper offset
 * yang sama dengan konfirmasi) — fungsi ini hanya memformat.
 */
export function purchaseSettlementOutlook(
  p: Pick<PurchaseReturnPreview, "orderOutstanding" | "payableReduction" | "externalRemaining" | "allowedSettlementTypes">,
  formatMoney: (n: number) => string,
): string[] {
  const out: string[] = [`Sisa hutang order sebelum retur: ${formatMoney(p.orderOutstanding)}`];
  if (p.payableReduction > 0) {
    out.push(`Mengurangi hutang otomatis saat dikonfirmasi: ${formatMoney(p.payableReduction)}`);
  }
  if (p.externalRemaining > 0) {
    const labels = p.allowedSettlementTypes.map((t) => purchaseSettlementTypeLabel(t).toLowerCase()).join(" atau ");
    out.push(`Kelebihan ${formatMoney(p.externalRemaining)} dapat diselesaikan sebagai ${labels}`);
  } else {
    out.push("Seluruh nilai retur mengurangi hutang: tidak ada yang perlu direfund atau dijadikan saldo supplier");
  }
  return out;
}
