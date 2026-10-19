import { z } from "zod";
import { dateFilter, idFilter, pageFilter, searchFilter } from "@/shared/lib/list-schema";

// Filter jejak audit (GET /audit-logs). State hidup di URL.
// branchId: "" = cabang aktif + peristiwa global (bawaan server),
// "all" = semua cabang, angka = satu cabang.
export const auditListFilterSchema = z.object({
  branchId: z.string().regex(/^(all|\d*)$/),
  action: searchFilter,
  entity: searchFilter,
  entityId: idFilter,
  actorId: idFilter,
  from: dateFilter,
  to: dateFilter,
  page: pageFilter,
});

export type AuditListFilter = z.infer<typeof auditListFilterSchema>;

export const AUDIT_LIST_DEFAULTS: AuditListFilter = {
  branchId: "",
  action: "",
  entity: "",
  entityId: 0,
  actorId: 0,
  from: "",
  to: "",
  page: 1,
};
