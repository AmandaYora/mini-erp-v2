// Bentuk JSON backend: audit/presentation recordView
// (id, action, entity, entityId, branchId, actorId, note, createdAt).

export interface AuditRecord {
  id: number;
  action: string;
  entity: string;
  entityId: number;
  branchId: number;
  actorId: number;
  note: string;
  createdAt: string;
}
