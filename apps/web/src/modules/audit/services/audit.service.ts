import { apiPage } from "@/shared/services/http-client";
import { compactParams, positiveId } from "@/shared/lib/list-query";
import type { AuditRecord } from "@/modules/audit/types";

export interface AuditFilterParams {
  /** "" = cabang aktif + global; "all" = semua cabang; angka = satu cabang. */
  branchId?: string;
  /** Awalan aksi: "product." menemukan product.create/update. */
  action?: string;
  entity?: string;
  entityId?: number;
  actorId?: number;
  /** YYYY-MM-DD (hari WIB), inklusif. */
  from?: string;
  to?: string;
}

export interface AuditListParams extends AuditFilterParams {
  page?: number;
  limit?: number;
}

export function buildAuditListQuery(p: AuditListParams): Record<string, unknown> {
  return compactParams({
    branchId: p.branchId,
    action: p.action,
    entity: p.entity,
    entityId: positiveId(p.entityId),
    actorId: positiveId(p.actorId),
    from: p.from,
    to: p.to,
    page: p.page ?? 1,
    limit: p.limit ?? 20,
  });
}

export const auditService = {
  // GET /api/v1/audit-logs?branchId&action&entity&entityId&actorId&from&to&
  // page&limit — paginasi, terbaru dulu (diurutkan backend); tiap baris
  // membawa actorType/actorName/branchName dan before/after/metadata.
  list: (params: AuditListParams, signal?: AbortSignal) =>
    apiPage<AuditRecord>("/api/v1/audit-logs", buildAuditListQuery(params), signal),
};
