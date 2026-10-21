// Service pembayaran — HANYA via helper http-client (apiGet/apiPage/
// apiPost/apiUpload). Bentuk respons terverifikasi dari backend:
// - GET /api/v1/payments?search&method&direction&status&partyId&orderType&
//   orderId&from&to&page&limit → paginasi paymentView[] (handler.go: List;
//   semua filter di server, partyName terisi).
// - GET /api/v1/payments/:id → paymentView (handler.go: Get).
// - POST /api/v1/payments {partyId, amount, method?, paidAt?, notes?,
//   allocations?[{orderType, orderId, amount}]} → paymentView.
//   F2a: create juga menerima clientRef (kunci idempoten), reference (<=100)
//   dan amountTendered (hanya method cash, >= amount); view membawa clientRef,
//   reference, amountTendered, change, pairId. Method: cash|transfer|check|
//   offset — offset khusus sistem (tidak bisa dipilih user).
// - POST /api/v1/payments/:id/cancel {reason} → paymentView (cancelReason,
//   cancelledAt, cancelledBy). Web selalu mengirim alasan. Server menolak
//   (409) bila pembatalan membuat order yang sudah selesai jadi belum lunas
//   ("Order X (terms) sudah selesai; membatalkan pembayaran ini membuatnya
//   belum lunas…") → UI mengarahkan ke Koreksi pembayaran / Retur.
// - POST /api/v1/payments/:id/correct {method, amount, paidAt?, reference?,
//   amountTendered?, notes?, reason} → paymentView pembayaran PENGGANTI (201;
//   200 bila permintaan yang sama diulang). Yang asli dibatalkan dengan alasan
//   itu; detail membawa correctsPaymentId/Number dan correctedById/Number.
// - POST /api/v1/payments/settle-credit {partyId, returnType, returnId,
//   orderType, orderId, amount, notes?} → paymentView (kaki bill).
// - GET /api/v1/payments/party-balances?partyId= → PartyBalance langsung.
// - GET /api/v1/payments/open-parties?side=receivable|payable&search&page&limit
//   → {items:[{partyId,code,name,side,orderCount,oldestDate,outstanding}],
//   total,totalOutstanding,page,limit,totalPages} (PY-01, handler.go: OpenParties).
// - GET /api/v1/payments/party-ledger?partyId=&page&limit → paginasi paymentView[].
// - GET /api/v1/payments/:id/proofs → [{id, originalName, mime, sizeBytes, url}].
// - POST /api/v1/payments/:id/proof multipart field "file" → {url}.

import {
  apiGet,
  apiPage,
  apiPostFull,
  apiUploadFull,
} from "@/shared/services/http-client";
import { compressImageForUpload } from "@/shared/lib/image-compression";
import { compactParams, positiveId } from "@/shared/lib/list-query";
import type {
  OpenPartiesPage,
  OpenPartySide,
  PartyBalance,
  Payment,
  PaymentCorrectPayload,
  PaymentCreatePayload,
  PaymentProof,
  SettleCreditPayload,
  UseCreditPayload,
  CreditBalances,
} from "@/modules/payment/types";

export interface PaymentFilterParams {
  /** Nomor, catatan, referensi, atau nama/kontak pihak. */
  search?: string;
  method?: string;
  /** "in" | "out". */
  direction?: string;
  /** "active" | "cancelled". */
  status?: string;
  partyId?: number;
  /** "sales" | "purchase" | "sales_return" | "purchase_return" — wajib bersama orderId. */
  orderType?: string;
  orderId?: number;
  /** YYYY-MM-DD (WIB), inklusif, atas tanggal bayar. */
  from?: string;
  to?: string;
}

export interface PaymentListParams extends PaymentFilterParams {
  page: number;
  limit: number;
}

export function buildPaymentListQuery(p: PaymentListParams): Record<string, unknown> {
  return compactParams({
    search: p.search,
    method: p.method,
    direction: p.direction,
    status: p.status,
    partyId: positiveId(p.partyId),
    orderType: p.orderType,
    orderId: positiveId(p.orderId),
    from: p.from,
    to: p.to,
    page: p.page,
    limit: p.limit,
  });
}

export const paymentService = {
  list: (p: PaymentListParams, signal?: AbortSignal) =>
    apiPage<Payment>("/api/v1/payments", buildPaymentListQuery(p), signal),

  get: (id: number) => apiGet<Payment>(`/api/v1/payments/${id}`),

  create: (body: PaymentCreatePayload) =>
    apiPostFull<Payment>("/api/v1/payments", body),

  /** Alasan disimpan pada pembayaran (dan kaki pasangannya) serta di jejak audit. */
  cancel: (id: number, reason: string) =>
    apiPostFull<Payment>(`/api/v1/payments/${id}/cancel`, { reason }),

  /** Koreksi salah input: membuat pembayaran pengganti dan membatalkan yang asli. */
  correct: (id: number, body: PaymentCorrectPayload) =>
    apiPostFull<Payment>(`/api/v1/payments/${id}/correct`, body),

  settleCredit: (body: SettleCreditPayload) =>
    apiPostFull<Payment>("/api/v1/payments/settle-credit", body),

  /** Pakai saldo uang muka sebuah retur ke tagihan terbuka pihak yang sama (N-41). */
  useCredit: (body: UseCreditPayload) =>
    apiPostFull<Payment>("/api/v1/payments/use-credit", body),

  /** Retur milik pihak yang masih menyimpan saldo, beserta yang terpakai/sisa. */
  creditBalances: (partyId: number) =>
    apiGet<CreditBalances>("/api/v1/payments/credit-balances", { partyId }),

  balance: (partyId: number) =>
    apiGet<PartyBalance>("/api/v1/payments/party-balances", { partyId }),

  /** Daftar semua pihak berpiutang/berhutang (PY-01), terlama dulu. */
  openParties: (p: { side: OpenPartySide; search?: string; page: number; limit: number }) =>
    apiGet<OpenPartiesPage>("/api/v1/payments/open-parties", {
      side: p.side,
      search: p.search?.trim() ? p.search.trim() : undefined,
      page: p.page,
      limit: p.limit,
    }),

  ledger: (partyId: number, page: number, limit: number) =>
    apiPage<Payment>("/api/v1/payments/party-ledger", {
      partyId,
      page,
      limit,
    }),

  proofs: (id: number) =>
    apiGet<PaymentProof[]>(`/api/v1/payments/${id}/proofs`),

  // OP-03: gambar dikompres di klien dulu (gagal → berkas asli).
  uploadProof: async (id: number, file: File) => {
    const form = new FormData();
    form.append("file", await compressImageForUpload(file));
    return apiUploadFull<{ url: string }>(`/api/v1/payments/${id}/proof`, form);
  },
};
