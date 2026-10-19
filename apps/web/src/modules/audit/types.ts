// Bentuk JSON backend: audit/presentation recordView
// (id, action, entity, entityId, branchId, actorId, actorType, actorName?,
// branchName?, note, createdAt, before, after, metadata). before/after/metadata
// adalah JSON mentah dari audit_log_details (null bila entri tanpa rincian).

export type AuditActorType = "user" | "system";

export interface AuditRecord {
  id: number;
  action: string;
  entity: string;
  entityId: number;
  /** 0 = peristiwa global (tanpa cabang). */
  branchId: number;
  actorId: number;
  actorType: AuditActorType | string;
  /** Terisi bila pengguna masih ditemukan (best-effort di server). */
  actorName?: string;
  branchName?: string;
  note: string;
  createdAt: string;
  before?: unknown;
  after?: unknown;
  metadata?: unknown;
}
