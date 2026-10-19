import { z } from "zod";
import {
  csvOf,
  dateFilter,
  idFilter,
  pageFilter,
  searchFilter,
} from "@/shared/lib/list-schema";
import { PURCHASE_RETURN_STATUSES } from "@/modules/purchasereturn/types";
import type { PurchaseReturnFilterParams } from "@/modules/purchasereturn/services/purchase-return.service";

// Filter daftar retur (purchasereturn). State hidup di URL (useUrlFilters).
export const purchasereturnListFilterSchema = z.object({
  search: searchFilter,
  status: csvOf(PURCHASE_RETURN_STATUSES),
  purchaseOrderId: idFilter,
  from: dateFilter,
  to: dateFilter,
  page: pageFilter,
});

export type PurchaseReturnListFilter = z.infer<typeof purchasereturnListFilterSchema>;

export const PURCHASERETURN_LIST_DEFAULTS: PurchaseReturnListFilter = {
  search: "",
  status: "",
  purchaseOrderId: 0,
  from: "",
  to: "",
  page: 1,
};

export function toPurchaseReturnFilterParams(f: PurchaseReturnListFilter): PurchaseReturnFilterParams {
  return {
    search: f.search,
    status: f.status,
    purchaseOrderId: f.purchaseOrderId,
    from: f.from,
    to: f.to,
  };
}
