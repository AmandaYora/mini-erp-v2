// Cerminan 1:1 dari view backend (sales/presentation/handler.go: orderView +
// orderPayload/linePayload). Semua kunci JSON camelCase.
// Uang integer rupiah, qty float (mendukung pecahan), tanggal YYYY-MM-DD.
// Harga dihitung server (member/katalog) — klien TIDAK mengirim unitPrice.
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
}

export interface SalesOrder {
  id: number;
  number: string;
  branchId: number;
  partyId: number;
  partyName: string;
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
  items: SalesOrderItem[];
}

/** Satu baris kiriman: tanpa harga (server yang memberi harga). */
export interface SalesOrderLinePayload {
  productId: number;
  variantId: number;
  uom: string;
  qty: number;
  discountPct?: number;
  discountNominal?: number;
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
  taxInvoiceNumber?: string;
  taxInvoiceDate?: string;
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
  return terms ?? "-";
}
