import {
  apiGet,
  apiPage,
  apiPostFull,
  apiPutFull,
} from "@/shared/services/http-client";
import type {
  Address,
  AddressPayload,
  CustomerDetail,
  MemberType,
  MemberTypeCreatePayload,
  MemberTypeCustomerCount,
  MemberTypeUpdatePayload,
  Party,
  PartyBalanceView,
  PartyCreatePayload,
  PartyOrderRow,
  PartyUpdatePayload,
  SupplierDetail,
} from "@/modules/party/types";

export interface PartyListParams {
  search?: string;
  status?: string;
  /** "1" = nama dipakai lebih dari satu pihak aktif (alert A10). */
  duplicateName?: string;
  page: number;
  limit: number;
}

function listParams(p: PartyListParams): Record<string, unknown> {
  const search = p.search?.trim() ? p.search.trim() : undefined;
  // "all" = Semua status (tanpa filter); kosong juga tanpa filter.
  const status = p.status && p.status !== "all" ? p.status : undefined;
  return { search, status, duplicateName: p.duplicateName || undefined, page: p.page, limit: p.limit };
}

function searchParams(search?: string, status?: string): Record<string, unknown> {
  return {
    search: search?.trim() ? search.trim() : undefined,
    status: status && status !== "all" ? status : undefined,
  };
}

// --- customers ---

export const customerService = {
  list: (p: PartyListParams, signal?: AbortSignal) =>
    apiPage<Party>("/api/v1/customers", listParams(p), signal),
  get: (id: number) => apiGet<CustomerDetail>(`/api/v1/customers/${id}`),
  create: (body: PartyCreatePayload) => apiPostFull<Party>("/api/v1/customers", body),
  update: (id: number, body: PartyUpdatePayload) => apiPutFull<Party>(`/api/v1/customers/${id}`, body),
  archive: (id: number) => apiPostFull<unknown>(`/api/v1/customers/${id}/archive`),
  restore: (id: number) => apiPostFull<unknown>(`/api/v1/customers/${id}/restore`),

  // Buku alamat hanya ada untuk customer (routes.go: tidak ada endpoint
  // address di bawah /suppliers).
  listAddresses: (id: number) => apiGet<Address[]>(`/api/v1/customers/${id}/addresses`),
  createAddress: (id: number, body: AddressPayload) =>
    apiPostFull<Address>(`/api/v1/customers/${id}/addresses`, body),
  updateAddress: (id: number, addrId: number, body: AddressPayload) =>
    apiPutFull<Address>(`/api/v1/customers/${id}/addresses/${addrId}`, body),
  archiveAddress: (id: number, addrId: number) =>
    apiPostFull<unknown>(`/api/v1/customers/${id}/addresses/${addrId}/archive`),
};

// --- suppliers (info saja, tanpa alamat) ---

export const supplierService = {
  list: (p: PartyListParams, signal?: AbortSignal) =>
    apiPage<Party>("/api/v1/suppliers", listParams(p), signal),
  get: (id: number) => apiGet<SupplierDetail>(`/api/v1/suppliers/${id}`),
  create: (body: PartyCreatePayload) => apiPostFull<Party>("/api/v1/suppliers", body),
  update: (id: number, body: PartyUpdatePayload) => apiPutFull<Party>(`/api/v1/suppliers/${id}`, body),
  archive: (id: number) => apiPostFull<unknown>(`/api/v1/suppliers/${id}/archive`),
  restore: (id: number) => apiPostFull<unknown>(`/api/v1/suppliers/${id}/restore`),
};

// --- tipe member (list tidak paginated: data = array langsung) ---

export const memberTypeService = {
  list: (search?: string, status?: string) =>
    apiGet<MemberType[]>("/api/v1/member-types", searchParams(search, status)),
  create: (body: MemberTypeCreatePayload) => apiPostFull<MemberType>("/api/v1/member-types", body),
  update: (id: number, body: MemberTypeUpdatePayload) =>
    apiPutFull<MemberType>(`/api/v1/member-types/${id}`, body),
  /** Dampak arsip (D3): jumlah pelanggan AKTIF pada tipe ini; butuh member_types.view. */
  customerCount: (id: number) =>
    apiGet<MemberTypeCustomerCount>(`/api/v1/member-types/${id}/customer-count`),
  archive: (id: number) => apiPostFull<unknown>(`/api/v1/member-types/${id}/archive`),
  restore: (id: number) => apiPostFull<unknown>(`/api/v1/member-types/${id}/restore`),
};

// --- ringkasan lintas-modul untuk tab detail (hanya endpoint, tanpa impor modul lain) ---

export const partyActivityService = {
  /** Order terbaru pihak ini: penjualan untuk customer, pembelian untuk supplier. */
  recentOrders: (kind: "customer" | "supplier", partyId: number, limit = 10) =>
    apiPage<PartyOrderRow>(
      kind === "customer" ? "/api/v1/sales-orders" : "/api/v1/purchase-orders",
      { partyId, page: 1, limit },
    ),
  /** Saldo tagihan & pembayaran (perlu izin payment.view). */
  balance: (partyId: number) =>
    apiGet<PartyBalanceView>("/api/v1/payments/party-balances", { partyId }),
};
