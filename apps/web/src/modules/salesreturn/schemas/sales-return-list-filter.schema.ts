import { z } from "zod";
import {
  csvOf,
  dateFilter,
  idFilter,
  pageFilter,
  searchFilter,
} from "@/shared/lib/list-schema";
import { SALES_RETURN_STATUSES } from "@/modules/salesreturn/types";
import type { SalesReturnFilterParams } from "@/modules/salesreturn/services/sales-return.service";

// Filter daftar retur (salesreturn). State hidup di URL (useUrlFilters).
export const salesreturnListFilterSchema = z.object({
  search: searchFilter,
  status: csvOf(SALES_RETURN_STATUSES),
  salesOrderId: idFilter,
  from: dateFilter,
  to: dateFilter,
  page: pageFilter,
});

export type SalesReturnListFilter = z.infer<typeof salesreturnListFilterSchema>;

export const SALESRETURN_LIST_DEFAULTS: SalesReturnListFilter = {
  search: "",
  status: "",
  salesOrderId: 0,
  from: "",
  to: "",
  page: 1,
};

export function toSalesReturnFilterParams(f: SalesReturnListFilter): SalesReturnFilterParams {
  return {
    search: f.search,
    status: f.status,
    salesOrderId: f.salesOrderId,
    from: f.from,
    to: f.to,
  };
}
