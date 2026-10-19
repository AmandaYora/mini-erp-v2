import { z } from "zod";
import { dateFilter, enumOrEmpty, idFilter, pageFilter, searchFilter } from "@/shared/lib/list-schema";
import { JOURNAL_SOURCE_TYPES, JOURNAL_STATUSES } from "@/modules/finance/types";

// Filter daftar jurnal dan biaya. State hidup di URL; tanggal kosong = tanpa batas.

export const journalListFilterSchema = z.object({
  search: searchFilter,
  from: dateFilter,
  to: dateFilter,
  accountId: idFilter,
  sourceType: enumOrEmpty(JOURNAL_SOURCE_TYPES),
  status: enumOrEmpty(JOURNAL_STATUSES),
  page: pageFilter,
});

export type JournalListFilter = z.infer<typeof journalListFilterSchema>;

export const JOURNAL_LIST_DEFAULTS: JournalListFilter = {
  search: "",
  from: "",
  to: "",
  accountId: 0,
  sourceType: "",
  status: "",
  page: 1,
};

export const expenseListFilterSchema = z.object({
  search: searchFilter,
  from: dateFilter,
  to: dateFilter,
  status: enumOrEmpty(["posted", "cancelled"] as const),
  expenseAccountId: idFilter,
  payAccountId: idFilter,
  page: pageFilter,
});

export type ExpenseListFilter = z.infer<typeof expenseListFilterSchema>;

export const EXPENSE_LIST_DEFAULTS: ExpenseListFilter = {
  search: "",
  from: "",
  to: "",
  status: "",
  expenseAccountId: 0,
  payAccountId: 0,
  page: 1,
};
