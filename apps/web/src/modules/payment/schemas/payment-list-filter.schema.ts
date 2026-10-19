import { z } from "zod";
import {
  dateFilter,
  enumOrEmpty,
  idFilter,
  pageFilter,
  searchFilter,
} from "@/shared/lib/list-schema";
import type { PaymentFilterParams } from "@/modules/payment/services/payment.service";

// Filter daftar Pembayaran (GET /payments). State hidup di URL.
export const paymentListFilterSchema = z.object({
  search: searchFilter,
  method: enumOrEmpty(["cash", "transfer", "check", "offset"] as const),
  direction: enumOrEmpty(["in", "out"] as const),
  status: enumOrEmpty(["active", "cancelled"] as const),
  partyId: idFilter,
  orderType: enumOrEmpty(["sales", "purchase"] as const),
  orderId: idFilter,
  from: dateFilter,
  to: dateFilter,
  page: pageFilter,
});

export type PaymentListFilter = z.infer<typeof paymentListFilterSchema>;

export const PAYMENT_LIST_DEFAULTS: PaymentListFilter = {
  search: "",
  method: "",
  direction: "",
  status: "",
  partyId: 0,
  orderType: "",
  orderId: 0,
  from: "",
  to: "",
  page: 1,
};

/** orderType hanya berarti bersama orderId (server menolak orderId tanpa jenis). */
export function toPaymentFilterParams(f: PaymentListFilter): PaymentFilterParams {
  return {
    search: f.search,
    method: f.method,
    direction: f.direction,
    status: f.status,
    partyId: f.partyId,
    orderType: f.orderId > 0 ? f.orderType : "",
    orderId: f.orderType !== "" ? f.orderId : 0,
    from: f.from,
    to: f.to,
  };
}
