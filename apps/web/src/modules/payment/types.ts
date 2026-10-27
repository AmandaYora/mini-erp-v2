// Cerminan 1:1 dari view backend (payment/presentation/handler.go):
// paymentView, PartyBalance (contracts/client.go), dan Proofs (service.go).
// Semua kunci JSON camelCase.

export interface PaymentAllocation {
  orderType: string;
  orderId: number;
  orderNumber: string;
  amount: number;
}

export interface Payment {
  id: number;
  number: string;
  branchId: number;
  partyId: number;
  partyName: string;
  partyType: string;
  /** Kontak pihak — hanya di detail (GET /payments/{id}), untuk bukti pembayaran. */
  partyPhone?: string;
  partyAddress?: string;
  direction: string;
  amount: number;
  method: string;
  paidAt: string;
  notes: string;
  status: string;
  /** Kunci idempoten pembuatan (unik per cabang); kosong untuk pembayaran lama. */
  clientRef: string;
  /** Referensi transfer / nomor cek. */
  reference: string;
  /** Rekening perusahaan transfer/cek (ADR-0026): id akun buku besar; 0 = akun pemetaan "bank". */
  cashAccountId?: number;
  /** Uang tunai yang diserahkan (hanya cash); 0 = tidak dicatat. */
  amountTendered: number;
  /** Kembalian turunan (tendered - amount), tidak disimpan. */
  change: number;
  /** ID kaki lawan pada offset kredit (0 = tidak berpasangan). */
  pairId: number;
  /** Kaki pakai saldo uang muka: retur sumber saldo (kosong/0 untuk pembayaran lain). */
  creditSourceType?: string;
  creditSourceId?: number;
  /** Pembayaran ini pengganti hasil koreksi dari pembayaran lain (0 = bukan koreksi). */
  correctsPaymentId?: number;
  /** Nomor pembayaran yang dikoreksi — hanya terisi di detail. */
  correctsPaymentNumber?: string;
  /** Pembayaran pengganti yang mengoreksi pembayaran ini (0 = tidak dikoreksi) — hanya di detail. */
  correctedById?: number;
  correctedByNumber?: string;
  /** Jejak pembatalan: alasan, waktu (WIB) dan ID pengguna; kosong untuk pembatalan lama. */
  cancelReason?: string;
  cancelledAt?: string | null;
  cancelledBy?: number;
  allocations: PaymentAllocation[];
}

// --- payload keluar (kunci persis struct Create/SettleCredit di handler.go) ---

export interface AllocationInput {
  orderType: string;
  orderId: number;
  amount: number;
}

export interface PaymentCreatePayload {
  partyId: number;
  amount: number;
  method?: string;
  paidAt?: string;
  notes?: string;
  /** Kunci idempoten: pengiriman ulang dengan clientRef sama tidak menggandakan. */
  clientRef?: string;
  /** Uang diterima (hanya cash, harus >= amount). */
  amountTendered?: number;
  /** Referensi transfer / nomor cek (maks 100 karakter). */
  reference?: string;
  /** Rekening perusahaan transfer/cek (ADR-0026). Kosong = rekening default. */
  cashAccountId?: number;
  /** Kosong = backend alokasi FIFO otomatis (handler.go: Create). */
  allocations?: AllocationInput[];
}

/** Koreksi pembayaran (POST /payments/:id/correct): kunci persis handler Correct. */
export interface PaymentCorrectPayload {
  method: string;
  amount: number;
  /** YYYY-MM-DD; tanggal yang sama dengan aslinya mempertahankan jam aslinya. */
  paidAt?: string;
  reference?: string;
  /** Hanya cash, harus >= amount. */
  amountTendered?: number;
  /** Kosong = catatan pembayaran asli dipakai. */
  notes?: string;
  /** Wajib, 5–255 karakter; tercatat sebagai alasan batal pembayaran asli dan di audit. */
  reason: string;
  /** Rekening transfer/cek (ADR-0026); kosong = rekening pembayaran asli bila masih aktif, lalu default. */
  cashAccountId?: number;
}

/** Pakai saldo uang muka retur ke tagihan (N-41): kunci persis handler UseCredit. */
export type UseCreditPayload = SettleCreditPayload;

/** Satu retur yang menyimpan saldo pihak (GET credit-balances). */
export interface CreditBalance {
  returnType: "sales_return" | "purchase_return";
  returnId: number;
  number: string;
  credited: number;
  used: number;
  remaining: number;
}

export interface CreditBalances {
  items: CreditBalance[];
  totalRemaining: number;
}

export interface SettleCreditPayload {
  partyId: number;
  returnType: string;
  returnId: number;
  orderType: string;
  orderId: number;
  amount: number;
  notes?: string;
}

// --- saldo & ledger (GET party-balances / party-ledger) ---

export interface OrderBalance {
  orderType: string;
  orderId: number;
  number: string;
  grandTotal: number;
  paid: number;
  outstanding: number;
}

export interface PartyBalance {
  partyId: number;
  partyName: string;
  totalBilled: number;
  totalPaid: number;
  totalReturned: number;
  totalRefunded: number;
  outstanding: number;
  orders: OrderBalance[];
  returns: OrderBalance[];
}

/** Sisi daftar pihak bersaldo (PY-01): piutang pelanggan atau hutang supplier. */
export type OpenPartySide = "receivable" | "payable";

/** Satu pihak yang masih punya dokumen belum lunas (GET /payments/open-parties). */
export interface OpenParty {
  partyId: number;
  code: string;
  name: string;
  side: OpenPartySide;
  /** Jumlah dokumen (order) belum lunas. */
  orderCount: number;
  /** Jatuh tempo (atau tanggal order) tertua, YYYY-MM-DD. */
  oldestDate: string;
  outstanding: number;
}

export interface OpenPartiesPage {
  items: OpenParty[];
  /** Jumlah pihak seluruh hasil filter. */
  total: number;
  /** Total sisa seluruh hasil filter (bukan hanya halaman ini). */
  totalOutstanding: number;
  page: number;
  limit: number;
  totalPages: number;
}

export interface PaymentProof {
  id: number;
  originalName: string;
  mime: string;
  sizeBytes: number;
  url: string;
}

// --- label Indonesia ---

export function methodLabel(method: string | null | undefined): string {
  switch ((method ?? "").toLowerCase()) {
    case "cash":
      return "Tunai";
    case "transfer":
      return "Transfer";
    case "check":
      return "Cek";
    case "offset":
      return "Offset";
    case "advance":
      return "Saldo uang muka";
    default:
      return method ?? "-";
  }
}

export function directionLabel(direction: string | null | undefined): string {
  switch ((direction ?? "").toLowerCase()) {
    case "in":
      return "Masuk";
    case "out":
      return "Keluar";
    default:
      return direction ?? "-";
  }
}

export function orderTypeLabel(t: string | null | undefined): string {
  switch (t) {
    case "purchase":
      return "Order Beli";
    case "sales":
      return "Order Jual";
    case "sales_return":
      return "Retur Jual";
    case "purchase_return":
      return "Retur Beli";
    default:
      return t ?? "-";
  }
}

/** Metode yang boleh dipilih pengguna — "offset" hanya dibuat sistem (settle-credit). */
export const USER_PAYMENT_METHODS = ["cash", "transfer", "check"] as const;

export type UserPaymentMethod = (typeof USER_PAYMENT_METHODS)[number];

/** Label kolom referensi menurut metode. */
export function referenceLabel(method: string | null | undefined): string {
  return method === "check" ? "Nomor Cek" : "Referensi Transfer";
}

/** Tipe dokumen retur = pembayaran ini refund (uang kembali), bukan tagihan. */
export function isReturnOrderType(t: string | null | undefined): boolean {
  return t === "sales_return" || t === "purchase_return";
}

/**
 * Jenis pembayaran: refund bila mengalokasi ke dokumen retur, offset bila
 * metode offset (non-tunai, berpasangan), selain itu pembayaran tagihan.
 */
export function paymentKind(p: Pick<Payment, "method" | "allocations">): "offset" | "refund" | "bill" {
  if (p.method === "offset") return "offset";
  if (p.allocations.some((a) => isReturnOrderType(a.orderType))) return "refund";
  return "bill";
}

export function paymentKindLabel(p: Pick<Payment, "method" | "allocations">): string {
  switch (paymentKind(p)) {
    case "offset":
      return "Offset kredit";
    case "refund":
      return "Refund retur";
    default:
      return "Pembayaran tagihan";
  }
}

/**
 * Pembayaran yang boleh dikoreksi: aktif, metode yang dipilih pengguna (bukan kaki
 * offset/pakai saldo), tidak berpasangan, dan teralokasi ke dokumen (aturan backend Correct).
 */
export function isCorrectable(
  p: Pick<Payment, "status" | "method" | "pairId" | "creditSourceType" | "allocations">,
): boolean {
  return (
    p.status === "active" &&
    (USER_PAYMENT_METHODS as readonly string[]).includes(p.method) &&
    !p.pairId &&
    !p.creditSourceType &&
    p.allocations.length > 0
  );
}

/**
 * Pesan penjaga pembatalan dari backend (guardCancel): order COD/prepaid yang
 * sudah selesai tidak boleh jadi belum lunas. Tambahkan petunjuk ke Retur.
 */
export function cancelGuardHint(message: string | null | undefined): string | null {
  if (!message) return null;
  if (message.includes("sudah selesai") && message.includes("belum lunas")) {
    return "Order tunai/COD yang sudah selesai tidak bisa dibatalkan pembayarannya. Bila salah input, gunakan Koreksi pembayaran; untuk mengembalikan nilainya gunakan Retur (Retur Jual/Retur Beli).";
  }
  return null;
}
