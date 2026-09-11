// Service POS — HANYA via helper http-client (apiGet/apiPage/apiPost).
// Mandiri di dalam modul pos (tanpa impor service modul lain). Rute +
// permission backend yang dipakai:
// - GET /api/v1/products/search-options?q= (products.view)
// - GET /api/v1/products?search=&categoryId=&status=&page=&limit= (products.view — katalog per kategori)
// - GET /api/v1/product-categories (products.view — drilldown kategori)
// - GET /api/v1/products/:id (products.view — untuk varian/satuan/harga)
// - POST /api/v1/stock/scan-product {barcode} (stock.view)
// - GET /api/v1/customers?search=&status=&page=&limit= (customers.view)
// - GET /api/v1/customers/:id (customers.view — tipe member panel pelanggan)
// - GET /api/v1/member-types (member_types.manage|view — label tipe member)
// - POST /api/v1/pricing/quote {customerId, productIds} (pricing.quote — harga member)
// - POST /api/v1/sales-orders (sales.create)
// - POST /api/v1/sales-orders/:id/confirm (sales.update)
// - POST /api/v1/deliveries (delivery.create — stok keluar di POS)
// - POST /api/v1/deliveries/:id/confirm (delivery.create)
// - GET /api/v1/stock/locations (stock.view — lokasi asal barang POS)
// - POST /api/v1/payments (payment.create)

import { apiGet, apiPage, apiPost } from "@/shared/services/http-client";
import type {
  PosCategory,
  PosCreatedDelivery,
  PosCreatedOrder,
  PosCreatedPayment,
  PosCustomer,
  PosCustomerDetail,
  PosDeliveryPayload,
  PosLocation,
  PosMemberType,
  PosOrderPayload,
  PosPaymentPayload,
  PosProductDetail,
  PosProductListItem,
  PosProductOption,
  PricingQuote,
  ScanResult,
} from "@/modules/pos/types";

export const posService = {
  /** Katalog per kategori untuk drilldown (status aktif, 20/baris halaman). */
  searchProducts: (q?: string) =>
    apiGet<PosProductOption[]>("/api/v1/products/search-options", {
      q: q?.trim() ? q.trim() : undefined,
    }),

  /** Daftar kategori aktif untuk drilldown katalog. */
  categories: () => apiGet<PosCategory[]>("/api/v1/product-categories"),

  /** Produk per kategori + pencarian teks (satu halaman katalog). */
  products: (params: { search?: string; categoryId?: number | null; page?: number; limit?: number }) =>
    apiPage<PosProductListItem>("/api/v1/products", {
      search: params.search?.trim() ? params.search.trim() : undefined,
      categoryId: params.categoryId ?? undefined,
      status: "active",
      page: params.page ?? 1,
      limit: params.limit ?? 20,
    }),

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

  /** Detail customer terpilih — membawa tipe member untuk panel pelanggan. */
  customerDetail: (id: number) =>
    apiGet<PosCustomerDetail>(`/api/v1/customers/${id}`),

  /** Daftar tipe member — untuk label kode member di panel pelanggan. */
  memberTypes: () => apiGet<PosMemberType[]>("/api/v1/member-types"),

  /** Quote harga member untuk isi keranjang (0 = walk-in, harga katalog).
   * Gagal (mis. tanpa izin pricing.quote) = tampilkan harga katalog. */
  pricingQuote: (customerId: number, productIds: number[]) =>
    apiPost<PricingQuote>("/api/v1/pricing/quote", { customerId, productIds }),

  /** Lokasi asal barang POS (default: GDG bila ada). */
  locations: () => apiGet<PosLocation[]>("/api/v1/stock/locations"),

  /** Langkah 1 checkout: buat order channel pos (server yang memberi harga). */
  createOrder: (body: PosOrderPayload) =>
    apiPost<PosCreatedOrder>("/api/v1/sales-orders", body),

  /** Langkah 2 checkout: konfirmasi order (tanpa body). */
  confirmOrder: (id: number) =>
    apiPost<PosCreatedOrder>(`/api/v1/sales-orders/${id}/confirm`),

  /** Langkah 3 checkout: surat jalan dari SO terkonfirmasi (draf). */
  createDelivery: (body: PosDeliveryPayload) =>
    apiPost<PosCreatedDelivery>("/api/v1/deliveries", body),

  /** Langkah 4 checkout: konfirmasi SJ — stok keluar di sini. */
  confirmDelivery: (id: number) =>
    apiPost<PosCreatedDelivery>(`/api/v1/deliveries/${id}/confirm`),

  /** Langkah 5 checkout: catat pembayaran lunas 1 order. amount WAJIB
   * grandTotal dari respons langkah 1 (server reprices), bukan estimasi klien. */
  createPayment: (body: PosPaymentPayload) =>
    apiPost<PosCreatedPayment>("/api/v1/payments", body),
};
