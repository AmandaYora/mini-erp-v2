import { z } from "zod";
import {
  csvOf,
  dateFilter,
  enumOrEmpty,
  idFilter,
  pageFilter,
  searchFilter,
} from "@/shared/lib/list-schema";
import { DELIVERY_STATUSES } from "@/modules/delivery/types";
import type { DeliveryFilterParams } from "@/modules/delivery/services/delivery.service";

// Filter daftar Pengiriman (GET /deliveries). State hidup di URL.
// `unordered` disimpan sebagai "" | "1" (param URL selalu string) dan
// diterjemahkan ke boolean di toDeliveryFilterParams.
export const deliveryListFilterSchema = z.object({
  search: searchFilter,
  status: csvOf(DELIVERY_STATUSES),
  salesOrderId: idFilter,
  salesReturnId: idFilter,
  documentKind: enumOrEmpty(["order", "replacement"] as const),
  origin: enumOrEmpty(["order", "direct"] as const),
  unordered: enumOrEmpty(["1"] as const),
  signature: enumOrEmpty(["none", "signed", "missing"] as const),
  from: dateFilter,
  to: dateFilter,
  page: pageFilter,
});

export type DeliveryListFilter = z.infer<typeof deliveryListFilterSchema>;

export const DELIVERY_LIST_DEFAULTS: DeliveryListFilter = {
  search: "",
  status: "",
  salesOrderId: 0,
  salesReturnId: 0,
  documentKind: "",
  origin: "",
  unordered: "",
  signature: "",
  from: "",
  to: "",
  page: 1,
};

export function toDeliveryFilterParams(f: DeliveryListFilter): DeliveryFilterParams {
  return {
    search: f.search,
    status: f.status,
    salesOrderId: f.salesOrderId,
    salesReturnId: f.salesReturnId,
    documentKind: f.documentKind,
    origin: f.origin,
    unordered: f.unordered === "1",
    signature: f.signature,
    from: f.from,
    to: f.to,
  };
}
