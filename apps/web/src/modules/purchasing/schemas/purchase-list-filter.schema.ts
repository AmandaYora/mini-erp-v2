import { z } from "zod";
import {
  csvOf,
  dateFilter,
  enumOrEmpty,
  idFilter,
  pageFilter,
  searchFilter,
} from "@/shared/lib/list-schema";
import type { PurchaseOrderFilterParams } from "@/modules/purchasing/services/purchase-order.service";

export const PURCHASE_STATUSES = ["draft", "confirmed", "completed", "cancelled"] as const;
export const PURCHASE_SORT_KEYS = ["id", "date", "number", "total", "status"] as const;

export const purchaseListFilterSchema = z.object({
  search: searchFilter,
  status: csvOf(PURCHASE_STATUSES),
  paymentTerms: enumOrEmpty(["cod", "net", "prepaid"] as const),
  partyId: idFilter,
  from: dateFilter,
  to: dateFilter,
  sort: z.enum(PURCHASE_SORT_KEYS),
  dir: z.enum(["asc", "desc"]),
  page: pageFilter,
});

export type PurchaseListFilter = z.infer<typeof purchaseListFilterSchema>;

export const PURCHASE_LIST_DEFAULTS: PurchaseListFilter = {
  search: "",
  status: "",
  paymentTerms: "",
  partyId: 0,
  from: "",
  to: "",
  sort: "id",
  dir: "desc",
  page: 1,
};

export function toPurchaseFilterParams(f: PurchaseListFilter): PurchaseOrderFilterParams {
  const isDefaultSort = f.sort === "id" && f.dir === "desc";
  return {
    search: f.search,
    status: f.status,
    paymentTerms: f.paymentTerms,
    partyId: f.partyId,
    from: f.from,
    to: f.to,
    sort: isDefaultSort ? "" : f.sort,
    dir: isDefaultSort ? "" : f.dir,
  };
}
