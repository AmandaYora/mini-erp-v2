import { z } from "zod";
import {
  csvOf,
  dateFilter,
  enumOrEmpty,
  idFilter,
  pageFilter,
  searchFilter,
} from "@/shared/lib/list-schema";
import type { SalesFilterParams } from "@/modules/sales/services/sales.service";
import { SALES_STATUSES } from "@/modules/sales/types";

export const SALES_SORT_KEYS = ["id", "date", "number", "total", "status"] as const;
export const PAYMENT_TERMS = ["cod", "net", "prepaid"] as const;
export const PAYMENT_STATUSES = ["unpaid", "partial", "paid", "overdue"] as const;

// Skema filter daftar Order Jual. State-nya hidup di URL (useUrlFilters);
// tipe filter diturunkan lewat z.infer, bukan ditulis manual.
export const salesListFilterSchema = z.object({
  search: searchFilter,
  status: csvOf(SALES_STATUSES),
  channel: enumOrEmpty(["regular", "pos"] as const),
  paymentTerms: enumOrEmpty(PAYMENT_TERMS),
  paymentStatus: enumOrEmpty(PAYMENT_STATUSES),
  partyId: idFilter,
  from: dateFilter,
  to: dateFilter,
  sort: z.enum(SALES_SORT_KEYS),
  dir: z.enum(["asc", "desc"]),
  page: pageFilter,
});

export type SalesListFilter = z.infer<typeof salesListFilterSchema>;

export const SALES_LIST_DEFAULTS: SalesListFilter = {
  search: "",
  status: "",
  channel: "",
  paymentTerms: "",
  paymentStatus: "",
  partyId: 0,
  from: "",
  to: "",
  sort: "id",
  dir: "desc",
  page: 1,
};

/** Filter UI -> param service (urutan bawaan id desc tidak dikirim). */
export function toSalesFilterParams(f: SalesListFilter): SalesFilterParams {
  const isDefaultSort = f.sort === "id" && f.dir === "desc";
  return {
    search: f.search,
    status: f.status,
    channel: f.channel,
    paymentTerms: f.paymentTerms,
    paymentStatus: f.paymentStatus,
    partyId: f.partyId,
    from: f.from,
    to: f.to,
    sort: isDefaultSort ? "" : f.sort,
    dir: isDefaultSort ? "" : f.dir,
  };
}
