// Tipe POS — cerminan kunci backend yang sudah diverifikasi (semua camelCase):
// - GET /api/v1/products/search-options → {id, code, name, sellingPrice}
//   (product/presentation/handler.go: SearchOptions).
// - POST /api/v1/stock/scan-product {barcode} → {product:{id,code,name},
//   variant:{id,code,name}, balances:[...]} (stock/application/service.go: Scan).
// - POST /sales-orders {channel, partyId, items, paymentTerms} → orderView
//   {id, number, grandTotal, ...} (sales/presentation/handler.go: orderView).
// - POST /payments {partyId, method, amount, allocations:[{orderType, orderId,
//   amount}]} → paymentView {id, number, ...} (payment/presentation/handler.go).
// Uang integer rupiah. Harga final selalu dihitung ulang server — angka klien
// hanya estimasi (catalogPrice).

export type PayMethod = "cash" | "transfer";

/** Mode checkout: bayar sekarang (lunas) atau Bayar Nanti (piutang, termin net). */
export type CheckoutMode = "pay_now" | "pay_later";

export interface PosProductOption {
  id: number;
  code: string;
  name: string;
  sellingPrice: number;
}

export interface PosProductVariant {
  id: number;
  code: string;
  name: string;
  barcode: string;
  isDefault: boolean;
  status: string;
}

/** Detail produk minimal untuk menambah baris (GET /api/v1/products/:id). */
export interface PosProductDetail {
  id: number;
  code: string;
  name: string;
  baseUom: string;
  purchaseUom: string;
  salesUom: string;
  sellingPrice: number;
  status: string;
  variants: PosProductVariant[];
}

export interface ScanBalance {
  branchId: number;
  productId: number;
  variantId: number;
  locationId: number;
  onHand: number;
  reserved: number;
  available: number;
}

export interface ScanResult {
  product: { id: number; code: string; name: string };
  variant: { id: number; code: string; name: string };
  balances: ScanBalance[];
}

export interface PosCustomer {
  id: number;
  code: string;
  name: string;
  phone: string | null;
  member: { id: number; code: string } | null;
}

/** Detail customer untuk panel POS (GET /api/v1/customers/:id menanamkan member). */
export interface PosCustomerDetail extends PosCustomer {
  addresses: {
    id: number;
    label: string | null;
    recipient: string | null;
    phone: string | null;
    text: string;
    isPrimary: boolean;
    status: string;
  }[];
}

/** Kategori produk (GET /api/v1/product-categories → categoryView). */
export interface PosCategory {
  id: number;
  code: string;
  name: string;
  parentId: number | null;
  status: string;
}

/** Baris GET /api/v1/products (productView) — subset yang dipakai katalog POS. */
export interface PosProductListItem {
  id: number;
  code: string;
  name: string;
  categoryId: number | null;
  categoryName: string | null;
  sellingPrice: number;
  status: string;
}

/** Satu baris hasil POST /api/v1/pricing/quote. */
export interface PricingQuoteLine {
  productId: number;
  standardPrice: number;
  memberPrice: number;
  applied: boolean;
  memberInactive: boolean;
}

/** Hasil POST /api/v1/pricing/quote — harga member yang berlaku untuk keranjang. */
export interface PricingQuote {
  memberType: string;
  memberInactive: boolean;
  lines: PricingQuoteLine[];
}

/** Tipe member (GET /api/v1/member-types → memberView) — untuk label panel. */
export interface PosMemberType {
  id: number;
  code: string;
  name: string;
  status: string;
}

/** Satu baris keranjang. unitPrice = harga katalog untuk estimasi klien. */
export interface CartLine {
  key: number;
  productId: number;
  productCode: string;
  productName: string;
  variantId: number;
  variantCode: string;
  variantName: string;
  uom: string;
  qty: number;
  unitPrice: number;
}

export interface PosOrderLinePayload {
  productId: number;
  variantId: number;
  uom: string;
  qty: number;
}

/** Body POST /sales-orders untuk channel POS. partyId 0 = walk-in.
 * paymentTerms "cod" = bayar sekarang; "net" + dueDate (YYYY-MM-DD) =
 * Bayar Nanti (sales/application/service.go: validateHeader). */
export interface PosOrderPayload {
  channel: "pos";
  partyId: number;
  paymentTerms: "cod" | "net";
  dueDate?: string;
  items: PosOrderLinePayload[];
}

/** Subset orderView yang dipakai POS (id/number/grandTotal). */
export interface PosCreatedOrder {
  id: number;
  number: string;
  grandTotal: number;
}

export interface PosPaymentAllocation {
  orderType: "sales";
  orderId: number;
  amount: number;
}

export interface PosPaymentPayload {
  partyId: number;
  method: PayMethod;
  amount: number;
  allocations: PosPaymentAllocation[];
}

export interface PosCreatedPayment {
  id: number;
  number: string;
}

export interface PosLocation {
  id: number;
  code: string;
  name: string;
  branchId: number;
  status: string;
}

export interface PosDeliveryLinePayload {
  productId: number;
  variantId: number;
  locationId: number;
  uom: string;
  qty: number;
}

/** Body POST /deliveries untuk mengeluarkan stok POS. */
export interface PosDeliveryPayload {
  salesOrderId: number;
  items: PosDeliveryLinePayload[];
}

/** Subset noteView yang dipakai POS (id/number). */
export interface PosCreatedDelivery {
  id: number;
  number: string;
}

/** Ringkasan sukses yang ditampilkan setelah checkout. */
export interface CompletedSale {
  orderId: number;
  orderNumber: string;
  grandTotal: number;
  tendered: number;
  change: number;
  method: PayMethod;
  /** true = Bayar Nanti: order terkonfirmasi sebagai piutang, tanpa pembayaran. */
  payLater: boolean;
  /** Jatuh tempo piutang (YYYY-MM-DD) — hanya untuk Bayar Nanti. */
  dueDate: string | null;
  paymentId: number | null;
  /** true bila pembayaran belum tercatat (tanpa izin / gagal) — wajib
   * dicatat manual di /payments. */
  paymentPending: boolean;
}

export function payMethodLabel(method: PayMethod | string | null | undefined): string {
  if (method === "cash") return "Tunai";
  if (method === "transfer") return "Transfer";
  if (method === "pay_later") return "Bayar Nanti (Piutang)";
  return method ?? "-";
}
