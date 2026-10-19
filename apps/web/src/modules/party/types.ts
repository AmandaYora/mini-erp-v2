// Cerminan 1:1 dari view backend (party/presentation/handler.go):
// partyView, addressView, memberView. Semua kunci JSON camelCase.

export interface PartyMemberRef {
  id: number;
  code: string;
}

export interface Party {
  id: number;
  code: string;
  type: string;
  name: string;
  phone: string | null;
  email: string | null;
  address: string | null;
  notes: string | null;
  member: PartyMemberRef | null;
  status: string;
  /** Waktu diarsipkan (null bila aktif/dipulihkan). */
  archivedAt?: string | null;
  /** "Informasi Tambahan" bebas (maks 20 kunci). Selalu objek dari server. */
  attributes: Record<string, string>;
}

export type Customer = Party;
export type Supplier = Party;

/** GET /customers/:id dan GET /suppliers/:id menanamkan addresses. */
export interface PartyDetail extends Party {
  addresses: Address[];
}

export type CustomerDetail = PartyDetail;
export type SupplierDetail = PartyDetail;

export interface Address {
  id: number;
  label: string | null;
  recipient: string | null;
  phone: string | null;
  text: string;
  isPrimary: boolean;
  sortOrder: number;
  status: string;
}

/** GET /member-types/{id}/customer-count. */
export interface MemberTypeCustomerCount {
  memberTypeId: number;
  activeCustomers: number;
}

export interface MemberType {
  id: number;
  code: string;
  name: string;
  description: string | null;
  basis: string;
  direction: string;
  type: string;
  value: number;
  /** none | nearest | up | down (nama lama sudah dilipat backend). */
  roundingMode: string;
  roundingStep: number;
  status: string;
}

// --- payload keluar (kunci persis partyPayload/addressPayload/memberPayload) ---

export interface PartyCreatePayload {
  code?: string;
  name: string;
  phone?: string;
  email?: string;
  address?: string;
  notes?: string;
  memberTypeId?: number | null;
  /** Tidak dikirim = kosong; {} = kosong. */
  attributes?: Record<string, string>;
}

export interface PartyUpdatePayload {
  name: string;
  phone?: string;
  email?: string;
  address?: string;
  notes?: string;
  memberTypeId?: number | null;
  clearMember?: boolean;
  /** Tidak dikirim = tak berubah; {} = dikosongkan; selain itu menggantikan seluruh blok. */
  attributes?: Record<string, string>;
}

export interface AddressPayload {
  label?: string;
  recipient?: string;
  phone?: string;
  text: string;
  isPrimary?: boolean;
  sortOrder?: number;
}

export interface MemberTypeCreatePayload {
  code: string;
  name: string;
  description?: string;
  basis: string;
  direction: string;
  type: string;
  value: number;
  mode: string;
  step: number;
  confirmed?: boolean;
}

export type MemberTypeUpdatePayload = Omit<MemberTypeCreatePayload, "code">;

/** Ringkasan order di tab Transaksi (GET /sales-orders | /purchase-orders?partyId=). */
export interface PartyOrderRow {
  id: number;
  number: string;
  orderDate: string;
  grandTotal: number;
  status: string;
}

export interface PartyOrderBalance {
  orderType: string;
  orderId: number;
  number: string;
  grandTotal: number;
  paid: number;
  outstanding: number;
}

/** GET /payments/party-balances?partyId= — saldo tagihan/pembayaran satu pihak. */
export interface PartyBalanceView {
  partyId: number;
  partyName: string;
  totalBilled: number;
  totalPaid: number;
  totalReturned: number;
  totalRefunded: number;
  outstanding: number;
  orders: PartyOrderBalance[];
}
