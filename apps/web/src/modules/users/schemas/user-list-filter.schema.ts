import { z } from "zod";
import { enumOrEmpty, pageFilter, searchFilter } from "@/shared/lib/list-schema";

// Filter daftar pengguna (GET /users). State hidup di URL (`use-url-filters`) sehingga
// tautan seperti /users?search=superadmin (alert kesehatan data A1) langsung terfilter.
export const userListFilterSchema = z.object({
  search: searchFilter,
  status: enumOrEmpty(["active", "inactive"] as const),
  page: pageFilter,
});

export type UserListFilter = z.infer<typeof userListFilterSchema>;

export const USER_LIST_DEFAULTS: UserListFilter = { search: "", status: "", page: 1 };
