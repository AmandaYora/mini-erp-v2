// Tipe POS — cerminan kunci backend yang sudah diverifikasi (semua camelCase):
// - GET /api/v1/products?status=active&page=&limit= → PosProductListItem[] + meta
//   (product/presentation/handler.go: List).
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

export interface PosProductVariant {
  id: number;
  code: string;
  name: string;
  barcode: string;
  isDefault: boolean;
  status: string;
  /** Varian bawaan tersembunyi ("Standar") — tidak ditawarkan bila ada varian terlihat. */
  isHidden?: boolean;
  sortOrder?: number;
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
  /** Harga jual minimum per satuan dasar (0 = tanpa batas). */
  minSellingPrice: number;
  /** Isi satuan jual dalam satuan dasar (mis. 12 untuk DUS isi 12). */
  salesFactor?: number;
  tracked?: boolean;
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

/** Pilihan varian saat kode produk memuat lebih dari satu varian. */
export interface ScanVariantChoice {
  id: number;
  code: string;
  name: string;
  barcode: string;
  /** Stok tersedia varian ini di lokasi jual cabang (satuan jual); undefined = tidak diketahui. */
  available?: number;
}

/** Status hasil pindai: ok | variant_required | inactive (string agar status baru tak merusak tipe). */
export type ScanStatus = "ok" | "variant_required" | "inactive" | (string & {});

export interface ScanResult {
  status: ScanStatus;
  /** Jalur pencocokan: barcode | variant_code | product_code. */
  via?: string;
  product: { id: number; code: string; name: string };
  /** null bila status variant_required / inactive. */
  variant: { id: number; code: string; name: string } | null;
  /** Terisi hanya bila status variant_required. */
  variants?: ScanVariantChoice[];
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
  /** Foto utama (URL /media bertanda tangan, kedaluwarsa setelah TTL); null = tanpa foto. */
  primaryImageUrl?: string | null;
  /** false = tidak dilacak stok (jasa/non-stok): tile tanpa angka stok. */
  tracked?: boolean;
  /** Stok tersedia cabang aktif dalam satuan jual (hanya dengan izin stock.view dan produk dilacak). */
  stockAvailable?: number | null;
  stockUom?: string;
  /** Stok (satuan dasar) ≤ stok minimum produk (> 0). */
  belowMinStock?: boolean;
  /** Varian aktif (termasuk "Standar" tersembunyi) — untuk label "N varian". */
  variants?: PosProductVariant[];
}

/** Satu lokasi stok cabang (GET /api/v1/stock/locations). */
export interface PosLocation {
  id: number;
  code: string;
  name: string;
  kind: string;
  status: string;
}

/** Satu baris hasil POST /api/v1/pricing/quote. */
export interface PricingQuoteLine {
  productId: number;
  standardPrice: number;
  memberPrice: number;
  applied: boolean;
  memberInactive: boolean;
  /** Galat khusus baris ini (harga dasar kosong, konversi satuan, dst); baris lain tetap berharga. */
  error?: string;
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
  /** Harga minimum per satuan dasar dari detail produk (klem diskon). */
  minSellingPrice: number;
  /** Diskon persen baris (0–100). */
  discountPct: number;
  /** Diskon nominal baris (rupiah, per baris). */
  discountNominal: number;
}

export interface PosOrderLinePayload {
  productId: number;
  variantId: number;
  uom: string;
  qty: number;
  discountPct?: number;
  discountNominal?: number;
}

/** Body POST /sales-orders untuk channel POS. partyId 0 = walk-in.
 * paymentTerms "cod" = bayar sekarang; "net" + dueDate (YYYY-MM-DD) =
 * Bayar Nanti (sales/application/service.go: validateHeader). */
export interface PosOrderPayload {
  channel: "pos";
  partyId: number;
  paymentTerms: "cod" | "net";
  dueDate?: string;
  /** UUID per keranjang — kunci idempoten (replay = 200 replayed:true). */
  clientRef: string;
  /** PPN opsional (K-03): dihilangkan = tanpa pajak. Server menghitung ulang. */
  taxType?: "include" | "exclude";
  taxRate?: number;
  /** Catatan transaksi (SL-07); dihilangkan bila kosong. */
  notes?: string;
  /** Alamat kirim opt-in dari buku alamat pelanggan (SL-07); hanya untuk pelanggan terdaftar. */
  shipToAddressId?: number;
  /** Rekening tujuan (ADR-0026): rekening transfer kasir; dihilangkan = default perusahaan. */
  receiptAccountId?: number;
  items: PosOrderLinePayload[];
}

/** Subset orderView yang dipakai POS (id/number/grandTotal). */
export interface PosCreatedOrder {
  id: number;
  number: string;
  grandTotal: number;
  status?: string;
  paymentTerms?: string;
  replayed?: boolean;
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
  /** Idempoten: sama dengan clientRef keranjang. */
  clientRef?: string;
  amountTendered?: number;
  /** Rekening transfer (ADR-0026). */
  cashAccountId?: number;
}

export interface PosCreatedPayment {
  id: number;
  number: string;
  amount?: number;
  amountTendered?: number;
  change?: number;
  allocations?: { orderType: string; orderId: number; amount: number }[];
}

export interface PosDeliveryLinePayload {
  productId: number;
  variantId: number;
  /** Dikosongkan = server mengalokasikan otomatis lintas lokasi. */
  locationId?: number;
  uom: string;
  qty: number;
  /** Kunci tahanan stok (pos:<ref>:<product>:<variant>:<lokasi>) yang dikonsumsi. */
  reservationKey?: string;
}

/** Body POST /deliveries untuk mengeluarkan stok POS. */
export interface PosDeliveryPayload {
  salesOrderId: number;
  clientRef: string;
  items: PosDeliveryLinePayload[];
}

/** Subset noteView yang dipakai POS (id/number). */
export interface PosCreatedDelivery {
  id: number;
  number: string;
  status?: string;
  replayed?: boolean;
}

/** Settlement satu panggilan pada konfirmasi SJ (bayar sekarang). */
export interface PosSettlement {
  mode: "pay_now";
  method: PayMethod;
  /** Uang diterima — hanya tunai; server menghitung kembalian. */
  amountTendered?: number;
  /** Rekening transfer (ADR-0026). */
  cashAccountId?: number;
}

export type PosNextStep =
  | "confirm"
  | "create_delivery"
  | "confirm_delivery"
  | "pay"
  | "done"
  | "cancelled";

/** GET /sales-orders/by-client-ref/{ref}/state. */
export interface PosOrderState {
  order: PosCreatedOrder;
  deliveries: { id: number; number: string; status: string }[];
  paidAmount: number;
  outstanding: number;
  nextStep: PosNextStep;
}

/** POST /stock/allocations/suggest. */
export interface StockSuggestion {
  lines: { locationId: number; qty: number }[];
  shortfall: number;
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
