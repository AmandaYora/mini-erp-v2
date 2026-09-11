// Service pembayaran — HANYA via helper http-client (apiGet/apiPage/
// apiPost/apiUpload). Bentuk respons terverifikasi dari backend:
// - GET /api/v1/payments?partyId&status&page&limit → paginasi paymentView[]
//   (handler.go: List — TIDAK ada param search/method; filter itu di klien).
// - GET /api/v1/payments/:id → paymentView (handler.go: Get).
// - POST /api/v1/payments {partyId, amount, method?, paidAt?, notes?,
//   allocations?[{orderType, orderId, amount}]} → paymentView.
// - POST /api/v1/payments/:id/cancel (tanpa body) → paymentView.
// - POST /api/v1/payments/settle-credit {partyId, returnType, returnId,
//   orderType, orderId, amount, notes?} → paymentView (kaki bill).
// - GET /api/v1/payments/party-balances?partyId= → PartyBalance langsung.
// - GET /api/v1/payments/party-ledger?partyId=&page&limit → paginasi paymentView[].
// - GET /api/v1/payments/:id/proofs → [{id, originalName, mime, sizeBytes, url}].
// - POST /api/v1/payments/:id/proof multipart field "file" → {url}.

import {
  apiGet,
  apiPage,
  apiPostFull,
  apiUploadFull,
} from "@/shared/services/http-client";
import type {
  PartyBalance,
  PartyLookup,
  Payment,
  PaymentCreatePayload,
  PaymentProof,
  SettleCreditPayload,
} from "@/modules/payment/types";

export interface PaymentListParams {
  partyId?: number;
  status?: string;
  page: number;
  limit: number;
}

function listParams(p: PaymentListParams): Record<string, unknown> {
  return {
    partyId: p.partyId && p.partyId > 0 ? p.partyId : undefined,
    status: p.status ? p.status : undefined,
    page: p.page,
    limit: p.limit,
  };
}

export const paymentService = {
  list: (p: PaymentListParams) =>
    apiPage<Payment>("/api/v1/payments", listParams(p)),

  get: (id: number) => apiGet<Payment>(`/api/v1/payments/${id}`),

  create: (body: PaymentCreatePayload) =>
    apiPostFull<Payment>("/api/v1/payments", body),

  // Tanpa body — backend hanya butuh :id (handler.go: Cancel).
  cancel: (id: number) =>
    apiPostFull<Payment>(`/api/v1/payments/${id}/cancel`),

  settleCredit: (body: SettleCreditPayload) =>
    apiPostFull<Payment>("/api/v1/payments/settle-credit", body),

  balance: (partyId: number) =>
    apiGet<PartyBalance>("/api/v1/payments/party-balances", { partyId }),

  ledger: (partyId: number, page: number, limit: number) =>
    apiPage<Payment>("/api/v1/payments/party-ledger", {
      partyId,
      page,
      limit,
    }),

  proofs: (id: number) =>
    apiGet<PaymentProof[]>(`/api/v1/payments/${id}/proofs`),

  uploadProof: (id: number, file: File) => {
    const form = new FormData();
    form.append("file", file);
    return apiUploadFull<{ url: string }>(`/api/v1/payments/${id}/proof`, form);
  },
};

export const paymentLookupService = {
  /** Customer + supplier aktif untuk SearchSelect (maks 50 per sisi). */
  partyOptions: async (): Promise<PartyLookup[]> => {
    const [customers, suppliers] = await Promise.all([
      apiPage<PartyLookup>("/api/v1/customers", {
        status: "active",
        page: 1,
        limit: 50,
      }),
      apiPage<PartyLookup>("/api/v1/suppliers", {
        status: "active",
        page: 1,
        limit: 50,
      }),
    ]);
    return [...customers.items, ...suppliers.items];
  },
};
