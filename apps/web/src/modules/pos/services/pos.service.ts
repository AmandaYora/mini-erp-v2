// Service POS — HANYA via helper http-client (apiGet/apiPage/apiPost).
// Mandiri di dalam modul pos (tanpa impor service modul lain). Rute +
// permission backend yang dipakai:
// - GET /api/v1/products?search=&categoryId=&status=&page=&limit= (products.view — katalog
//   "Semua" & per kategori, dimuat per halaman saat di-scroll)
// - GET /api/v1/product-categories (products.view — drilldown kategori)
// - GET /api/v1/products/:id (products.view — untuk varian/satuan/harga)
// - POST /api/v1/stock/scan-product {barcode} (stock.view)
// - GET /api/v1/customers?search=&status=&page=&limit= (customers.view)
// - POST /api/v1/customers (customers.create — pelanggan baru dari kasir, PR-02)
// - GET /api/v1/customers/:id (customers.view — tipe member panel pelanggan)
// - GET /api/v1/member-types (member_types.manage|view — label tipe member)
// - POST /api/v1/pricing/quote {customerId, productIds} (pricing.quote — harga member)
// - POST /api/v1/sales-orders (sales.create)
// - POST /api/v1/sales-orders/:id/confirm (sales.update)
// - POST /api/v1/deliveries (delivery.create — stok keluar di POS)
// - POST /api/v1/deliveries/:id/confirm (delivery.create)
// - GET /api/v1/sales-orders/by-client-ref/:ref/state (sales.view — resume checkout)
// - POST /api/v1/stock/allocations/suggest (stock.view — saran lokasi + shortfall)
// - POST /api/v1/stock/reservations/hold|release (stock.reserve — tahanan keranjang)
// - POST /api/v1/payments (payment.create)
// - GET /api/v1/payments?orderType=sales&orderId= (payment.view — pembayaran order, resume/kembalian)

import { apiGet, apiPage, apiPost } from "@/shared/services/http-client";
import { readApiError } from "@/modules/pos/lib/api-error";
import type {
  PosCategory,
  PosCreatedDelivery,
  PosCreatedOrder,
  PosCreatedPayment,
  PosCustomer,
  PosCustomerDetail,
  PosDeliveryPayload,
  PosMemberType,
  PosOrderPayload,
  PosOrderState,
  PosSettlement,
  PosPaymentPayload,
  PosProductDetail,
  PosProductListItem,
  PricingQuote,
  ScanResult,
  StockSuggestion,
} from "@/modules/pos/types";

export const posService = {
  /** Daftar kategori aktif untuk drilldown katalog. */
  categories: () => apiGet<PosCategory[]>("/api/v1/product-categories"),

  /** Satu halaman katalog aktif: semua (categoryId null) atau per kategori + pencarian teks. */
  products: (
    params: { search?: string; categoryId?: number | null; page?: number; limit?: number },
    signal?: AbortSignal,
  ) =>
    apiPage<PosProductListItem>(
      "/api/v1/products",
      {
        search: params.search?.trim() ? params.search.trim() : undefined,
        categoryId: params.categoryId ?? undefined,
        status: "active",
        page: params.page ?? 1,
        limit: params.limit ?? 20,
      },
      signal,
    ),

  /** Detail produk untuk varian default, satuan jual, dan harga katalog. */
  productDetail: (id: number) =>
    apiGet<PosProductDetail>(`/api/v1/products/${id}`),

  /** Pindai barcode → {product, variant, balances}. 404 = barcode tak dikenal. */
  scanBarcode: (barcode: string) =>
    apiPost<ScanResult>("/api/v1/stock/scan-product", { barcode }),

  /** Customer aktif untuk SearchSelect (walk-in = kosongkan pilihan). */
  searchCustomers: (search?: string) =>
    apiPage<PosCustomer>("/api/v1/customers", {
      search: search?.trim() ? search.trim() : undefined,
      status: "active",
      page: 1,
      limit: 20,
    }),

  /** Pelanggan baru dari kasir: nama, telepon, tipe member (opsional). */
  createCustomer: (body: { name: string; phone?: string; memberTypeId?: number }) =>
    apiPost<PosCustomer>("/api/v1/customers", body),

  /** Detail customer terpilih — membawa tipe member untuk panel pelanggan. */
  customerDetail: (id: number) =>
    apiGet<PosCustomerDetail>(`/api/v1/customers/${id}`),

  /** Daftar tipe member — untuk label kode member di panel pelanggan. */
  memberTypes: () => apiGet<PosMemberType[]>("/api/v1/member-types"),

  /** Quote harga member untuk isi keranjang (0 = walk-in, harga katalog).
   * Gagal (mis. tanpa izin pricing.quote) = tampilkan harga katalog. */
  pricingQuote: (customerId: number, productIds: number[]) =>
    apiPost<PricingQuote>("/api/v1/pricing/quote", { customerId, productIds }),

  /** State checkout keranjang di server. null = order belum pernah dibuat
   * (404) → mulai dari langkah 1. Error lain dilempar ulang. */
  orderState: async (ref: string): Promise<PosOrderState | null> => {
    try {
      return await apiGet<PosOrderState>(
        `/api/v1/sales-orders/by-client-ref/${encodeURIComponent(ref)}/state`,
      );
    } catch (err) {
      if (readApiError(err).status === 404) return null;
      throw err;
    }
  },

  /** Saran alokasi lintas lokasi + shortfall (tanpa efek samping). */
  suggestAllocation: (body: { productId: number; variantId: number; qty: number }) =>
    apiPost<StockSuggestion>("/api/v1/stock/allocations/suggest", body),

  /** Tahan stok (upsert menurut key — ubah qty aman). */
  holdStock: (body: {
    productId: number;
    variantId: number;
    locationId: number;
    qty: number;
    key: string;
    ttlMinutes: number;
  }) => apiPost<{ id: number; key: string }>("/api/v1/stock/reservations/hold", body),

  /** Lepas tahanan menurut key. */
  releaseStock: (key: string) =>
    apiPost<null>("/api/v1/stock/reservations/release", { key }),

  /** Langkah 1 checkout: buat order channel pos (server yang memberi harga). */
  createOrder: (body: PosOrderPayload) =>
    apiPost<PosCreatedOrder>("/api/v1/sales-orders", body),

  /** Langkah 2 checkout: konfirmasi order (tanpa body). */
  confirmOrder: (id: number) =>
    apiPost<PosCreatedOrder>(`/api/v1/sales-orders/${id}/confirm`),

  /** Langkah 3 checkout: surat jalan dari SO terkonfirmasi (draf). */
  createDelivery: (body: PosDeliveryPayload) =>
    apiPost<PosCreatedDelivery>("/api/v1/deliveries", body),

  /** Langkah 4 checkout: konfirmasi SJ — stok keluar di sini. Bayar sekarang
   * membawa settlement agar serah + pembayaran selesai dalam satu panggilan. */
  confirmDelivery: (id: number, settlement?: PosSettlement) =>
    apiPost<PosCreatedDelivery>(
      `/api/v1/deliveries/${id}/confirm`,
      settlement ? { settlement } : undefined,
    ),

  /** Hanya untuk resume di langkah `pay` (SJ sudah terkonfirmasi tanpa
   * settlement): catat pembayaran lunas 1 order sebesar sisa tagihan server. */
  createPayment: (body: PosPaymentPayload) =>
    apiPost<PosCreatedPayment>("/api/v1/payments", body),
};

/** Pembayaran tersimpan untuk satu order POS (kembalian dari data server).
 * Disaring server lewat alokasi (orderType=sales&orderId) — bukan 20 pembayaran
 * terbaru cabang, yang melewatkan order lama di cabang ramai.
 * Best-effort: butuh izin payment.view; kegagalan = null. */
export async function findPosPayment(
  partyId: number,
  orderId: number,
): Promise<PosCreatedPayment | null> {
  const page = await apiPage<PosCreatedPayment & { status?: string }>("/api/v1/payments", {
    orderType: "sales",
    orderId,
    partyId: partyId > 0 ? partyId : undefined,
    status: "active",
    page: 1,
    limit: 20,
  });
  return (
    page.items.find((p) => p.allocations?.some((a) => a.orderId === orderId)) ?? null
  );
}
