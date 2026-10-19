// Label tampilan entri audit (fungsi murni; dipakai tabel dan panel rincian).
import { entityName } from "@/modules/audit/lib/audit-labels";
import type { AuditRecord } from "@/modules/audit/types";

export function isSystemActor(r: Pick<AuditRecord, "actorType" | "actorId">): boolean {
  return r.actorType === "system" || r.actorId === 0;
}

export function actorLabel(r: Pick<AuditRecord, "actorType" | "actorId" | "actorName">): string {
  if (isSystemActor(r)) return "Sistem";
  return r.actorName ? r.actorName : `Pengguna #${r.actorId}`;
}

export function branchLabel(r: Pick<AuditRecord, "branchId" | "branchName">): string {
  return r.branchId === 0 ? "Global" : (r.branchName ?? `#${r.branchId}`);
}

export function entityLabel(r: Pick<AuditRecord, "entity" | "entityId">): string {
  if (!r.entity) return "-";
  const name = entityName(r.entity);
  return r.entityId > 0 ? `${name} #${r.entityId}` : name;
}
