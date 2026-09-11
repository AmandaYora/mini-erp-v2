import { apiPage } from "@/shared/services/http-client";
import type { AuditRecord } from "@/modules/audit/types";

export interface AuditListParams {
  action?: string;
  entity?: string;
  page?: number;
  limit?: number;
}

function cleanParams(p: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(p)) {
    if (v === undefined || v === null) continue;
    if (typeof v === "string" && v.trim() === "") continue;
    out[k] = v;
  }
  return out;
}

export const auditService = {
  // GET /api/v1/audit-logs?action=&entity=&page=&limit= — paginasi,
  // terbaru dulu (diurutkan backend).
  list: (params: AuditListParams) =>
    apiPage<AuditRecord>(
      "/api/v1/audit-logs",
      cleanParams({
        action: params.action,
        entity: params.entity,
        page: params.page ?? 1,
        limit: params.limit ?? 20,
      }),
    ),
};
